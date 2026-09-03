# tts-mode

Speaks short summaries of Claude's work aloud. Off by default.

## Use

    /tts on              turn it on for this session
    /tts off             turn it off
    /tts                 report the current state and any instructions
    /tts <request>       turn it on, shaped by what you asked for
    /tts on <request>    the same, said explicitly

A request is freeform. "keep it to five words", "one line per turn, and say
which file you touched", "speak in Spanish". It is read by Claude, turned into
instructions addressed to itself, and stored with the session; the command
prints what it became so you can see whether it understood you.

Those instructions take precedence over the style and length defaults below,
including the word and line counts, so asking for longer lines works. They do
not override the rule against speaking secrets, credentials, or full filesystem
paths, which holds however a request is worded.

A second request is merged with what is already stored, so `/tts also say which
file` keeps the earlier wording rather than replacing it. `/tts on` with no
request clears the instructions. `/tts off` clears everything.

A lone word that nearly spells a subcommand is refused rather than taken as a
request, so `/tts of` does not turn TTS on with "of" as its instruction.

On means Claude speaks one line when it starts multi-step work and one at the
end of each turn, capped at three lines a turn and fifteen words a line by
default. A session's own instructions can raise that; the code caps one say at
1,200 characters, cut at a word boundary, never inside a word. Off
means the instruction is never injected, so no summary is even requested.
Nothing else about the conversation changes either way.

The switch is per session. Enabling it in one terminal leaves your other
sessions silent, and background jobs never speak.

Turning it on takes effect from your next prompt, because the instruction is
injected when a prompt is submitted.

## Requirements

An ElevenLabs API key in `~/.secrets/elevenlabs.env`:

    ELEVENLABS_API_KEY=your-key-here

Set `TTSMODE_ENV_FILE` to read it from somewhere else. `ELEVENLABS_API_KEY` in
the environment wins over both.

Either `mpv` or `ffplay` on `PATH` for playback. Go, to build the helper on
first use. `setsid` detaches playback from the shell that asked for it; without
it, as on macOS, the job is backgrounded with `disown` instead.

## Environment

    ELEVENLABS_API_KEY   the key, if you would rather not use the env file
    TTSMODE_ENV_FILE     where to read the key from
    TTSMODE_STATE_DIR    where session state and the log live
    TTSMODE_API_BASE     override the API host

`HOME`, `XDG_CACHE_HOME`, `TTSMODE_STATE_DIR` and `TTSMODE_ENV_FILE` must be
absolute. A relative one resolves against whatever directory a hook happened
to run in, so it is refused rather than followed. Without `HOME`, both
`XDG_CACHE_HOME` and `TTSMODE_STATE_DIR` are required: one holds the compiled binary, the other
holds session state and the log.

`TTSMODE_API_BASE` matters only for an isolated data-residency workspace, which
is a separate account on a host such as `https://api.eu.residency.elevenlabs.io`.
The default is the generic host.

## Cost

Billing is per character. A fifteen-word line runs about a hundred characters,
so a turn with three lines is roughly three hundred. Against a plan that
includes 300,000 characters a month, that is on the order of a thousand turns
before the allowance is gone, and per-character billing after it.

Bitrate and voice settings do not affect price. The voice's rate multiplier
does, which is why this one is pinned to 1.0.

Check your own allowance and usage with:

    GET /v1/user/subscription
    GET /v1/usage/character-stats

## Why the spoken line goes in on stdin

The injected instruction tells Claude to pass the line through a heredoc with
a quoted delimiter, not as a quoted argument. Claude's Bash executor performs
shell substitution, and double quotes do not disable `$(...)`, backticks, or
variable expansion. The summary is written by a model that has just been
reading files and tool output, so a line quoting a filename or an error string
would have run as shell source. A quoted heredoc is not expanded at all.

The delimiter is a fixed uncommon token, and both the command and the closing
delimiter are emitted at column zero, because bash requires the terminator
unindented. An indented one runs to end of input and swallows whatever the
agent runs next.

Arguments still work when calling `tts-say.sh` by hand.

## How a line is spoken

The say wrapper returns at once. It prints any failure an earlier line hit,
then detaches a job that splits the text into pieces at sentence ends (about
220 characters each), synthesizes them three at a time, and drops them in a
queue under `~/.claude/tts-mode/queue`. Whoever holds the player lock plays
tickets in the order they were requested, one piece at a time with a short
pause between clips, waiting for a piece that is still being synthesized. The first piece plays while the rest
are still in flight, and two sessions never talk over each other. A piece
that never arrives is abandoned after 45 seconds so a crashed job cannot
wedge the queue.

## When it goes quiet

Failures never fail a turn: a dead API must not stop the work. They are
reported two ways. The next spoken line prints `tts-mode: earlier spoken
line failed: ...` to the model that asked, so it can tell you. And
`~/.claude/tts-mode/log` records every one. Missing key, network error, no
audio player, and a cut line each log one line there.

## Voice

Fixed to Zoe, `XdflFrQO8wbGpWMNZHFr`, at settings chosen by listening: model
`eleven_v3`, stability 0.3, similarity boost 0.9, style 0.48, speed 0.88,
speaker boost on, rendered at 192 kbps.

Any replacement must have `rate` of 1.0 and `live_moderation_enabled` false.
The first is cost: a rate of 2.0 doubles the per-character price. The second is
latency: moderation adds a round trip to every line, which is the wrong trade
for a voice that talks while you work. Both fields come from
`GET /v1/shared-voices`.

## Layout

    hooks/hooks.json        UserPromptSubmit injects, SessionStart prunes
    hooks/ttsmode/control.go  parses what you typed after /tts
    hooks/run-ttsmode.sh    builds the binary on first use, then runs it
    hooks/tts-say.sh        reports earlier failures, then detaches a say
    hooks/tts-say-detached.sh  the detached half, run under setsid
    hooks/ttsmode/chunk.go  splits text at sentences and words, applies the cap
    hooks/ttsmode/queue.go  tickets, concurrent synthesis, ordered playback
    hooks/ttsmode/          the Go source
    commands/tts.md         the /tts slash command
