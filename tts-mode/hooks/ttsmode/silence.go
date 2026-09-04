package main

import (
	"time"
)

// tailPad is the silence appended to every clip after synthesis. The API
// ends a clip within about 0.2 seconds of the last sound, so back-to-back
// clips sounded mashed together and the final clip stopped as if cut off,
// with the audio sink sometimes dropping the last buffer as the player
// exited. Silence inside the stream fixes both for every player: the real
// audio is flushed before the player reaches end of input, and the gap
// between clips comes with the clip, so the queue needs no sleep of its own.
//
// Padding the text instead was measured and rejected. Eleven v3 ignores SSML
// break tags, a trailing ellipsis added nothing in three runs, and "[pause]"
// added about three seconds in two runs out of five and nothing in the
// other three. Padding in the player was rejected too: mpv can append
// silence through a filter, but ffplay stops at end of input, so a machine
// with only ffplay kept the cut.
const tailPad = 350 * time.Millisecond

// padSilence appends d of silence to an MP3 stream as frames that match the
// stream's own header, so no decoder sees a format change. Audio that does
// not parse as MP3 is returned unchanged: a pad is a nicety, and refusing to
// play over it would be the wrong trade.
//
// A Layer III frame whose side information and main data are all zero
// decodes as digital silence, which is what makes this possible without an
// encoder: the frame is the stream's header with the padding bit cleared,
// followed by zeros to the frame length.
func padSilence(audio []byte, d time.Duration) []byte {
	if d <= 0 {
		return audio
	}
	header, frameLen, frameDur, ok := firstFrame(audio)
	if !ok {
		return audio
	}
	frames := int((d + frameDur - 1) / frameDur)
	out := make([]byte, 0, len(audio)+frames*frameLen)
	out = append(out, audio...)
	frame := make([]byte, frameLen)
	copy(frame, header[:])
	// Bit 1 of byte 2 is the padding flag. Cleared, so every appended frame
	// is the unpadded length computed below.
	frame[2] &^= 0x02
	for i := 0; i < frames; i++ {
		out = append(out, frame...)
	}
	return out
}

// firstFrame finds the first Layer III frame header, skipping an ID3v2 tag,
// and reports the header, the unpadded frame length in bytes, and the
// frame's duration.
func firstFrame(audio []byte) (header [4]byte, frameLen int, frameDur time.Duration, ok bool) {
	i := 0
	if len(audio) >= 10 && audio[0] == 'I' && audio[1] == 'D' && audio[2] == '3' {
		// The size is four syncsafe bytes: seven bits each.
		size := int(audio[6]&0x7f)<<21 | int(audio[7]&0x7f)<<14 | int(audio[8]&0x7f)<<7 | int(audio[9]&0x7f)
		i = 10 + size
	}
	for ; i+4 <= len(audio); i++ {
		if audio[i] != 0xff || audio[i+1]&0xe0 != 0xe0 {
			continue
		}
		frameLen, frameDur, ok = frameShape(audio[i : i+4])
		if !ok {
			continue
		}
		copy(header[:], audio[i:i+4])
		return header, frameLen, frameDur, true
	}
	return header, 0, 0, false
}

// frameShape decodes a Layer III frame header into its unpadded byte length
// and duration. Anything but Layer III, or a reserved field, is not ok.
func frameShape(h []byte) (frameLen int, frameDur time.Duration, ok bool) {
	version := (h[1] >> 3) & 3 // 3 = MPEG-1, 2 = MPEG-2, 0 = MPEG-2.5
	layer := (h[1] >> 1) & 3   // 1 = Layer III
	bitrateIndex := h[2] >> 4
	rateIndex := (h[2] >> 2) & 3
	if version == 1 || layer != 1 || bitrateIndex == 0 || bitrateIndex == 15 || rateIndex == 3 {
		return 0, 0, false
	}
	var bitrate int // kbit/s
	var sampleRate int
	var samples int
	switch version {
	case 3:
		bitrate = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}[bitrateIndex]
		sampleRate = []int{44100, 48000, 32000}[rateIndex]
		samples = 1152
	case 2:
		bitrate = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}[bitrateIndex]
		sampleRate = []int{22050, 24000, 16000}[rateIndex]
		samples = 576
	default:
		bitrate = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}[bitrateIndex]
		sampleRate = []int{11025, 12000, 8000}[rateIndex]
		samples = 576
	}
	frameLen = samples / 8 * bitrate * 1000 / sampleRate
	frameDur = time.Duration(samples) * time.Second / time.Duration(sampleRate)
	return frameLen, frameDur, true
}

// paddedSynth appends tailPad of silence to whatever the wrapped Synth
// returns. It wraps the synth rather than the player so every playback
// path, queued or direct, mpv or ffplay, gets the same clip.
type paddedSynth struct {
	Synth
}

// Speak synthesizes through the wrapped Synth and pads the result.
func (p paddedSynth) Speak(text string) ([]byte, error) {
	audio, err := p.Synth.Speak(text)
	if err != nil {
		return nil, err
	}
	return padSilence(audio, tailPad), nil
}
