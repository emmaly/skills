package main

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// monoHeader is what the API sends for mp3_44100_192: MPEG-1 Layer III,
// 192 kbit/s, 44.1 kHz, mono, no padding. One frame is 626 bytes and lasts
// 1152 samples, about 26.1 ms.
var monoHeader = []byte{0xff, 0xfb, 0xb0, 0xc0}

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

// 350 ms of 26.1 ms frames rounds up to 14 whole frames, each a copy of the
// stream's header followed by zeros, appended after the original bytes.
func TestPadSilenceAppendsMatchingSilentFrames(t *testing.T) {
	in := clip(3)
	out := padSilence(in, tailPad)
	if !bytes.HasPrefix(out, in) {
		t.Fatal("original bytes were changed")
	}
	added := out[len(in):]
	if len(added) != 14*626 {
		t.Fatalf("added %d bytes, want %d", len(added), 14*626)
	}
	for i := 0; i < 14; i++ {
		frame := added[i*626 : (i+1)*626]
		if !bytes.Equal(frame[:4], monoHeader) {
			t.Fatalf("frame %d header % x, want % x", i, frame[:4], monoHeader)
		}
		if !bytes.Equal(frame[4:], make([]byte, 622)) {
			t.Fatalf("frame %d body is not silent", i)
		}
	}
}

// A stream whose first frame has the padding bit set still gets unpadded
// silent frames, so the declared and actual lengths agree.
func TestPadSilenceClearsPaddingBit(t *testing.T) {
	in := append([]byte{}, monoHeader...)
	in[2] |= 0x02
	in = append(in, bytes.Repeat([]byte{0xaa}, 627-4)...)
	out := padSilence(in, tailPad)
	added := out[len(in):]
	if len(added)%626 != 0 {
		t.Fatalf("added %d bytes, not a whole number of unpadded frames", len(added))
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
	out := padSilence(in, tailPad)
	added := out[len(in):]
	// 350 ms / 24 ms rounds up to 15 frames.
	if len(added) != 15*576 {
		t.Fatalf("added %d bytes, want %d", len(added), 15*576)
	}
	if !bytes.Equal(added[:4], header) {
		t.Fatalf("appended header % x, want % x", added[:4], header)
	}
}

// Bytes that are not MP3, or a zero pad, come back untouched. A pad is a
// nicety, and refusing to play over it would be the wrong trade.
func TestPadSilenceLeavesUnparseableAudioAlone(t *testing.T) {
	garbage := []byte("not an mp3 at all")
	if out := padSilence(garbage, tailPad); !bytes.Equal(out, garbage) {
		t.Fatalf("garbage was changed: %q", out)
	}
	in := clip(1)
	if out := padSilence(in, 0); !bytes.Equal(out, in) {
		t.Fatal("zero pad changed the audio")
	}
	if out := padSilence(nil, tailPad); out != nil {
		t.Fatalf("nil audio became %v", out)
	}
}

// paddedSynth pads what the wrapped synth returns and passes its errors
// through unchanged.
func TestPaddedSynth(t *testing.T) {
	in := clip(2)
	synth := paddedSynth{&fakeSynth{audio: in}}
	out, err := synth.Speak("hello")
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if len(out) != len(in)+14*626 {
		t.Fatalf("got %d bytes, want %d", len(out), len(in)+14*626)
	}
	boom := errors.New("boom")
	if _, err := (paddedSynth{&fakeSynth{err: boom}}).Speak("hello"); !errors.Is(err, boom) {
		t.Fatalf("error not passed through: %v", err)
	}
}

// The pad is short enough to be a breath, not a pause the listener notices.
func TestTailPadIsABreath(t *testing.T) {
	if tailPad < 200*time.Millisecond || tailPad > 600*time.Millisecond {
		t.Fatalf("tailPad is %v", tailPad)
	}
}
