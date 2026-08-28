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
	if err := os.Chtimes(filepath.Join(dir, "stale"), old, old); err != nil {
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
