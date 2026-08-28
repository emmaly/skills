package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnvFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "elevenlabs.env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return path
}

// The environment wins, so a caller can override without editing the file.
func TestAPIKeyPrefersEnvironment(t *testing.T) {
	env := func(name string) string {
		if name == "ELEVENLABS_API_KEY" {
			return "from-env"
		}
		return ""
	}
	path := writeEnvFile(t, "ELEVENLABS_API_KEY=from-file\n")

	key, err := apiKey(env, path)
	if err != nil {
		t.Fatalf("apiKey: %v", err)
	}
	if key != "from-env" {
		t.Fatalf("got %q, want from-env", key)
	}
}

func TestAPIKeyParsesFile(t *testing.T) {
	cases := map[string]string{
		"plain":            "ELEVENLABS_API_KEY=abc123\n",
		"with comment":     "# a comment\n\nELEVENLABS_API_KEY=abc123\n",
		"double quoted":    "ELEVENLABS_API_KEY=\"abc123\"\n",
		"single quoted":    "ELEVENLABS_API_KEY='abc123'\n",
		"trailing spaces":  "ELEVENLABS_API_KEY=abc123   \n",
		"other keys first": "OTHER=1\nELEVENLABS_API_KEY=abc123\n",
		"export prefix":    "export ELEVENLABS_API_KEY=abc123\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			key, err := apiKey(noEnv, writeEnvFile(t, body))
			if err != nil {
				t.Fatalf("apiKey: %v", err)
			}
			if key != "abc123" {
				t.Fatalf("got %q, want abc123", key)
			}
		})
	}
}

func TestAPIKeyMissingFile(t *testing.T) {
	if _, err := apiKey(noEnv, filepath.Join(t.TempDir(), "absent.env")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestAPIKeyAbsentFromFile(t *testing.T) {
	if _, err := apiKey(noEnv, writeEnvFile(t, "SOMETHING_ELSE=1\n")); err == nil {
		t.Fatal("expected an error when the key is not in the file")
	}
}

// The placeholder shipped in the file template must not read as a real key.
func TestAPIKeyRejectsPlaceholder(t *testing.T) {
	if _, err := apiKey(noEnv, writeEnvFile(t, "ELEVENLABS_API_KEY=replace-me\n")); err == nil {
		t.Fatal("expected an error for the placeholder value")
	}
}

// A key in the environment that is still the placeholder must fall through to
// the file rather than being used.
func TestAPIKeyPlaceholderInEnvFallsThrough(t *testing.T) {
	env := func(name string) string {
		if name == "ELEVENLABS_API_KEY" {
			return "replace-me"
		}
		return ""
	}
	key, err := apiKey(env, writeEnvFile(t, "ELEVENLABS_API_KEY=real-key\n"))
	if err != nil {
		t.Fatalf("apiKey: %v", err)
	}
	if key != "real-key" {
		t.Fatalf("got %q, want real-key", key)
	}
}

// A trailing comment on an unquoted value became part of the header and came
// back as a 401, which looks exactly like a wrong key.
func TestAPIKeyStripsInlineComment(t *testing.T) {
	key, err := apiKey(noEnv, writeEnvFile(t, "ELEVENLABS_API_KEY=abc123  # main key\n"))
	if err != nil {
		t.Fatalf("apiKey: %v", err)
	}
	if key != "abc123" {
		t.Fatalf("got %q, want abc123", key)
	}
}

// Inside quotes a hash is data, not a comment.
func TestAPIKeyKeepsHashInsideQuotes(t *testing.T) {
	key, err := apiKey(noEnv, writeEnvFile(t, "ELEVENLABS_API_KEY=\"abc#123\"\n"))
	if err != nil {
		t.Fatalf("apiKey: %v", err)
	}
	if key != "abc#123" {
		t.Fatalf("got %q, want abc#123", key)
	}
}
