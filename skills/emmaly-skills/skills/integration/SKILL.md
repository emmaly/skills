---
name: integration
description: This skill should be used when pushing code, opening a PR, or merging. Covers the mandatory local Claude review loop, the push/merge process, and the CodeRabbit PR gate.
---

Once implementation is complete and local verification passes (`go vet`, `gofmt`, tests, frontend build), proceed directly through this workflow without waiting for further instruction:

**CRITICAL: The local review gate is the ready-for-review transition.** Never mark a PR ready for review, and never push to a non-draft PR, without a clean local review of the branch first. Pushes to a draft PR are not gated; a draft is working state. The local review is Claude's built-in `code-review` skill, not CodeRabbit. CodeRabbit reviews only the ready PR, as a final third-party gate before merging. Its 5 reviews/hour limit is shared by PR reviews, `coderabbit review` CLI runs, and manual `@coderabbitai review` triggers, so only the PR gate may spend it: no CodeRabbit CLI runs and no CodeRabbit-plugin review skills or agents during a session. Any in-session review request goes to the built-in `code-review` skill.

1. **Commit everything first**: The review must see the exact tree that will be pushed, so commit the change, including any new files. Before committing new files, run `git ls-files --others --exclude-standard` and read the list; scratch files and anything secret that is not gitignored must not be committed
2. **Local review**: Invoke the built-in `code-review` skill at high effort against the branch's diff from its PR base. `git fetch` first and diff against the remote base (`origin/main...HEAD`); a stale local `main` inflates the scope with already-merged commits and mismatches the diff CodeRabbit will see
3. **Address findings**: Fix actionable issues; file GitHub issues for deferred items
4. **Re-review if changed**: If step 3 produced commits, review again before proceeding. When the fix is small, this round may target just the changes since the last review, at lower effort; a large rework repeats the full review. The gate is clean when the review reports no findings other than items already deferred to GitHub issues
5. **Push and update or open the PR**: If a PR already exists, push to it. Otherwise, if more pushes are likely (iterating on CI, expecting follow-ups), open the PR as a **draft** and iterate there; with the gate clean and no more pushes expected, push and open it ready for review. CodeRabbit skips drafts unless the repo's `.coderabbit.yaml` sets `reviews.auto_review.drafts: true`; check for that override once per repo
6. **Mark ready and wait**: Once the branch is final and the gate is clean, mark the draft ready for review. Wait 5 minutes, then check the PR's commit status; once checks pass, fetch and read all CodeRabbit review comments
7. **If the PR review is clean**: Merge the PR and delete the remote and local feature branch; do not wait for confirmation
8. **If the PR review has findings**: Convert the PR back to draft, fix locally, and go back to step 1. Batch the fixes into one ready-for-review round

Skip the local review gate, or merge without the CodeRabbit PR review, ONLY if explicitly requested.

## Budgeting the CodeRabbit limit

Each ready-for-review round costs one CodeRabbit review; local iteration and draft pushes cost none. If the limit is hit, keep working in drafts until it resets.

## CodeRabbit tips (PR gate only)

- **Defer findings**: reply to any review comment with `@coderabbitai create a GitHub issue for this` to defer to follow-up issues
- **Auto-paused reviews**: CodeRabbit auto-pauses after many commits; use `@coderabbitai resume` to un-pause
- **Paginated reviews**: use `?per_page=100` when fetching via API: `gh api 'repos/{owner}/{repo}/pulls/{number}/reviews?per_page=100'`
- **Duplicate comments**: means the issue was previously flagged and is still unfixed
- **State**: `COMMENTED` = no actionable items; `CHANGES_REQUESTED` = has actionable comments
