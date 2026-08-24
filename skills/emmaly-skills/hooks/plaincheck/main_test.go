package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A hook that fires on every Write and every Bash call has two failure modes
// that cost more than the rule is worth: a false positive blocks unrelated
// work, and a crash on a malformed payload wedges the session. Both are covered
// here, and the false-positive cases outnumber the catches on purpose.
//
// The dash characters and the commit verb are built from pieces so this file
// does not trip the very check it tests.

const (
	em = "—"
	en = "–"
	gc = "git " + "commit"
)

// exitFor feeds a payload to run and reports the exit code.
func exitFor(t *testing.T, payload any) int {
	t.Helper()

	var body []byte
	switch value := payload.(type) {
	case string:
		body = []byte(value)
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = encoded
	}

	var stderr bytes.Buffer
	return run(bytes.NewReader(body), &stderr)
}

func writeInput(path, content string) map[string]any {
	return map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": path, "content": content},
	}
}

func bashInput(command string) map[string]any {
	return map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": command},
	}
}

func TestChecker(t *testing.T) {
	cases := []struct {
		name    string
		want    int
		payload any
	}{
		{"prose with a dash", 2, writeInput("a.md", "A line "+em+" with a dash.")},
		{"clean prose", 0, writeInput("a.md", "This is clean. No dash.")},
		{"dash inside backticks is quoted, not written", 0,
			writeInput("a.md", "Ban the `"+em+"` character.")},
		{"dash inside a fenced block", 0,
			writeInput("a.md", "text\n```\nfoo "+em+" bar\n```\nmore")},
		{"source file is out of scope", 0, writeInput("a.go", "// x "+em+" y")},
		{"en dash counts too", 2, writeInput("a.md", "lines 25"+en+"875")},

		{"Edit introducing a dash", 2, map[string]any{
			"tool_name": "Edit",
			"tool_input": map[string]any{
				"file_path": "a.md", "old_string": "x", "new_string": "a " + em + " b",
			},
		}},
		{"Edit removing a dash is allowed", 0, map[string]any{
			"tool_name": "Edit",
			"tool_input": map[string]any{
				"file_path": "a.md", "old_string": "a " + em + " b", "new_string": "a. b",
			},
		}},
		{"MultiEdit checks every edit", 2, map[string]any{
			"tool_name": "MultiEdit",
			"tool_input": map[string]any{"file_path": "a.md", "edits": []any{
				map[string]any{"new_string": "ok"},
				map[string]any{"new_string": "bad " + em + " here"},
			}},
		}},

		// Fence shapes. Only an opening fence may carry an info string, and a
		// fence closes only on its own character at its own length or longer.
		{"longer fence survives a shorter inner marker", 0,
			writeInput("a.md", "intro\n````\n```\nfoo "+em+" bar\n```\n````\nafter")},
		{"prose after a longer fence closes is still checked", 2,
			writeInput("a.md", "````\n```\ncode\n```\n````\nprose "+em+" here")},
		{"tilde fence is not closed by backticks", 0,
			writeInput("a.md", "~~~\n```\nfoo "+em+" bar\n~~~")},
		{"equal-length marker with an info string does not close a block", 0,
			writeInput("a.md", "````\n````python\nfoo "+em+" bar\n````")},
		{"opening fence may still carry an info string", 2,
			writeInput("a.md", "```python\ncode\n```\nprose "+em+" here")},

		// Commit messages.
		{"commit message with a dash", 2, bashInput(gc + ` -m "fix: thing ` + em + ` other"`)},
		{"clean commit message", 0, bashInput(gc + ` -m "fix: thing"`)},
		{"searching for a dash is not writing one", 0, bashInput(`grep -rn "` + em + `" .`)},
		{"heredoc body is scanned, not just -m", 2,
			bashInput(gc + " -F - <<EOF\nfix: thing\n\nbody " + em + " here\nEOF")},
		{"global options before the subcommand", 2,
			bashInput(`git -C /repo commit -m "fix: a ` + em + ` b"`)},
		{"global options, clean message", 0,
			bashInput(`git -C /repo commit -m "fix: a. b"`)},
		{"--namespace= before the subcommand", 2,
			bashInput(`git --namespace=review commit -m "a ` + em + ` b"`)},
		{"--namespace with a separate value", 2,
			bashInput(`git --namespace review commit -m "a ` + em + ` b"`)},
		{"commit later in a compound command", 2,
			bashInput(`git status && ` + gc + ` -m "a ` + em + ` b"`)},
		{"compound command, clean message", 0,
			bashInput(`git status && ` + gc + ` -m "a. b"`)},
		{"an absolute path to git still counts", 2,
			bashInput(`/usr/bin/git commit -m "a ` + em + ` b"`)},
		{"a different git subcommand is not a commit", 0,
			bashInput(`git log --format=%B | grep "` + em + `"`)},

		// Malformed payloads must exit 0 rather than raise.
		{"malformed payload does not wedge the session", 0, "not json"},
		{"valid JSON of the wrong shape", 0, "5"},
		{"JSON array instead of an object", 0, "[1, 2]"},
		{"missing tool_input", 0, map[string]any{"tool_name": "Write"}},
		{"unrelated tool", 0, map[string]any{
			"tool_name":  "Read",
			"tool_input": map[string]any{"file_path": "a.md"},
		}},
		{"MultiEdit edits is not a list", 0, map[string]any{
			"tool_name":  "MultiEdit",
			"tool_input": map[string]any{"file_path": "a.md", "edits": "oops"},
		}},
		{"MultiEdit edits holds a non-dict", 0, map[string]any{
			"tool_name":  "MultiEdit",
			"tool_input": map[string]any{"file_path": "a.md", "edits": []any{"oops"}},
		}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := exitFor(t, test.payload); got != test.want {
				t.Errorf("want exit %d, got %d", test.want, got)
			}
		})
	}
}

