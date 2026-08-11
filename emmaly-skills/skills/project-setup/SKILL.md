---
name: project-setup
description: This skill should be used when starting a new project or setting up project scaffolding. Covers the `.secrets/` convention, `.gitignore`, README/PRD/AGENTS.md/CLAUDE.md documentation structure, Dockerfile/containerization, and CI conventions.
---

## Secrets

- Standardize on a `.secrets/` directory for all secret files (API credentials, `.env`, etc); always gitignore this directory
- Every project must have a `.gitignore` that excludes secrets, env files, and build artifacts

## Documentation

- Be thorough: usage examples, a config reference, and the architecture decisions worth remembering. Documentation is what makes a project resumable after months away, so write it for someone who has forgotten everything
- `README.md`: surface-level getting-started info. Ensure LLMs can orient quickly from the README alone, with references to deeper docs. If a PRD exists, reference it from the README
- `docs/PRD.md`: encourage creating a Product Requirements Document for new projects. Captures goals, scope, user stories, and constraints before implementation begins. Multi-feature projects split it into `docs/PRD/*.md` rather than one growing file
- `docs/*.md`: everything else (architecture, design decisions, API docs, etc)
- `AGENTS.md`: project-level agent instructions go in `AGENTS.md` (tool-agnostic)
- `CLAUDE.md`: if an `AGENTS.md` exists, `CLAUDE.md` should be a plain text file whose first line is `@AGENTS.md` (imports the shared instructions, like a symlink would). Add any Claude-specific instructions below that import line; never duplicate the `AGENTS.md` content

## Containerization

- For projects that will be deployed as a container, scaffold a `Dockerfile` alongside the project code
- Use multi-stage builds: build stage(s) for compiling, minimal final stage (e.g., `alpine`) for the runtime image
- Match the Go version in the `golang` build stage to what's available on Docker Hub, not the locally installed version (see `emmaly-skills:go` containerization notes)

## CI

- GitHub Actions for CI/CD where applicable
