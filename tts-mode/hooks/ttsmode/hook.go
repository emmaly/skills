package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// instructionTemplate is injected on every prompt while TTS is on. It carries
// the absolute path to the say wrapper, the heredoc delimiter twice, the
// absolute path to the control script, and the session's extra instructions.
//
// Re-injecting every turn rather than once per session is deliberate: a single
// instruction at session start falls out of attention as context grows, and a
// missed line is invisible to the user until they notice the silence.
//
// The length guidance bounds both chatter and cost. A closing summary of
// forty to eighty words is about five hundred characters, and billing is per
// character. The code caps one say at maxSpokenChars regardless.
const instructionTemplate = `## Spoken output is ON for this session

The person has their eyes closed. They can still type, and they can hear you.
Your written response is unchanged: write everything you normally would, at
the length you normally would. Speech is a second channel that keeps them
oriented without the screen. It is a summary of the screen, never a reading
of it.

Speak by running this command. Both the opening line and the closing
delimiter must start at column zero, with no indentation, or the shell reads
to end of input and swallows whatever you run next:

~~~
%s <<'%s'
<text>
%s
~~~

The quoted delimiter keeps the shell from reading your line as source, so it
is safe to include a filename, an error string, or anything else you were just
looking at. Do not switch to passing the line as a quoted argument.

When to speak:

- Once at the end of every turn, whatever the turn was: work, an answer, or a
  question back to them.
- Once when you begin work you expect to take several tool calls, saying what
  you are about to do.
- Once at each real checkpoint inside longer work: a build finished, a test
  failed, a decision you made on the way. Not once per tool call.

What to say:

- The closing summary is what a listener needs to know where things stand:
  what happened, what changed, what you found, and whether anything needs
  them. Two to four sentences, forty to eighty words. Long enough to
  understand the state of things, short enough that it never feels like a
  reading.
- A progress line is one sentence, up to about twenty words.
- If the turn answered a question, speak the answer, not the fact that you
  answered.
- Summarize, do not transcribe. Never read the written response aloud.
- Say things the way you would across a room. "The config loader" rather
  than its path, "the second test" rather than its function name, "one of
  the session files" rather than its filename.
- Never speak secrets, tokens, keys, credentials, or full filesystem paths.
  Speech goes to an outside service and into a room; the terminal does not.
- Never speak identifiers that are noise when heard: UUIDs, hashes, commit
  ids, file and voice ids, long URLs, port or process numbers. Describe what
  the thing is instead.

If they ask you to use a different voice for this session, run this once and
tell them it is set. It applies from the next spoken line. The word default
restores the global voice:

~~~
%s voice <voice-id>
~~~
%s`

// extraTemplate carries the session's own instructions. It says plainly that
// they win, because the guidance above names specific numbers and a request
// like "one short line a turn" is unfollowable next to a forty-word summary
// rule that does not yield.
//
// The precedence is scoped to style and length on purpose. Granting it over
// "the rules above" also handed it the rule against speaking secrets and full
// paths, and a request as ordinary as "say which file you touched" is then
// enough to license reading a path aloud in a room.
const extraTemplate = `
## Instructions for this session

These were set for this session. They take precedence over the style and
length guidance above, including the sentence and word counts:

%s

They do not override the rule against speaking secrets, tokens, credentials,
full filesystem paths, or identifiers that are noise when heard. Those hold
however these are worded.
`

// hookPayload is the slice of the hook event this cares about. Everything else
// in the payload is ignored on purpose, so an added field upstream cannot
// break parsing.
type hookPayload struct {
	SessionID string `json:"session_id"`
}

// runHook decides whether to ask for spoken summaries, and returns the exit
// code. It always returns 0. A hook that can fail a turn is worse than a hook
// that silently does nothing.
//
// Session resolution order is the documented one: the --session flag passed as
// override, then the session_id in the payload, then the environment.
func runHook(stdin io.Reader, stdout io.Writer, store Store, env func(string) string, override, wrapperPath string) int {
	session := override
	if session == "" {
		session = sessionFromPayload(stdin)
	}
	if session == "" {
		session = env("CLAUDE_CODE_SESSION_ID")
	}
	if session == "" || !store.Enabled(session) {
		return 0
	}
	var extra string
	if text := store.Instructions(session); text != "" {
		extra = fmt.Sprintf(extraTemplate, text)
	}
	// The control script sits beside the say wrapper. Derived rather than
	// passed, so the two can never point at different plugin roots.
	controlPath := filepath.Join(filepath.Dir(wrapperPath), "run-ttsmode.sh")
	fmt.Fprintf(stdout, instructionTemplate, shellQuote(wrapperPath), heredocDelimiter, heredocDelimiter, shellQuote(controlPath), extra)
	return 0
}

// heredocDelimiter closes the heredoc. It only has to be a token that will not
// appear as a whole line of spoken prose.
//
// Drawing it fresh each turn was tried and reverted. There is nothing to
// guess: the delimiter is printed in the same instruction the model reads, and
// the model is the only party that authors the spoken line, so randomness buys
// nothing against a line written to match it. What it cost was reliability,
// since the model then had to reproduce twelve random hex characters exactly,
// twice, with earlier turns' tokens still in the transcript. One mis-copied
// character reproduces the indented-terminator bug: the heredoc runs to end of
// input and silently swallows whatever runs next.
const heredocDelimiter = "TTS_LINE_9f3c1a"

// shellQuote wraps a path so the shell reads it as one argument.
//
// The plugin root is not ours to choose, and an unquoted path containing a
// space would have the shell treat the first segment as the command. TTS would
// be on and simply produce nothing.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// sessionFromPayload reads the session id, treating any parse failure as
// absent rather than fatal.
func sessionFromPayload(stdin io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		return ""
	}
	var payload hookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.SessionID)
}
