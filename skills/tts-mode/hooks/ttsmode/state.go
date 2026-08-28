package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store holds one file per enabled session. Presence of the file means TTS is
// on for that session; the contents are a timestamp, kept only so there is
// something readable to look at when debugging.
//
// Per-session state rather than one global switch: enabling audio in the
// terminal you are sitting at should not make an unrelated session in another
// window start talking hours later.
//
// Session files live in a sessions/ subdirectory rather than in Dir itself, so
// that the log Dir also holds can never be mistaken for a session, pruned as a
// stale one, or deleted by an "off" for a session literally named "log".
type Store struct {
	Dir string
}

// ErrBadSession is returned for a session id that could address anything other
// than one plain file inside the sessions directory. The id arrives from a
// hook payload, which is not ours to trust.
var ErrBadSession = errors.New("invalid session id")

// sessionsDir keeps session files apart from anything else under Dir.
func (s Store) sessionsDir() string {
	return filepath.Join(s.Dir, "sessions")
}

// LogPath is where failures are recorded. It sits beside the sessions
// directory rather than inside it, so pruning cannot delete the one file the
// user is told to read when speech goes quiet.
func (s Store) LogPath() string {
	return filepath.Join(s.Dir, "log")
}

// path validates a session id and returns the file that represents it.
//
// The check is a whitelist in spirit: after cleaning, the id must still be
// exactly one path element that is not a traversal. A blacklist of separators
// missed "." entirely, which cleaned to the directory itself and turned
// Disable into a recursive wipe of every other session.
func (s Store) path(session string) (string, error) {
	if session == "" {
		return "", fmt.Errorf("%w: empty", ErrBadSession)
	}
	if session != filepath.Base(session) || session == "." || session == ".." {
		return "", fmt.Errorf("%w: %q", ErrBadSession, session)
	}
	if filepath.IsAbs(session) || filepath.Clean(session) != session {
		return "", fmt.Errorf("%w: %q", ErrBadSession, session)
	}
	return filepath.Join(s.sessionsDir(), session), nil
}

// Enable turns TTS on for a session. Calling it twice is not an error.
func (s Store) Enable(session string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionsDir(), 0o700); err != nil {
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
	info, err := os.Stat(target)
	if err != nil {
		return false
	}
	// A directory named like a session is not an enabled session.
	return info.Mode().IsRegular()
}

// Prune removes session files last modified more than age ago and reports how
// many it removed. Sessions end without notice, so there is no shutdown path
// to clean up from.
func (s Store) Prune(age time.Duration, now time.Time) (int, error) {
	dir := s.sessionsDir()
	entries, err := os.ReadDir(dir)
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
			if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}
