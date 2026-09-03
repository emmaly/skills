# skills

Emmaly's Claude Code plugin marketplace. Three plugins: conventions
(`emmaly-skills`), API documentation research (`api-explorer`), and spoken
summaries (`tts-mode`).

## Install

```
/plugin marketplace add emmaly/skills
/plugin install emmaly-skills@emmaly
```

The marketplace is named `emmaly`, so plugins are addressed as `<plugin>@emmaly`
whatever the repo is called. Install the other two the same way, by name.

## emmaly-skills

Nine skills covering how Emmaly works and what the code should look like.

### standards

Working style, language choice, stack, and deployment targets. The rule that
does the most work is `## Language choice`: Go for everything that executes,
including one-off scripts and throwaway analysis, with shell allowed only as
thin glue. It is written as a rule rather than a preference because the
preference wording kept losing to habit. Also covers the `~/.secrets/*.env` and
`envwith` pattern, the preferred stack (Go 1.26+, SvelteKit and Svelte 5 runes,
Tailwind, DaisyUI, podman, cloudflared, SSE before WebSocket), and the
self-hosted k3s cluster as the default deployment target.

Loaded into every session by a hook, so it rarely needs invoking.

### plain-language

How every human-readable output is written: chat replies, commit messages, PR
bodies, READMEs, comments, docstrings, UI copy, error messages, log lines.
Three rules carry most of it. Answer in the first sentence, keep a chat reply
to six lines, and use no em dashes. The rest names the specific tells, from
banned words and sentence shapes to narrative drama and personified code, plus
a precedence section for when a required output shape conflicts with it.

Also loaded into every session, for the reason in the skill itself: a style rule
that only fires when someone remembers to invoke it never fires on the output
that needs it most.

### go

Module and `go.mod` conventions, error wrapping, sentinel errors, `log/slog`
logging, project layout, preferred libraries (chi, gorilla, sqlite),
containerization, and testing.

### svelte

SvelteKit with Svelte 5 runes (`$state`, `$derived`, `$effect`), Tailwind,
DaisyUI, TypeScript over JavaScript, static builds served by a Go binary, and
Vitest.

### git-workflow

Branch naming, conventional commits, PR descriptions, and the GitHub issue
workflow.

### integration

The path from finished code to a merged PR. A local review gate is mandatory
before a PR is marked ready for review or pushed to when it is not a draft, and
that review is Claude's built-in `code-review` skill at high effort. CodeRabbit
reviews only the ready PR, as the final gate before merge, because its
5 reviews/hour limit is shared across PR reviews, CLI runs, and manual triggers.
Iteration happens in drafts, which cost nothing against that limit.

### project-setup

Scaffolding a new project: the `.secrets/` convention, `.gitignore`, the
README, PRD, AGENTS.md and CLAUDE.md documentation layout, Dockerfile and
containerization, and CI.

### home-assistant

REST and WebSocket access patterns against a Home Assistant instance, Bearer
token auth, a Go WebSocket example, and the common calls for states, services,
dashboards, logs, and HACS.

### doc-review

Reviewing a project's documentation for accuracy, completeness, and
consistency. It gathers findings first, then walks them past you one at a time
rather than rewriting anything unasked.

### The hooks behind them

A `SessionStart` hook prints the bodies of `standards` and `plain-language` into
every session, including after a compact. `hooks/emit-skill-body.sh` takes a
skill directory and a heading, so adding a third always-on skill is a line of
config rather than another script.

`hooks/plaincheck` is the only mechanical gate. It is a Go binary that rejects
em and en dashes on `Write`, `Edit`, and `MultiEdit`, and on `git commit` run
through Bash. It checks that one rule and nothing else, because it is the only
rule with no false positives: a word-list check would fire on the ban list
itself, on quoted tool output, and on every legitimate term of art. It reads
only the text being written, so editing an old document does not block unrelated
work, and it skips backticks and fenced blocks in prose, which is the escape
hatch for quoting a dash. On a machine with no Go toolchain it exits 0 with a
note on stderr rather than blocking the work.

## api-explorer

Research a third-party API before writing a client for it. It runs before any
implementation skill and generates no client code, on purpose.

Given an API to integrate with, it discovers the documentation, fetches it,
caches the raw artifacts, and normalizes them into a manifest holding auth,
conventions, types, endpoints, and the dependency graph between them. The
implementation is then written against the manifest.

Everything lands in `~/.cache/api-explorer/`, shared across projects, one
directory per API. Raw fetches are kept as timestamped snapshots next to the
manifest, previous manifests are archived rather than overwritten, and a filtered
manifest can be saved per scope when only part of a large API matters.

## tts-mode

Speaks short summaries of Claude's work aloud through ElevenLabs. Off by
default.

```
/tts on           turn it on for this session
/tts off          turn it off
/tts              report the current state
/tts voice <id>   use an ElevenLabs voice for this session only
```

On means Claude speaks a summary at the end of every turn, a line when it
starts longer work, and a line at real checkpoints inside it. The written
response is unchanged; speech is for someone listening with their eyes closed.
Off means the instruction is never injected, so no summary is even requested.
The switch is per session, so enabling it in one terminal leaves the others
silent, and background jobs never speak.

It needs an ElevenLabs API key in `~/.secrets/elevenlabs.env` and either `mpv`
or `ffplay` on `PATH`. Billing is per character, so a typical turn costs
roughly 500 to 700 characters. `tts-mode/README.md` has the environment
variables, the cost math, and why the spoken line is passed on stdin instead of
as an argument.

## Layout

```
.claude-plugin/marketplace.json   lists all three plugins
emmaly-skills/                    skills, the SessionStart emitter, plaincheck
api-explorer/                     API documentation research
tts-mode/                         the /tts command and the ttsmode hook
docs/superpowers/                 plans and specs
```

Each plugin carries its own `.claude-plugin/plugin.json` and its own version.
The two Go helpers are built into the user's cache on first use, so no compiled
binary is ever committed here.

## Working on a plugin

Iterate against the working tree, with no release involved:

```
claude --plugin-dir ./emmaly-skills
```

Run the Go tests where they live, one module per plugin:

```
cd emmaly-skills/hooks/plaincheck && go test ./...
cd tts-mode/hooks/ttsmode && go test ./...
```

## Releasing

An installed plugin is served from a version-keyed cache, not from this
checkout. A skill edit without a version bump never loads, and there is no
visible symptom when that happens. It has cost this repo a release before.

Bump `version` in both `.claude-plugin/marketplace.json` and the plugin's own
`.claude-plugin/plugin.json` (format `YYYYMMDDNNN`), then
`/plugin marketplace update emmaly`, `/plugin update <plugin>@emmaly`, and
restart the session so the hooks re-register. Verify by asserting the exact new
version directory exists under `~/.claude/plugins/cache/emmaly/`.

The full procedure, and the state of each decision behind these skills, is in
`emmaly-skills/NEXT.md`.
