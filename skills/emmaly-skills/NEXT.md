# Next: emmaly-skills

Status as of 2026-08-24: `plain-language` rewritten after real use. Emmaly's
complaint was verbosity first, AI-isms second, so the skill now leads with a
"Length comes first" section (answer in sentence one, six-line default reply, no
unrequested summary or next-steps sections) and adds a "No drama" section for
narrative tells like "here's where it gets interesting" and personified code.
Ideas were taken from `~/Downloads/unslop.md` and reworded, not copied. Released
as `20260824001`.

Status as of 2026-08-24, later the same day: an efficacy review of
`plain-language` found seven gaps and all seven are fixed in `20260824002`.
The one measurement worth keeping: em dashes appeared in 15 of the 32 commit
messages written before 2026-08-18, when the skill landed, and in 0 of the 11
written after. That is the only hard adherence number this repo has.

What the review changed:

1. **Precedence section.** The skill contradicted the harness, which can require
   a narration line, a restatement, or a `result:` marker, and contradicted the
   PR template in `git-workflow`. Nothing said which wins. Now a required shape
   wins, and the rules apply inside it.
2. **Severity has a sanctioned form.** Banning `crucial`, `vital`, and stakes
   talk left no way to flag real danger. The fix is in `No drama`: name the
   trigger and the consequence, and put it in the first sentence. The emphasis
   words stay banned as substitutes for the fact, not as a rule against warning
   anyone.
3. **Terms of art are exempt.** The ban list hit attack vector, first-class
   function, comprehensive test coverage, and the agent harness. The old
   exception covered identifiers only. Test is now whether a reader in that
   field would hear a substitution as a different claim.
4. **`Short-form text` section.** The description promised error messages, log
   lines, and UI copy; every rule in the body was chat-shaped.
5. **A hook now enforces the dash rule.** See below.
6. **`If you follow nothing else`.** 49 rules with flat priority, roughly 40 of
   them single words, meant the three that matter competed with the rest.
7. **The revision pass is a checklist, not a loop.** "Repeat until a pass
   changes nothing, up to three" was unverifiable and cheap to claim.

- **`hooks/plaincheck` is the only mechanical gate.** PostToolUse on
  Write/Edit/MultiEdit checks prose files, PreToolUse on Bash checks
  `git commit`. It looks for em and en dashes and nothing else, because that is
  the one rule with no false positives. Word-list and sentence-shape checks would
  fire on the ban list itself, on quoted tool output, and on every legitimate
  term of art, so those stay with the model. Two deliberate narrowings: it reads
  only the text being written, never the rest of the file, so editing an old
  document does not block unrelated work, and in prose files it skips backticks
  and fenced blocks, which is the escape hatch for quoting a dash. That hatch is
  prose-only: the commit-message branch scans the raw command, and a file passed
  to `-F`, so there is no way to quote a dash into a commit message. Run
  `go test ./...` in `hooks/plaincheck` before changing it. The false-positive
  cases matter more than the catches: this hook fires on every Write and every
  Bash call, so it is only worth having while it stays quiet.
- **`hooks/run-plaincheck.sh` builds the checker on first use** into
  `${XDG_CACHE_HOME:-~/.cache}/emmaly-skills/plaincheck`, then execs it, and
  rebuilds whenever a `.go` file is newer than the binary. Hooks need an
  executable to call and a compiled binary cannot ship in the repo: it would be
  platform-specific and would go stale against its own source. Measured after
  the first build, 20 invocations took 94ms, so roughly 5ms per tool call.
  Verified by hand, since it is the one piece the Go tests cannot reach: cold
  build blocks correctly, a touched source file triggers a rebuild, and both an
  absent Go toolchain and a broken build exit 0 with a note on stderr rather
  than blocking the work. That last part matters. A gate that fails closed on a
  missing compiler would be worse than no gate.

**In progress:** watch whether replies actually get shorter. If they do not, the
next lever is a harder cap in the length section, not more banned words. The
banned lists are still a living list; add tells as they slip through.

