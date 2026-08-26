---
name: integration
description: This skill should be used when pushing code, opening a PR, or merging. Covers the mandatory local Claude review loop, the push/merge process, and the CodeRabbit PR gate.
---

Once implementation is complete and local verification passes (`go vet`, `gofmt`, tests, frontend build), proceed directly through this workflow without waiting for further instruction:

**CRITICAL: Never mark a PR ready for review, and never push to a non-draft PR, without a clean local review first.** The local review is Claude's built-in `code-review` skill, not CodeRabbit. CodeRabbit reviews only the PR, as a final third-party gate before merging. Its 5 reviews/hour limit is shared by PR reviews, `coderabbit review` CLI runs, and manual `@coderabbitai review` triggers, so only the PR gate may spend it: no CodeRabbit CLI runs and no CodeRabbit-plugin review skills or agents during a session. Any in-session review request goes to the built-in `code-review` skill.

1. **Local review** (MANDATORY before any non-draft push or ready-for-review): Commit the change, including any new files (a staged-but-uncommitted file does not appear in the branch diff and would reach GitHub unreviewed), then invoke the built-in `code-review` skill at high effort against the branch's diff from main
2. **Address findings**: Fix actionable issues; file GitHub issues for deferred items
3. **Re-review if changed**: If step 2 produced commits, go back to step 1. The gate is clean when the review reports no findings other than items already deferred to GitHub issues
4. **Push and open PR**: If more pushes are likely (iterating on CI, expecting follow-ups), open the PR as a **draft**. CodeRabbit skips drafts unless the repo's `.coderabbit.yaml` sets `reviews.auto_review.drafts: true`, so check for that override once per repo; with drafts skipped, the PR costs one review per ready-for-review round, not per push. Draft pushes whose only purpose is to run CI (workflow debugging) may skip the local review; everything else follows step 1 first
5. **Mark ready and wait**: Once the branch is final and the local review is clean, mark the PR ready for review (or push the non-draft PR). Wait 5 minutes, then check the PR's commit status; once checks pass, fetch and read all CodeRabbit review comments
6. **If the PR review is clean**: Merge the PR and delete the remote and local feature branch; do not wait for confirmation
7. **If the PR review has findings**: Convert the PR back to draft, fix locally, and go back to step 1. Each round of ready-for-review costs one more CodeRabbit review, so batch the fixes into one round

Skip the local review ONLY if explicitly requested.

## Budgeting the CodeRabbit limit

- Local iteration is free: the pre-push gate is Claude, with no hourly cap
- A PR costs one CodeRabbit review per ready-for-review round when the draft flow above is followed; a PR whose review finds issues costs one more per fix round
- If the limit is hit anyway, keep working in drafts and wait for the reset; do not merge without a CodeRabbit PR review

## CodeRabbit tips (PR gate only)

- **Defer findings**: reply to any review comment with `@coderabbitai create a GitHub issue for this` to defer to follow-up issues
- **Auto-paused reviews**: CodeRabbit auto-pauses after many commits; use `@coderabbitai resume` to un-pause
- **Paginated reviews**: use `?per_page=100` when fetching via API: `gh api 'repos/{owner}/{repo}/pulls/{number}/reviews?per_page=100'`
- **Duplicate comments**: means the issue was previously flagged and is still unfixed
- **State**: `COMMENTED` = no actionable items; `CHANGES_REQUESTED` = has actionable comments
