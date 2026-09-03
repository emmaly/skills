package main

import (
	"fmt"
	"io"
	"strings"
)

// maxInstructionBytes bounds the freeform text. It is injected on every prompt
// while TTS is on, so an accidental paste would otherwise be paid for in every
// turn of the session.
const maxInstructionBytes = 2000

// rewriteMarker is how control asks its caller to normalize freeform text
// before it is stored.
//
// The parsing stays here rather than in the slash command because the model is
// the thing being guarded against: a typo like "of" has to be refused before
// anything treats it as a request, and a rewrite step would happily turn it
// into a plausible-looking instruction.
const rewriteMarker = "NEEDS_INSTRUCTION"

// currentMarker introduces the instructions already stored for this session,
// when there are any.
//
// set replaces wholesale, so a follow-up request phrased the natural way,
// "also mention which file", would otherwise drop everything set before it.
// Sending the current text along lets the rewrite step merge instead.
const currentMarker = "CURRENT_INSTRUCTIONS"

// keywords are the exact subcommands a person can type.
var keywords = []string{"on", "off", "status", "voice"}

// longestKeyword bounds how long a word can be and still read as a typo.
// Derived rather than written down, so adding a keyword cannot leave the cap
// too low to measure it.
var longestKeyword = func() int {
	longest := 0
	for _, keyword := range keywords {
		if len(keyword) > longest {
			longest = len(keyword)
		}
	}
	return longest
}()

// runControl handles the raw argument string a person typed after /tts.
//
// It reads stdin rather than an argument so the caller never has to build a
// shell command containing the text.
func runControl(stdin io.Reader, stdout, stderr io.Writer, store Store, session string) int {
	raw, err := io.ReadAll(io.LimitReader(stdin, maxInstructionBytes+1))
	if err != nil {
		fmt.Fprintf(stderr, "ttsmode: read request: %v\n", err)
		return 1
	}
	if len(raw) > maxInstructionBytes {
		fmt.Fprintf(stderr, "ttsmode: instructions are longer than %d bytes\n", maxInstructionBytes)
		return 1
	}

	text := strings.TrimSpace(string(raw))
	first, rest := splitFirstWord(text)

	switch strings.ToLower(first) {
	case "":
		return run([]string{"status"}, strings.NewReader(""), stdout, stderr, envFor(store, session))
	case "off":
		if rest != "" {
			fmt.Fprintln(stderr, "ttsmode: off takes no instructions")
			return 1
		}
		return run([]string{"off"}, strings.NewReader(""), stdout, stderr, envFor(store, session))
	case "status":
		if rest != "" {
			fmt.Fprintln(stderr, "ttsmode: status takes no instructions")
			return 1
		}
		return run([]string{"status"}, strings.NewReader(""), stdout, stderr, envFor(store, session))
	case "voice":
		// Exactly one word: an id, or "default". A sentence after it would be
		// a request, and mixing the two in one command is how an id ends up
		// stored as prose or prose stored as an id.
		if rest == "" || strings.ContainsAny(rest, " \t\n\r") {
			fmt.Fprintln(stderr, "ttsmode: voice takes one voice id, or the word default")
			return 1
		}
		return run([]string{"voice", rest}, strings.NewReader(""), stdout, stderr, envFor(store, session))
	case "on":
		if rest == "" {
			return run([]string{"on"}, strings.NewReader(""), stdout, stderr, envFor(store, session))
		}
		// Turn it on before handing the request off to be rewritten. The person
		// said "on" plainly, so that half of the request should survive a rewrite
		// step that never runs. Existing instructions are carried through, so
		// enabling here cannot wipe them.
		if err := store.Enable(session, store.Instructions(session)); err != nil {
			fmt.Fprintf(stderr, "ttsmode: %v\n", err)
			return 1
		}
		text = rest
	default:
		// A lone word that nearly spells a subcommand is a typo, not a
		// request. Turning TTS on with "of" as its instruction is the wrong
		// answer to someone trying to turn it off.
		if near := nearestKeyword(text); near != "" {
			fmt.Fprintf(stderr, "ttsmode: %q looks like a typo for %q; say what you mean, or add words to make it an instruction\n", text, near)
			return 1
		}
	}

	// Freeform. Hand it back to be turned into an instruction rather than
	// storing it raw, so a vague request becomes something the agent can
	// actually follow.
	fmt.Fprintf(stdout, "%s\n%s\n", rewriteMarker, text)
	if current := store.Instructions(session); current != "" {
		fmt.Fprintf(stdout, "\n%s\n%s\n", currentMarker, current)
	}
	return 0
}

