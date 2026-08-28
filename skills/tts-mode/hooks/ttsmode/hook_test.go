package main

import (
	"bytes"
	"strings"
	"testing"
)

func noEnv(string) string { return "" }

// The load-bearing test. When TTS is off the hook must emit nothing at all, so
// no summary is ever requested. Suppressing summaries after the fact would
// still cost tokens and still risk them appearing.
func TestHookSilentWhenDisabled(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	var out bytes.Buffer

	code := runHook(strings.NewReader(`{"session_id":"abc"}`), &out, store, noEnv, "", "/plugin/hooks/tts-say.sh")

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("wrote %q, want nothing", out.String())
	}
}

func TestHookEmitsInstructionWhenEnabled(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("abc"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	var out bytes.Buffer

	code := runHook(strings.NewReader(`{"session_id":"abc"}`), &out, store, noEnv, "", "/plugin/hooks/tts-say.sh")

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	// Compared case-insensitively: the assertion is that each constraint is
	// stated, not that it appears at a particular position in a sentence.
	got := out.String()
	lowered := strings.ToLower(got)
	for _, want := range []string{"/plugin/hooks/tts-say.sh", "fifteen words", "three spoken lines"} {
		if !strings.Contains(lowered, want) {
			t.Fatalf("instruction missing %q, got:\n%s", want, got)
		}
	}
}

// The wrapper path is absolute and computed at runtime, because Claude's Bash
// environment has no CLAUDE_PLUGIN_ROOT to expand.
func TestHookUsesGivenWrapperPath(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("abc"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	var out bytes.Buffer

	runHook(strings.NewReader(`{"session_id":"abc"}`), &out, store, noEnv, "", "/somewhere/else/tts-say.sh")

	if !strings.Contains(out.String(), "/somewhere/else/tts-say.sh") {
		t.Fatal("instruction did not use the supplied wrapper path")
	}
}

// Falling back to the environment matters because not every hook event carries
// a session id in its payload.
func TestHookFallsBackToEnvSession(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("from-env"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	env := func(name string) string {
		if name == "CLAUDE_CODE_SESSION_ID" {
			return "from-env"
		}
		return ""
	}
	var out bytes.Buffer

	runHook(strings.NewReader(`{}`), &out, store, env, "", "/plugin/hooks/tts-say.sh")

	if out.Len() == 0 {
		t.Fatal("expected the instruction using the session id from the environment")
	}
}

// A malformed payload must not wedge the session, and must not turn TTS on.
func TestHookSurvivesGarbagePayload(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	var out bytes.Buffer

	code := runHook(strings.NewReader("not json at all"), &out, store, noEnv, "", "/plugin/hooks/tts-say.sh")

	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("wrote %q on a bad payload, want nothing", out.String())
	}
}

// The documented order puts --session ahead of the payload. Before this, the
// hook branch discarded the flag entirely and the doc comment was wrong.
func TestHookPrefersOverrideOverPayload(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("flagged"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	var out bytes.Buffer

	runHook(strings.NewReader(`{"session_id":"payload"}`), &out, store, noEnv, "flagged", "/plugin/hooks/tts-say.sh")

	if out.Len() == 0 {
		t.Fatal("the --session override was ignored in favour of the payload")
	}
}
