package main

import (
	"bytes"
	"strings"
	"testing"
)

// control runs runControl for one typed argument with an empty environment
// and returns the exit code, stdout, and stderr.
func control(t *testing.T, dir, session, raw string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runControl(strings.NewReader(raw), &out, &errOut, Store{Dir: dir}, session, noEnv)
	return code, out.String(), errOut.String()
}

// on, off, status, and an empty argument route to their subcommands.
func TestControlPlainSubcommands(t *testing.T) {
	// One directory for the whole table: an "on" in one row has to still be
	// visible to the "status" in the next.
	dir := t.TempDir()
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"", "OFF"},
		{"on", "ON"},
		{"status", "ON"},
		{"off", "OFF"},
		{"status", "OFF"},
	} {
		code, out, errOut := control(t, dir, "s1", tc.raw)
		if code != 0 {
			t.Fatalf("%q: exit %d: %s", tc.raw, code, errOut)
		}
		if !strings.Contains(out, tc.want) {
			t.Fatalf("%q: got %q, want %s", tc.raw, out, tc.want)
		}
	}
}

// Freeform text is handed back for rewriting rather than stored raw.
func TestControlAsksForARewrite(t *testing.T) {
	for _, raw := range []string{
		"keep it to five words",
		"on keep it to five words",
	} {
		code, out, errOut := control(t, t.TempDir(), "s1", raw)
		if code != 0 {
			t.Fatalf("%q: exit %d: %s", raw, code, errOut)
		}
		if !strings.HasPrefix(out, rewriteMarker+"\n") {
			t.Fatalf("%q: got %q", raw, out)
		}
		if !strings.Contains(out, "keep it to five words") {
			t.Fatalf("%q: request was not passed through: %q", raw, out)
		}
		if strings.Contains(out, "on keep") {
			t.Fatalf("%q: the on prefix was kept: %q", raw, out)
		}
	}
}

// Turning TTS on with "of" as its instruction is the wrong answer to someone
// trying to turn it off.
func TestControlRejectsTypos(t *testing.T) {
	for raw, want := range map[string]string{
		"of":     "off",
		"ofc":    "off",
		"statu":  "status",
		"statuz": "status",
		"o":      "on",
		"onn":    "on",
	} {
		code, _, errOut := control(t, t.TempDir(), "s1", raw)
		if code == 0 {
			t.Fatalf("%q was accepted", raw)
		}
		if !strings.Contains(errOut, want) {
			t.Fatalf("%q: message does not suggest %q: %s", raw, want, errOut)
		}
	}
}

// A phrase is a request even when it starts oddly, and a real word is an
// instruction rather than a misspelling.
func TestControlKeepsRealInstructions(t *testing.T) {
	for _, raw := range []string{"be terse", "shorter", "one line only"} {
		code, out, errOut := control(t, t.TempDir(), "s1", raw)
		if code != 0 {
			t.Fatalf("%q was rejected: %s", raw, errOut)
		}
		if !strings.HasPrefix(out, rewriteMarker) {
			t.Fatalf("%q: got %q", raw, out)
		}
	}
}

// off and status take no trailing text.
func TestControlRejectsTrailingTextOnOffAndStatus(t *testing.T) {
	for _, raw := range []string{"off now please", "status of things"} {
		code, _, errOut := control(t, t.TempDir(), "s1", raw)
		if code == 0 {
			t.Fatalf("%q was accepted", raw)
		}
		if errOut == "" {
			t.Fatalf("%q: rejected with no message", raw)
		}
	}
}

// A request over maxInstructionBytes is refused rather than stored.
func TestControlRejectsOverlongInstructions(t *testing.T) {
	code, _, errOut := control(t, t.TempDir(), "s1", strings.Repeat("x", maxInstructionBytes+1))
	if code == 0 {
		t.Fatal("an overlong request was accepted")
	}
	if !strings.Contains(errOut, "longer than") {
		t.Fatalf("unhelpful message: %s", errOut)
	}
}

// set stores the instructions and turns the session on.
func TestSetStoresAndEnables(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := runSet(strings.NewReader("- Five words a line.\n- No file paths.\n"), &out, &errOut, Store{Dir: dir}, "s1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	store := Store{Dir: dir}
	if !store.Enabled("s1") {
		t.Fatal("set did not turn TTS on")
	}
	if got := store.Instructions("s1"); !strings.Contains(got, "Five words a line.") {
		t.Fatalf("stored %q", got)
	}
	// The user has to be able to see what their request became.
	if !strings.Contains(out.String(), "Five words a line.") {
		t.Fatalf("output does not show the stored instruction: %q", out.String())
	}
}

// set with nothing on stdin is an error, not a way to clear instructions.
func TestSetRejectsEmpty(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runSet(strings.NewReader("  \n "), &out, &errOut, Store{Dir: t.TempDir()}, "s1"); code == 0 {
		t.Fatal("an empty instruction was accepted")
	}
}
