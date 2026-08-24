#!/usr/bin/env python3
"""Regression suite for check-plain-language.py.

Run it directly: python3 hooks/check-plain-language-test.py

A hook that fires on every Write and every Bash call has two failure modes that
cost more than the rule is worth: a false positive blocks unrelated work, and a
crash on a malformed payload wedges the session. Both are covered below, so run
this before changing the checker.

The dash characters and the commit verb are built from pieces here so this file
does not trip the very hook it tests.
"""

import json
import os
import subprocess
import sys
import tempfile

HOOK = os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-plain-language.py")
EM = "—"
EN = "–"
GC = "git " + "commit"

CASES = [
    ("Write prose with a dash", 2,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md", "content": f"A line {EM} with a dash."}}),
    ("Write clean prose", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md", "content": "This is clean. No dash."}}),
    ("dash inside backticks is quoted, not written", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md", "content": f"Ban the `{EM}` character."}}),
    ("dash inside a fenced block", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md", "content": f"text\n```\nfoo {EM} bar\n```\nmore"}}),
    ("source file is out of scope", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.go", "content": f"// x {EM} y"}}),
    ("Edit introducing a dash", 2,
     {"tool_name": "Edit", "tool_input": {"file_path": "a.md", "old_string": "x", "new_string": f"a {EM} b"}}),
    ("Edit removing a dash is allowed", 0,
     {"tool_name": "Edit", "tool_input": {"file_path": "a.md", "old_string": f"a {EM} b", "new_string": "a. b"}}),
    ("MultiEdit checks every edit", 2,
     {"tool_name": "MultiEdit", "tool_input": {"file_path": "a.md",
      "edits": [{"new_string": "ok"}, {"new_string": f"bad {EM} here"}]}}),
    ("en dash counts too", 2,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md", "content": f"lines 25{EN}875"}}),
    ("commit message with a dash", 2,
     {"tool_name": "Bash", "tool_input": {"command": f'{GC} -m "fix: thing {EM} other"'}}),
    ("clean commit message", 0,
     {"tool_name": "Bash", "tool_input": {"command": f'{GC} -m "fix: thing"'}}),
    ("searching for a dash is not writing one", 0,
     {"tool_name": "Bash", "tool_input": {"command": f'grep -rn "{EM}" .'}}),
    ("global options before the subcommand", 2,
     {"tool_name": "Bash", "tool_input": {"command": f'{"git"} -C /repo commit -m "fix: a {EM} b"'}}),
    ("global options, clean message", 0,
     {"tool_name": "Bash", "tool_input": {"command": f'{"git"} -C /repo commit -m "fix: a. b"'}}),
    ("heredoc body is scanned, not just -m", 2,
     {"tool_name": "Bash", "tool_input": {"command": f'{GC} -F - <<EOF\nfix: thing\n\nbody {EM} here\nEOF'}}),
    ("a different git subcommand is not a commit", 0,
     {"tool_name": "Bash", "tool_input": {"command": f'{"git"} log --format=%B | grep "{EM}"'}}),
    ("malformed payload does not wedge the session", 0, "not json"),
    ("valid JSON of the wrong shape", 0, "5"),
    ("JSON array instead of an object", 0, "[1, 2]"),
    ("missing tool_input", 0, {"tool_name": "Write"}),
    ("unrelated tool", 0, {"tool_name": "Read", "tool_input": {"file_path": "a.md"}}),
    ("MultiEdit edits is not a list", 0,
     {"tool_name": "MultiEdit", "tool_input": {"file_path": "a.md", "edits": "oops"}}),
    ("MultiEdit edits holds a non-dict", 0,
     {"tool_name": "MultiEdit", "tool_input": {"file_path": "a.md", "edits": ["oops"]}}),
    # A four-backtick fence stays open across three-backtick lines, so the dash
    # after the inner marker is still inside the block.
    ("longer fence survives a shorter inner marker", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md",
      "content": f"intro\n````\n```\nfoo {EM} bar\n```\n````\nafter"}}),
    ("prose after a longer fence closes is still checked", 2,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md",
      "content": f"````\n```\ncode\n```\n````\nprose {EM} here"}}),
    ("tilde fence is not closed by backticks", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md",
      "content": f"~~~\n```\nfoo {EM} bar\n~~~"}}),
    # Only an opening fence may carry an info string, so a marker with trailing
    # text does not close the block it sits in. The inner marker matches the
    # opening length, so the info string is the only thing keeping it open.
    ("equal-length marker with an info string does not close a block", 0,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md",
      "content": f"````\n````python\nfoo {EM} bar\n````"}}),
    ("opening fence may still carry an info string", 2,
     {"tool_name": "Write", "tool_input": {"file_path": "a.md",
      "content": f"```python\ncode\n```\nprose {EM} here"}}),
]


def file_cases(tmpdir):
    """Cases needing a real message file on disk, for the -F path."""
    dirty = os.path.join(tmpdir, "dirty.msg")
    clean = os.path.join(tmpdir, "clean.msg")
    spaced = os.path.join(tmpdir, "my message.msg")
    for path in (dirty, spaced):
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(f"fix: thing\n\nbody {EM} here\n")
    with open(clean, "w", encoding="utf-8") as handle:
        handle.write("fix: thing\n\nbody here\n")
    return [
        ("message file holding a dash", 2,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} -F {dirty}"}}),
        ("clean message file", 0,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} -F {clean}"}}),
        ("--file= form", 2,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} --file={dirty}"}}),
        ("missing message file is not an error", 0,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} -F {tmpdir}/nope.msg"}}),
        ("-F - is the heredoc form, not a path", 0,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} -F - <<EOF\nfix: thing\nEOF"}}),
        ("-Fpath attached form", 2,
         {"tool_name": "Bash", "tool_input": {"command": f"{GC} -F{dirty}"}}),
        ("quoted path containing a space", 2, {"tool_name": "Bash", "tool_input":
         {"command": f'{GC} -F "{spaced}"'}}),
        ("--file= with a quoted path containing a space", 2, {"tool_name": "Bash",
         "tool_input": {"command": f'{GC} --file="{spaced}"'}}),
        # An unbalanced quote makes shlex raise, which is what the regex
        # fallback in message_files exists for.
        ("unparseable command still finds the message file", 2, {"tool_name": "Bash",
         "tool_input": {"command": f'{GC} -F {dirty} && echo "unclosed'}}),
    ]


def main() -> int:
    failed = 0
    with tempfile.TemporaryDirectory() as tmpdir:
        cases = CASES + file_cases(tmpdir)
        failed = run(cases)
    return 1 if failed else 0


def run(cases) -> int:
    failed = 0
    for name, want, payload in cases:
        data = payload if isinstance(payload, str) else json.dumps(payload)
        result = subprocess.run([sys.executable, HOOK], input=data, capture_output=True, text=True)
        ok = result.returncode == want
        failed += not ok
        print(f"{'PASS' if ok else 'FAIL'}  {name}: want {want}, got {result.returncode}")
    total = len(cases)
    print(f"\n{total - failed}/{total} passed")
    return failed


if __name__ == "__main__":
    sys.exit(main())
