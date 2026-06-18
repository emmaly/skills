---
name: deploy
description: This skill should be used when the user asks to "set up deployment", "add a deploy script", or "deploy to remote" — scaffolds a podman-compose deployment to a remote server over SSH with cloudflared, versioning, rollback, and secure .env handling.
---

# Deploy Skill

Scaffolds a podman-compose deployment setup in the current project. Use when the user asks to "set up deployment", "add deploy script", "deploy to remote", or similar.

## What This Skill Does

Copies and adapts deployment templates into the current project:

1. **deploy.sh** — Main script with subcommands: `deploy`, `status`, `logs`, `teardown`, `rollback`, `preflight`
2. **podman-compose.yml** — Compose file with app service + cloudflared tunnel
3. **deploy.conf** — Non-secret deployment target config (host, user, project name)
4. **.secrets/.env** — Secret environment variables (gitignored)

## Scaffolding Steps

When invoked, do the following:

1. Read the template files from this skill's `templates/` directory
2. Copy `deploy.sh` into the project root, make executable
3. Copy `podman-compose.yml` into the project root, adapting:
   - Replace `${PROJECT_NAME}` placeholder with the actual project name
   - Adjust services as needed (add/remove based on project requirements)
   - Configure networking per the network conventions below
4. Create `.secrets/` directory if it doesn't exist
5. Ensure `.gitignore` includes `.secrets/` and `*.tar` (image tarballs)
6. Create `deploy.conf` from the example, prompting the user for:
   - `DEPLOY_HOST` — remote server hostname or IP
   - `DEPLOY_USER` — SSH user on remote
   - `PROJECT_NAME` — used for image naming and remote directory
7. Create `.secrets/.env` from `env.example`, telling the user which values to fill in
8. Copy `env.example` into the project root for reference

## Deploy Flow Summary

The deploy script uses tarball-over-SSH (no registry needed):

```
Local: build → save images to .tar → scp to remote → load → up -d
```

- Secrets: `.secrets/.env` transferred as `.env` with chmod 600
- Versioning: git commit hash (8-char short) stamped in `VERSION` file
- Rollback: previous deployment snapshot preserved on remote, `rollback` swaps current ↔ previous (including .env)
- Cloudflared: pulled fresh on remote, token injected via compose variable substitution from `.env`

## Network Conventions

Two patterns exist for cloudflared access. Choose one per project:

### Companion container (default in this template)
The project bundles its own `cloudflared` service in the compose file. All services communicate over the stack's own `internal` bridge network. No external network is needed — the cloudflared container *is* the tunnel endpoint.

### Shared instance (alternative)
A shared cloudflared instance runs outside this stack on the host. The app joins an external `cloudflared` network so the shared tunnel can reach it. Remove the `cloudflared` service from the compose file and add `cloudflared: external: true` to networks.

**Important:** When using the shared instance pattern, `container_name` must be set explicitly (the template uses `${PROJECT_NAME}`). Cloudflared resolves services by container name on the shared network. Without it, podman-compose derives the name from the working directory + service (e.g., `current_app_1`), which won't match the tunnel config.

### General rules
- **Never** connect a container to both `cloudflared` and `caddy` external networks
- Use `caddy` external network instead of `cloudflared` for private/internal-only access
- Always use an `internal` bridge network for inter-service communication within the stack

## Customization Points

When adapting for a specific project:

- Add volume mounts for persistent data
- Add additional services (database, cache, etc.)
- Add health check endpoints if the app supports them
- Adjust `get_local_images()` in deploy.sh if image naming doesn't follow the convention
