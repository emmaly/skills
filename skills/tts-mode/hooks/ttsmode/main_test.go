package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envWith(pairs map[string]string) func(string) string {
	return func(name string) string { return pairs[name] }
}

func TestOnStatusOffCycle(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "sess",
		"TTSMODE_STATE_DIR":      dir,
	})

	var out bytes.Buffer
	if code := run([]string{"status"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("status exit %d", code)
	}
	if !strings.Contains(strings.ToLower(out.String()), "off") {
		t.Fatalf("status said %q, want off", out.String())
	}

	out.Reset()
	if code := run([]string{"on"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("on exit %d", code)
	}

	out.Reset()
	run([]string{"status"}, strings.NewReader(""), &out, &out, env)
	if !strings.Contains(strings.ToLower(out.String()), "on") {
		t.Fatalf("status said %q, want on", out.String())
	}

	out.Reset()
	run([]string{"off"}, strings.NewReader(""), &out, &out, env)
	out.Reset()
	run([]string{"status"}, strings.NewReader(""), &out, &out, env)
	if !strings.Contains(strings.ToLower(out.String()), "off") {
		t.Fatalf("status said %q, want off", out.String())
	}
}

func TestUnknownSubcommandExitsNonZero(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"wat"}, strings.NewReader(""), &out, &out, envWith(nil))
	if code == 0 {
		t.Fatal("an unknown subcommand should report failure to the caller")
	}
}

// No subcommand at all is a usage error, not a crash.
func TestNoSubcommandExitsNonZero(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, &out, envWith(nil)); code == 0 {
		t.Fatal("missing subcommand should report failure")
	}
}

func TestPruneSubcommandRuns(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{"TTSMODE_STATE_DIR": dir})
	var out bytes.Buffer
	if code := run([]string{"prune"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("prune exit %d", code)
	}
}

// The session flag overrides the environment, so the slash command can act on
// a session it was told about explicitly.
func TestSessionFlagOverridesEnv(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "from-env",
		"TTSMODE_STATE_DIR":      dir,
	})
	var out bytes.Buffer
	run([]string{"on", "--session", "explicit"}, strings.NewReader(""), &out, &out, env)

	if _, err := os.Stat(filepath.Join(dir, "explicit")); err != nil {
		t.Fatalf("expected state for the flagged session: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "from-env")); err == nil {
		t.Fatal("the environment session should not have been touched")
	}
}

// The hook subcommand must stay silent when the session is off, all the way
// through the dispatch layer and not only in runHook.
func TestHookSubcommandSilentWhenOff(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{"TTSMODE_STATE_DIR": dir})
	var out bytes.Buffer

	code := run([]string{"hook"}, strings.NewReader(`{"session_id":"quiet"}`), &out, &out, env)

	if code != 0 {
		t.Fatalf("hook exit %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("hook wrote %q while off", out.String())
	}
}

// A say with no configured key must not fail the turn.
func TestSayWithoutKeyExitsZero(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"TTSMODE_STATE_DIR": dir,
		"TTSMODE_ENV_FILE":  filepath.Join(dir, "absent.env"),
	})
	var out bytes.Buffer

	if code := run([]string{"say", "hello"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("say exit %d, want 0", code)
	}
}

// Say with no text is a usage error, caught before any API call.
func TestSayWithoutTextExitsNonZero(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"say"}, strings.NewReader(""), &out, &out, envWith(nil)); code == 0 {
		t.Fatal("say with no text should report a usage error")
	}
}
