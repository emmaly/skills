---
name: integration
description: This skill should be used when pushing code, opening a PR, or merging — the mandatory local Claude review loop, push/merge process, and the CodeRabbit PR gate.
---

Once implementation is complete and local verification passes (`go vet`, `gofmt`, tests, frontend build), proceed directly through this workflow without waiting for further instruction:

**CRITICAL: Never push code to GitHub without a clean local review first.** The local review is Claude's built-in `code-review` skill, not CodeRabbit. CodeRabbit reviews only the PR, as a final third-party gate before merging. It is rate-limited to 5 reviews/hour, so it must never run in the inner loop: no `coderabbit review` CLI runs, and no CodeRabbit-plugin review skills or agents during a session. Any in-session review request goes to the built-in `code-review` skill.

1. **Local review** (MANDATORY before any push): Invoke the built-in `code-review` skill at high effort against the branch's diff from main. Diff-based review misses untracked files, so a new file would otherwise reach GitHub unreviewed — `git add` the files the change introduces before reviewing
2. **Address findings**: Fix actionable issues; file GitHub issues for deferred items
3. **Re-review if changed**: If step 2 produced commits, go back to step 1. Do NOT push until local review is clean
4. **Push and open PR**: Only after local review reports no findings, push the branch and open/update the PR. If more pushes are likely (iterating on CI, expecting follow-ups), open it as a **draft** — CodeRabbit does not auto-review drafts by default, so the PR costs one review no matter how many pushes it takes. Mark it ready for review once the branch is final
5. **Wait for PR review**: After marking ready (or pushing a non-draft), wait 5 minutes, then check the PR's commit status; once checks pass, fetch and read all CodeRabbit review comments
6. **If the PR review is clean**: Merge the PR and delete the remote and local feature branch; do not wait for confirmation
7. **If the PR review has findings**: Fix locally, go back to step 1 (local review); do NOT push incremental fixes without a clean local review first. If the fix round will take several pushes, convert the PR back to draft first

Skip the local review ONLY if explicitly requested.

## Budgeting the CodeRabbit limit

The 5 reviews/hour budget is spent only by PR reviews of ready (non-draft) PRs. Keeping under it:

- Local iteration is free: the pre-push gate is Claude, with no hourly cap
- One PR ≈ one review when the draft flow above is followed
- If the limit is hit anyway, wait it out; do not substitute a push without any PR review

## CodeRabbit tips (PR gate only)

- **Defer findings**: reply to any review comment with `@coderabbitai create a GitHub issue for this` to defer to follow-up issues
- **Auto-paused reviews**: CodeRabbit auto-pauses after many commits; use `@coderabbitai resume` to un-pause
- **Paginated reviews**: use `?per_page=100` when fetching via API: `gh api 'repos/{owner}/{repo}/pulls/{number}/reviews?per_page=100'`
- **Duplicate comments**: means the issue was previously flagged and is still unfixed
- **State**: `COMMENTED` = no actionable items; `CHANGES_REQUESTED` = has actionable comments
