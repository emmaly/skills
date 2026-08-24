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

Escape hatch, prose files only: text inside backticks or a fenced code block is
skipped, so a document that quotes a dash to talk about one still passes. The
commit branch has no such hatch, since it scans the raw command and any file
given to -F.

Two known limits. The prose scan is line-based, so an inline backtick span
wrapped across two lines is not recognised as code and its dash is still
flagged; keep such a span on one line, or fence it. And a message built from a
shell variable, or piped in from another process, cannot be resolved here.

Depends on python3 only, no third-party modules. Exit 2 is the code that returns
stderr to Claude; every other outcome exits 0 so a malformed payload can never
wedge a session.
"""

import json
import re
import shlex
import sys

DASHES = "—–"  # em, en
DASH_RE = re.compile(f"[{DASHES}]")
PROSE_SUFFIXES = (".md", ".mdx", ".markdown", ".txt", ".rst")

# `git commit`, including the forms that put global options first, so
# `git -C /repo commit` is not missed.
GIT_COMMIT_RE = re.compile(
    r"\bgit\b(?:\s+(?:-C\s+\S+|-c\s+\S+|--git-dir=\S+|--work-tree=\S+"
    r"|-P|--no-pager|--no-replace-objects))*\s+commit\b"
)

# -F path, --file path, --file=path. The message lives in the file, not in the
# command, so the file has to be read to see it.
MESSAGE_FILE_RE = re.compile(
    r"(?:^|\s)(?:-F|--file)(?:=(?P<eq>[^\s]+)|\s+(?P<sep>[^\s]+))"
)

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
            closes = (
                char == fence[0]
                and length >= fence[1]
                and not line[match.end():].strip()
            )
            if closes:
                fence = None
                out.append("")
                continue
            # A shorter or different marker inside a block is content, and so
            # is one carrying an info string: only an opening fence may have
            # text after the marker.
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


def message_files(command: str):
    """Paths given to -F, --file, --file=, or -Fpath, holding the message text.

    Tokenised the way a shell would, so a quoted path with a space in it stays
    one path. A command shlex cannot parse, an unbalanced quote in a heredoc for
    instance, falls back to the regex, which handles the unquoted forms.

    `-` means stdin, which is the heredoc case, and its text is already in the
    command.
    """
    try:
        tokens = shlex.split(command, comments=False, posix=True)
    except ValueError:
        return [
            raw
            for match in MESSAGE_FILE_RE.finditer(command)
            for raw in [(match.group("eq") or match.group("sep") or "").strip("\"'")]
            if raw and raw != "-"
        ]

    paths = []
    expecting = False
    for token in tokens:
        if expecting:
            expecting = False
            if token != "-":
                paths.append(token)
            continue
        if token in ("-F", "--file"):
            expecting = True
        elif token.startswith("--file="):
            paths.append(token[len("--file="):])
        elif token.startswith("-F") and len(token) > 2:
            paths.append(token[2:])
    return [p for p in paths if p and p != "-"]


def read_text(path: str) -> str:
    """Contents of a message file, or an empty string if it cannot be read."""
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except OSError:
        return ""


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
        if not GIT_COMMIT_RE.search(command):
            return
        # The whole command is scanned, not just a -m value. A message also
        # arrives through a heredoc, which is how the long messages in this
        # repo are written, so reading only -m would miss the common path. The
        # cost is a false positive when an unrelated part of a compound command
        # holds a dash, and the fix there is to run the commit on its own.
        if DASH_RE.search(command):
            block(
                "plain-language: this commit message contains an em or en dash. "
                "Use a period or a comma, then run the command again."
            )
        # A message passed as a file is not in the command text at all, so read
        # it. Known limit: a path built from a shell variable cannot be resolved
        # here, and neither can a message piped in from another process.
        for path in message_files(command):
            if DASH_RE.search(read_text(path)):
                block(
                    f"plain-language: the commit message in {path} contains an "
                    "em or en dash. Use a period or a comma, then run the "
                    "command again."
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
