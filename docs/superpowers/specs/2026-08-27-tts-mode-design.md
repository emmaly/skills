# TTS mode design

Date: 2026-08-27
Status: approved, not yet implemented

## Purpose

Let the user hear what Claude is doing without reading the terminal. While TTS
mode is on, Claude speaks a short summary of its work through ElevenLabs. While
it is off, nothing changes: no summaries are requested, no audio is produced,
and no cost is incurred.

The switch is a slash command. Turning it on and off is the only difference
between the two states. Normal conversation is unaffected either way.

## Decisions

These were settled with the user before design and are not open:

| Decision | Choice |
| --- | --- |
| State scope | Per session |
| Spoken content | Final response plus progress on multi-step work |
| Voice | Zoe, `XdflFrQO8wbGpWMNZHFr` |
| Placement | New plugin, `skills/tts-mode/` |
| plaincheck | Applies to spoken summaries, no exemption |
| Progress trigger | Start of a multi-step task, plus the end |
| Git | Feature branch and commits, no push |
| Failure behavior | Silent, log and exit zero |
| Secrets | Never spoken |

## Architecture

One Go binary, `hooks/ttsmode`, with three subcommands. A single binary keeps
the state format in one place and matches the existing `plaincheck` precedent.

### `ttsmode hook`

Reads the hook payload on stdin, extracts `session_id`, and checks state.

- Off: prints nothing, exits zero. This is the load-bearing behavior. When TTS
  is off there is no injected instruction, so Claude never produces a summary.
- On: prints the instruction block described below as additional context.

Wired to `UserPromptSubmit` so it re-injects every turn. Injecting once at
session start is not enough, because the instruction falls out of attention as
context grows.

### `ttsmode on|off|status`

Writes or removes `~/.claude/tts-mode/<session_id>`. `status` prints the
current state and exits zero either way.

Session id comes from the `CLAUDE_CODE_SESSION_ID` environment variable when
present, and from a `--session` flag otherwise, so the slash command and the
hook agree on which file to touch. The variable name was verified against a
live session on 2026-08-27; `CLAUDE_SESSION_ID` does not exist.

Background jobs and subagents run under their own session id, so a background
job started from an enabled session will not speak. That is the intended
behavior. Audio should follow the terminal the user is sitting at, not every
process spawned from it.

### `ttsmode say "<text>"`

Checks that TTS is still on for the session, then renders the text and plays
it.

The state check is not redundant with the hook. The instruction is re-injected
every turn, so after an `off` many stale copies remain in the transcript and
nothing counter-instructs them. Without the check, `off` would stop future
requests but not the ones already in context, and the switch would not actually
stop speech or spend.

1. Read `ELEVENLABS_API_KEY` from the environment, falling back to parsing
   `~/.secrets/elevenlabs.env`. The hook runs unattended, so it cannot rely on
   `envwith` wrapping it.
2. POST to `https://api.us.elevenlabs.io/v1/text-to-speech/{voice}` with
   `output_format=mp3_44100_192`.
3. Write the audio to a temporary file, play it, delete it.

Playback is backgrounded so the calling turn never waits on audio. A `flock`
on a per-session lock file serializes playback, so a progress line and a
closing line queue rather than talk over each other.

### State

One file per session under `~/.claude/tts-mode/sessions/`, mode 0700 on the
directories. The file holds the enable timestamp; presence means enabled. The
log sits beside that directory at `~/.claude/tts-mode/log`, not inside it, so
pruning can never delete the one file the user is told to read when speech goes
quiet, and a session id of `log` cannot collide with it.

Session ids are validated as exactly one path element that is not a traversal.
A blacklist of separators is not enough: an id of `.` cleans to the directory
itself, which makes `Enabled` report on for a session nobody enabled and turns
`Disable` into a recursive wipe of every other session.

A `SessionStart` hook prunes files older than seven days. Sessions end without
notice, so cleanup cannot depend on a shutdown path.

## The injected instruction

The text `ttsmode hook` prints when enabled:

- Speak one line when starting work expected to take several tool calls.
- Speak one line at the end of the turn, summarizing the outcome.
- At most three spoken lines per turn.
- Fifteen words or fewer per line.
- Never speak secrets, tokens, credentials, or full filesystem paths. Spoken
  output carries into a room that the terminal does not.
- Call it as `ttsmode say "<text>"`.

Fifteen words is roughly six seconds of speech and about 0.02 cents.

## Voice settings

Fixed in the binary, matching what the user approved:

    model_id:          eleven_v3
    voice_id:          XdflFrQO8wbGpWMNZHFr
    stability:         0.3
    similarity_boost:  0.9
    style:             0.48
    speed:             0.88
    use_speaker_boost: true
    output_format:     mp3_44100_192

Two constraints govern any future voice change. The voice must have `rate` of
1.0, because a higher rate doubles the per-character cost. It must have
`live_moderation_enabled` false, because moderation adds latency to every
line. Both fields are readable from `GET /v1/shared-voices`.

## Failure behavior

Every failure path logs one line to `~/.claude/tts-mode/log` and exits zero:
missing key, network error, non-200 response, no audio player found.

A broken TTS setup must never fail a turn. The user asked for a mode whose only
effect is whether audio plays, and an error that interrupts work would break
that.

## Security

- The API key is read at call time and never written to disk, logs, or command
  output.
- Spoken text is bounded by the instruction rules above.
- The state directory is 0700. It holds no secrets, only timestamps.
- `ttsmode say` takes its text as an argument, so the summary is visible in the
  transcript. This is intended: hidden from casual reading, recoverable on
  inspection.

## Testing

Go table tests, following the `plaincheck` pattern:

- State round-trip: on, status, off, status.
- Hook contract with TTS off produces empty stdout. This is the test that
  protects the core promise.
- Hook contract with TTS on includes the call form and the word cap.
- Env file parsing: comments, blank lines, quoted values, a missing file.
- Prune keeps recent files and removes old ones.

The HTTP call and the audio player sit behind interfaces, so no test reaches
ElevenLabs or opens an audio device.

## Files

New plugin at `skills/tts-mode/`:

    .claude-plugin/plugin.json
    commands/tts.md
    hooks/hooks.json
    hooks/ttsmode/go.mod
    hooks/ttsmode/main.go
    hooks/ttsmode/main_test.go
    README.md

One edit to `.claude-plugin/marketplace.json` to register the plugin.

## Out of scope

- Speech input. This is output only.
- Streaming or partial-sentence speech.
- Per-project or global toggles. Session scope was chosen deliberately.
- Voice selection at runtime. The voice is fixed until the user asks otherwise.

## Known risk

The feature depends on Claude honoring an injected instruction. Re-injecting
every turn reduces drift but does not eliminate it. Expect an occasional missed
line rather than complete coverage. If misses become common, the fallback is a
`Stop` hook that speaks a fixed phrase when a turn ends without a spoken line,
which trades summary quality for reliability.