**Backfilled the same day.** The repo held 61 em and en dashes across 11 prose
files, all written before 2026-08-18: 8 in skill frontmatter descriptions, 49 in
prose bodies, 4 inside a fenced example. The count matters less than where they
sat. Only the 8 descriptions loaded every session, in the skill listing, so they
were the ones actually competing with the rule. Emmaly chose to convert all 61
anyway, `api-explorer` included, on the grounds that it is the same repo under
the same rules. The repo is now at zero, and the hook keeps it there.

Two conventions came out of that pass, and they are worth keeping:

- **A definition-list dash becomes a colon.** A bolded term at the start of a
  list item is followed by `:`, never by a dash. The alternative was to carve
  that pattern out of both the skill and the hook. Zero with no exceptions is
  what makes the rule cheap to follow and cheap to check, and 13 lines was not
  worth spending that on.
- **A quoted literal goes in backticks.** `NEXT.md` quotes its own earlier
  wording, `lines 25-875`. Backticks mark it as the literal string it is, and
  they are also the hook's escape hatch, so quoting a dash stays possible.

**Go only, as of 2026-08-24.** `standards/SKILL.md` gained a `## Language
choice` section, and it is a rule rather than a stack preference. The old
wording was a "Preferred Stack" bullet listing Go, which reads as a
recommendation, so LLM output kept reaching for Python on anything framed as a
script or a one-off. That is exactly how the plain-language hook got written in
Python in the first place, in this very repo, hours before the rule landed.

The repo now has no Python at all. The checker is Go with `go test` coverage,
and the Home Assistant WebSocket example was rewritten in Go against
`gorilla/websocket`, verified by extracting the block and building it. One
Python mention survives on purpose: `api-explorer` lists Go, Python, and JS/TS
as SDK languages worth reading for API documentation. Reading is not writing,
and the rule says so.

If a future session argues that a quick script is the exception, it is not. That
argument is named in the rule.

## Where things stand

- `hooks/emit-skill-body.sh <skill-dir> <heading>` prints a skill body into every
  session (startup, resume, clear, **compact**). `hooks.json` calls it twice, for
  `standards` and for `plain-language`. It was `emit-standards.sh` until
  2026-08-18, hardcoded to one skill; generalising it was cheaper than a second
  near-identical script. It is the only *automatic* loader. The
  `<!-- emmaly:standards -->` block was removed from `~/.claude/CLAUDE.md` and the
  `apply-standards` skill that wrote it is deleted. `standards` is still an
  invokable skill, so invoking it explicitly does put the same body in context a
  second time; that is a deliberate escape hatch, not a bug, and its description
  says to invoke it only when asked what the standards are. The same applies to
  `plain-language`.
- **`plain-language` is always-on by design.** It governs every human-language
  output: chat, commit messages, PR bodies, READMEs, comments, UI copy, error and
  log strings. It is emitted rather than left as an on-demand skill because a
  skill only invoked when someone remembers it would never fire on the outputs
  that need it most. Cost of that choice: roughly 800 words of context per
  session on top of the standards. If context pressure ever forces a trim, cut
  the word list before cutting the revision pass. The pass is what makes it
  iterate.
- **A new machine needs this plugin and nothing else.** The `## Working style`
  bullets moved out of `~/.claude/CLAUDE.md` into `standards/SKILL.md` on
  2026-08-09, and that file is now empty. Install the marketplace, enable
  `emmaly-skills`, and every universal rule loads. The dotfiles repo is no longer
  required to get moving. The tradeoff: `CLAUDE.md` used to load unconditionally,
  whereas the hook only fires when the plugin is enabled. Put machine-specific
  instructions in `CLAUDE.md`; put anything universal in the skill.

## Releasing a change (do not skip)

The plugin is **not** served live from `~/Projects/emmaly`. Installing copies the
tree into a version-keyed cache at
`~/.claude/plugins/cache/emmaly/emmaly-skills/<version>/`, and that copy is what
loads. The version string is the refresh trigger:

1. Bump `version` in **both** `.claude-plugin/marketplace.json` and
   `skills/emmaly-skills/.claude-plugin/plugin.json` (format `YYYYMMDDNNN`).
2. `/plugin marketplace update emmaly`
3. `/plugin update emmaly-skills@emmaly`
4. Restart the session. SessionStart hooks are registered at launch.

