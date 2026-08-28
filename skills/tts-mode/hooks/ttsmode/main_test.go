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

	if _, err := os.Stat(filepath.Join(dir, "sessions", "explicit")); err != nil {
		t.Fatalf("expected state for the flagged session: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "from-env")); err == nil {
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
//
// The session is enabled first on purpose. Without that, an empty session id
// is rejected by the store, say returns at the "TTS is off" branch, and the
// key path this test is named for is never reached.
func TestSayWithoutKeyExitsZero(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "keyless",
		"TTSMODE_STATE_DIR":      dir,
		"TTSMODE_ENV_FILE":       filepath.Join(dir, "absent.env"),
	})
	var out bytes.Buffer
	if code := run([]string{"on"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("on exit %d", code)
	}
	out.Reset()

	if code := run([]string{"say", "hello"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("say exit %d, want 0", code)
	}

	logged, err := os.ReadFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logged), "no api key") {
		t.Fatalf("expected the key path to be reached, log says %q", logged)
	}
}

// The wrapper needs somewhere to record what it could not do, since a dropped
// line with nothing in the log is the failure the README rules out.
func TestLogSubcommandWritesToTheLog(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{"TTSMODE_STATE_DIR": dir})
	var out bytes.Buffer

	if code := run([]string{"log", "dropped", "a", "line"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("log exit %d", code)
	}

	logged, err := os.ReadFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logged), "dropped a line") {
		t.Fatalf("log says %q", logged)
	}
}

// Say with no text is a usage error, caught before any API call.
func TestSayWithoutTextExitsNonZero(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"say"}, strings.NewReader(""), &out, &out, envWith(nil)); code == 0 {
		t.Fatal("say with no text should report a usage error")
	}
}

// The off switch has to stop speech that is already requested. Instructions
// are re-injected every turn, so after an off many stale copies remain in the
// transcript; if say did not check state, those would keep speaking and
// spending after the user turned it off.
func TestSayIsIgnoredWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "eleven.env")
	if err := os.WriteFile(envFile, []byte("ELEVENLABS_API_KEY=would-be-used\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "quiet",
		"TTSMODE_STATE_DIR":      dir,
		"TTSMODE_ENV_FILE":       envFile,
	})

	var out bytes.Buffer
	if code := run([]string{"say", "should not speak"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("say exit %d, want 0", code)
	}

	logged, err := os.ReadFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logged), "TTS is off") {
		t.Fatalf("expected the log to record the refusal, got %q", logged)
	}
}

// A model that drops the quotes must not have its line truncated at the first
// space, silently losing everything after it.
func TestSayJoinsAllArguments(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "loud",
		"TTSMODE_STATE_DIR":      dir,
		"TTSMODE_ENV_FILE":       filepath.Join(dir, "absent.env"),
	})

	var out bytes.Buffer
	run([]string{"on"}, strings.NewReader(""), &out, &out, env)
	out.Reset()

	// No key is configured, so this stops at key resolution. What matters is
	// that it got past the enabled check with the whole line intact, which the
	// log records as a key failure rather than a refusal.
	if code := run([]string{"say", "three", "word", "line"}, strings.NewReader(""), &out, &out, env); code != 0 {
		t.Fatalf("say exit %d, want 0", code)
	}
	logged, err := os.ReadFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(logged), "TTS is off") {
		t.Fatal("say was refused for a session that is on")
	}
	if !strings.Contains(string(logged), "no api key") {
		t.Fatalf("expected a key failure, got %q", logged)
	}
}

// say joins every remaining argument into the spoken line, so scanning the
// whole list for --session let a line that merely contained that word swallow
// the next one and drop both.
func TestSessionFlagOnlyLeading(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{
		"CLAUDE_CODE_SESSION_ID": "sess",
		"TTSMODE_STATE_DIR":      dir,
		"TTSMODE_ENV_FILE":       filepath.Join(dir, "absent.env"),
	})

	var out bytes.Buffer
	run([]string{"on"}, strings.NewReader(""), &out, &out, env)
	out.Reset()

	// The word appears mid-text. It must stay text.
	run([]string{"say", "Fixed", "the", "--session", "flag", "parsing"}, strings.NewReader(""), &out, &out, env)

	logged, err := os.ReadFile(filepath.Join(dir, "log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(logged), "TTS is off") {
		t.Fatal("a --session inside the spoken text hijacked the session id")
	}
}

// Leading --session still works, since the slash command relies on it.
func TestSessionFlagStillWorksLeading(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{"TTSMODE_STATE_DIR": dir})
	var out bytes.Buffer

	run([]string{"on", "--session", "explicit"}, strings.NewReader(""), &out, &out, env)

	if _, err := os.Stat(filepath.Join(dir, "sessions", "explicit")); err != nil {
		t.Fatalf("leading --session stopped working: %v", err)
	}
}

// Prune never walks the log, so without a cap it grows for the life of the
// install.
func TestLogIsCapped(t *testing.T) {
	dir := t.TempDir()
	env := envWith(map[string]string{"TTSMODE_STATE_DIR": dir})
	logPath := filepath.Join(dir, "log")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(logPath, bytes.Repeat([]byte("x"), maxLogBytes+1), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	var out bytes.Buffer
	run([]string{"log", "after the cap"}, strings.NewReader(""), &out, &out, env)

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() > maxLogBytes {
		t.Fatalf("log is %d bytes, cap is %d", info.Size(), maxLogBytes)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "after the cap") {
		t.Fatal("the newest entry was lost")
	}
}
