package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The queue serializes playback across every session of one user while
// letting synthesis run ahead of it.
//
// The old shape held one lock around synthesis and playback together, so a
// second line waited on the first line's network round trip before its own
// began, and a turn with three lines paid three round trips end to end. Now
// each say takes a ticket, synthesizes its pieces concurrently, drops them in
// the queue directory, and whoever holds the player lock drains tickets in
// order. Playback order is ticket order, not synthesis-finish order.
//
// Files under <state>/queue:
//
//	counter        the last ticket handed out, updated under flock
//	player.lock    held by the process currently draining
//	<T>.n          written first; holds the piece count for ticket T
//	<T>.<i>.mp3    piece i, renamed into place when complete
//	<T>.<i>.fail   piece i could not be synthesized; the error text
//
// A ticket whose pieces never arrive is abandoned after pieceWait so a
// crashed producer cannot wedge every later line.

const (
	// synthParallel bounds concurrent API calls per say.
	synthParallel = 3
	// pieceWait is how long the drainer waits for a piece to appear before it
	// gives up on that ticket. Synthesis of one short piece is a few seconds.
	pieceWait = 45 * time.Second
	// pollEvery is the drainer's sleep while waiting on a piece or the lock.
	pollEvery = 100 * time.Millisecond
)

// Queue is the on-disk queue for one user.
type Queue struct {
	Dir string
}

