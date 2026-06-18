---
name: standards
description: Emmaly's core collaboration style and preferred technology stack — loaded in every conversation
---
- Pair programming style
- Expert-level: skip introductory explanations
- High autonomy: proceed without asking unless a decision is genuinely ambiguous or high-risk

## Preferred Stack

- Go 1.25+ (run `go version` at the start of a new project)
- SvelteKit + Svelte 5 (runes: `$state`, `$derived`, `$effect`) + Tailwind CSS + DaisyUI
- Node.js 24+ (build toolchain only; never used as a production server when a Go server exists)
- TypeScript preferred over JavaScript, always
- podman-compose for containerization
- cloudflared for public access
- SSE used proactively and plentifully for quick feedback/status/events from server to client
- WebSocket used only when SSE isn't sufficient

## Environment Variables

- **Never `source` a `.env` file directly** — the user's shell is `fish`, so `source .env` will fail on `export KEY=VALUE` syntax.
- Use `envwith` to load `.env` files and run commands with those variables overlaid on the current environment:
  ```
  envwith -f .secrets/.env -- <command> [args...]
  ```
- Install if not already available: `go install github.com/emmaly/envwith@latest`
- `envwith` loads the file, overlays its variables onto the current environment, then executes the subcommand provided after `--`.

## Deployment Targets

Projects vary widely; choose based on project needs:

- **Cloud**: Google Cloud Run, Firebase Functions
- **On-prem containers**: podman-compose to a remote host, often paired with a cloudflared container
- **CLI tools**: standalone binaries, often just for the local machine
- **Windows**: occasionally Windows desktop or Windows Services (not often the primary target)
- **Primary target is always Linux** (server or desktop/laptop) unless stated otherwise

## Emmaly Plugin Skills

The `emmaly-skills` plugin provides skills for Go, Svelte, git workflow, integration, project setup, deployment, Home Assistant, and documentation review. Check the available skills list and invoke the relevant `emmaly-skills:*` skill when working in those areas.
