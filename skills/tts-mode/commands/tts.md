---
description: Turn spoken output on or off, with optional instructions for how it should sound
---

Run the tts-mode control command and report its output, nothing else.

## Step 1

Pass the argument through exactly as typed, on stdin, using a quoted heredoc so
the shell does not read any of it as source. Both the command and the closing
delimiter must start at column zero:

```
bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" control <<'TTS_ARGS'
$ARGUMENTS
TTS_ARGS
```

Do not interpret the argument yourself. `on`, `off`, `status`, an empty
argument, and typos of those are all decided by the command, not by you.

## Step 2

If the output does **not** begin with `NEEDS_INSTRUCTION`, print it and stop.

If it does, the rest of the output is a request about how spoken output should
sound. Turn that request into instructions for yourself, then store them:

```
bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" set <<'TTS_SET'
<your instructions>
TTS_SET
```

Print that command's output and stop.

Write the instructions as a short list of imperatives addressed to yourself,
the agent that will be speaking. Resolve what the request implies rather than
restating it:

- Give a number where the request implies one. "Keep it short" becomes a word
  count, not the word "short".
- Say what to do, not what the person asked for. "They want fewer lines" is not
  an instruction; "Speak once per turn, at the end" is.
- Carry over anything about content, not only length: what to mention, what to
  leave out, tone, language.
- Contradict the defaults outright when the request does. The defaults are
  fifteen words a line and three lines a turn, and these instructions take
  precedence over them, so a request for longer lines has to say the new
  number.
- Keep it under ten lines. It is injected on every prompt.

## Both steps

Print the command's output and stop. Do not summarize it, do not explain what
changed, and do not speak this turn.

Turning it on takes effect from the next prompt, because the instruction is
injected when a prompt is submitted rather than when this command runs.

Argument: $ARGUMENTS