// runSet stores an instruction that has already been normalized, and turns TTS
// on. Separate from control so the rewrite happens between the two.
func runSet(stdin io.Reader, stdout, stderr io.Writer, store Store, session string) int {
	raw, err := io.ReadAll(io.LimitReader(stdin, maxInstructionBytes+1))
	if err != nil {
		fmt.Fprintf(stderr, "ttsmode: read instructions: %v\n", err)
		return 1
	}
	if len(raw) > maxInstructionBytes {
		fmt.Fprintf(stderr, "ttsmode: instructions are longer than %d bytes\n", maxInstructionBytes)
		return 1
	}

	text := strings.TrimSpace(string(raw))
	if text == "" {
		fmt.Fprintln(stderr, "ttsmode: set needs an instruction on stdin")
		return 1
	}
	if err := store.Enable(session, text); err != nil {
		fmt.Fprintf(stderr, "ttsmode: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Spoken output is ON for this session.\n\nInstructions for this session:\n\n%s\n", indent(text))
	return 0
}

// splitFirstWord separates the leading word from the rest, so "on be terse"
// can be told from an instruction that merely starts with a keyword.
func splitFirstWord(text string) (string, string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ""
	}
	if index := strings.IndexFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}); index >= 0 {
		return trimmed[:index], strings.TrimSpace(trimmed[index:])
	}
	return trimmed, ""
}

// nearestKeyword reports which subcommand a single short word was probably
// meant to be, or empty when it reads as a real instruction.
//
// Only single words are considered, and only short ones. A phrase is a
// request even if it starts oddly, and "terse" is a plausible instruction
// while "statu" is not.
//
// The cap is one over the longest keyword so that a single insertion into the
// longest one is still measured. At a flat six it was not: "statuss" is seven
// characters, so every insertion typo of "status" was waved through as a
// request while deletions like "statu" were caught.
func nearestKeyword(text string) string {
	if strings.ContainsAny(text, " \t\n\r") || len(text) > longestKeyword+1 {
		return ""
	}
	word := strings.ToLower(text)
	for _, keyword := range keywords {
		if word == keyword {
			return ""
		}
	}
	// Abbreviations before edit distance. "of" is one character short of "off"
	// and one substitution from "on", and the person was reaching for off.
	//
	// Only the abbreviation direction. Also matching a word that merely starts
	// with a keyword refused real one-word instructions: "once", "only", "one",
	// and "offer" all begin with one and all mean what they say.
	for _, keyword := range keywords {
		if strings.HasPrefix(keyword, word) {
			return keyword
		}
	}
	for _, keyword := range keywords {
		if editDistanceWithin1(word, keyword) {
			return keyword
		}
	}
	return ""
}

// editDistanceWithin1 reports whether one insertion, deletion, or substitution
// turns a into b. Written directly rather than as a full distance matrix
// because one is the only answer that matters here.
func editDistanceWithin1(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		if len(a) == len(b) {
			return a[i+1:] == b[i+1:]
		}
		return a[i:] == b[i+1:]
	}
	return true
}

// indent offsets stored text so it reads as a quoted block rather than as more
// of the surrounding message.
func indent(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

// envFor rebuilds the environment the nested run needs, so control can reuse
// the existing subcommands rather than duplicating their output and error
// handling.
func envFor(store Store, session string) func(string) string {
	return func(name string) string {
		switch name {
		case "TTSMODE_STATE_DIR":
			return store.Dir
		case "CLAUDE_CODE_SESSION_ID":
			return session
		}
		return ""
	}
}
