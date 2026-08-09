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

Verify with `ls ~/.claude/plugins/cache/emmaly/emmaly-skills/` — a new version
directory should exist.

**This has bitten before.** The commits of 2026-06-18 changed skills without
bumping the version, so the cache stayed on `20260415001` and none of it ever
loaded. If a skill edit seems to have no effect, check the cache directory before
debugging anything else. For iterating without a release, run
`claude --plugin-dir ./skills/emmaly-skills`.

## To do

- **Kubernetes deploy skill.** The `deploy` skill (podman-compose over SSH to a
  single remote host) was removed rather than ported; on-prem now means the k3s
  cluster in `~/Projects/kube`. Before writing a replacement, read that repo's
  `README.md` and `docs/MIGRATING-A-PROJECT.md` — it already carries the real
  procedure (Ingress + `cloudflared/route.sh <host>`, Longhorn volumes, GHCR
  images). The skill should point at those docs, not restate them; the migration
  is still in progress (2 of 18 services as of 2026-08-09), so the conventions
  are not yet frozen.
- `standards/SKILL.md` → "Deployment Targets" still needs a Kubernetes-shaped
  answer once that skill exists; right now it just points at `~/Projects/kube`.
