---
description: Turn spoken output on or off for this session
---

Run the tts-mode control command and report only its output, nothing else.

- If the argument is `on`, run: `bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" on`
- If the argument is `off`, run: `bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" off`
- If there is no argument, run: `bash "${CLAUDE_PLUGIN_ROOT}/hooks/run-ttsmode.sh" status`

Argument: $ARGUMENTS

Print the command's output and stop. Do not summarize it, do not explain what
changed, and do not speak this turn.

Turning it on takes effect from the next prompt, because the instruction is
injected when a prompt is submitted rather than when this command runs.
