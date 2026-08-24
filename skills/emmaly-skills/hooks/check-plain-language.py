#!/usr/bin/env python3
"""Mechanical gate for the one plain-language rule a machine can judge.

`plain-language` is otherwise instruction-only, so adherence decays over a long
session. Em and en dashes are the exception: they are unambiguous, so a hook can
catch them without guessing. Word-list and sentence-shape rules stay with the
model, because a checker for those would fire on the ban list itself, on quoted
tool output, and on every legitimate term of art.

Two events, both registered in hooks.json:

  PostToolUse  Write/Edit/MultiEdit on a prose file. Checks only the text being
               written, never the rest of the file, so editing a document that
               predates the skill does not block unrelated work.
  PreToolUse   Bash running `git commit`. Blocks before the commit exists,
               which is the only cheap moment: rewording afterward means an
               amend or a rebase.

Escape hatch: text inside backticks or a fenced code block is skipped, so a
document that quotes a dash to talk about one still passes. Known limit: the
scan is line-based, so an inline backtick span wrapped across two lines is not
recognised as code and its dash is still flagged. Keep such a span on one line,
or fence it.

Depends on python3 only, no third-party modules. Exit 2 is the code that returns
stderr to Claude; every other outcome exits 0 so a malformed payload can never
wedge a session.
"""

import json
import re
import sys

DASHES = "—–"  # em, en
DASH_RE = re.compile(f"[{DASHES}]")
PROSE_SUFFIXES = (".md", ".mdx", ".markdown", ".txt", ".rst")

FENCE_RE = re.compile(r"^\s*(`{3,}|~{3,})")
INLINE_CODE_RE = re.compile(r"`[^`\n]*`")


def strip_code(text: str) -> str:
    """Blank out fenced blocks and inline code, keeping line numbers intact.

    A fence closes only on the same character at the same length or longer, so
    a four-backtick block that contains three-backtick lines stays one block.
    Getting this wrong flips the parser inside out and reports every later line
    as prose, or none of them.
    """
    out = []
    fence = None  # (character, length) of the fence currently open
    for line in text.split("\n"):
        match = FENCE_RE.match(line)
        if match:
            marker = match.group(1)
            char, length = marker[0], len(marker)
            if fence is None:
                fence = (char, length)
                out.append("")
                continue
            if char == fence[0] and length >= fence[1]:
                fence = None
                out.append("")
                continue
            # A shorter or different marker inside a block is content.
        out.append("" if fence else INLINE_CODE_RE.sub("", line))
    return "\n".join(out)


def offenders(text: str):
    """Return (line_number, line) for each line with a dash outside code."""
    return [
        (i, line.strip())
        for i, line in enumerate(strip_code(text).split("\n"), start=1)
        if DASH_RE.search(line)
    ]


def written_text(tool_input: dict) -> str:
    """Just the text this call authors, not the whole file.

    Every shape is checked rather than assumed. A hook that raises on an
    unexpected payload fails the tool call it was only supposed to inspect.
    """
    if "content" in tool_input:  # Write
        return str(tool_input["content"])
    if "edits" in tool_input:  # MultiEdit
        edits = tool_input["edits"]
        if not isinstance(edits, list):
            return ""
        return "\n".join(
            str(e.get("new_string", "")) for e in edits if isinstance(e, dict)
        )
    return str(tool_input.get("new_string", ""))  # Edit


def block(message: str):
    print(message, file=sys.stderr)
    sys.exit(2)


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        return
    if not isinstance(payload, dict):
        return  # Valid JSON, wrong shape. A bare number parses fine.

    tool = payload.get("tool_name", "")
    tool_input = payload.get("tool_input") or {}
    if not isinstance(tool_input, dict):
        return

    if tool == "Bash":
        command = str(tool_input.get("command", ""))
        if "git commit" in command and DASH_RE.search(command):
            block(
                "plain-language: this commit message contains an em or en dash. "
                "Use a period or a comma, then run the command again."
            )
        return

    if tool in ("Write", "Edit", "MultiEdit"):
        path = str(tool_input.get("file_path", ""))
        if not path.lower().endswith(PROSE_SUFFIXES):
            return
        found = offenders(written_text(tool_input))
        if found:
            lines = "\n".join(f"  {n}: {t}" for n, t in found[:5])
            more = f"\n  and {len(found) - 5} more" if len(found) > 5 else ""
            block(
                f"plain-language: em or en dash in text just written to {path}. "
                f"Replace each with a period or a comma, then edit the file "
                f"again.\n{lines}{more}"
            )


if __name__ == "__main__":
    main()
