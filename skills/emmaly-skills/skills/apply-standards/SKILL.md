---
name: apply-standards
description: This skill should be used to "apply standards" or "persist standards globally" — writes or updates the standards section in ~/.claude/CLAUDE.md from emmaly-skills:standards so it loads in every conversation across all projects.
---

# Apply Standards to ~/.claude/CLAUDE.md

This skill writes the content of `emmaly-skills:standards` into `~/.claude/CLAUDE.md` so it is loaded automatically in every conversation, across all projects.

**This is not needed for Claude Code.** The plugin's `SessionStart` hook (`hooks/emit-standards.sh`) already injects the standards into every session where `emmaly-skills` is enabled. Use this skill when the standards need to persist somewhere the plugin doesn't reach: other agent tooling that reads `~/.claude/CLAUDE.md`, or machines where the plugin isn't installed. Running both is harmless but duplicates the content in context.

## Steps

1. **Read the standards source**: Read `${CLAUDE_PLUGIN_ROOT}/skills/standards/SKILL.md` (the sibling `standards` skill in this plugin). Strip the YAML frontmatter (everything between the opening and closing `---` lines). Keep only the body content.

2. **Read `~/.claude/CLAUDE.md`**: If it doesn't exist, create it. Read its current contents.

3. **Check for existing markers**: Look for the marker pair `<!-- emmaly:standards -->` and `<!-- /emmaly:standards -->`.

4. **Update or insert**:
   - **If markers exist**: Replace everything from `<!-- emmaly:standards -->` through `<!-- /emmaly:standards -->` (inclusive) with the new block below.
   - **If no markers**: Append the new block to the end of the file.

5. **Block format** (use exactly this structure):
   ```
   <!-- emmaly:standards -->
   ## Standards

   {body content from standards/SKILL.md}

   <!-- /emmaly:standards -->
   ```

6. **Report**: Tell the user whether the section was added or updated, and confirm the target file path.
