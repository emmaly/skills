#!/usr/bin/env bash
set -euo pipefail

# Emits the body of the `standards` skill as SessionStart context.
#
# SessionStart is one of the few hook events whose stdout is added directly to
# Claude's context, so printing the standards here makes them active in every
# session where this plugin is enabled — no need to write them into
# ~/.claude/CLAUDE.md. The `apply-standards` skill remains the way to persist
# them for tools that don't load this plugin.
#
# No matcher is set in hooks.json, so this fires on startup, resume, clear, and
# compact — the last one matters, since compaction can summarize the standards
# away.

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SKILL_FILE="${PLUGIN_ROOT}/skills/standards/SKILL.md"

# Never fail a session start over this.
if [[ ! -f "$SKILL_FILE" ]]; then
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
