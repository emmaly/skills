# Next — emmaly-skills

Status as of 2026-08-09: laptop work recovered and merged. Standards now load via
the SessionStart hook; the `deploy` skill is gone.

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

- **Kubernetes deploy skill — designed, then deliberately deferred 2026-08-09.**
  The `deploy` skill (podman-compose over SSH to a single remote host) was
  removed rather than ported; on-prem now means the k3s cluster in
  `~/Projects/kube`.

  Two things were settled in that conversation, so the next attempt does not
  restart from zero:

  1. **The skill's job would be steady-state cluster conventions** — the rules
     that decide whether a deployment is correct — not the podlap migration
     procedure and not day-2 kubectl recipes.
  2. **`docs/MIGRATING-A-PROJECT.md` already has the seam to cut along.** Lines
     25–875 are durable conventions (non-negotiable rules, what the cluster does
     and does not provide, required layout, storage, images, secrets, networking,
     pod hygiene, probes, resources). Line 876 onward is `## The migration
     procedure`, which is podlap-specific and dies with the wipe. The first half
     is the skill; the second half is not.

  **Deferred until after the podlap wipe**, because `unifi` and
  `charmcrafterlite` + `charmy-webfetch` are still unmigrated and are exactly the
  shapes that would rewrite conventions — UDP that an HTTP Ingress cannot carry,
  adopted devices holding an inform URL, and a pair that is not containerised at
  all. Freezing rules into a skill before those two land would bake in conventions
  they are about to challenge. Let events settle the seam rather than judgement.

  Note the earlier caution here — "the conventions are not yet frozen, 2 of 18
  services" — is superseded: thirteen have migrated, and
  `docs/MIGRATION-STATUS.md` now states the contract is done being written by
  migrations. The reason to wait is the two unusual shapes, not immaturity.

  When picking this up, the open question is where the conventions should live:
  move them out of the kube contract into the skill (one source of truth, but a
  second PR against `~/Projects/kube`), or keep kube authoritative and inline only
  the decisive rules. Do not simply duplicate them — that is the drift failure
  this plugin just spent a session removing.
- `standards/SKILL.md` → "Deployment Targets" still needs a Kubernetes-shaped
  answer once that skill exists; right now it just points at `~/Projects/kube`.
