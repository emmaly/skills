# emmaly/skills

A Claude Code plugin marketplace. Three plugins, all installed from this repo.

The profile README that used to live here moved to
[emmaly/emmaly](https://github.com/emmaly/emmaly) on 2026-09-01, so this repo
holds nothing but the plugins.

## Install

```
/plugin marketplace add emmaly/skills
/plugin install emmaly-skills@emmaly
```

The marketplace is named `emmaly`, so plugins are addressed as
`<plugin>@emmaly` regardless of the repo name. If you added the marketplace
before the rename, remove and re-add it: GitHub's redirect from the old name
was replaced by the profile repo, which carries no `marketplace.json`.

## Plugins

| Plugin | What it does |
| --- | --- |
| `emmaly-skills` | Collaboration style, preferred stack, and conventions for Go, Svelte, git, integration, project setup, Home Assistant, docs review, and plain language. Two of its skills load into every session through a `SessionStart` hook. |
| `api-explorer` | Discovers, fetches, caches, and normalizes third-party API documentation before anything is implemented against it. Research only, and it writes no client code. |
| `tts-mode` | Speaks short summaries of Claude's work aloud through ElevenLabs, toggled per session with `/tts`. Off by default. |

Each plugin has its own version and its own manifest. `emmaly-skills` and
`tts-mode` carry Go helpers that their hook wrappers build into the user's
cache on first use, so a compiled binary is never committed here.

## Layout

```
.claude-plugin/marketplace.json   the marketplace, listing all three plugins
emmaly-skills/             conventions, plus the plaincheck hook
api-explorer/              API documentation research
tts-mode/                  spoken summaries, plus the ttsmode hook
docs/                             TODO list and design records
```

Each plugin directory holds its own `README.md` or `NEXT.md` where it has one.
Start at `emmaly-skills/NEXT.md` for the state of that plugin and for
the release procedure, which is easy to get wrong: the version string in the
manifests is what invalidates the installed cache, and a skill edit without a
bump never loads.

## Working on the plugins

Iterate without releasing:

```
claude --plugin-dir ./emmaly-skills
```

Run the Go tests where they live, one module per plugin:

```
cd emmaly-skills/hooks/plaincheck && go test ./...
cd tts-mode/hooks/ttsmode && go test ./...
```

Open work is listed in `docs/TODO.md`.
