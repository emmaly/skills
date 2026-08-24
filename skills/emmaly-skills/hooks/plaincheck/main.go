// Command plaincheck is the mechanical gate for the one plain-language rule a
// machine can judge.
//
// The plain-language skill is otherwise instruction-only, so adherence decays
// over a long session. Em and en dashes are the exception: they are
// unambiguous, so a hook can catch them without guessing. Word-list and
// sentence-shape rules stay with the model, because a checker for those would
// fire on the ban list itself, on quoted tool output, and on every legitimate
// term of art.
//
// Two events, both registered in hooks.json:
//
//	PostToolUse  Write/Edit/MultiEdit on a prose file. Checks only the text
//	             being written, never the rest of the file, so editing a
//	             document that predates the skill does not block unrelated
//	             work.
//	PreToolUse   Bash running `git commit`. Blocks before the commit exists,
//	             which is the only cheap moment: rewording afterward means an
//	             amend or a rebase.
//
// Escape hatch, prose files only: text inside backticks or a fenced code block
// is skipped, so a document that quotes a dash to talk about one still passes.
// The commit branch has no such hatch, since it scans the raw command and any
// file given to -F.
//
// Two known limits. The prose scan is line-based, so an inline backtick span
// wrapped across two lines is not recognised as code and its dash is still
// flagged; keep such a span on one line, or fence it. And a message built from
// a shell variable, or piped in from another process, cannot be resolved here.
//
// Exit 2 is the code that returns stderr to Claude. Every other outcome exits
// 0, so a malformed payload can never wedge a session.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	emDash = '—'
	enDash = '–'
)

// proseSuffixes are the files whose text this checks. Source files are out of
// scope: a dash in a string literal or a test fixture is not prose.
var proseSuffixes = []string{".md", ".mdx", ".markdown", ".txt", ".rst"}

var (
	fenceRe      = regexp.MustCompile("^[ \t]*(`{3,}|~{3,})")
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")

	// Fallback for a command that cannot be tokenised. Any run of global
	// options is allowed between `git` and `commit` rather than a fixed list,
	// since enumerating them turned into whack-a-mole.
	gitCommitRe = regexp.MustCompile(`\bgit\b(?:\s+-{1,2}[^\s]*(?:\s+[^\s-][^\s]*)?)*\s+commit\b`)

	// -F path, --file path, --file=path, for the same fallback.
	messageFileRe = regexp.MustCompile(`(?:^|\s)(?:-F|--file)(?:=(\S+)|\s+(\S+))`)
)

// gitOptsWithValue are global options taking their value as a separate token,
// so the value is not mistaken for the subcommand.
var gitOptsWithValue = map[string]bool{
	"-C":           true,
	"-c":           true,
	"--git-dir":    true,
	"--work-tree":  true,
	"--namespace":  true,
	"--exec-path":  true,
	"--config-env": true,
}

// hasDash reports whether the text holds an em or en dash.
func hasDash(s string) bool {
	return strings.ContainsRune(s, emDash) || strings.ContainsRune(s, enDash)
}

// offender is one line of prose carrying a dash, with its line number.
type offender struct {
	Line int
	Text string
}

// stripCode blanks out fenced blocks and inline code, keeping line numbers
// intact.
//
// A fence closes only on the same character, at the same length or longer,
// carrying nothing after the marker: only an opening fence may have an info
// string. Getting this wrong flips the parser inside out and reports every
// later line as prose, or none of them.
func stripCode(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))

	var fenceChar byte
	var fenceLen int
	open := false

	for _, line := range lines {
		if m := fenceRe.FindStringSubmatchIndex(line); m != nil {
			marker := line[m[2]:m[3]]
			char, length := marker[0], len(marker)
			if !open {
				open, fenceChar, fenceLen = true, char, length
				out = append(out, "")
				continue
			}
			closes := char == fenceChar &&
				length >= fenceLen &&
				strings.TrimSpace(line[m[3]:]) == ""
			if closes {
				open = false
				out = append(out, "")
				continue
			}
			// A shorter or different marker inside a block is content, and so
			// is one carrying an info string.
		}
		if open {
			out = append(out, "")
			continue
		}
		out = append(out, inlineCodeRe.ReplaceAllString(line, ""))
	}
	return out
}

// offenders returns each line holding a dash outside code.
func offenders(text string) []offender {
	var found []offender
	for i, line := range stripCode(text) {
		if hasDash(line) {
			found = append(found, offender{Line: i + 1, Text: strings.TrimSpace(line)})
		}
	}
	return found
}

// splitCommand tokenises a command the way a shell would, so a quoted path
// with a space in it stays one token. It reports an error on an unterminated
// quote, which is the signal to fall back to pattern matching.
func splitCommand(s string) ([]string, error) {
	var (
		tokens   []string
		cur      strings.Builder
		hasToken bool
		quote    rune
	)
	runes := []rune(s)

	flush := func() {
		if hasToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			hasToken = false
		}
	}

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			} else {
				cur.WriteRune(c)
			}
		case quote == '"':
			if c == '\\' && i+1 < len(runes) {
				next := runes[i+1]
				if next == '"' || next == '\\' || next == '$' || next == '`' || next == '\n' {
					i++
					if next != '\n' {
						cur.WriteRune(next)
					}
					continue
				}
				cur.WriteRune(c)
				continue
			}
			if c == '"' {
				quote = 0
			} else {
				cur.WriteRune(c)
			}
		case c == '\\':
			if i+1 >= len(runes) {
				return nil, fmt.Errorf("trailing backslash")
			}
			i++
			if runes[i] != '\n' {
				cur.WriteRune(runes[i])
				hasToken = true
			}
		case c == '\'' || c == '"':
			quote = c
			hasToken = true
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		default:
			cur.WriteRune(c)
			hasToken = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return tokens, nil
}

