#!/usr/bin/env bash
set -euo pipefail

# Emits the body of a skill as SessionStart context.
#
# Usage: emit-skill-body.sh <skill-dir-name> <heading>
#
# SessionStart is one of the few hook events whose stdout is added directly to
# Claude's context, so printing a skill body here makes it active in every
# session where this plugin is enabled, with no need to write it into
# ~/.claude/CLAUDE.md. This is the only mechanism that loads these skills
# automatically; if the same content is ever also copied into a CLAUDE.md, it
# will be in context twice. (Invoking the skill by hand also re-emits it. That
# is intended, and those skills' descriptions say to do so only on request.)
#
# No matcher is set in hooks.json, so this fires on startup, resume, clear, and
# compact. The last one matters, since compaction can summarize the body away.

if [[ $# -ne 2 ]]; then
    echo "emit-skill-body.sh: usage: emit-skill-body.sh <skill-dir-name> <heading>" >&2
    exit 0
fi

SKILL_NAME="$1"
HEADING="$2"

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SKILL_FILE="${PLUGIN_ROOT}/skills/${SKILL_NAME}/SKILL.md"

# Never fail a session start over this, but say so on stderr. A silent exit 0
# looks identical to a session that simply has nothing to emit. Readable, not
# merely present: an unreadable file would otherwise reach awk and trip `set -e`
# with no explanation.
if [[ ! -r "$SKILL_FILE" ]]; then
    echo "emmaly-skills: ${SKILL_NAME} not loaded, cannot read ${SKILL_FILE}" >&2
    exit 0
fi

echo "## ${HEADING}"
echo

# Print the body, dropping the YAML frontmatter if present.
awk '
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm == 1 && $0 == "---" { in_fm = 2; next }
    in_fm != 1 { print }
' "$SKILL_FILE"
