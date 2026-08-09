#!/usr/bin/env bash
set -euo pipefail

# Emits the body of the `standards` skill as SessionStart context.
#
# SessionStart is one of the few hook events whose stdout is added directly to
# Claude's context, so printing the standards here makes them active in every
# session where this plugin is enabled — no need to write them into
# ~/.claude/CLAUDE.md. This is the only mechanism that loads the standards
# automatically; if they are ever also copied into a CLAUDE.md, they will be in
# context twice. (Invoking the `standards` skill by hand also re-emits them —
# that is intended, and its description says to do so only on request.)
#
# No matcher is set in hooks.json, so this fires on startup, resume, clear, and
# compact — the last one matters, since compaction can summarize the standards
# away.

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SKILL_FILE="${PLUGIN_ROOT}/skills/standards/SKILL.md"

# Never fail a session start over this — but say so on stderr, or a silent
# exit 0 looks identical to a session that simply has no standards. Readable,
# not merely present: an unreadable file would otherwise reach awk and trip
# `set -e` with no explanation.
if [[ ! -r "$SKILL_FILE" ]]; then
    echo "emmaly-skills: standards not loaded, cannot read ${SKILL_FILE}" >&2
    exit 0
fi

echo "## Standards (emmaly-skills)"
echo

# Print the body, dropping the YAML frontmatter if present.
awk '
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm == 1 && $0 == "---" { in_fm = 2; next }
    in_fm != 1 { print }
' "$SKILL_FILE"
