package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSpeakableSplitsOnSentences(t *testing.T) {
	text := strings.Repeat("This sentence is about forty characters long. ", 8)
	chunks := speakable(text, discardf)
	if len(chunks) < 2 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if utf8.RuneCountInString(c) > chunkChars {
			t.Fatalf("chunk over %d: %q", chunkChars, c)
		}
		if !strings.HasSuffix(c, ".") {
			t.Fatalf("chunk does not end on a sentence: %q", c)
		}
	}
	if joined := strings.Join(chunks, " "); joined != strings.TrimSpace(text) {
		t.Fatalf("chunks lost text:\n%q\n%q", joined, strings.TrimSpace(text))
	}
}

func TestSpeakableNeverCutsInsideAWord(t *testing.T) {
	// One long sentence with no terminator, so only word boundaries can split.
	text := strings.Repeat("alpha beta gamma delta ", 80)
	chunks := speakable(text, discardf)
	words := map[string]bool{"alpha": true, "beta": true, "gamma": true, "delta": true}
	for _, c := range chunks {
		for _, w := range strings.Fields(c) {
			if !words[w] {
				t.Fatalf("chunk boundary split a word: %q in %q", w, c)
			}
		}
	}
}

func TestSpeakableCapsAtWordBoundary(t *testing.T) {
	text := strings.Repeat("word ", maxSpokenChars)
	var logged string
	chunks := speakable(text, func(f string, a ...any) { logged = f })
	total := 0
	for _, c := range chunks {
		total += utf8.RuneCountInString(c) + 1
		for _, w := range strings.Fields(c) {
			if w != "word" {
				t.Fatalf("cap split a word: %q", w)
			}
		}
	}
	if total-1 > maxSpokenChars {
		t.Fatalf("sent %d characters, cap is %d", total-1, maxSpokenChars)
	}
	if !strings.Contains(logged, "cut to") {
		t.Fatal("cap applied without a log line")
	}
}

func TestSpeakableKeepsMultibyteRunesIntact(t *testing.T) {
	text := strings.Repeat("é", maxSpokenChars+50)
	for _, c := range speakable(text, discardf) {
		if !utf8.ValidString(c) {
			t.Fatalf("invalid UTF-8 in chunk: %q", c)
		}
	}
}

func TestSpeakableEmpty(t *testing.T) {
	if got := speakable("  \n\t ", discardf); got != nil {
		t.Fatalf("expected nil, got %q", got)
	}
}
