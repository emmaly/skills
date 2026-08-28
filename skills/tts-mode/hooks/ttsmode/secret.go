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

	file, err := os.Open(envFile)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", envFile, err)
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
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return name, value, true
}
