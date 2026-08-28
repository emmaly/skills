# tts-mode

Speaks short summaries of Claude's work aloud. Off by default.

## Use

    /tts on       turn it on for this session
    /tts off      turn it off
    /tts          report the current state

On means Claude speaks one line when it starts multi-step work and one at the
end of each turn, capped at three lines a turn and fifteen words a line. Off
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
first use. `flock` is used to keep two lines from overlapping; without it, as
on macOS, playback still works but is not serialized.

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

The delimiter is drawn fresh each turn, so a spoken line cannot be written to
match it and close the heredoc early. Both the command and the delimiter are
emitted at column zero, because bash requires the terminator unindented.

Arguments still work when calling `tts-say.sh` by hand.

## When it goes quiet

Failures are silent on purpose: a dead API must never fail a turn. Look in
`~/.claude/tts-mode/log` to find out why. Missing key, network error, and no
audio player each log one line there.

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
    hooks/run-ttsmode.sh    builds the binary on first use, then runs it
    hooks/tts-say.sh        backgrounds and serializes playback
    hooks/ttsmode/          the Go source
    commands/tts.md         the /tts slash command
