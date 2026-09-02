# skills

Emmaly's Claude Code plugin marketplace. Three plugins live here, and nothing
else does.

This repo was `emmaly/emmaly` until 2026-09-01, when it was renamed and the
GitHub profile README moved to a fresh
[emmaly/emmaly](https://github.com/emmaly/emmaly). If you added the marketplace
under the old name, remove and re-add it: the rename redirect was replaced by
the profile repo, which carries no `marketplace.json`.

## Install

```
/plugin marketplace add emmaly/skills
/plugin install emmaly-skills@emmaly
```

The marketplace is named `emmaly`, so plugins are addressed as `<plugin>@emmaly`
whatever the repo is called. Install the other two the same way, by name.

## The plugins

### emmaly-skills

Collaboration style, preferred stack, and per-domain conventions. Nine skills:

| Skill | Covers |
| --- | --- |
| `standards` | Working style, Go-only language rule, preferred stack, deployment targets |
| `plain-language` | How every human-readable output is written, from chat to log lines |
| `go` | Modules, error wrapping, sentinel errors, `log/slog`, layout, testing |
| `svelte` | SvelteKit and Svelte 5 runes, Tailwind, DaisyUI, TypeScript, Vitest |
| `git-workflow` | Branch names, conventional commits, PR bodies, issue workflow |
| `integration` | The local review gate before any push, and the CodeRabbit PR gate |
| `project-setup` | `.secrets/`, `.gitignore`, docs layout, containers, CI |
| `home-assistant` | REST and WebSocket access patterns, token auth, common calls |
| `doc-review` | Reviewing docs for accuracy, completeness, and consistency |

Two of those load without being asked. A `SessionStart` hook prints the bodies
of `standards` and `plain-language` into every session, including after a
compact, because a style rule that only fires when someone remembers it never
fires on the output that needs it.

One mechanical gate backs the prose: `hooks/plaincheck`, a Go binary that
rejects em and en dashes in prose writes and in commit messages. It checks that
one rule and no others, since it is the only one with no false positives. It
reads just the text being written, skips backticks and fenced blocks in prose,
and stays silent otherwise. On a machine with no Go toolchain it exits 0 with a
note on stderr rather than blocking work.

### api-explorer

Research a third-party API before writing a client for it. Discovers the
documentation, fetches it, caches it, and normalizes it into a manifest the
implementation can be written against. It generates no client code, on purpose.

### tts-mode

Speaks short summaries of Claude's work aloud through ElevenLabs. Off by
default, toggled per session with `/tts on`, `/tts off`, and `/tts` to report
state. When on, Claude speaks one line at the start of multi-step work and one
at the end of a turn, capped at three lines a turn and fifteen words a line.

Needs an ElevenLabs API key in `~/.secrets/elevenlabs.env` and either `mpv` or
`ffplay` on `PATH`. Billing is per character, so a three-line turn costs roughly
300 characters. See `tts-mode/README.md` for the environment variables, the cost
math, and why the spoken line is passed on stdin.

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
binary is ever committed.

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
`emmaly-skills/NEXT.md`. Start there.
