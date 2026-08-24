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

FENCE_RE = re.compile(r"^\s*(```|~~~)", re.M)
INLINE_CODE_RE = re.compile(r"`[^`\n]*`")


def strip_code(text: str) -> str:
    """Blank out fenced blocks and inline code, keeping line numbers intact."""
    lines = text.split("\n")
    out = []
    in_fence = False
    for line in lines:
        if FENCE_RE.match(line):
            in_fence = not in_fence
            out.append("")
            continue
        out.append("" if in_fence else INLINE_CODE_RE.sub("", line))
    return "\n".join(out)


def offenders(text: str):
    """Return (line_number, line) for each line with a dash outside code."""
    return [
        (i, line.strip())
        for i, line in enumerate(strip_code(text).split("\n"), start=1)
        if DASH_RE.search(line)
    ]


def written_text(tool_input: dict) -> str:
    """Just the text this call authors, not the whole file."""
    if "content" in tool_input:  # Write
        return str(tool_input["content"])
    if "edits" in tool_input:  # MultiEdit
        return "\n".join(str(e.get("new_string", "")) for e in tool_input["edits"])
    return str(tool_input.get("new_string", ""))  # Edit


def block(message: str):
    print(message, file=sys.stderr)
    sys.exit(2)


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        return

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
