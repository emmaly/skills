package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// monoHeader is what the API sends for mp3_44100_192: MPEG-1 Layer III,
// 192 kbit/s, 44.1 kHz, mono, no CRC, no padding. One frame is 626 bytes and
// lasts 1152 samples, about 26.1 ms.
var monoHeader = []byte{0xff, 0xfb, 0xb0, 0xc0}

// padFrames is how many 26.1 ms frames it takes to cover tailPad: 350 ms
// rounds up to 14.
const padFrames = 14

// clip builds a fake stream: an ID3v2 tag, then n frames of the header
// followed by arbitrary bytes.
func clip(n int) []byte {
	// A ten-byte ID3 header with a syncsafe size of 6, then six bytes of tag.
	out := []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 6, 1, 2, 3, 4, 5, 6}
	for i := 0; i < n; i++ {
		out = append(out, monoHeader...)
		out = append(out, bytes.Repeat([]byte{0xaa}, 626-4)...)
	}
	return out
}

// The appended frames are whole, silent, and a copy of the stream's header,
// placed after the original bytes.
func TestPadSilenceAppendsMatchingSilentFrames(t *testing.T) {
	in := clip(3)
	out, ok := padSilence(in, tailPad)
	if !ok {
		t.Fatal("stream not recognized")
	}
	if !bytes.HasPrefix(out, in) {
		t.Fatal("original bytes were changed")
	}
	added := out[len(in):]
	if len(added) != padFrames*626 {
		t.Fatalf("added %d bytes, want %d", len(added), padFrames*626)
	}
	for i := 0; i < padFrames; i++ {
		frame := added[i*626 : (i+1)*626]
		if !bytes.Equal(frame[:4], monoHeader) {
			t.Fatalf("frame %d header % x, want % x", i, frame[:4], monoHeader)
		}
		if !bytes.Equal(frame[4:], make([]byte, 622)) {
			t.Fatalf("frame %d body is not silent", i)
		}
	}
}

// A source header with the padding bit set, or CRC protection on, still
// yields self-consistent silent frames: unpadded length, no CRC field.
func TestPadSilenceNormalizesPaddingAndCRC(t *testing.T) {
	in := append([]byte{}, monoHeader...)
	in[1] &^= 0x01 // CRC protected
	in[2] |= 0x02  // padding bit
	in = append(in, bytes.Repeat([]byte{0xaa}, 627-4)...)
	out, ok := padSilence(in, tailPad)
	if !ok {
		t.Fatal("stream not recognized")
	}
	added := out[len(in):]
	if len(added) != padFrames*626 {
		t.Fatalf("added %d bytes, want %d unpadded frames", len(added), padFrames)
	}
	if added[1]&0x01 == 0 {
		t.Fatalf("appended frame still claims a CRC: % x", added[:4])
	}
	if added[2]&0x02 != 0 {
		t.Fatalf("appended frame keeps the padding bit: % x", added[:4])
	}
}

// The pad follows the stream's own format. A 48 kHz stereo header gives a
// different frame length and duration, so a different byte count.
func TestPadSilenceFollowsTheStreamFormat(t *testing.T) {
	// MPEG-1 Layer III, 192 kbit/s, 48 kHz, stereo: 576 bytes, 24 ms.
	header := []byte{0xff, 0xfb, 0xb4, 0x00}
	in := append(append([]byte{}, header...), bytes.Repeat([]byte{0xaa}, 576-4)...)
	out, _ := padSilence(in, tailPad)
	added := out[len(in):]
	// 350 ms / 24 ms rounds up to 15 frames.
	if len(added) != 15*576 {
		t.Fatalf("added %d bytes, want %d", len(added), 15*576)
	}
	if !bytes.Equal(added[:4], header) {
		t.Fatalf("appended header % x, want % x", added[:4], header)
	}
}

// Bytes inside a frame can look like a header. A candidate is accepted only
// when the next frame starts where it says it ends, so a stream that begins
// mid-frame syncs on the first real frame, not the lookalike.
func TestPadSilenceSkipsFalseSync(t *testing.T) {
	// A lookalike stereo 48 kHz header (576-byte frames) buried in junk,
	// followed by two real mono frames that do line up with each other.
	junk := append([]byte{0x01, 0x02}, []byte{0xff, 0xfb, 0xb4, 0x00}...)
	junk = append(junk, bytes.Repeat([]byte{0x33}, 100)...)
	in := append(junk, clip(2)[16:]...) // clip without its ID3 tag
	out, ok := padSilence(in, tailPad)
	if !ok {
		t.Fatal("real frames not found")
	}
	added := out[len(in):]
	if len(added) != padFrames*626 || !bytes.Equal(added[:4], monoHeader) {
		t.Fatalf("padded with the lookalike's format: %d bytes, header % x", len(added), added[:4])
	}
}

