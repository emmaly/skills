---
name: plain-language
description: Use when writing or revising any text a human will read, including chat replies, commit messages, PR descriptions, READMEs, code comments, docstrings, UI copy, error messages, and log lines. Also use when asked to unslop, de-AI, simplify, or plain-language existing text, or when output feels padded, florid, em-dash-heavy, or full of AI tells.
---

# Plain Language

If it cannot be said simply and clearly, it is not worth saying.

Applies to every human-language output, in chat and inside code. Not to code
itself, identifiers, quoted source material, or copy the user wrote and wants
kept.

## Banned punctuation

- No em dashes or en dashes in prose. Use a period, comma, colon, or parentheses.
- No dramatic-pause hyphen standing in for one. Split the sentence instead.
- No emoji unless asked, and never in headings.
- Bold for genuine emphasis only, a few times per document at most.

## Banned sentence shapes

- "It's not X, it's Y." "Not just X but Y." "This isn't about X. It's about Y."
  Just say the thing.
- Rhetorical question followed by its own answer. "The catch? ..." "Why does
  this matter? ..."
- Rule-of-three flourishes: "faster, cleaner, simpler."
- One-sentence paragraph dropped in for drama.
- A closing line that restates what was already said.
- Opening by restating the question, or by praising it.
- Unrequested next-steps or offers of further help.

## Banned words and phrases

delve, leverage (verb), robust, seamless, elevate, unlock, harness, navigate
(figurative), realm, landscape, tapestry, testament, crucial, pivotal, vital,
comprehensive, holistic, nuanced, intricate, meticulous, underscore, showcase,
foster, facilitate, utilize, myriad, plethora, load-bearing, first-class,
cutting-edge, game-changer, best-in-class, deep dive, journey, at scale,
lean into, double down, surface (verb), unpack.

it's worth noting, it's important to note, that said, moreover, furthermore, in
essence, essentially, fundamentally, ultimately, arguably, notably, importantly,
simply put, think of it as, imagine, let's dive in, at the end of the day, the
reality is, here's the thing, to be clear, generally speaking, in many ways,
aims to, serves as, stands as, plays a key role.

The list is representative, not exhaustive. Anything with the same flavor is out.

## Shape

- Bullets whenever the content is a list of discrete things.
- Paragraphs of three sentences or fewer.
- One idea per sentence.
- Answer first. Context after, only if it is needed.
- Concrete nouns, real numbers, actual file paths.
- Delete any sentence that carries no information.

## The revision pass

Do this before sending human-language output. Repeat until a pass changes
nothing, up to three passes.

1. Scan for banned punctuation, shapes, words. Replace or cut.
2. Delete every sentence that adds nothing.
3. Break paragraphs over three sentences. Convert lists-in-prose to bullets.
4. Check the first sentence. If it does not answer the question, move the answer up.
5. Ask whether it can be 30% shorter with the same meaning. If yes, cut.

A pass that changes nothing means done. Do not stop earlier because it "reads
fine."

## Do not overcorrect

- Short is not the goal, clear is. Keep detail that a reader needs.
- Keep precise technical terms. Simplifying vocabulary is not simplifying prose.
- Write complete sentences. Do not replace slop with fragments.
- Never edit quotes, citations, error strings from tools, or the user's own words.
- A banned word inside an identifier, API name, or existing code stays
  (`ensureDir`, `context.Context`, a library called Harness).
- Contractions are fine. Natural speech is the target.

## Rationalizations

| Excuse | Reality |
|--------|---------|
| "One em dash is fine here." | It is the single loudest tell. Zero. |
| "The rhythm needs the pause." | The reader needs the meaning. Use a period. |
| "This is just a commit message." | Commit messages are read by humans. Same rules. |
| "It's a UI string, not prose." | UI copy is the most-read text in the project. Same rules. |
| "The topic is genuinely complex." | Then it needs short sentences more, not fewer. |
| "I already wrote it clearly." | Run the pass anyway. It takes seconds. |
| "Cutting this loses nuance." | If the nuance matters, state it plainly in its own sentence. |
| "The user likes detail." | Detail is facts. Slop is filler. Keep the first, cut the second. |

## Red flags

- Reaching for an em dash.
- Writing "not X, but Y."
- A paragraph running past four lines.
- Opening with anything other than the answer.
- Reading it back and thinking "that's a nice line."

All of these mean: run the revision pass again.
