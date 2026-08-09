# Next — emmaly-skills

Status as of 2026-08-09: laptop work recovered and merged. Standards now load via
the SessionStart hook; the `deploy` skill is gone.

## Where things stand

- `hooks/emit-standards.sh` prints `skills/standards/SKILL.md` into every session
  (startup, resume, clear, **compact**). This is the *only* mechanism that loads
  the standards — the `<!-- emmaly:standards -->` block was removed from
  `~/.claude/CLAUDE.md`, and the `apply-standards` skill that wrote it is deleted.
  If the standards ever appear twice in context, something re-added that block.
- The plugin is installed from a **directory** marketplace pointing at
  `~/Projects/emmaly` itself, so changes on `main` are live after a
  `/plugin` reload — there is no publish step.

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
