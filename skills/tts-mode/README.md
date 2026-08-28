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
first use.

## Cost

Billing is per character, so a fifteen-word line is roughly two hundredths of a
cent. Bitrate and voice settings do not affect price.

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
