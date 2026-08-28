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
// roughly six seconds of speech and about two hundredths of a cent.
const instructionTemplate = `## Spoken output is ON for this session

Speak your work aloud by running this command:

    %s "<text>"

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
func runHook(stdin io.Reader, stdout io.Writer, store Store, env func(string) string, wrapperPath string) int {
	session := sessionFromPayload(stdin)
	if session == "" {
		session = env("CLAUDE_CODE_SESSION_ID")
	}
	if session == "" || !store.Enabled(session) {
		return 0
	}
	fmt.Fprintf(stdout, instructionTemplate, wrapperPath)
	return 0
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
