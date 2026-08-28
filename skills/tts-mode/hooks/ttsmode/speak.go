package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Voice and settings are fixed rather than configurable. They were chosen by
// listening, and two of them are constraints rather than taste: the voice must
// have rate 1.0, because a higher rate doubles the per-character cost, and it
// must have live moderation off, because moderation adds latency to every line.
const (
	voiceID      = "XdflFrQO8wbGpWMNZHFr"
	modelID      = "eleven_v3"
	outputFormat = "mp3_44100_192"
	defaultBase  = "https://api.us.elevenlabs.io"
)

// Synth turns text into audio bytes.
type Synth interface {
	Speak(text string) ([]byte, error)
}

// Player sends audio bytes to a device.
type Player interface {
	Play(audio []byte) error
}

// runSay renders one line and plays it. It always returns 0: speech is a
// convenience, and no failure of it should fail the turn that asked for it.
func runSay(text string, synth Synth, player Player, logf func(string, ...any)) int {
	text = strings.TrimSpace(text)
	if text == "" {
		logf("empty text, nothing to speak")
		return 0
	}

	audio, err := synth.Speak(text)
	if err != nil {
		logf("synthesis failed: %v", err)
		return 0
	}
	if err := player.Play(audio); err != nil {
		logf("playback failed: %v", err)
	}
	return 0
}

// ElevenLabs is the real Synth.
type ElevenLabs struct {
	Key     string
	BaseURL string
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

	url := fmt.Sprintf("%s/v1/text-to-speech/%s?output_format=%s", base, voiceID, outputFormat)
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

	audio, err := readAllLimited(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned %s", resp.Status)
	}
	return audio, nil
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

// CommandPlayer writes the audio to a temporary file and hands it to the first
// player found on the system.
type CommandPlayer struct{}

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

	for _, candidate := range [][]string{
		{"mpv", "--no-video", "--really-quiet"},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet"},
	} {
		path, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		args := append(candidate[1:], file.Name())
		return exec.Command(path, args...).Run()
	}
	return fmt.Errorf("no audio player found, install mpv or ffplay")
}
