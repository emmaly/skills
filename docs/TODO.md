# TODO

Written 2026-09-01 from a full read of the three plugins, their docs, and
`docs/superpowers/`. Everything here was checked against the code, not inferred
from the prose. Items are grouped by what breaks if they are left alone.

Working state when this was written: branch `feat/tts-mode-instructions`, one
commit (`2888f3f`) plus uncommitted edits in `tts-mode/`. Superseded; see
the next section.

## Since this was written

The repo was split on 2026-09-01. `emmaly/emmaly` was renamed to
`emmaly/skills`, a new public `emmaly/emmaly` was created holding only the
profile README, and the local checkout moved from `~/Projects/emmaly` to
`~/Projects/skills`. Branch `chore/split-skills-repo` carries the repo-side
work: the marketplace README, the new manifest URLs, and the flatten that moved
the three plugins out of `skills/` to the root. It is committed locally and not
pushed. Paths below use the flattened layout. Items 14, 22, 23 and 24 record
what that left open.

On 2026-09-02 the split landed on `main` and was pushed, the tts-mode WIP was
committed (`7467f57`), and `fix/tts-mode-queue` merged as `2bd7c4e`: spoken
lines are chunked at sentence ends, queued per user so sessions never overlap,
detached under `setsid`, capped at 1,200 characters at a word boundary, and
played with a 350 ms gap between clips. Items 11, 23, and 24 closed with that.
Item 1 is now the blocker: `main` carries behavior the version string does
not.
## Correctness

1. **Bump `tts-mode` now; `main` is ahead of its version string.** Both
   `.claude-plugin/marketplace.json` and
   `tts-mode/.claude-plugin/plugin.json` still read `20260827001`, last
   touched in `d752022`. `2888f3f`, `7467f57`, `1958cb1`, and `aa22236` all
   change behavior without a bump, so the version-keyed cache keeps serving
   the old build and none of it loads. `NEXT.md` calls this out as a failure
   that has already bitten this repo once, in June. The branches have landed
   (2026-09-02), so this is one commit: set both to `20260903001`.

2. **`run-plaincheck.sh` fails the tool call when `HOME` is unset.** Line 18
   expands `${HOME}` under `set -u`, so the shell exits before the `ERR` trap
   can reach `give_up`. Verified: `env -u HOME bash hooks/run-plaincheck.sh`
   prints `HOME: unbound variable` and exits 1. The wrapper's own comment says
   it must never fail the call it was only meant to inspect. Fix is the pattern
   already in `run-ttsmode.sh`: read `HOME_DIR="${HOME:-}"` and refuse
   explicitly.

3. **`run-plaincheck.sh` accepts a relative `XDG_CACHE_HOME`.**
   `run-ttsmode.sh` refuses one and documents why: the cache holds a binary the
   script then execs, and a relative path resolves against whatever directory
   the hook ran in, which is a tree someone else can pre-create. Same exposure
   here, same fix. Backport the absolute-path loop.

4. **`run-plaincheck.sh` misses staleness in `go.mod` and `go.sum`.** Its
   rebuild check watches `*.go` only (line 43). `run-ttsmode.sh` watches all
   three, because a Go directive or dependency bump changes the build with no
   `.go` file touched, and the cached binary keeps running with no sign it is
   stale.

## Tests

5. **Four new behaviors in `control.go` have no test.** `control_test.go` is
   unchanged by the working tree, and none of these are covered:
   - `longestKeyword+1` now catches insertion typos. Before it, `statuss` was
     seven characters against a flat cap of six and was waved through as a
     request.
   - The prefix rule is abbreviation-only now, so `once`, `only`, and `offer`
     survive as real one-word instructions.
   - `CURRENT_INSTRUCTIONS` is emitted when the session already has stored
     text.
   - `/tts on <request>` enables before handing off to the rewrite step.

   The first two are the ones worth table tests: they are the exact cases the
   old rule got wrong in both directions.

## Documentation accuracy

6. **The tts-mode spec still says it is unbuilt.**
   `docs/superpowers/specs/2026-08-27-tts-mode-design.md:4` reads
   `Status: approved, not yet implemented`. The plan file beside it carries a
   proper superseded banner; the spec does not, and it is the file someone
   reads first. Stale claims inside it:
   - "three subcommands". There are nine (`main.go:6`).
   - Session id order is reversed. The spec puts `CLAUDE_CODE_SESSION_ID`
     first and `--session` second; the code is `--session`, then the payload,
     then the environment (`main.go:18`).
   - API base is given as `https://api.us.elevenlabs.io`. `speak.go:28` uses
     `https://api.elevenlabs.io`, and the README says the default is the
     generic host. The plan repeats the wrong host in its Global Constraints.
   - Under Security: "`ttsmode say` takes its text as an argument, so the
     summary is visible in the transcript". Reversed by `c039c8f`. The line now
     goes in on stdin through a quoted heredoc, and that is a security property
     rather than an implementation detail.
   - The Files list omits `control.go`, `state.go`, `speak.go`, `secret.go`,
     `tts-say.sh`, and `run-ttsmode.sh`.
   - Nothing about freeform instructions, which is now half the surface.

   Cheapest fix is a banner like the plan's, plus corrections to the host and
   the session order, since those two get copied into code.

7. **api-explorer Phase 3 contradicts its own numbering.** The list is
   introduced as "Try these in order, stopping when you have a usable spec".
   Item 5 is HTML doc sites, described as "the least reliable path, and the last
   one to reach for", telling the reader to work through "steps 6 through 10"
   first. So a reader following the order stops at the worst source, and the
   cross-reference also skips step 11, the Wayback Machine. Move HTML to the
   end and renumber.

