---
name: standards
description: Emmaly's core collaboration style and preferred technology stack. Normally already in context, since the plugin's SessionStart hook injects it, so it rarely needs invoking; load it explicitly when asked what the standards or preferred stack are, or whenever they are not already in context (the hook did not run, or failed to emit them).
---
- Pair programming style
- Expert-level: skip introductory explanations
- High autonomy: proceed without asking unless a decision is genuinely ambiguous or high-risk

## Working style

- **Many projects, long gaps. Design for cheap resumption.** I run several builds in parallel and any one routinely goes dormant for months (ADHD, not abandonment). At the end of a work chunk, leave a short **NEXT/status breadcrumb** future-me can restart from in one read: top of README, a `## Next` block, or a clear commit body. Breadcrumbs beat sprint-momentum; a legible parked branch is worth more than an extra hour of velocity now.
- **Commit in focused chunks, not just at the end.** Conventional-commit messages between units of work so a branch picked up months later is legible. (See `emmaly-skills:git-workflow`.)
- **Security-conscious by default.** Threat-model lightly even on small projects: least standing access, segmentation, secrets out of the repo (`~/.secrets/*.env` pattern), audit-friendly logs. Design that way without being asked.
- **Embedded / IoT is in scope.** Occasional ESP32 (esp. **ESP32-C6**) + **Thread/Matter**; **Home Assistant** is standing home infra (see `emmaly-skills:home-assistant`). ESPHome / Arduino-ESP32 toolchains.

## Preferred Stack

- Go 1.26+ (run `go version` at the start of a new project)
- SvelteKit + Svelte 5 (runes: `$state`, `$derived`, `$effect`) + Tailwind CSS + DaisyUI
- Node.js 26+ (build toolchain only; never used as a production server when a Go server exists)
- TypeScript preferred over JavaScript, always
- podman for local containers and image builds
- cloudflared for public access
- SSE used proactively and plentifully for quick feedback/status/events from server to client
- WebSocket used only when SSE isn't sufficient

## Environment Variables

- **Never `source` a `.env` file directly.** The user's shell is `fish`, so `source .env` will fail on `export KEY=VALUE` syntax.
- Use `envwith` to load `.env` files and run commands with those variables overlaid on the current environment:
  ```
  envwith -f .secrets/.env -- <command> [args...]
  ```
- Install if not already available: `go install github.com/emmaly/envwith@latest`
- `envwith` loads the file, overlays its variables onto the current environment, then executes the subcommand provided after `--`.

## Deployment Targets

**Default to the self-hosted Kubernetes cluster.** Unless a project says otherwise, assume it is deployed there.

- **Kubernetes (default)**: a self-hosted k3s cluster. Manifests live in the project's own repo, images in `ghcr.io/emmaly/*`, storage on Longhorn (the default StorageClass). The conventions live in `~/Projects/kube`: start at that README, then `docs/MIGRATING-A-PROJECT.md` from `## The rules that are not negotiable` through `## Pod hygiene`. There is no deployment skill yet, so read those rather than inventing a deployment shape
  - **Deploying there is the default; publishing to the internet is not.** `cloudflared/route.sh <host>` plus an Ingress makes a service publicly reachable. Ask first, unless the project already owns that hostname and you are restoring it. Cluster-internal by default; public on purpose
- **Cloud**: Google Cloud Run or Firebase Functions, when there is a reason to be off-cluster
- **CLI tools**: standalone binaries, often just for the local machine
- **Windows**: occasionally Windows desktop or Windows Services (not often the primary target)
- **Primary target is always Linux** (server or desktop/laptop) unless stated otherwise

## Emmaly Plugin Skills

The `emmaly-skills` plugin provides skills for Go, Svelte, git workflow, integration, project setup, Home Assistant, documentation review, and plain language. The available skills list is authoritative. Check it, and invoke the relevant `emmaly-skills:*` skill when working in those areas.

`emmaly-skills:plain-language` governs all human-language output, always. Like these standards, it is normally already in context because the SessionStart hook emits it, so it rarely needs invoking. Invoke it explicitly if it is not in context (the hook did not run, or failed to emit it).

Before pushing anything to GitHub, invoke `emmaly-skills:integration`. It carries a mandatory local review gate that must run before every push.
