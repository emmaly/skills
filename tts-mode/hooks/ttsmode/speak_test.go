package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

type fakeSynth struct {
	audio []byte
	err   error
	got   string
}

// Speak records the text it was given and returns the canned audio or error.
func (f *fakeSynth) Speak(text string) ([]byte, error) {
	f.got = text
	return f.audio, f.err
}

type fakePlayer struct {
	got []byte
	err error
}

// Play records the audio it was given and returns the canned error.
func (f *fakePlayer) Play(audio []byte) error {
	f.got = audio
	return f.err
}

// discardf is a logf that drops everything, for tests that do not care what
// was logged.
func discardf(string, ...any) {}

// The happy path: text goes to the synth, audio goes to the player, exit 0.
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

// A player that fails must not fail the turn: exit 0, failure in the log.
func TestSayExitsZeroOnPlayerFailure(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3-bytes")}
	player := &fakePlayer{err: errors.New("no audio device")}

	if code := runSay("anything", synth, player, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
}

// Whitespace-only text is nothing to speak: no API call, exit 0.
func TestSayRejectsEmptyText(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3-bytes")}
	if code := runSay("   ", synth, &fakePlayer{}, discardf); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if synth.got != "" {
		t.Fatal("empty text must not reach the API and spend credits")
	}
}

// The request carries the key header, the default voice in the path, the
// output format in the query, and the fixed model and voice settings.
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
	if !strings.Contains(gotPath, defaultVoiceID) {
		t.Fatalf("path %q missing the default voice id", gotPath)
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

// A non-200 status is an error, not audio.
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

// The base URL defaults to the generic host, overridable for an isolated
// residency workspace. Nothing outside tests set it before.
func TestElevenLabsDefaultBaseIsGenericHost(t *testing.T) {
	if defaultBase != "https://api.elevenlabs.io" {
		t.Fatalf("defaultBase is %q, want the generic host", defaultBase)
	}
}

// Billing is per character and the word limit lives only in an instruction a
// model can ignore, so the cap has to be in the code.
func TestSayTruncatesOverlongText(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3")}
	long := strings.Repeat("a", maxSpokenChars*3)

	if code := runSay(long, synth, &fakePlayer{}, discardf); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(synth.got) > maxSpokenChars {
		t.Fatalf("sent %d characters, cap is %d", len(synth.got), maxSpokenChars)
	}
}

// A normal line is untouched.
func TestSayLeavesNormalTextAlone(t *testing.T) {
	synth := &fakeSynth{audio: []byte("mp3")}
	line := "Tests pass and the branch is ready."

	runSay(line, synth, &fakePlayer{}, discardf)

	if synth.got != line {
		t.Fatalf("got %q, want %q", synth.got, line)
	}
}

// Truncation must not split a multi-byte character. Invalid UTF-8 is silently
// replaced with U+FFFD during encoding, so the API would be billed to speak a
// replacement character.
func TestSayTruncatesOnRuneBoundary(t *testing.T) {
	// 399 ASCII characters then a three-byte one, so a byte slice at 400 would
	// land inside that character.
	long := strings.Repeat("a", maxSpokenChars-1) + "é" + strings.Repeat("b", 50)

	synth := &fakeSynth{audio: []byte("mp3")}
	runSay(long, synth, &fakePlayer{}, discardf)

	if !utf8.ValidString(synth.got) {
		t.Fatalf("truncated text is not valid UTF-8: %q", synth.got)
	}
	if n := utf8.RuneCountInString(synth.got); n > maxSpokenChars {
		t.Fatalf("sent %d runes, cap is %d", n, maxSpokenChars)
	}
	if !strings.HasSuffix(synth.got, "é") {
		t.Fatalf("expected the boundary rune intact, got %q", synth.got[len(synth.got)-8:])
	}
}
