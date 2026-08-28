package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnableDisableRoundTrip(t *testing.T) {
	store := Store{Dir: t.TempDir()}

	if store.Enabled("abc") {
		t.Fatal("a fresh session must start disabled")
	}
	if err := store.Enable("abc"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !store.Enabled("abc") {
		t.Fatal("enable did not take effect")
	}
	if err := store.Disable("abc"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if store.Enabled("abc") {
		t.Fatal("disable did not take effect")
	}
}

// Enabling twice must not error. The slash command is idempotent by design:
// saying /tts on when it is already on is not a mistake worth reporting.
func TestEnableIsIdempotent(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("abc"); err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if err := store.Enable("abc"); err != nil {
		t.Fatalf("second enable: %v", err)
	}
}

// Disabling a session that was never enabled must not error either.
func TestDisableUnknownSession(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Disable("never-seen"); err != nil {
		t.Fatalf("disable: %v", err)
	}
}

// Sessions must not see each other. Enabling in one terminal leaving another
// terminal silent is the whole reason state is per session.
func TestSessionsAreIsolated(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("one"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if store.Enabled("two") {
		t.Fatal("session two must not be affected by session one")
	}
}

// A session id arrives from a hook payload, so it must never be able to escape
// the state directory.
func TestSessionIDCannotEscapeDir(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}

	if err := store.Enable("../escaped"); err == nil {
		t.Fatal("a traversing session id must be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped")); !os.IsNotExist(err) {
		t.Fatal("a file was written outside the state directory")
	}
}

// A dot cleans to the sessions directory itself, which turned Disable into a
// recursive wipe of every other session and made Enabled report on for a
// session nobody enabled. Table covers the neighbours of that case too.
func TestSessionIDRejectsDirectoryAliases(t *testing.T) {
	store := Store{Dir: t.TempDir()}

	// A backslash is deliberately absent: on Linux it is a legal character in
	// a single filename, so rejecting it would not be the property under test.
	// The property is that an id resolves to exactly one element inside the
	// sessions directory and never to the directory itself.
	for _, session := range []string{".", "..", "./", "a/b", "/abs", "", "sub/../x"} {
		t.Run(session, func(t *testing.T) {
			if err := store.Enable(session); err == nil {
				t.Fatalf("Enable(%q) must be rejected", session)
			}
			if err := store.Disable(session); err == nil {
				t.Fatalf("Disable(%q) must be rejected", session)
			}
			if store.Enabled(session) {
				t.Fatalf("Enabled(%q) must be false", session)
			}
		})
	}
}

// The specific damage from the dot case: one session enabled, then an off for
// "." must not take it down with it.
func TestDisableDotDoesNotWipeOtherSessions(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("real"); err != nil {
		t.Fatalf("enable: %v", err)
	}

	if err := store.Disable("."); err == nil {
		t.Fatal("disabling the dot session must be rejected")
	}
	if !store.Enabled("real") {
		t.Fatal("an unrelated session was disabled")
	}
}

// The log sits beside the sessions directory, so pruning cannot delete the one
// file the README tells the user to read when speech goes quiet.
func TestPruneLeavesTheLogAlone(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}
	if err := store.Enable("recent"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := os.WriteFile(store.LogPath(), []byte("old failure\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(store.LogPath(), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if _, err := store.Prune(7*24*time.Hour, now); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := os.Stat(store.LogPath()); err != nil {
		t.Fatalf("prune removed the log: %v", err)
	}
}

// A session literally named "log" must not collide with the log file.
func TestSessionNamedLogIsIndependentOfTheLog(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := os.MkdirAll(store.Dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(store.LogPath(), []byte("a failure\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if store.Enabled("log") {
		t.Fatal("the log file must not read as an enabled session")
	}
}

func TestPruneRemovesOldKeepsNew(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}

	if err := store.Enable("fresh"); err != nil {
		t.Fatalf("enable fresh: %v", err)
	}
	if err := store.Enable("stale"); err != nil {
		t.Fatalf("enable stale: %v", err)
	}

	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "sessions", "stale"), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	removed, err := store.Prune(7*24*time.Hour, now)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed %d files, want 1", removed)
	}
	if !store.Enabled("fresh") {
		t.Fatal("prune removed a recent session")
	}
	if store.Enabled("stale") {
		t.Fatal("prune left a stale session")
	}
}

// Prune reads mtime, so a session enabled long ago and still alive would be
// switched off by the next unrelated session that starts. Enabled refreshes
// the file to make mtime a last-seen time instead.
func TestEnabledRefreshesStaleSession(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	if err := store.Enable("long-lived"); err != nil {
		t.Fatalf("enable: %v", err)
	}

	target := filepath.Join(store.Dir, "sessions", "long-lived")
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(target, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if !store.Enabled("long-lived") {
		t.Fatal("session should still be enabled")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Fatal("Enabled did not refresh the mtime, so prune will delete a live session")
	}

	// And now prune must leave it alone.
	removed, err := store.Prune(7*24*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 0 {
		t.Fatalf("prune removed %d live sessions", removed)
	}
}
