---
name: integration
description: This skill should be used when pushing code, opening a PR, or merging. Covers the mandatory local CodeRabbit review loop, push/merge process, and PR-gate conventions.
---

Once implementation is complete and local verification passes (`go vet`, `gofmt`, tests, frontend build), proceed directly through this workflow without waiting for further instruction:

**CRITICAL: Never push code to GitHub without a clean local CodeRabbit review first.** Every push (whether the initial PR push or a follow-up fix) must be preceded by a passing `coderabbit review --base main`. The PR-level CodeRabbit review exists only as a final gate before merging as a third-party proof/receipt, not as the primary review. If code reaches GitHub with issues that local review would have caught, the workflow has failed.

1. **Local CR review** (MANDATORY before any push): Run `coderabbit review --base main` on the feature branch. Untracked files are NOT reviewed, so a new file would otherwise reach GitHub unreviewed. `git add` the files the change introduces before reviewing, which is enough to bring them into scope. Prefer that over `--include-untracked`: that flag pulls in *every* non-ignored untracked file in the tree, including scratch files and anything secret that is not gitignored, and sends them off-machine. If you do use it, run `git ls-files --others --exclude-standard` first and read the list. Reviews can take several minutes; allow a generous timeout rather than killing the run
2. **Address findings**: Fix actionable issues; file GitHub issues for deferred items
3. **Re-review if changed**: If step 2 produced commits, go back to step 1. Do NOT push until local review is clean
4. **Push and open PR**: Only after local review reports no findings, push the branch and open/update the PR
5. **Wait for PR review**: Wait 5 minutes, then check the PR's commit status; once checks pass, fetch and read all CodeRabbit review comments
6. **If the PR review is clean**: Merge the PR and delete the remote and local feature branch; do not wait for confirmation
7. **If the PR review has findings**: Fix locally, go back to step 1 (local CR review); do NOT push incremental fixes without a clean local review first

Skip CodeRabbit review ONLY if explicitly requested.

## CodeRabbit Tips

- **Scope flags**: `coderabbit review` defaults to tracked changes. `--committed` reviews only committed changes, `--uncommitted` only staged/unstaged edits, `--include-untracked` adds every non-ignored untracked file (see the caveat in step 1). Plain text is the default output; there is no `--plain` flag (it was removed, and passing it errors out). `--agent` emits structured findings, which is the better choice when Claude is driving
- **Defer findings**: reply to any review comment with `@coderabbitai create a GitHub issue for this` to defer to follow-up issues
- **Auto-paused reviews**: CodeRabbit auto-pauses after many commits; use `@coderabbitai resume` to un-pause
- **Paginated reviews**: use `?per_page=100` when fetching via API: `gh api 'repos/{owner}/{repo}/pulls/{number}/reviews?per_page=100'`
- **Duplicate comments**: means the issue was previously flagged and is still unfixed
- **State**: `COMMENTED` = no actionable items; `CHANGES_REQUESTED` = has actionable comments
