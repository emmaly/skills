// Command ttsmode speaks short summaries of Claude's work aloud while enabled,
// and does nothing at all while disabled.
//
// Subcommands:
//
//	hook    Read a hook payload on stdin. Print the summary instruction when
//	        this session is enabled, print nothing when it is not.
//	on      Enable for this session.
//	off     Disable for this session.
//	status  Report the current state.
//	say     Render one line and play it.
//	log     Append a message to the log, for the shell wrapper's use.
//	prune   Remove state files from sessions that ended long ago.
//
// Session id resolution, in order: the --session flag, the session_id in the
// hook payload, then CLAUDE_CODE_SESSION_ID. State is per session so that
// enabling audio in one terminal leaves other sessions silent.
//
// Exit codes: only a usage error exits non-zero. Every runtime failure logs and
// exits zero, because a hook that can fail a turn is worse than one that
// quietly does nothing.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const pruneAge = 7 * 24 * time.Hour

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, env func(string) string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ttsmode hook|on|off|status|say|prune")
		return 2
	}

	command, rest := args[0], args[1:]
	sessionFlag, rest := takeSessionFlag(rest)
	session := sessionFlag
	if session == "" {
		session = env("CLAUDE_CODE_SESSION_ID")
	}
	// Usage errors are decided before anything touches the filesystem. They
	// do not depend on a state directory, and reporting "no HOME" for a
	// misspelled subcommand would send the reader after the wrong problem.
	switch command {
	case "hook", "on", "off", "status", "say", "log", "prune":
	default:
		fmt.Fprintf(stderr, "ttsmode: unknown subcommand %q\n", command)
		return 2
	}
	if command == "say" && len(rest) == 0 {
		fmt.Fprintln(stderr, `usage: ttsmode say "<text>"`)
		return 2
	}

	dir, err := stateDir(env)
	if err != nil {
		// Same split as the shell wrappers. on/off/status are typed by a
		// person, so silence would read as success while TTS stayed off.
		// Everything else runs unattended and must not fail the turn, and
		// there is nowhere to log because the log lives in the directory we
		// just failed to resolve.
		switch command {
		case "on", "off", "status":
			fmt.Fprintf(stderr, "ttsmode: %v; set HOME or TTSMODE_STATE_DIR\n", err)
			return 1
		}
		return 0
	}
	store := Store{Dir: dir}
	logf := logger(store)

	switch command {
	case "hook":
		return runHook(stdin, stdout, store, env, sessionFlag, wrapperPath(env))

	case "on":
		if err := store.Enable(session); err != nil {
			fmt.Fprintf(stderr, "ttsmode: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Spoken output is ON for this session.")
		return 0

	case "off":
		if err := store.Disable(session); err != nil {
			fmt.Fprintf(stderr, "ttsmode: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Spoken output is OFF for this session.")
		return 0

	case "status":
		if store.Enabled(session) {
			fmt.Fprintln(stdout, "Spoken output is ON for this session.")
		} else {
			fmt.Fprintln(stdout, "Spoken output is OFF for this session.")
		}
		return 0

	case "say":
		// Speech is gated on the live state, not only on the instruction that
		// asked for it. The instruction is re-injected every turn, so after an
		// "off" many stale copies remain in the transcript and nothing
		// counter-instructs them. Without this check, "off" would stop future
		// requests but not the ones already in context, and the switch would
		// not actually stop speech or spend.
		if !store.Enabled(session) {
			logf("say ignored: TTS is off for this session")
			return 0
		}
		// apiKey prefers ELEVENLABS_API_KEY and only falls back to the file, so
		// a refused path is not itself a failure. Bailing on it here broke the
		// documented HOME-less setup, where the key is in the environment and
		// no file is ever opened. The refusal is reported only when there is
		// also no key, which is when it explains the silence.
		envFile, pathErr := envFilePath(env)
		key, err := apiKey(env, envFile)
		if err != nil {
			if pathErr != nil {
				logf("%v", pathErr)
			}
			logf("no api key: %v", err)
			return 0
		}
		// Join rather than take the first argument. The wrapper passes the line
		// as one argument, but a hand-run call that splits it would otherwise
		// be truncated at the first space with the rest silently dropped.
		client := ElevenLabs{Key: key, BaseURL: env("TTSMODE_API_BASE")}
		return runSay(strings.Join(rest, " "), client, CommandPlayer{}, logf)

	case "log":
		// Lets the shell wrapper record what it could not do. A dropped line
		// with nothing in the log is exactly the failure the README promises
		// cannot happen, and the wrapper has no other way to write there.
		if len(rest) > 0 {
			logf("%s", strings.Join(rest, " "))
		}
		return 0

	case "prune":
		removed, err := store.Prune(pruneAge, time.Now())
		if err != nil {
			logf("prune: %v", err)
			return 0
		}
		if removed > 0 {
			logf("pruned %d stale session files", removed)
		}
		return 0

	}

	// Unreachable while the validation list above and the dispatch switch agree.
	// Saying so beats a bare exit 2: for the unattended subcommands the wrapper
	// discards the exit code, so drift would be a silent no-op with nothing in
	// the log, which is the outcome the user-facing split exists to prevent.
	// Both, because neither alone reaches every caller: the wrapper sends the
	// unattended subcommands to /dev/null, and the log is what the README
	// tells the user to read when speech goes quiet.
	logf("subcommand %q passed validation but has no handler", command)
	fmt.Fprintf(stderr, "ttsmode: subcommand %q passed validation but has no handler\n", command)
	return 2
}

// takeSessionFlag pulls an optional --session out of the front of the argument
// list. It reports only what the flag said, empty when absent, so callers can
// tell an explicit flag from the environment fallback. The hook needs that
// difference: its documented order puts the flag ahead of the payload but the
// environment behind it.
//
// Only the leading position is scanned. say joins every remaining argument
// into the spoken line, so scanning the whole list let a line that merely
// contained the word --session swallow the next word as a session id and drop
// both, sending the rest to a session that was probably not enabled.
func takeSessionFlag(args []string) (string, []string) {
	if len(args) >= 2 && args[0] == "--session" {
		return args[1], args[2:]
	}
	return "", args
}

// stateDir reports where session state and the log live, or an error when
// there is nowhere safe to put them.
//
// It deliberately has no fallback. Deriving a path from an unset HOME gave a
// fixed, predictable location that any local user could pre-create, and both
// the state write and the log append follow symlinks. The shell wrappers
// refuse in the same situation, so refusing here keeps the two halves
// agreeing about what "no HOME" means.
func stateDir(env func(string) string) (string, error) {
	if dir := env("TTSMODE_STATE_DIR"); dir != "" {
		// Absolute only. A relative override resolves against whatever
		// directory the hook ran in, which is the exposure the "." fallback
		// was removed to close.
		if !filepath.IsAbs(dir) {
			return "", fmt.Errorf("TTSMODE_STATE_DIR is not an absolute path: %q", dir)
		}
		return dir, nil
	}
	home, err := homeDir(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "tts-mode"), nil
}

// envFilePath reports where to read the API key from, or an error when there
// is no safe path to read.
//
// A relative override is refused for the same reason as the state directory,
// and here it selects which key gets used: a hook running in a checked-out
// repository would read that project's own .secrets/elevenlabs.env.
func envFilePath(env func(string) string) (string, error) {
	if path := env("TTSMODE_ENV_FILE"); path != "" {
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("TTSMODE_ENV_FILE is not an absolute path: %q", path)
		}
		return path, nil
	}
	home, err := homeDir(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".secrets", "elevenlabs.env"), nil
}

