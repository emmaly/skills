package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// instructionTemplate is injected on every prompt while TTS is on. It carries
// one placeholder, the absolute path to the say wrapper.
//
// Re-injecting every turn rather than once per session is deliberate: a single
// instruction at session start falls out of attention as context grows, and a
// missed line is invisible to the user until they notice the silence.
//
// The word cap and the line cap bound both chatter and cost. Fifteen words is
// roughly six seconds of speech and about a hundred characters, and billing is
// per character.
const instructionTemplate = `## Spoken output is ON for this session

Speak your work aloud by running this command. Both the opening line and the
closing delimiter must start at column zero, with no indentation, or the shell
reads to end of input and swallows whatever you run next:

~~~
%s <<'%s'
<text>
%s
~~~

The quoted delimiter keeps the shell from reading your line as source, so it
is safe to include a filename, an error string, or anything else you were just
looking at. Do not switch to passing the line as a quoted argument.

When to speak:

- One line when you begin work you expect to take several tool calls.
- One line at the end of the turn, saying what happened.
- At most three spoken lines per turn.

How to write the line:

- Fifteen words or fewer. It is heard, not read.
- Never speak secrets, tokens, credentials, or full filesystem paths.
  Spoken output carries into a room that the terminal does not.
- Say what happened, not what you are about to narrate.

This is in addition to your normal written response, which is unchanged.
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
	fmt.Fprintf(stdout, instructionTemplate, shellQuote(wrapperPath), heredocDelimiter, heredocDelimiter)
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
