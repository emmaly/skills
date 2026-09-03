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

// voicePrefix marks the one header line a session file may carry between the
// timestamp and the instructions. A header rather than a sidecar file: prune
// works from mtime, and a sidecar would be pruned out from under a session
// whose main file Enabled keeps refreshing.
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
	return s.write(session, s.Voice(session), instructions)
}

// SetVoice records a voice for the session and turns TTS on. An empty id
// returns the session to the global default.
func (s Store) SetVoice(session, voice string) error {
	if voice != "" && !validVoiceID(voice) {
		return fmt.Errorf("%w: %q", ErrBadVoice, voice)
	}
	return s.write(session, voice, s.Instructions(session))
}

// write lays the file out as timestamp, optional voice header, instructions.
// A file written before either header existed has only the first line, and
// still reads correctly as enabled with none.
func (s Store) write(session, voice, instructions string) error {
	target, err := s.path(session)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionsDir(), 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	body := time.Now().UTC().Format(time.RFC3339) + "\n"
	if voice != "" {
		body += voicePrefix + voice + "\n"
	}
	body += instructions
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
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

// read splits a session file into its voice header and instructions. The
// header is recognized by its prefix, so a file from before the header existed
// reads its second line as instructions, as it always did.
func (s Store) read(session string) (voice, instructions string) {
	target, err := s.path(session)
	if err != nil {
		return "", ""
	}
	body, err := os.ReadFile(target)
	if err != nil {
		return "", ""
	}
	_, rest, found := strings.Cut(string(body), "\n")
	if !found {
		return "", ""
	}
	// The line is a header only when it parses as an id. An instruction that
	// happens to start with "voice=" is otherwise prose, and consuming it
	// would silently drop the first line someone wrote.
	if strings.HasPrefix(rest, voicePrefix) {
		line, after, _ := strings.Cut(rest, "\n")
		if id := strings.TrimSpace(strings.TrimPrefix(line, voicePrefix)); validVoiceID(id) {
			voice = id
			rest = after
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
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state: %w", err)
	}
	return nil
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
