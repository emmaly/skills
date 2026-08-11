---
name: standards
description: Emmaly's core collaboration style and preferred technology stack. Normally already in context, since the plugin's SessionStart hook injects it, so it rarely needs invoking; load it explicitly when asked what the standards or preferred stack are, or whenever they are not already in context (the hook did not run, or failed to emit them).
---
Emmaly (she/her).

- Pair programming style
- Expert-level: skip introductory explanations
- High autonomy: proceed without asking unless a decision is genuinely ambiguous or high-risk

## Working style

- **Many projects, long gaps. Design for cheap resumption.** I run several builds in parallel and any one routinely goes dormant for months (ADHD, not abandonment). At the end of a work chunk, leave a short **NEXT/status breadcrumb** future-me can restart from in one read: top of README, a `## Next` block, or a clear commit body. Breadcrumbs beat sprint-momentum; a legible parked branch is worth more than an extra hour of velocity now.
- **Commit in focused chunks, not just at the end.** Conventional-commit messages between units of work so a branch picked up months later is legible. (See `emmaly-skills:git-workflow`.)
- **Security-conscious by default.** Threat-model lightly even on small projects: least standing access, segmentation, secrets out of the repo (`~/.secrets/*.env` pattern), audit-friendly logs. Design that way without being asked.
- **Embedded / IoT is in scope.** Occasional ESP32 (esp. **ESP32-C6**) + **Thread/Matter**; **Home Assistant** is standing home infra (see `emmaly-skills:home-assistant`). ESPHome / Arduino-ESP32 toolchains.
- **Integration between SaaS systems over APIs is a frequent project shape.** Expect to be writing or wrapping REST clients about as often as building an application from scratch.

## Language choice

**Go, unless I have said otherwise for this specific task.** This is a rule, not
a preference, and it covers everything that executes: services, CLIs, hooks,
generators, migrations, one-off scripts, throwaway analysis, and whatever you
were about to write to check your own work.

- **Never write Python.** Not for a script, not for a quick calculation, not
  because it is a few lines shorter, not because the file is temporary and
  nobody will see it. If Python looks like the obvious tool, that is the habit
  this rule exists to override.
- Reaching for Python is the specific failure to watch for. It arrives as "this
  is just a small script", "it's only for testing", or "this is throwaway", and
  those are the cases this rule is aimed at, not the exceptions to it.
- **Shell is allowed only as thin glue**: launching a process, wiring an
  environment, a few lines with no logic worth testing. Anything with branching,
  parsing, or arithmetic is a Go program instead.
- Other languages follow the platform, not preference: TypeScript in the
  browser, and whatever the hardware demands on embedded.
- **Anything else needs approval first.** State what you want to use, why Go
  will not do the job, and wait for an answer. Do not start writing and ask
  afterward.
- Reading Python is fine. Third-party SDKs, upstream examples, and vendor
  documentation are reference material, and quoting them is not writing Python.

The bar for "an intensely good reason" is high, and being asked to do something
quickly is not one. A slower Go answer beats a fast Python one every time.

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
  - **Reach for the narrowest exposure that works.** Cluster-internal is the default. A LAN address comes from a MetalLB service via `loadBalancerClass` — also the answer for UDP or anything else an HTTP Ingress cannot carry. Private remote access is a Tailscale-operator service, which gives it its own tailnet node with a real certificate. Public is the last step, not the assumed one: `cloudflared/route.sh <host>` plus an Ingress puts it on the internet. Always ask first — owning the hostname already is not the same as being authorised to publish it today
- **Cloud**: Google Cloud Run or Firebase Functions, when there is a reason to be off-cluster
- **CLI tools**: standalone binaries, often just for the local machine
- **Windows**: occasionally Windows desktop or Windows Services (not often the primary target)
- **Primary target is always Linux** (server or desktop/laptop) unless stated otherwise

## Emmaly Plugin Skills

The `emmaly-skills` plugin provides skills for Go, Svelte, git workflow, integration, project setup, Home Assistant, documentation review, and plain language. The available skills list is authoritative. Check it, and invoke the relevant `emmaly-skills:*` skill when working in those areas.

`emmaly-skills:plain-language` governs all human-language output, always. Like these standards, it is normally already in context because the SessionStart hook emits it, so it rarely needs invoking. Invoke it explicitly if it is not in context (the hook did not run, or failed to emit it).

Before pushing anything to GitHub, invoke `emmaly-skills:integration`. It carries a mandatory local review gate and routes all in-session reviews to Claude's built-in `code-review` skill; CodeRabbit is reserved for the PR merge gate.

**Keep `CLAUDE.md` files thin.** Anything true across machines and jobs belongs in this skill, which every machine gets from the plugin. A `CLAUDE.md` should hold only what is specific to that machine, employer, or project. When a rule turns out to apply everywhere, move it here rather than copying it into a second `CLAUDE.md`.

This skill is **public**. Emmaly's own infrastructure conventions are fine here and are deliberately named above. Anything belonging to an employer or client is not — no company names, internal hostnames, ticket systems, or customer details. If a rule cannot be stated without those, it belongs in that machine's `CLAUDE.md`.
