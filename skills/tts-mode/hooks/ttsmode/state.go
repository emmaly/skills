package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store holds one file per enabled session. Presence of the file means TTS is
// on for that session; the contents are a timestamp, kept only so there is
// something readable to look at when debugging.
//
// Per-session state rather than one global switch: enabling audio in the
// terminal you are sitting at should not make an unrelated session in another
// window start talking hours later.
type Store struct {
	Dir string
}

// ErrBadSession is returned for a session id that could address a file outside
// Dir. The id arrives from a hook payload, which is not ours to trust.
var ErrBadSession = errors.New("invalid session id")

func (s Store) path(session string) (string, error) {
	if session == "" {
		return "", fmt.Errorf("%w: empty", ErrBadSession)
	}
	if strings.ContainsAny(session, `/\`) || strings.Contains(session, "..") {
		return "", fmt.Errorf("%w: %q", ErrBadSession, session)
	}
	return filepath.Join(s.Dir, session), nil
}

// Enable turns TTS on for a session. Calling it twice is not an error.
func (s Store) Enable(session string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	stamp := time.Now().UTC().Format(time.RFC3339) + "\n"
	if err := os.WriteFile(target, []byte(stamp), 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Disable turns TTS off. A session that was never enabled is not an error.
func (s Store) Disable(session string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}
	return nil
}

// Enabled reports whether TTS is on. Any error reads as off, because the hook
// calls this on every prompt and the safe failure is silence.
func (s Store) Enabled(session string) bool {
	target, err := s.path(session)
	if err != nil {
		return false
	}
	_, err = os.Stat(target)
	return err == nil
}

// Prune removes state files last modified more than age ago and reports how
// many it removed. Sessions end without notice, so there is no shutdown path
// to clean up from.
func (s Store) Prune(age time.Duration, now time.Time) (int, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read state dir: %w", err)
	}

	cutoff := now.Add(-age)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(s.Dir, entry.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
