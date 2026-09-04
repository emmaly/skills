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
// stream's own header, so no decoder sees a format change. It reports false,
// with the audio unchanged, when the bytes do not parse as MP3: a pad is a
// nicety, and refusing to play over it would be the wrong trade.
//
// A Layer III frame whose side information and main data are all zero
// decodes as digital silence, which is what makes this possible without an
// encoder: the frame is the stream's header, normalized to no padding and no
// CRC, followed by zeros to the frame length.
func padSilence(audio []byte, d time.Duration) ([]byte, bool) {
	if d <= 0 {
		return audio, true
	}
	header, frameLen, frameDur, ok := firstFrame(audio)
	if !ok {
		return audio, false
	}
	frames := int((d + frameDur - 1) / frameDur)
	out := make([]byte, 0, len(audio)+frames*frameLen)
	out = append(out, audio...)
	frame := make([]byte, frameLen)
	copy(frame, header[:])
	// Bit 0 of byte 1 set means no CRC, so a zero CRC field can never
	// disagree with the zero side info. Bit 1 of byte 2 is the padding flag,
	// cleared so every appended frame is the unpadded length.
	frame[1] |= 0x01
	frame[2] &^= 0x02
	for i := 0; i < frames; i++ {
		out = append(out, frame...)
	}
	return out, true
}

// firstFrame finds the first Layer III frame header, skipping an ID3v2 tag,
// and reports the header, the unpadded frame length in bytes, and the
// frame's duration.
//
// A candidate counts only when the next frame starts where this one says it
// ends, or this one runs to the end of the stream. Audio bytes can look like
// a header, and accepting one of those would declare a format the stream
// does not have.
func firstFrame(audio []byte) (header [4]byte, frameLen int, frameDur time.Duration, ok bool) {
	i := 0
	if len(audio) >= 10 && audio[0] == 'I' && audio[1] == 'D' && audio[2] == '3' {
		// The size is four syncsafe bytes: seven bits each.
		size := int(audio[6]&0x7f)<<21 | int(audio[7]&0x7f)<<14 | int(audio[8]&0x7f)<<7 | int(audio[9]&0x7f)
		i = 10 + size
	}
	for ; i+4 <= len(audio); i++ {
		if !isSync(audio[i:]) {
			continue
		}
		frameLen, frameDur, ok = frameShape(audio[i : i+4])
		if !ok {
			continue
		}
		next := i + frameLen + int(audio[i+2]>>1&1)
		if next < len(audio) && !isSync(audio[next:]) {
			continue
		}
		copy(header[:], audio[i:i+4])
		return header, frameLen, frameDur, true
	}
	return header, 0, 0, false
}

// isSync reports whether b starts with the eleven-bit frame sync.
func isSync(b []byte) bool {
	return len(b) >= 2 && b[0] == 0xff && b[1]&0xe0 == 0xe0
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
	var bitrate, sampleRate, samples int // kbit/s, Hz, per frame
	if version == 3 {
		bitrate = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}[bitrateIndex]
		sampleRate = []int{44100, 48000, 32000}[rateIndex]
		samples = 1152
	} else {
		bitrate = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}[bitrateIndex]
		sampleRate = []int{22050, 24000, 16000}[rateIndex]
		if version == 0 {
			sampleRate /= 2
		}
		samples = 576
	}
	frameLen = samples / 8 * bitrate * 1000 / sampleRate
	frameDur = time.Duration(samples) * time.Second / time.Duration(sampleRate)
	return frameLen, frameDur, true
}

// paddedSynth appends tailPad of silence to whatever the wrapped Synth
// returns. Both say paths wrap their synth in it, so no call site can forget
// the pad. A stream the padder cannot read is logged once per piece, since
// the gap and the tail silently disappearing is exactly the failure the log
// exists to explain.
type paddedSynth struct {
	Synth
	logf func(string, ...any)
}

// Speak synthesizes through the wrapped Synth and pads the result.
func (p paddedSynth) Speak(text string) ([]byte, error) {
	audio, err := p.Synth.Speak(text)
	if err != nil {
		return nil, err
	}
	out, ok := padSilence(audio, tailPad)
	if !ok && p.logf != nil {
		p.logf("tail pad skipped: audio is not an MP3 stream this build can read")
	}
	return out, nil
}