// TestMessageFiles covers the -F path, where the message is not in the command
// at all and the file has to be read.
func TestMessageFiles(t *testing.T) {
	dir := t.TempDir()
	dirty := filepath.Join(dir, "dirty.msg")
	clean := filepath.Join(dir, "clean.msg")
	spaced := filepath.Join(dir, "my message.msg")

	for _, name := range []string{dirty, spaced} {
		if err := os.WriteFile(name, []byte("fix: thing\n\nbody "+em+" here\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(clean, []byte("fix: thing\n\nbody here\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", clean, err)
	}

	cases := []struct {
		name    string
		want    int
		command string
	}{
		{"message file holding a dash", 2, gc + " -F " + dirty},
		{"clean message file", 0, gc + " -F " + clean},
		{"--file= form", 2, gc + " --file=" + dirty},
		{"-Fpath attached form", 2, gc + " -F" + dirty},
		{"quoted path containing a space", 2, gc + ` -F "` + spaced + `"`},
		{"--file= with a quoted path containing a space", 2, gc + ` --file="` + spaced + `"`},
		{"missing message file is not an error", 0, gc + " -F " + filepath.Join(dir, "nope.msg")},
		{"-F - is the heredoc form, not a path", 0, gc + " -F - <<EOF\nfix: thing\nEOF"},
		// An unterminated quote makes the tokeniser give up, which is what the
		// regex fallback exists for.
		{"unparseable command still finds the message file", 2,
			gc + " -F " + dirty + ` && echo "unclosed`},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := exitFor(t, bashInput(test.command)); got != test.want {
				t.Errorf("want exit %d, got %d", test.want, got)
			}
		})
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{name: "plain", in: "git commit -m x", want: []string{"git", "commit", "-m", "x"}},
		{name: "double quotes hold a space", in: `git -F "my file"`,
			want: []string{"git", "-F", "my file"}},
		{name: "single quotes are literal", in: `echo 'a "b" c'`,
			want: []string{"echo", `a "b" c`}},
		{name: "escaped space", in: `git -F my\ file`, want: []string{"git", "-F", "my file"}},
		{name: "empty quotes make an empty token", in: `a "" b`, want: []string{"a", "", "b"}},
		{name: "unterminated quote", in: `echo "oops`, wantErr: true},
		{name: "trailing backslash", in: `echo x\`, wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := splitCommand(test.in)
			if test.wantErr {
				if err == nil {
					t.Fatalf("want an error, got tokens %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(test.want, "\x00") {
				t.Errorf("want %q, got %q", test.want, got)
			}
		})
	}
}