Verify by asserting the *exact* new version directory exists. Listing the parent
will happily show you the old one and look like success. Read the expected
version out of the manifest rather than typing it, or this check goes stale on
the next bump and starts passing against the previous release:

```sh
v=$(jq -r .version ~/Projects/emmaly/skills/emmaly-skills/.claude-plugin/plugin.json)
test -d ~/.claude/plugins/cache/emmaly/emmaly-skills/"$v" && echo "ok: $v"
```

Both `.claude-plugin/marketplace.json` and `skills/emmaly-skills/.claude-plugin/plugin.json`
declare a version, and this procedure bumps them together. Which one the resolver
actually reads is unconfirmed. In the official marketplace the entry-level
`version` is optional (14 of 284 entries set it). Keeping them in lockstep is
correct either way; do not drop one without testing which field drives the
refresh, because getting this wrong strands every session on a stale build with
no visible symptom.

**This has bitten before.** The commits of 2026-06-18 changed skills without
bumping the version, so the cache stayed on `20260415001` and none of it ever
loaded. If a skill edit seems to have no effect, check the cache directory before
debugging anything else. For iterating without a release, run
`claude --plugin-dir ./skills/emmaly-skills`.

## To do

- **Kubernetes deploy skill: designed 2026-08-09, deferred, un-deferred
  2026-08-11.** The `deploy` skill (podman-compose over SSH to a single remote
  host) was removed rather than ported; on-prem now means the k3s cluster in
  `~/Projects/kube`, and that is now the *default* target for new work rather
  than one option among several.

  Two things were settled in that conversation, so the next attempt does not
  restart from zero:

  1. **The skill's job would be steady-state cluster conventions.** That means
     the rules that decide whether a deployment is correct, not the podlap
     migration procedure and not day-2 kubectl recipes.
  2. **`docs/MIGRATING-A-PROJECT.md` already has the seam to cut along.**
     Everything from `## The rules that are not negotiable` through
     `## Pod hygiene` is durable convention: what the cluster does and does not
     provide, required layout, storage, images, secrets, networking, pod hygiene.
     From `## The migration procedure` onward is podlap-specific and dies with
     the wipe. The first half is the skill; the second half is not.

     Cut along the **headings**, not line numbers. This note originally said
     `lines 25-875` and the seam had moved to 905 within two days, because every
     migration patches the contract.

  **No longer deferred, as of 2026-08-11.** Kubernetes is now the default
  deployment target (`standards/SKILL.md` says so), so the skill should be
  written rather than waited on. What changed: `charmcrafterlite` and
  `charmy-webfetch` migrated together as one pod on 2026-08-09, and that round
  produced **five** contract fixes, the smallest of any migration. The series
  ran 10, 3, 8, 8, 9, 7, 11, 7, 9, 9, 12, 5. The conventions have converged.

  **Confirm two things first, because the ledger and the room disagree.**
  Emmaly reports on 2026-08-11 that `unifi` migrated and that podlap is probably
  empty, pending a manual check. `~/Projects/kube/docs/MIGRATION-STATUS.md` does
  not agree: as of commit `950850d` it still lists `unifi` as deferred in three
  places, including its row in the services table, and the newest commit there is
  the charmcrafterlite round. That ledger calls itself "the only thing that
  survives a fresh session", so if `unifi` did move, **the gap is in the ledger,
  not here**. Fix it there first. `unifi` also matters to the skill's content:
  it is the one workload needing UDP that an HTTP Ingress cannot carry, so how it
  was solved belongs in the non-HTTP section.

  When picking this up, the open question is where the conventions should live:
  move them out of the kube contract into the skill (one source of truth, but a
  second PR against `~/Projects/kube`), or keep kube authoritative and inline only
  the decisive rules. Do not simply duplicate them. That is the drift failure
  this plugin just spent a session removing.
- `standards/SKILL.md` → "Deployment Targets" now names Kubernetes as the
  default and carries the shape (project-repo manifests, `ghcr.io/emmaly/*`
  images, Longhorn storage, `route.sh` + Ingress). Revisit once the skill exists,
  so the standards can point at it instead of at `~/Projects/kube` directly.