// ticket hands out the next sequence number under a lock, so two says that
// start at the same moment cannot share one.
func (q Queue) ticket() (int64, error) {
	if err := os.MkdirAll(q.Dir, 0o700); err != nil {
		return 0, fmt.Errorf("create queue dir: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(q.Dir, "counter"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open counter: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0, fmt.Errorf("lock counter: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	var last int64
	if body, err := os.ReadFile(f.Name()); err == nil {
		last, _ = strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	}
	next := last + 1
	if err := f.Truncate(0); err != nil {
		return 0, fmt.Errorf("reset counter: %w", err)
	}
	if _, err := f.WriteAt([]byte(strconv.FormatInt(next, 10)), 0); err != nil {
		return 0, fmt.Errorf("write counter: %w", err)
	}
	return next, nil
}

func (q Queue) countPath(t int64) string { return filepath.Join(q.Dir, fmt.Sprintf("%d.n", t)) }
func (q Queue) piecePath(t int64, i int) string {
	return filepath.Join(q.Dir, fmt.Sprintf("%d.%d.mp3", t, i))
}
func (q Queue) failPath(t int64, i int) string {
	return filepath.Join(q.Dir, fmt.Sprintf("%d.%d.fail", t, i))
}

// writeAtomic lands a file by rename, so the drainer never plays a half-written
// piece.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// runSayQueued speaks text through the queue. It always returns 0: speech is
// a convenience, and no failure of it should fail the turn that asked for it.
// Failures are logged and also recorded for the next say to report.
func runSayQueued(text string, synth Synth, player Player, q Queue, logf func(string, ...any)) int {
	synth = paddedSynth{synth, logf}
	chunks := speakable(text, logf)
	if len(chunks) == 0 {
		logf("empty text, nothing to speak")
		return 0
	}
	t, err := q.ticket()
	if err != nil {
		// No queue means no ordering, but silence is worse. Speak directly.
		logf("queue unavailable, speaking unqueued: %v", err)
		return playDirect(chunks, synth, player, logf)
	}
	if err := writeAtomic(q.countPath(t), []byte(strconv.Itoa(len(chunks)))); err != nil {
		logf("queue unavailable, speaking unqueued: %v", err)
		return playDirect(chunks, synth, player, logf)
	}

	// Synthesize concurrently; pieces land as they finish. The drainer plays
	// them in index order, waiting for each in turn.
	var wg sync.WaitGroup
	sem := make(chan struct{}, synthParallel)
	failures := make([]string, len(chunks))
	for i, chunk := range chunks {
		wg.Add(1)
		go func(i int, chunk string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			audio, err := synth.Speak(chunk)
			if err == nil {
				err = writeAtomic(q.piecePath(t, i), audio)
			}
			if err != nil {
				failures[i] = err.Error()
				_ = writeAtomic(q.failPath(t, i), []byte(err.Error()))
			}
		}(i, chunk)
	}

	// Drain while synthesis runs, so the first piece plays as soon as it
	// lands rather than after the last one does.
	q.drain(t, player, logf)
	wg.Wait()

	for i, msg := range failures {
		if msg != "" {
			logf("synthesis failed for piece %d: %s", i+1, msg)
			q.noteFailure(fmt.Sprintf("synthesis failed: %s", msg))
		}
	}
	return 0
}

// playDirect is the fallback when the queue directory cannot be used.
func playDirect(chunks []string, synth Synth, player Player, logf func(string, ...any)) int {
	for _, chunk := range chunks {
		audio, err := synth.Speak(chunk)
		if err != nil {
			logf("synthesis failed: %v", err)
			return 0
		}
		if err := player.Play(audio); err != nil {
			logf("playback failed: %v", err)
			return 0
		}
	}
	return 0
}

// drain plays tickets in order until the queue is empty, or until this
// process's own ticket has been played by someone else.
//
// Lock handling: a non-blocking try. If another process holds the lock it is
// draining, and it will reach our ticket unless it exits between deciding the
// queue is empty and releasing the lock. That window is closed by looping:
// while our ticket still exists we keep trying the lock, so an orphaned
// ticket is picked up by its own producer.
func (q Queue) drain(mine int64, player Player, logf func(string, ...any)) {
	for {
		if _, err := os.Stat(q.countPath(mine)); err != nil {
			return // played by another drainer
		}
		lock, err := os.OpenFile(filepath.Join(q.Dir, "player.lock"), os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			logf("cannot open player lock: %v", err)
			return
		}
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			lock.Close()
			if !errors.Is(err, syscall.EWOULDBLOCK) {
				logf("cannot take player lock: %v", err)
				return
			}
			time.Sleep(pollEvery)
			continue
		}
		q.drainLocked(player, logf)
		syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		lock.Close()
		// Loop once more: if our ticket is gone we return, otherwise we race
		// for the lock again.
	}
}

// drainLocked plays every ticket in order, oldest first, and returns when
// none remain. Caller holds the player lock.
func (q Queue) drainLocked(player Player, logf func(string, ...any)) {
	for {
		t, ok := q.oldest()
		if !ok {
			return
		}
		q.playTicket(t, player, logf)
		_ = os.Remove(q.countPath(t))
	}
}

// oldest returns the lowest ticket with a count file.
func (q Queue) oldest() (int64, bool) {
	matches, _ := filepath.Glob(filepath.Join(q.Dir, "*.n"))
	var tickets []int64
	for _, m := range matches {
		base := strings.TrimSuffix(filepath.Base(m), ".n")
		if t, err := strconv.ParseInt(base, 10, 64); err == nil {
			tickets = append(tickets, t)
		}
	}
	if len(tickets) == 0 {
		return 0, false
	}
	sort.Slice(tickets, func(i, j int) bool { return tickets[i] < tickets[j] })
	return tickets[0], true
}

// playTicket plays each piece of one ticket in order, waiting for pieces that
// are still being synthesized.
func (q Queue) playTicket(t int64, player Player, logf func(string, ...any)) {
	body, err := os.ReadFile(q.countPath(t))
	if err != nil {
		return
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil || n <= 0 {
		return
	}
	info, err := os.Stat(q.countPath(t))
	if err != nil {
		return
	}
	deadline := info.ModTime().Add(pieceWait)

	for i := 0; i < n; i++ {
		piece := q.piecePath(t, i)
		fail := q.failPath(t, i)
		for {
			if _, err := os.Stat(fail); err == nil {
				os.Remove(fail)
				break
			}
			audio, err := os.ReadFile(piece)
			if err == nil {
				os.Remove(piece)
				// No sleep between pieces. Each clip carries tailPad of
				// silence from synthesis, and that is the gap.
				if err := player.Play(audio); err != nil {
					logf("playback failed: %v", err)
					q.noteFailure(fmt.Sprintf("playback failed: %v", err))
				}
				deadline = time.Now().Add(pieceWait)
				break
			}
			if time.Now().After(deadline) {
				logf("ticket %d piece %d never arrived, abandoning the rest", t, i+1)
				q.noteFailure(fmt.Sprintf("a line was abandoned: piece %d of %d never arrived", i+1, n))
				for j := i; j < n; j++ {
					os.Remove(q.piecePath(t, j))
					os.Remove(q.failPath(t, j))
				}
				return
			}
			time.Sleep(pollEvery)
		}
	}
}

// failuresPath is where failures wait to be reported to the next say call.
func (q Queue) failuresPath() string { return filepath.Join(q.Dir, "failures") }

// noteFailure records a failure for the caller of the next say to see. The
// background job that hit it has no stdout anyone reads; the next say does.
func (q Queue) noteFailure(msg string) {
	if err := os.MkdirAll(q.Dir, 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(q.failuresPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
}

// TakeFailures returns recorded failures and clears them. Lines are kept
// short and bounded so a burst cannot flood the caller's context.
func (q Queue) TakeFailures() []string {
	body, err := os.ReadFile(q.failuresPath())
	if err != nil {
		return nil
	}
	_ = os.Remove(q.failuresPath())
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	const keep = 5
	if len(lines) > keep {
		lines = append([]string{fmt.Sprintf("%d earlier failures not shown", len(lines)-keep)}, lines[len(lines)-keep:]...)
	}
	var out []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