// wrapperPath is the absolute path to tts-say.sh, which the instruction tells
// Claude to run. Claude's Bash environment has no CLAUDE_PLUGIN_ROOT to expand,
// so the path is resolved here, where that variable is available.
func wrapperPath(env func(string) string) string {
	if root := env("CLAUDE_PLUGIN_ROOT"); root != "" {
		return filepath.Join(root, "hooks", "tts-say.sh")
	}
	return "tts-say.sh"
}

// homeDir refuses rather than falling back to the current directory. Returning
// "." put state and the log under whatever directory a hook happened to run
// in, which is the same predictable-path exposure as a fixed /tmp path and
// also littered the user's projects.
//
// os.UserHomeDir is deliberately not used as a fallback. On Linux it returns
// $HOME and errors when that is unset, so it is redundant with the lookup
// below, and because it reads the process environment directly it would
// bypass the injected env and make the two disagree under test.
func homeDir(env func(string) string) (string, error) {
	home := env("HOME")
	if home == "" {
		return "", errors.New("HOME is not set")
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("HOME is not an absolute path: %q", home)
	}
	return home, nil
}

// maxLogBytes caps the diagnostic log. Prune only walks the sessions
// directory, so without a cap this file grows for the life of the install. A
// quarter megabyte is thousands of failures, far more than anyone reads.
const maxLogBytes = 256 << 10

// logger appends to the store's log. Speech failures are invisible by design,
// so there has to be somewhere to look when it goes quiet.
func logger(store Store) func(string, ...any) {
	return func(format string, args ...any) {
		if err := os.MkdirAll(store.Dir, 0o700); err != nil {
			return
		}
		path := store.LogPath()
		record := fmt.Sprintf("%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))

		// A record longer than the whole cap would leave the file above the
		// limit however often it is truncated. say caps its own text, but log
		// takes whatever the wrapper joined together.
		if len(record) > maxLogBytes {
			cut := maxLogBytes - 1
			for cut > 0 && !utf8.RuneStart(record[cut]) {
				cut--
			}
			record = record[:cut] + "\n"
		}

		// Measure the record against the cap, not just the file. Checking only
		// the existing size let the file finish one whole record above the
		// limit.
		if info, err := os.Stat(path); err == nil && info.Size()+int64(len(record)) > maxLogBytes {
			// Start over rather than keep a tail. What matters when speech goes
			// quiet is the most recent failure, and it lands immediately below.
			_ = os.Truncate(path, 0)
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer file.Close()
		io.WriteString(file, record)
	}
}
