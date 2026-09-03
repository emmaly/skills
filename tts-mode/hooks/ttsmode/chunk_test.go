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

// A sentence over the target but under the ceiling is sent whole. Splitting
// it at a word is what a listener hears as the voice stopping mid-sentence.
func TestSpeakableKeepsALongSentenceWhole(t *testing.T) {
	sentence := strings.TrimSpace(strings.Repeat("word ", 70)) + "." // 350 chars
	if n := utf8.RuneCountInString(sentence); n <= chunkChars || n > maxChunkChars {
		t.Fatalf("test sentence is %d chars, want between %d and %d", n, chunkChars, maxChunkChars)
	}
	text := "Short one. " + sentence + " Short two."
	chunks := speakable(text, discardf)
	found := false
	for _, c := range chunks {
		if c == sentence {
			found = true
		}
	}
	if !found {
		t.Fatalf("long sentence was split: %q", chunks)
	}
}

// Past the ceiling a sentence is still split, at words, within the ceiling.
func TestSpeakableSplitsPastTheCeiling(t *testing.T) {
	sentence := strings.TrimSpace(strings.Repeat("word ", 120)) + "." // 600 chars
	chunks := speakable(sentence, discardf)
	if len(chunks) < 2 {
		t.Fatalf("expected a split, got %d chunk", len(chunks))
	}
	for _, c := range chunks {
		if utf8.RuneCountInString(c) > maxChunkChars {
			t.Fatalf("chunk over the ceiling: %d", utf8.RuneCountInString(c))
		}
		for _, w := range strings.Fields(c) {
			if w != "word" && w != "word." {
				t.Fatalf("split inside a word: %q", w)
			}
		}
	}
}
