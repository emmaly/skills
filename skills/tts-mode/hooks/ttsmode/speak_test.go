package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSynth struct {
	audio []byte
	err   error
	got   string
}

func (f *fakeSynth) Speak(text string) ([]byte, error) {
	f.got = text
	return f.audio, f.err
}

type fakePlayer struct {
	got []byte
	err error
}

func (f *fakePlayer) Play(audio []byte) error {
	f.got = audio
	return f.err
}

func discardf(string, ...any) {}

func TestSayRendersAndPlays(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3-bytes")}
	player := &fakePlayer{}

	if code := runSay("done building", synth, player, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if synth.got != "done building" {
		t.Fatalf("synth saw %q", synth.got)
	}
	if string(player.got) != "mp3-bytes" {
		t.Fatalf("player saw %q", player.got)
	}
}

// Every failure exits zero. A dead API or a missing player must never fail the
// turn that asked for speech.
func TestSayExitsZeroOnSynthFailure(t *testing.T) {
	synth := &fakeSynth{err: errors.New("network down")}
	player := &fakePlayer{}

	if code := runSay("anything", synth, player, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if player.got != nil {
		t.Fatal("player must not run when synthesis failed")
	}
}

func TestSayExitsZeroOnPlayerFailure(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3-bytes")}
	player := &fakePlayer{err: errors.New("no audio device")}

	if code := runSay("anything", synth, player, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
}

func TestSayRejectsEmptyText(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3-bytes")}
	if code := runSay("   ", synth, &fakePlayer{}, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if synth.got != "" {
		t.Fatal("empty text must not reach the API and spend credits")
	}
}

func TestElevenLabsPostsExpectedRequest(t *testing.T) {
	var gotPath, gotKey, gotQuery string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotKey = r.Header.Get("xi-api-key")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := ElevenLabs{Key: "secret", BaseURL: server.URL, HTTP: server.Client()}
	audio, err := client.Speak("hello")
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if string(audio) != "audio" {
		t.Fatalf("got %q", audio)
	}
	if !strings.Contains(gotPath, voiceID) {
		t.Fatalf("path %q missing the voice id", gotPath)
	}
	if gotKey != "secret" {
		t.Fatalf("key header %q", gotKey)
	}
	if !strings.Contains(gotQuery, "mp3_44100_192") {
		t.Fatalf("query %q missing the output format", gotQuery)
	}
	if gotBody["model_id"] != "eleven_v3" {
		t.Fatalf("model_id %v", gotBody["model_id"])
	}
	settings, ok := gotBody["voice_settings"].(map[string]any)
	if !ok {
		t.Fatal("voice_settings missing")
	}
	if settings["similarity_boost"] != 0.9 {
		t.Fatalf("similarity_boost %v, want 0.9", settings["similarity_boost"])
	}
	if settings["stability"] != 0.3 {
		t.Fatalf("stability %v, want 0.3", settings["stability"])
	}
}

func TestElevenLabsReportsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := ElevenLabs{Key: "bad", BaseURL: server.URL, HTTP: server.Client()}
	if _, err := client.Speak("hello"); err == nil {
		t.Fatal("expected an error for a 401")
	}
}

// The API's own explanation of a failure has to reach the log. Returning only
// the status turned a 422 about bad settings into a bare status line.
func TestElevenLabsErrorIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"detail":"voice_settings.speed out of range"}`))
	}))
	defer server.Close()

	client := ElevenLabs{Key: "k", BaseURL: server.URL, HTTP: server.Client()}
	_, err := client.Speak("hello")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "speed out of range") {
		t.Fatalf("error lost the API detail: %v", err)
	}
}