// A lookalike header near the end of a non-MP3 stream, whose claimed frame
// runs past the end, is not a frame. A lone real frame that ends exactly at
// the end is.
func TestPadSilenceRejectsLookalikeRunningPastTheEnd(t *testing.T) {
	junk := bytes.Repeat([]byte{0x11}, 1800)
	junk = append(junk, monoHeader...)
	junk = append(junk, bytes.Repeat([]byte{0x22}, 196)...)
	if _, ok := padSilence(junk, tailPad); ok {
		t.Fatal("lookalike running past the end was accepted")
	}
	one := clip(1)[16:]
	if _, ok := padSilence(one, tailPad); !ok {
		t.Fatal("single complete frame was rejected")
	}
}

// Bytes that are not MP3 come back untouched and are reported as such; a
// zero pad is a no-op that still counts as fine.
func TestPadSilenceLeavesUnparseableAudioAlone(t *testing.T) {
	garbage := []byte("not an mp3 at all")
	out, ok := padSilence(garbage, tailPad)
	if ok || !bytes.Equal(out, garbage) {
		t.Fatalf("garbage: ok=%v out=%q", ok, out)
	}
	in := clip(1)
	if out, ok := padSilence(in, 0); !ok || !bytes.Equal(out, in) {
		t.Fatal("zero pad changed the audio or reported failure")
	}
	if out, ok := padSilence(nil, tailPad); ok || out != nil {
		t.Fatalf("nil audio: ok=%v out=%v", ok, out)
	}
}

// paddedSynth pads what the wrapped synth returns, passes errors through,
// and logs when it had to skip the pad.
func TestPaddedSynth(t *testing.T) {
	var logged []string
	logf := func(f string, a ...any) { logged = append(logged, f) }

	in := clip(2)
	out, err := paddedSynth{&fakeSynth{audio: in}, logf}.Speak("hello")
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if len(out) != len(in)+padFrames*626 {
		t.Fatalf("got %d bytes, want %d", len(out), len(in)+padFrames*626)
	}
	if len(logged) != 0 {
		t.Fatalf("logged on success: %q", logged)
	}

	boom := errors.New("boom")
	if _, err := (paddedSynth{&fakeSynth{err: boom}, logf}).Speak("hello"); !errors.Is(err, boom) {
		t.Fatalf("error not passed through: %v", err)
	}

	if _, err := (paddedSynth{&fakeSynth{audio: []byte("opus?")}, logf}).Speak("hello"); err != nil {
		t.Fatalf("unreadable audio became an error: %v", err)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "tail pad skipped") {
		t.Fatalf("skip not logged: %q", logged)
	}
}

// Both say paths pad on their own, so a caller cannot forget to. The queued
// path is the one the wrapper uses; the direct path is its fallback.
func TestSayPathsPadEveryPiece(t *testing.T) {
	in := clip(2)
	want := len(in) + padFrames*626

	queued := &orderPlayer{}
	runSayQueued("one. two.", &fakeSynth{audio: in}, queued, Queue{Dir: filepath.Join(t.TempDir(), "queue")}, discardf)
	if len(queued.got) != 1 || len(queued.got[0]) != want {
		t.Fatalf("queued path played %d pieces, first %d bytes, want 1 piece of %d", len(queued.got), len(queued.got[0]), want)
	}

	direct := &fakePlayer{}
	runSay("one. two.", &fakeSynth{audio: in}, direct, discardf)
	if len(direct.got) != want {
		t.Fatalf("direct path played %d bytes, want %d", len(direct.got), want)
	}
}

// The padder reads MP3, so the API has to be asked for MP3. Switching the
// output format without teaching the padder the new container would drop
// the pad silently, apart from a log line per piece.
func TestOutputFormatIsMP3(t *testing.T) {
	if !strings.HasPrefix(outputFormat, "mp3_") {
		t.Fatalf("outputFormat is %q, and padSilence only reads MP3", outputFormat)
	}
}
