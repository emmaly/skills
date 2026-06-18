---
name: project-setup
description: Secrets, gitignore, documentation structure, CI conventions — use when starting a new project or setting up project scaffolding
---

## Secrets

- Standardize on a `.secrets/` directory for all secret files (API credentials, `.env`, etc); always gitignore this directory
- Every project must have a `.gitignore` that excludes secrets, env files, and build artifacts

## Documentation

- `README.md`: surface-level getting-started info. Ensure LLMs can orient quickly from the README alone, with references to deeper docs. If a PRD exists, reference it from the README
- `docs/PRD.md`: encourage creating a Product Requirements Document for new projects. Captures goals, scope, user stories, and constraints before implementation begins
- `docs/*.md`: everything else (architecture, design decisions, API docs, etc)
- `AGENTS.md`: project-level agent instructions go in `AGENTS.md` (tool-agnostic)
- `CLAUDE.md`: if an `AGENTS.md` exists, `CLAUDE.md` should be a plain text file whose first line is `@AGENTS.md` (imports the shared instructions, like a symlink would). Add any Claude-specific instructions below that import line; never duplicate the `AGENTS.md` content

## Containerization

- For projects that will be deployed via `emmaly:deploy`, scaffold a `Dockerfile` alongside the project code
- Use multi-stage builds: build stage(s) for compiling, minimal final stage (e.g., `alpine`) for the runtime image
- Match the Go version in the `golang` build stage to what's available on Docker Hub, not the locally installed version (see `emmaly:go` containerization notes)

## CI

- GitHub Actions for CI/CD where applicable
