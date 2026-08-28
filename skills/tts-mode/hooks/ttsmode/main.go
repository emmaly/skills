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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
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
	session, rest := takeSessionFlag(rest, env)
	store := Store{Dir: stateDir(env)}
	logf := logger(store.Dir)

	switch command {
	case "hook":
		return runHook(stdin, stdout, store, env, wrapperPath(env))

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
		if len(rest) == 0 {
			fmt.Fprintln(stderr, `usage: ttsmode say "<text>"`)
			return 2
		}
		key, err := apiKey(env, envFilePath(env))
		if err != nil {
			logf("no api key: %v", err)
			return 0
		}
		return runSay(rest[0], ElevenLabs{Key: key}, CommandPlayer{}, logf)

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

	default:
		fmt.Fprintf(stderr, "ttsmode: unknown subcommand %q\n", command)
		return 2
	}
}

// takeSessionFlag pulls an optional --session out of the argument list and
// falls back to the environment.
func takeSessionFlag(args []string, env func(string) string) (string, []string) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--session" {
			trimmed := append([]string{}, args[:i]...)
			trimmed = append(trimmed, args[i+2:]...)
			return args[i+1], trimmed
		}
	}
	return env("CLAUDE_CODE_SESSION_ID"), args
}

func stateDir(env func(string) string) string {
	if dir := env("TTSMODE_STATE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(homeDir(env), ".claude", "tts-mode")
}

func envFilePath(env func(string) string) string {
	if path := env("TTSMODE_ENV_FILE"); path != "" {
		return path
	}
	return filepath.Join(homeDir(env), ".secrets", "elevenlabs.env")
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

func homeDir(env func(string) string) string {
	if home := env("HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// logger appends to a log beside the state. Speech failures are invisible by
// design, so there has to be somewhere to look when it goes quiet.
func logger(dir string) func(string, ...any) {
	return func(format string, args ...any) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return
		}
		file, err := os.OpenFile(filepath.Join(dir, "log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer file.Close()
		fmt.Fprintf(file, "%s %s\n", time.Now().UTC().Format(time.RFC3339), fmt.Sprintf(format, args...))
	}
}
