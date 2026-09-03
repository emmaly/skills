package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// voicePrefix introduces the voice on the first line of a session file, after
// the timestamp. The first line is the one line instructions can never occupy,
// so a stored instruction that happens to begin with "voice=" cannot be read as
// the header. In the file rather than a sidecar: prune works from mtime, and a
// sidecar would be pruned out from under a session whose main file Enabled
// keeps refreshing.
const voicePrefix = "voice="

// ErrBadVoice is returned for a voice id that could not be a path segment of
// the API URL. The id is typed by a person and sent in a request path.
var ErrBadVoice = errors.New("invalid voice id")

// Enable turns TTS on for a session, with optional freeform instructions that
// shape what gets spoken. Calling it twice is not an error, and each call
// replaces the instructions, so "on" with no text is how they are cleared.
//
// A voice already chosen for the session is carried through. It is a setting,
// not an instruction, and clearing it on every "on" would have a person who
// picked a voice and then asked for shorter lines lose the voice.
func (s Store) Enable(session, instructions string) error {
	return s.update(session, func(voice, _ string) (string, string) {
		return voice, instructions
	})
}

// SetVoice records a voice for the session and turns TTS on. An empty id
// returns the session to the global default.
func (s Store) SetVoice(session, voice string) error {
	if voice != "" && !validVoiceID(voice) {
		return fmt.Errorf("%w: %q", ErrBadVoice, voice)
	}
	return s.update(session, func(_, instructions string) (string, string) {
		return voice, instructions
	})
}

// update applies a change to a session's voice and instructions under a
// lock. Each of Enable and SetVoice keeps the half it does not own, so two
// of them running at once, say the rewrite step's set racing a voice change
// the brief asked for, would each read the old file and the later rename
// would discard the other's change. The lock serializes the read and the
// write; writeAtomic still keeps concurrent readers from seeing a torn file.
func (s Store) update(session string, change func(voice, instructions string) (string, string)) error {
	if _, err := s.path(session); err != nil {
		return err
	}
	return s.locked(func() error {
		voice, instructions := s.read(session)
		voice, instructions = change(voice, instructions)
		return s.write(session, voice, instructions)
	})
}

// locked runs fn holding the store's one lock. Disable takes it too, so an
// off cannot land between an update's read and its write and be undone by
// the write.
func (s Store) locked(fn func() error) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	// The lock file sits beside the sessions directory, not in it, where a
	// file named like a session would read as an enabled one.
	lock, err := os.OpenFile(filepath.Join(s.Dir, "state.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open state lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock state: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// write lays the file out as one header line, then the instructions. The
// header is the timestamp, followed by the voice when there is one. A file
// from before the voice existed has a bare timestamp there and reads the
// same.
//
// The write is atomic. Enable and SetVoice each read the file and write it
// back, and the hook and detached say processes read it at any moment, so a
// truncate-then-write would hand a concurrent reader an empty file: a turn
// with no instructions and the default voice, with nothing in the log.
func (s Store) write(session, voice, instructions string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionsDir(), 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	header := time.Now().UTC().Format(time.RFC3339)
	if voice != "" {
		header += " " + voicePrefix + voice
	}
	body := header + "\n" + instructions
	if err := writeAtomic(target, []byte(body)); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// Instructions returns the freeform text stored with a session, or empty when
// there is none. Any error reads as none: the hook calls this on every prompt,
// and losing the extra guidance is better than failing the turn.
func (s Store) Instructions(session string) string {
	_, instructions := s.read(session)
	return instructions
}

// Voice returns the voice id chosen for the session, or empty when it should
// use the default. Any error reads as the default, for the same reason as
// Instructions.
func (s Store) Voice(session string) string {
	voice, _ := s.read(session)
	return voice
}

// read splits a session file into its voice and instructions. The voice is
// the "voice=" field on the first line; everything after that line is
// instructions, whatever it starts with.
func (s Store) read(session string) (voice, instructions string) {
	target, err := s.path(session)
	if err != nil {
		return "", ""
	}
	body, err := os.ReadFile(target)
	if err != nil {
		return "", ""
	}
	header, rest, found := strings.Cut(string(body), "\n")
	if !found {
		return "", ""
	}
	for _, field := range strings.Fields(header) {
		if id, ok := strings.CutPrefix(field, voicePrefix); ok && validVoiceID(id) {
			voice = id
		}
	}
	return voice, strings.TrimSpace(rest)
}

// validVoiceID accepts what ElevenLabs issues: ASCII letters and digits. The
// id is spliced into a URL path, so anything else is refused rather than
// escaped.
func validVoiceID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Disable turns TTS off. A session that was never enabled is not an error.
func (s Store) Disable(session string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	return s.locked(func() error {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove state: %w", err)
		}
		return nil
	})
}

// refreshAfter is how stale a session file may get before Enabled touches it.
// Pruning reads mtime, so without a refresh a session enabled more than the
// prune age ago and still alive gets switched off by the next unrelated
// session that starts. Touching on a timer rather than on every call keeps
// that from being a write per prompt.
const refreshAfter = time.Hour

// Enabled reports whether TTS is on. Any error reads as off, because the hook
// calls this on every prompt and the safe failure is silence.
//
// It also refreshes the file's mtime, making it a last-seen time rather than
// an enabled-at time, so a long-lived session is not pruned out from under
// itself.
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
	if !info.Mode().IsRegular() {
		return false
	}
	if now := time.Now(); now.Sub(info.ModTime()) > refreshAfter {
		// Best effort. A read-only state directory should not turn TTS off.
		_ = os.Chtimes(target, now, now)
	}
	return true
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
