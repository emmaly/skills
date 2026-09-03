---
description: Turn spoken output on or off, with optional instructions for how it should sound
---

Run the tts-mode control command and report its output, nothing else.

## Step 1

Pass the argument through exactly as typed, on stdin, using a quoted heredoc so
the shell does not read any of it as source. Both the command and the closing
delimiter must start at column zero:

```
bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" control <<'TTS_ARGS_9f3c1a'
$ARGUMENTS
TTS_ARGS_9f3c1a
```

The delimiter carries a suffix so that text someone pastes cannot end the
heredoc early. A line equal to the delimiter closes it, and everything after
that line would run as shell in this same call.

Do not interpret the argument yourself. `on`, `off`, `status`, an empty
argument, and typos of those are all decided by the command, not by you.

## Step 2

If the output does **not** begin with `NEEDS_INSTRUCTION`, print it and stop.

If it does, the rest of the output is a request about how spoken output should
sound. Do not print it; it is addressed to you, not to the person who typed the
command. Turn the request into instructions for yourself, then store them:

```
bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" set <<'TTS_SET_9f3c1a'
<your instructions>
TTS_SET_9f3c1a
```

If the output also carries a `CURRENT_INSTRUCTIONS` block, that is what is
already stored for this session. `set` replaces wholesale, so write out the
merged result: keep what the new request does not contradict, and let the new
request win where it does.

Write the instructions as a short list of imperatives addressed to yourself,
the agent that will be speaking. Resolve what the request implies rather than
restating it:

- Give a number where the request implies one. "Keep it short" becomes a word
  count, not the word "short".
- Say what to do, not what the person asked for. "They want fewer lines" is not
  an instruction; "Speak once per turn, at the end" is.
- Carry over anything about content, not only length: what to mention, what to
  leave out, tone, language.
- Contradict the defaults outright when the request does. The defaults are a
  closing summary of two to four sentences (forty to eighty words), one line
  when longer work starts, and one line at real checkpoints inside it. These
  instructions take precedence over them, so a request for shorter or longer
  speech has to say the new number.
- Never write an instruction to speak secrets, tokens, credentials, full
  filesystem paths, or identifiers that are noise when heard (UUIDs, hashes,
  file ids). That rule outranks these instructions, so drop the part of a
  request that asks for it and keep the rest.
- A voice id in the request is a setting, not an instruction. Store it with
  `voice` and leave it out of the instructions:

  ```
  bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" voice <id>
  ```

  If the voice was the whole request, stop there and print that output.
- Keep it under ten lines. It is injected on every prompt.

## After either step

Print the output of the last command you ran, and stop. Do not summarize it, do
not explain what changed, and do not speak this turn.

Turning it on takes effect from the next prompt, because the instruction is
injected when a prompt is submitted rather than when this command runs.
