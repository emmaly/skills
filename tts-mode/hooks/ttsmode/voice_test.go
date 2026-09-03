package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoiceRoundTrip(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.SetVoice("abc", "5N1BjZ10t6GcJUhZCP40"); err != nil {
		t.Fatalf("set voice: %v", err)
	}
	if !store.Enabled("abc") {
		t.Fatal("setting a voice should enable the session")
	}
	if got := store.Voice("abc"); got != "5N1BjZ10t6GcJUhZCP40" {
		t.Fatalf("got %q", got)
	}
	if err := store.SetVoice("abc", ""); err != nil {
		t.Fatalf("clear voice: %v", err)
	}
	if got := store.Voice("abc"); got != "" {
		t.Fatalf("voice survived a clear: %q", got)
	}
}

// The voice is a setting, not an instruction. A plain "on", or a set of new
// instructions, must not lose it; "off" must.
func TestVoiceSurvivesEnableAndDiesWithDisable(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.SetVoice("abc", "5N1BjZ10t6GcJUhZCP40"); err != nil {
		t.Fatalf("set voice: %v", err)
	}
	if err := store.Enable("abc", "be terse"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if got := store.Voice("abc"); got != "5N1BjZ10t6GcJUhZCP40" {
		t.Fatalf("enable lost the voice: %q", got)
	}
	if got := store.Instructions("abc"); got != "be terse" {
		t.Fatalf("voice header leaked into instructions: %q", got)
	}
	if err := store.SetVoice("abc", "XdflFrQO8wbGpWMNZHFr"); err != nil {
		t.Fatalf("set voice again: %v", err)
	}
	if got := store.Instructions("abc"); got != "be terse" {
		t.Fatalf("set voice lost the instructions: %q", got)
	}
	if err := store.Disable("abc"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if got := store.Voice("abc"); got != "" {
		t.Fatalf("voice survived off: %q", got)
	}
}

// The id goes into a URL path, so anything that is not an id is refused.
func TestVoiceRejectsBadIDs(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	for _, bad := range []string{"../etc", "a b", "id?x=1", "voice/with/slash", strings.Repeat("a", 65)} {
		if err := store.SetVoice("abc", bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

// A file from before the header existed puts instructions on line two.
func TestVoiceOnLegacyStateFile(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "sessions", "abc")
	if err := os.WriteFile(path, []byte("2026-08-27T00:00:00Z\nbe terse\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := store.Voice("abc"); got != "" {
		t.Fatalf("legacy file produced a voice: %q", got)
	}
	if got := store.Instructions("abc"); got != "be terse" {
		t.Fatalf("got %q", got)
	}
}

// An instruction that starts with the header prefix but is not an id is
// prose, and must not be eaten as a header.
func TestInstructionsStartingWithVoicePrefixAreKept(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	want := "voice=warm and slow\nsay which file"
	if err := store.Enable("abc", want); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if got := store.Voice("abc"); got != "" {
		t.Fatalf("prose read as a voice: %q", got)
	}
	if got := store.Instructions("abc"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestControlVoice(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := control(t, dir, "s1", "voice 5N1BjZ10t6GcJUhZCP40")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "5N1BjZ10t6GcJUhZCP40") {
		t.Fatalf("got %q", out)
	}
	if got := (Store{Dir: dir}).Voice("s1"); got != "5N1BjZ10t6GcJUhZCP40" {
		t.Fatalf("stored %q", got)
	}

	code, out, _ = control(t, dir, "s1", "status")
	if code != 0 || !strings.Contains(out, "Voice for this session: 5N1BjZ10t6GcJUhZCP40") {
		t.Fatalf("status does not report the voice: %q", out)
	}

	code, out, errOut = control(t, dir, "s1", "voice default")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "default voice") {
		t.Fatalf("got %q", out)
	}
	if got := (Store{Dir: dir}).Voice("s1"); got != "" {
		t.Fatalf("default left %q", got)
	}
}

// One word only. A sentence after "voice" is a request, and "voic" is a typo.
func TestControlVoiceRejectsProseAndTypos(t *testing.T) {
	dir := t.TempDir()
	for _, raw := range []string{"voice", "voice please use 5N1BjZ10t6GcJUhZCP40", "voic", "voice ../x"} {
		if code, _, _ := control(t, dir, "s1", raw); code == 0 {
			t.Fatalf("%q accepted", raw)
		}
	}
	if (Store{Dir: dir}).Enabled("s1") {
		t.Fatal("a refused voice command enabled the session")
	}
}

func TestSpeakUsesGivenVoice(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := ElevenLabs{Key: "k", BaseURL: server.URL, Voice: "5N1BjZ10t6GcJUhZCP40", HTTP: server.Client()}
	if _, err := client.Speak("hello"); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if !strings.Contains(gotPath, "5N1BjZ10t6GcJUhZCP40") {
		t.Fatalf("path %q missing the chosen voice", gotPath)
	}
}

// say picks the session's voice over the install-wide override, and the
// override over the default. The stub records the request path and then
// fails the request, so nothing reaches a real audio player on the machine
// running the tests.
func TestSayResolvesVoiceInOrder(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		http.Error(w, "stub", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dir := t.TempDir()
	vars := map[string]string{
		"CLAUDE_CODE_SESSION_ID": "loud",
		"TTSMODE_STATE_DIR":      dir,
		"TTSMODE_API_BASE":       server.URL,
		"ELEVENLABS_API_KEY":     "k",
	}
	env := envWith(vars)
	var out bytes.Buffer

	run([]string{"on"}, strings.NewReader(""), &out, &out, env)
	run([]string{"say", "one"}, strings.NewReader(""), &out, &out, env)

	vars["TTSMODE_VOICE_ID"] = "GlobalVoice0000000001"
	run([]string{"say", "two"}, strings.NewReader(""), &out, &out, env)

	run([]string{"voice", "SessionVoice000000001"}, strings.NewReader(""), &out, &out, env)
	run([]string{"say", "three"}, strings.NewReader(""), &out, &out, env)

	if len(paths) != 3 {
		t.Fatalf("expected three requests, got %d: %v", len(paths), paths)
	}
	for i, want := range []string{defaultVoiceID, "GlobalVoice0000000001", "SessionVoice000000001"} {
		if !strings.Contains(paths[i], want) {
			t.Fatalf("request %d used %q, want %s", i, paths[i], want)
		}
	}
}
