package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// slowSynth returns a distinct payload per chunk after a delay, so a later
// ticket can finish synthesizing before an earlier one.
type slowSynth struct {
	delay time.Duration
	fail  string
}

func (s slowSynth) Speak(text string) ([]byte, error) {
	time.Sleep(s.delay)
	if s.fail != "" && strings.Contains(text, s.fail) {
		return nil, errors.New("synth refused " + s.fail)
	}
	return []byte(text), nil
}

// orderPlayer records what was played, in order, across goroutines.
type orderPlayer struct {
	mu   sync.Mutex
	got  []string
	slow time.Duration
}

func (p *orderPlayer) Play(audio []byte) error {
	p.mu.Lock()
	p.got = append(p.got, string(audio))
	p.mu.Unlock()
	time.Sleep(p.slow)
	return nil
}

// pad makes a sentence long enough that the chunker cannot merge it with a
// neighbor, so each one is its own piece.
func pad(s string) string { return strings.Repeat("filler ", 28) + s }

func TestQueuePlaysTicketsInOrderEvenWhenLaterOneFinishesFirst(t *testing.T) {
	q := Queue{Dir: filepath.Join(t.TempDir(), "queue")}
	player := &orderPlayer{slow: 20 * time.Millisecond}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runSayQueued(pad("first line.")+" "+pad("second sentence."), slowSynth{delay: 150 * time.Millisecond}, player, q, discardf)
	}()
	time.Sleep(30 * time.Millisecond) // guarantee ticket order
	go func() {
		defer wg.Done()
		runSayQueued("later line.", slowSynth{delay: 10 * time.Millisecond}, player, q, discardf)
	}()
	wg.Wait()

	want := []string{pad("first line."), pad("second sentence."), "later line."}
	if strings.Join(player.got, "|") != strings.Join(want, "|") {
		t.Fatalf("played %q, want %q", player.got, want)
	}
	if left, _ := filepath.Glob(filepath.Join(q.Dir, "*.n")); len(left) != 0 {
		t.Fatalf("tickets left behind: %v", left)
	}
	if left, _ := filepath.Glob(filepath.Join(q.Dir, "*.mp3")); len(left) != 0 {
		t.Fatalf("pieces left behind: %v", left)
	}
}

func TestQueueSkipsFailedPieceAndRecordsIt(t *testing.T) {
	q := Queue{Dir: filepath.Join(t.TempDir(), "queue")}
	player := &orderPlayer{}

	runSayQueued(pad("good one.")+" "+pad("bad one.")+" "+pad("good two."), slowSynth{fail: "bad"}, player, q, discardf)

	if strings.Join(player.got, "|") != pad("good one.")+"|"+pad("good two.") {
		t.Fatalf("played %q", player.got)
	}
	failures := q.TakeFailures()
	if len(failures) != 1 || !strings.Contains(failures[0], "synth refused bad") {
		t.Fatalf("failures %q", failures)
	}
	if again := q.TakeFailures(); again != nil {
		t.Fatalf("failures not cleared: %q", again)
	}
}

func TestQueueFallsBackWhenDirUnusable(t *testing.T) {
	// A file where the queue directory should be makes MkdirAll fail.
	base := t.TempDir()
	blocker := filepath.Join(base, "queue")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	player := &orderPlayer{}
	runSayQueued("still spoken.", slowSynth{}, player, Queue{Dir: blocker}, discardf)
	if strings.Join(player.got, "|") != "still spoken." {
		t.Fatalf("played %q", player.got)
	}
}

func TestTakeFailuresBoundsOutput(t *testing.T) {
	q := Queue{Dir: t.TempDir()}
	for i := 0; i < 9; i++ {
		q.noteFailure("boom")
	}
	got := q.TakeFailures()
	if len(got) != 6 || !strings.HasPrefix(got[0], "4 earlier failures") {
		t.Fatalf("got %d lines: %q", len(got), got)
	}
}

// Each piece is a separate clip, and the API leaves almost no silence at the
// end of one, so the drainer has to put the gap in itself.
func TestQueuePausesBetweenPieces(t *testing.T) {
	q := Queue{Dir: filepath.Join(t.TempDir(), "queue")}
	player := &stampPlayer{}
	runSayQueued(pad("one.")+" "+pad("two."), slowSynth{}, player, q, discardf)
	if len(player.at) != 2 {
		t.Fatalf("played %d pieces", len(player.at))
	}
	if gap := player.at[1].Sub(player.at[0]); gap < pieceGap {
		t.Fatalf("gap between pieces was %v, want at least %v", gap, pieceGap)
	}
}

type stampPlayer struct{ at []time.Time }

func (p *stampPlayer) Play([]byte) error {
	p.at = append(p.at, time.Now())
	return nil
}
