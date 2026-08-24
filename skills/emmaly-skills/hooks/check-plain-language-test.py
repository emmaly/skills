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
]


def main() -> int:
    failed = 0
    for name, want, payload in CASES:
        data = payload if isinstance(payload, str) else json.dumps(payload)
        result = subprocess.run([sys.executable, HOOK], input=data, capture_output=True, text=True)
        ok = result.returncode == want
        failed += not ok
        print(f"{'PASS' if ok else 'FAIL'}  {name}: want {want}, got {result.returncode}")
    total = len(CASES)
    print(f"\n{total - failed}/{total} passed")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