// isGitCommit reports whether the command runs `git commit`, past any global
// options.
//
// Tokenised rather than pattern matched, so `git --namespace=x -C /repo commit`
// is recognised without listing every option git accepts. A compound command is
// scanned to the end, so the commit in `git status && git commit` still counts.
func isGitCommit(command string) bool {
	tokens, err := splitCommand(command)
	if err != nil {
		return gitCommitRe.MatchString(command)
	}
	for i, token := range tokens {
		if path.Base(token) != "git" {
			continue
		}
		if subcommandAt(tokens, i+1) == "commit" {
			return true
		}
	}
	return false
}

// subcommandAt skips global options from index onward and returns the first
// token that is not one, or "" if the command ends first.
func subcommandAt(tokens []string, index int) string {
	for index < len(tokens) {
		token := tokens[index]
		switch {
		case gitOptsWithValue[token]:
			index += 2
		case strings.HasPrefix(token, "-"):
			index++
		default:
			return token
		}
	}
	return ""
}

// messageFiles returns paths given to -F, --file, --file=, or -Fpath, which
// hold the message text.
//
// "-" means stdin, which is the heredoc case, and its text is already in the
// command.
func messageFiles(command string) []string {
	tokens, err := splitCommand(command)
	if err != nil {
		var paths []string
		for _, m := range messageFileRe.FindAllStringSubmatch(command, -1) {
			raw := strings.Trim(m[1]+m[2], `"'`)
			if raw != "" && raw != "-" {
				paths = append(paths, raw)
			}
		}
		return paths
	}

	var paths []string
	add := func(p string) {
		if p != "" && p != "-" {
			paths = append(paths, p)
		}
	}

	expecting := false
	for _, token := range tokens {
		switch {
		case expecting:
			expecting = false
			add(token)
		case token == "-F", token == "--file":
			expecting = true
		case strings.HasPrefix(token, "--file="):
			add(strings.TrimPrefix(token, "--file="))
		case strings.HasPrefix(token, "-F") && len(token) > 2:
			add(token[2:])
		}
	}
	return paths
}

// readText returns the contents of a message file, or an empty string if it
// cannot be read. Guessing wrong must not fail the commit.
func readText(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	return string(data)
}

// asString mirrors the payload loosely, since a field can arrive as any JSON
// type and a hook that panics on one fails the tool call it was inspecting.
func asString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

// writtenText returns just the text this call authors, not the whole file.
func writtenText(toolInput map[string]any) string {
	if content, ok := toolInput["content"]; ok { // Write
		return asString(content)
	}
	if edits, ok := toolInput["edits"]; ok { // MultiEdit
		list, ok := edits.([]any)
		if !ok {
			return ""
		}
		var parts []string
		for _, element := range list {
			edit, ok := element.(map[string]any)
			if !ok {
				continue
			}
			parts = append(parts, asString(edit["new_string"]))
		}
		return strings.Join(parts, "\n")
	}
	return asString(toolInput["new_string"]) // Edit
}

// isProseFile reports whether the path is one whose text this checks.
func isProseFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range proseSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// run returns the exit code: 2 blocks and returns the message to Claude, 0
// allows.
func run(stdin io.Reader, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}

	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return 0
	}
	payload, ok := decoded.(map[string]any)
	if !ok {
		return 0 // Valid JSON, wrong shape. A bare number parses fine.
	}

	toolName, _ := payload["tool_name"].(string)
	toolInput, ok := payload["tool_input"].(map[string]any)
	if !ok {
		return 0
	}

	block := func(format string, args ...any) int {
		fmt.Fprintf(stderr, format+"\n", args...)
		return 2
	}

	switch toolName {
	case "Bash":
		command := asString(toolInput["command"])
		if !isGitCommit(command) {
			return 0
		}
		// The whole command is scanned, not just a -m value. A message also
		// arrives through a heredoc, which is how the long messages in this
		// repo are written, so reading only -m would miss the common path. The
		// cost is a false positive when an unrelated part of a compound command
		// holds a dash, and the fix there is to run the commit on its own.
		if hasDash(command) {
			return block("plain-language: this commit message contains an em or en dash. " +
				"Use a period or a comma, then run the command again.")
		}
		// A message passed as a file is not in the command text at all, so read
		// it.
		for _, name := range messageFiles(command) {
			if hasDash(readText(name)) {
				return block("plain-language: the commit message in %s contains an em or "+
					"en dash. Use a period or a comma, then run the command again.", name)
			}
		}

	case "Write", "Edit", "MultiEdit":
		name := asString(toolInput["file_path"])
		if !isProseFile(name) {
			return 0
		}
		found := offenders(writtenText(toolInput))
		if len(found) == 0 {
			return 0
		}
		var lines []string
		for i, o := range found {
			if i == 5 {
				lines = append(lines, fmt.Sprintf("  and %d more", len(found)-5))
				break
			}
			lines = append(lines, fmt.Sprintf("  %d: %s", o.Line, o.Text))
		}
		return block("plain-language: em or en dash in text just written to %s. "+
			"Replace each with a period or a comma, then edit the file again.\n%s",
			name, strings.Join(lines, "\n"))
	}

	return 0
}

// main wires stdin and stderr to run and returns its exit code.
func main() {
	os.Exit(run(os.Stdin, os.Stderr))
}
