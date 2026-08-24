---
name: go
description: This skill should be used when working in Go code. Covers conventions for go.mod/modules, error wrapping, sentinel errors, log/slog logging, project layout, preferred libraries (chi, gorilla, sqlite), containerization, and testing.
---

Go is the default for everything that executes, including scripts and one-off
tooling. `emmaly-skills:standards` carries that rule under `## Language choice`,
along with the narrow cases where something else is allowed. This skill covers
how to write the Go, not whether to.

## Conventions

- Never manually edit `go.mod` or `go.sum`. Use `go mod init github.com/emmaly/<project>` to initialize and `go mod tidy` to sync
- Run `gofmt` and `go vet` often and before committing
- Supply docstrings for _all_ functions created or modified; keep docstrings accurate
- Wrap errors with context: `fmt.Errorf("doing something: %w", err)`
- Use sentinel errors (`var ErrNotFound = errors.New("not found")`) when callers need to check specific conditions
- Use `log/slog` for structured logging. Avoid `log` or `fmt.Println` for operational output
- Follow [golang-standards/project-layout](https://github.com/golang-standards/project-layout): `cmd/`, `internal/`, `pkg/`, etc; use only what the project requires

## Preferred Libraries

- HTTP routing: `chi`
- WebSocket & HTTP utilities: `gorilla`
- Database: `sqlite` preferred, `postgres` if needed

## Containerization

- Official `golang` Docker images lag behind bleeding-edge Go releases. When building in containers, set `go.mod` to the latest Go version available as a container image (check [Docker Hub golang tags](https://hub.docker.com/_/golang/tags)), not the locally installed version
- Run `go version` locally and check available container images before choosing the `go` directive in `go.mod`

## Testing

- Write tests when they provide real value, not for coverage metrics
- Focus on logic that is complex, error-prone, or critical
- Use `go test`
