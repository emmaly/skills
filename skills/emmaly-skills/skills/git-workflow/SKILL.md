---
name: git-workflow
description: This skill should be used when committing, branching, opening PRs, or filing GitHub issues — branch naming, conventional commits, PR descriptions, and issue workflow.
---

This skill covers naming and message conventions only. The push/PR/merge process itself lives in `emmaly-skills:integration`, which gates every push behind a mandatory local CodeRabbit review — **invoke it before pushing**, not this skill alone.

## Branching

- Never commit directly to `main`; always work in a feature/fix branch and open a PR
- When working on an issue, create a branch named `fix/<number>-short-desc` or `feature/<number>-short-desc`

## Commits

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`, etc
- Commit body should be detailed enough for an LLM to understand what changed and why

## Pull Requests

- PR descriptions must clearly explain what changed and why, optimized for automated LLM review tooling but clear for human review
- Include `Fixes #<number>` or `Closes #<number>` in the PR description to auto-close related issues on merge
- When updating a PR after review feedback, update the PR description to reflect current state, reply to relevant threads, and add a summary comment

## GitHub Issues

- Use `gh issue list` to review open issues and `gh issue view <number>` to read details
- If an issue is too large, break it into sub-issues or a checklist before starting work
