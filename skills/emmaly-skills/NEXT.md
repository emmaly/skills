# Next — emmaly-skills

Status as of 2026-08-11: laptop work recovered and merged. Standards now load via
the SessionStart hook — verified firing in a real session — and the `deploy`
skill is gone.

## Where things stand

- `hooks/emit-standards.sh` prints `skills/standards/SKILL.md` into every session
  (startup, resume, clear, **compact**). It is the only *automatic* loader — the
  `<!-- emmaly:standards -->` block was removed from `~/.claude/CLAUDE.md` and the
  `apply-standards` skill that wrote it is deleted. `standards` is still an
  invokable skill, so invoking it explicitly does put the same body in context a
  second time; that is a deliberate escape hatch, not a bug, and its description
  says to invoke it only when asked what the standards are.
- **A new machine needs this plugin and nothing else.** The `## Working style`
  bullets moved out of `~/.claude/CLAUDE.md` into `standards/SKILL.md` on
  2026-08-09, and that file is now empty — install the marketplace, enable
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
4. Restart the session — SessionStart hooks are registered at launch.

Verify by asserting the *exact* new version directory exists — listing the parent
will happily show you the old one and look like success. Read the expected
version out of the manifest rather than typing it, or this check goes stale on
the next bump and starts passing against the previous release:

```sh
v=$(jq -r .version ~/Projects/emmaly/skills/emmaly-skills/.claude-plugin/plugin.json)
test -d ~/.claude/plugins/cache/emmaly/emmaly-skills/"$v" && echo "ok: $v"
```

Both `.claude-plugin/marketplace.json` and `skills/emmaly-skills/.claude-plugin/plugin.json`
declare a version, and this procedure bumps them together. Which one the resolver
actually reads is unconfirmed — in the official marketplace the entry-level
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

- **Kubernetes deploy skill — designed 2026-08-09, deferred, un-deferred
  2026-08-11.** The `deploy` skill (podman-compose over SSH to a single remote
  host) was removed rather than ported; on-prem now means the k3s cluster in
  `~/Projects/kube`, and that is now the *default* target for new work rather
  than one option among several.

  Two things were settled in that conversation, so the next attempt does not
  restart from zero:

  1. **The skill's job would be steady-state cluster conventions** — the rules
     that decide whether a deployment is correct — not the podlap migration
     procedure and not day-2 kubectl recipes.
  2. **`docs/MIGRATING-A-PROJECT.md` already has the seam to cut along.**
     Everything from `## The rules that are not negotiable` through
     `## Pod hygiene` is durable convention — what the cluster does and does not
     provide, required layout, storage, images, secrets, networking, pod hygiene.
     From `## The migration procedure` onward is podlap-specific and dies with
     the wipe. The first half is the skill; the second half is not.

     Cut along the **headings**, not line numbers. This note originally said
     "lines 25–875" and the seam had moved to 905 within two days, because every
     migration patches the contract.

  **No longer deferred, as of 2026-08-11.** Kubernetes is now the default
  deployment target (`standards/SKILL.md` says so), so the skill should be
  written rather than waited on. What changed: `charmcrafterlite` and
  `charmy-webfetch` migrated together as one pod on 2026-08-09, and that round
  produced **five** contract fixes, the smallest of any migration — the series
  ran 10, 3, 8, 8, 9, 7, 11, 7, 9, 9, 12, 5. The conventions have converged.

  **Confirm two things first, because the ledger and the room disagree.**
  Emmaly reports on 2026-08-11 that `unifi` migrated and that podlap is probably
  empty, pending a manual check. `~/Projects/kube/docs/MIGRATION-STATUS.md` does
  not agree: as of commit `950850d` it still lists `unifi` as deferred in three
  places, including its row in the services table, and the newest commit there is
  the charmcrafterlite round. That ledger calls itself "the only thing that
  survives a fresh session", so if `unifi` did move, **the gap is in the ledger,
  not here** — fix it there first. `unifi` also matters to the skill's content:
  it is the one workload needing UDP that an HTTP Ingress cannot carry, so how it
  was solved belongs in the non-HTTP section.

  When picking this up, the open question is where the conventions should live:
  move them out of the kube contract into the skill (one source of truth, but a
  second PR against `~/Projects/kube`), or keep kube authoritative and inline only
  the decisive rules. Do not simply duplicate them — that is the drift failure
  this plugin just spent a session removing.
- `standards/SKILL.md` → "Deployment Targets" now names Kubernetes as the
  default and carries the shape (project-repo manifests, `ghcr.io/emmaly/*`
  images, Longhorn storage, `route.sh` + Ingress). Revisit once the skill exists,
  so the standards can point at it instead of at `~/Projects/kube` directly.
