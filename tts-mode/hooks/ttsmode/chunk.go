package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSpokenChars bounds one say call. The line guidance lives in the injected
// instruction, which is guidance a model can ignore and which a session's own
// instructions may raise; billing is per character, so a pasted stack trace
// would be synthesized and charged in full. This makes the README's cost
// estimate a property of the code instead. Twelve hundred characters is
// about five long spoken lines, the most a session has asked for.
const maxSpokenChars = 1200

// chunkChars is the target size of a piece sent to the API. Short pieces mean
// the first one is playing while the rest are still being synthesized, and a
// failure loses one sentence rather than the whole turn.
const chunkChars = 220

// maxChunkChars is the ceiling. A sentence longer than the target but within
// this is sent whole rather than split at a word, because a clip boundary in
// the middle of a sentence is heard as the voice stopping at a random word.
// Only a sentence longer than this is split at word boundaries.
const maxChunkChars = 2 * chunkChars

// speakable trims, caps, and splits text into pieces that each end on a
// sentence or at worst a word. The old fixed-offset truncation ended lines in
// the middle of a word, which is what a listener hears as the voice being cut
// off. Nothing here splits inside a word or inside a multi-byte character.
//
// logf is told when the cap was applied, because a silent cut is a line that
// vanished with nothing in the log.
func speakable(text string, logf func(string, ...any)) []string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil
	}
	if n := utf8.RuneCountInString(text); n > maxSpokenChars {
		cut := cutAtWord(text, maxSpokenChars)
		logf("text was %d characters, cut to %d at a word boundary", n, utf8.RuneCountInString(cut))
		text = cut
	}

	var chunks []string
	var current strings.Builder
	currentLen := 0
	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
		currentLen = 0
	}
	// Sentences pack together up to the target. One sentence over the target
	// but under the ceiling becomes a piece on its own.
	for _, sentence := range sentences(text) {
		for _, piece := range splitLong(sentence, maxChunkChars) {
			n := utf8.RuneCountInString(piece)
			if currentLen > 0 && currentLen+1+n > chunkChars {
				flush()
			}
			if currentLen > 0 {
				current.WriteByte(' ')
				currentLen++
			}
			current.WriteString(piece)
			currentLen += n
		}
	}
	flush()
	return chunks
}

// sentences splits on sentence-ending punctuation followed by a space. The
// terminator stays with its sentence so the voice still pauses on it.
func sentences(text string) []string {
	var out []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if (r == '.' || r == '!' || r == '?' || r == ';' || r == ':') && i+1 < len(runes) && runes[i+1] == ' ' {
			out = append(out, strings.TrimSpace(string(runes[start:i+1])))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(string(runes[start:])); rest != "" {
		out = append(out, rest)
	}
	return out
}

// splitLong breaks one sentence that is longer than limit at word boundaries.
// A single word longer than limit is cut at the limit, on a rune boundary; it
// is not speech anyway.
func splitLong(sentence string, limit int) []string {
	var out []string
	for utf8.RuneCountInString(sentence) > limit {
		piece := cutAtWord(sentence, limit)
		out = append(out, piece)
		sentence = strings.TrimSpace(strings.TrimPrefix(sentence, piece))
	}
	if sentence != "" {
		out = append(out, sentence)
	}
	return out
}

// cutAtWord returns the longest prefix of text within limit runes that ends
// at a word boundary, or the first limit runes when there is no boundary.
func cutAtWord(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	for i := limit; i > 0; i-- {
		if unicode.IsSpace(runes[i]) {
			return strings.TrimSpace(string(runes[:i]))
		}
	}
	return string(runes[:limit])
}
