package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Settings are fixed rather than configurable. They were chosen by listening.
//
// The voice has a default and two overrides: TTSMODE_VOICE_ID for the whole
// install, and a per-session choice stored with the session, which wins. Any
// voice used here should have rate 1.0, because a higher rate doubles the
// per-character cost, and live moderation off, because moderation adds latency
// to every line. Neither is checked by the code.
const (
	defaultVoiceID = "XdflFrQO8wbGpWMNZHFr"
	modelID        = "eleven_v3"
	outputFormat   = "mp3_44100_192"

	// The generic host, not a data-residency one. An account on an isolated
	// EU, India, or Singapore workspace needs a different base, so
	// TTSMODE_API_BASE overrides this without editing and rebuilding source.
	defaultBase = "https://api.elevenlabs.io"
)

// Synth turns text into audio bytes.
type Synth interface {
	Speak(text string) ([]byte, error)
}

// Player sends audio bytes to a device.
type Player interface {
	Play(audio []byte) error
}

// runSay renders text and plays it in this process, piece by piece, with no
// queue. The queued path in queue.go is what the wrapper uses; this is the
// fallback and the unit under test. It always returns 0: speech is a
// convenience, and no failure of it should fail the turn that asked for it.
func runSay(text string, synth Synth, player Player, logf func(string, ...any)) int {
	synth = paddedSynth{synth, logf}
	chunks := speakable(text, logf)
	if len(chunks) == 0 {
		logf("empty text, nothing to speak")
		return 0
	}
	return playDirect(chunks, synth, player, logf)
}

// ElevenLabs is the real Synth. An empty Voice means defaultVoiceID.
type ElevenLabs struct {
	Key     string
	BaseURL string
	Voice   string
	HTTP    *http.Client
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style"`
	Speed           float64 `json:"speed"`
	SpeakerBoost    bool    `json:"use_speaker_boost"`
}

type speakRequest struct {
	Text     string        `json:"text"`
	ModelID  string        `json:"model_id"`
	Settings voiceSettings `json:"voice_settings"`
}

// Speak posts one piece of text to the text-to-speech endpoint and returns
// the audio bytes. A non-200 response is an error carrying the API's own
// explanation, since that is what says which setting to change.
func (e ElevenLabs) Speak(text string) ([]byte, error) {
	base := e.BaseURL
	if base == "" {
		base = defaultBase
	}
	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	body, err := json.Marshal(speakRequest{
		Text:    text,
		ModelID: modelID,
		Settings: voiceSettings{
			Stability:       0.3,
			SimilarityBoost: 0.9,
			Style:           0.48,
			Speed:           0.88,
			SpeakerBoost:    true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	voice := e.Voice
	if voice == "" {
		voice = defaultVoiceID
	}
	url := fmt.Sprintf("%s/v1/text-to-speech/%s?output_format=%s", base, voice, outputFormat)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("xi-api-key", e.Key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call api: %w", err)
	}
	defer resp.Body.Close()

	body, err = readAllLimited(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		// The body is already read, and on an error it holds the API's own
		// explanation. Returning only the status turned a 422 about bad voice
		// settings into a bare status line, which does not tell the reader
		// what to change.
		return nil, fmt.Errorf("api returned %s: %s", resp.Status, snippet(body))
	}
	return body, nil
}

// snippet trims an error body down to something a log line can carry.
func snippet(body []byte) string {
	const max = 300
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "empty response body"
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > max {
		return text[:max] + "..."
	}
	return text
}

// readAllLimited caps the response so a wrong URL returning a large body
// cannot exhaust memory. Ten megabytes is far beyond any spoken line.
func readAllLimited(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

// playTimeout bounds a stuck player. A spoken line is seconds long, so a
// minute is generous. Without it, a player hung on a busy audio device holds
// the playback lock forever and every later line in every session waits behind
// it, silently, with nothing in the log because nothing failed.
const playTimeout = 60 * time.Second

// CommandPlayer writes the audio to a temporary file and hands it to the first
// player found on the system.
type CommandPlayer struct{}

// Play writes the audio to a temporary file and plays it with mpv, or ffplay
// when mpv is missing or fails, each bounded by playTimeout.
func (CommandPlayer) Play(audio []byte) error {
	file, err := os.CreateTemp("", "ttsmode-*.mp3")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.Write(audio); err != nil {
		file.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Try each player until one succeeds. Stopping at the first installed one
	// would strand a headless box that has mpv but no reachable audio device,
	// logging the same mpv failure forever while an installed ffplay that
	// would have worked is never tried.
	var lastErr error
	for _, candidate := range [][]string{
		{"mpv", "--no-video", "--no-terminal", "--really-quiet"},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	} {
		path, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), playTimeout)
		args := append(candidate[1:], file.Name())
		err = exec.CommandContext(ctx, path, args...).Run()
		timedOut := ctx.Err() != nil
		cancel()

		if err == nil {
			return nil
		}
		if timedOut {
			lastErr = fmt.Errorf("%s timed out after %s", candidate[0], playTimeout)
		} else {
			lastErr = fmt.Errorf("%s: %w", candidate[0], err)
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no audio player found, install mpv or ffplay")
}