8. **api-explorer hardcodes `~/.cache/api-explorer/`.** Both Go hooks in this
   repo honor `XDG_CACHE_HOME`. Pick one convention across the marketplace, and
   say which.

9. **tts-mode README `Layout` is out of order and incomplete.** It lists
   `hooks/ttsmode/control.go` before `hooks/run-ttsmode.sh` and then
   `hooks/ttsmode/` after both. It omits `.claude-plugin/plugin.json` and the
   other Go files. Either list the directory once or list every file.

10. **tts-mode README `Use` block omits `/tts status`.** It shows bare `/tts`
    for the same effect, but `status` is a real keyword (`control.go:31`), and
    typing it is what someone tries first.

11. **Done 2026-09-02.** The cap is 1,200 characters per say, cut at a word
    boundary (`chunk.go`), and the README names it under Use and under How a
    line is spoken. The Cost section still reasons from fifteen words; a
    session that raises the line length pays accordingly, and the cap is the
    ceiling.

12. **`commands/tts.md` overstates what the delimiter suffix buys.** Lines 19
    to 21 say the suffix means "text someone pastes cannot end the heredoc
    early". The delimiter is a fixed constant published in this repo, so a paste
    containing it still closes the heredoc. Two ways out: restate the real
    property, which is that accidental collision is now unlikely, or draw the
    delimiter per call for `control` specifically. Note that `hook.go:105`
    records why a random delimiter was tried and reverted, but that reasoning
    is about the line the model writes itself. It does not carry over to input
    a person pastes from somewhere else.

13. **The enable-early asymmetry is undocumented.** `/tts on <request>` turns
    TTS on before the rewrite step (`control.go:84`); a bare `/tts <request>`
    does not. If the model skips step 2 of `tts.md`, the first form is on with
    whatever instructions were already stored and the second stays silently
    off. Document it in the README, or make both forms agree.

## Repo and conventions

14. **Done 2026-09-01.** The root README is now the marketplace README, with
    the install line, the plugin table, and a pointer at the release procedure.
    The profile README moved to `emmaly/emmaly`.

15. **No `AGENTS.md` or `CLAUDE.md` at the repo root.** `project-setup` asks
    every project for both, and this repo is the one that ships that rule.

16. **`standards/SKILL.md` names only the emmaly-skills skills.** The
    "Emmaly Plugin Skills" section does not mention that the same marketplace
    ships `api-explorer` and `tts-mode`. That section is injected into every
    session by the SessionStart hook, so it is the one place the omission
    actually costs something.

17. **The release procedure is written for one plugin.** `NEXT.md` under
    "Releasing a change (do not skip)" names the emmaly-skills manifests by
    path. Three plugins now ship from this repo with independent version
    strings. Generalize the procedure, or give each plugin its own copy.

18. **No breadcrumb for `tts-mode` or `api-explorer`.** The working-style rule
    in `standards` asks for a NEXT or status breadcrumb at the end of each work
    chunk, and only `emmaly-skills` has one. `tts-mode` is no longer parked
    mid-feature (everything is on `main` as of 2026-09-02), so its breadcrumb
    is short: bump the version (item 1), add the `control.go` tests (item 5),
    consider ElevenLabs streaming only if first-piece latency still bothers
    anyone.

19. **Branch naming does not match the documented convention.**
    `git-workflow` prescribes `fix/<number>-short-desc` or
    `feature/<number>-short-desc`. Live branches are `feat/tts-mode-plugin`,
    `feat/tts-mode-instructions`, and `feat/claude-local-review-gate`: `feat/`
    rather than `feature/`, and no issue number. Practice has been consistent
    for months, so the convention text is probably the thing to change.

20. **`chore/refresh-next-and-go-version` is parked and genuinely unmerged.**
    Five commits of standards work that are not in `main`, including four
    exposure levels, an always-ask-before-publishing rule, and an admission that
    Kubernetes work needs the kube repo. Its worktree is clean at `70f1b48`.
    Decide whether it lands or gets dropped; leaving it is the case the
    working-style rule warns about.

21. **Seven worktrees under `.claude/worktrees/`, all clean.**
    `tts-queue` (`fix/tts-mode-queue`) and `chore-split-skills-repo` are merged
    into `main` as of 2026-09-02 and can go.
    `plain-language-trim` and `feat-claude-local-review-gate` are merged into
    `main` and can go. `plain-language-eval-fixes` was squash-merged as `#10`,
    so it only looks unmerged. `recover-and-hook` holds item 20.
    `plain-language-skill` sits on `local/asd-ste100-do-not-push`, whose tip
    commit removes the skill from this repo and says it moved to a local-only
    marketplace: confirm that landed somewhere before removing the worktree.

22. **Re-add the marketplace on every machine that has it.** GitHub's rename
    redirect for `emmaly/emmaly` was replaced by the new profile repo, which
    carries no `marketplace.json`. Anything still pointing at the old name
    resolves to the profile page and finds nothing. Run
    `/plugin marketplace remove emmaly` then
    `/plugin marketplace add emmaly/skills`, and confirm the three plugins
    still resolve as `<plugin>@emmaly`, since the marketplace name did not
    change.

23. **Done 2026-09-02.** The split merged first, the WIP was committed as
    `7467f57`, and the queue branch merged over the rename; git followed the
    moves and the added files were staged at the new paths.

24. **Done 2026-09-02.** `origin/main` is `2bd7c4e`, the flattened layout.

## Not a problem, checked

- Both `go.mod` files are on `go 1.26`, matching the standards.
- Zero em and en dashes across all tracked Markdown. The hook is holding.
- Version strings agree between `marketplace.json` and each `plugin.json`.
- The voice settings in `speak.go` match the README and the spec exactly.
