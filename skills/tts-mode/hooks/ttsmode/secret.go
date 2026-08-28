package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const keyName = "ELEVENLABS_API_KEY"

// placeholder is what the env file template ships with. Treating it as absent
// gives a clear error instead of a confusing 401 from the API.
const placeholder = "replace-me"

// apiKey resolves the ElevenLabs key, preferring the environment and falling
// back to parsing an env file.
//
// The file is parsed here rather than shelling out to envwith because this
// runs from a hook, unattended, where nothing wraps the process.
func apiKey(env func(string) string, envFile string) (string, error) {
	if value := strings.TrimSpace(env(keyName)); value != "" && value != placeholder {
		return value, nil
	}

	// No path prefix here: the *fs.PathError already renders as
	// "open <path>: ...", and wrapping it again produced the path twice in
	// every log line.
	file, err := os.Open(envFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, value, ok := splitEnvLine(scanner.Text())
		if !ok || name != keyName {
			continue
		}
		if value == "" || value == placeholder {
			return "", fmt.Errorf("%s in %s is still the placeholder", keyName, envFile)
		}
		return value, nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read %s: %w", envFile, err)
	}
	return "", errors.New(keyName + " not found in " + envFile)
}

// splitEnvLine parses one NAME=VALUE line, tolerating comments, blank lines, a
// leading export, and surrounding quotes.
func splitEnvLine(line string) (name, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")

	name, value, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)

	if quoted, ok := unquote(value); ok {
		return name, quoted, true
	}

	// Unquoted only: strip a trailing comment. Without this, a line like
	// `KEY=sk_abc  # main key` sent the comment as part of the header and the
	// API answered 401, which logs identically to a genuinely wrong key.
	// Inside quotes a hash is data, which is why this runs after unquote.
	if hash := strings.Index(value, "#"); hash >= 0 {
		value = strings.TrimSpace(value[:hash])
	}
	return name, value, true
}

// unquote removes one matching pair of surrounding quotes, reporting whether
// the value was quoted at all.
func unquote(value string) (string, bool) {
	if len(value) < 2 {
		return value, false
	}
	first, last := value[0], value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1], true
	}
	return value, false
}
