#!/usr/bin/env bash
set -euo pipefail

# Builds hooks/ttsmode once, then runs it with the given subcommand.
#
# Same reasoning as run-plaincheck.sh in emmaly-skills: hooks need an executable
# to call, and a compiled binary cannot ship in the repo because it would be
# platform-specific and would go stale against its own source. So it is built on
# first use into the user's cache and reused after that.
#
# This wrapper holds no logic beyond build-if-stale. Anything worth testing
# belongs in the Go code, where it can be.

# How a setup failure is reported depends on who asked.
#
# on/off/status are typed by a person, so /tts on printing nothing and exiting
# 0 would read as success while TTS stayed off. They fail loudly.
#
# Everything else runs unattended, from a hook or from the backgrounded say
# wrapper whose output goes to /dev/null. Those exit 0 and write to the log
# instead. That write has to happen here rather than in the binary, because
# these are the failures where the binary does not exist yet: no Go, no source,
# a build that will not compile.
SUBCOMMAND="${1:-}"
case "$SUBCOMMAND" in
    on | off | status) USER_FACING=1 ;;
    *) USER_FACING=0 ;;
esac

# HOME is read with a default because it is expanded under `set -u`, and an
# unset HOME would otherwise abort with a non-zero exit. Some systemd units and
# container shells have no HOME, and for the UserPromptSubmit hook that is
# exactly the "fail the turn it was only meant to observe" outcome this script
# exists to avoid.
HOME_DIR="${HOME:-}"

# Empty when there is nowhere to log. give_up handles that by staying quiet.
LOG_DIR="${TTSMODE_STATE_DIR:-${HOME_DIR:+${HOME_DIR}/.claude/tts-mode}}"

give_up() {
    if (( USER_FACING )); then
        echo "tts-mode: cannot run, $1" >&2
        exit 1
    fi
    # warm only triggers the build. The say that follows it hits the same
    # failure and reports it, so logging here would double every entry.
    if [[ "$SUBCOMMAND" == "warm" ]]; then
        exit 0
    fi
    # Every step is allowed to fail. Under errexit an unwritable log, for
    # instance one left root-owned by a sudo run, would abort this function
    # before its exit 0 and make the hook fail loudly: the precise outcome
    # give_up exists to prevent. An empty LOG_DIR fails the mkdir and skips
    # the write, which is the intended behavior when HOME is unset.
    if [[ -n "$LOG_DIR" ]] && mkdir -p "$LOG_DIR" 2>/dev/null; then
        printf '%s cannot run, %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" \
            >> "${LOG_DIR}/log" 2>/dev/null || true
    fi
    exit 0
}

# Everything below runs under `set -e`, so any unguarded failure would exit
# non-zero and read as a broken hook. Catch those too.
trap 'give_up "unexpected error in the wrapper"' ERR

# Refuse rather than fall back to /tmp. Deriving the cache from a fixed
# world-writable path gives every user on the host the same predictable
# location for a binary this script then executes: plant one there with an
# mtime newer than the source and the staleness check below skips the rebuild
# and runs it. Creating the directory is not a check, because mkdir -p
# succeeds on a directory someone else already owns.
# Only absolute paths. A relative override resolves against whatever
# directory the hook happened to run in, which is the same exposure as a fixed
# /tmp path: the tree can be pre-created by someone else, and here the
# artifact is a binary this script execs rather than a file it writes. The XDG
# spec says a relative XDG_CACHE_HOME must be ignored, so refusing one is also
# what the spec asks for.
for candidate in "$HOME_DIR" "${XDG_CACHE_HOME:-}" "${TTSMODE_STATE_DIR:-}"; do
    if [[ -n "$candidate" && "$candidate" != /* ]]; then
        give_up "paths must be absolute, got ${candidate}"
    fi
done

# Both halves are required, and they are different directories: the cache
# holds the binary, the state dir holds sessions and the log. Guarding only
# the cache let /tts on succeed with XDG_CACHE_HOME set and HOME unset, after
# which tts-say.sh refused every line and nothing was ever spoken or logged.
if [[ -z "$HOME_DIR" ]] && { [[ -z "${XDG_CACHE_HOME:-}" ]] || [[ -z "${TTSMODE_STATE_DIR:-}" ]]; }; then
    give_up "HOME is not set; set HOME, or set both XDG_CACHE_HOME and TTSMODE_STATE_DIR"
fi

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
export CLAUDE_PLUGIN_ROOT="$PLUGIN_ROOT"
SRC_DIR="${PLUGIN_ROOT}/hooks/ttsmode"
CACHE_DIR="${XDG_CACHE_HOME:-${HOME_DIR}/.cache}/tts-mode"
BIN="${CACHE_DIR}/ttsmode"

[[ -d "$SRC_DIR" ]] || give_up "no source at ${SRC_DIR}"

# Rebuild when the binary is missing or any source file is newer than it.
# go.mod and go.sum count as source: bumping the Go directive or a dependency
# changes the build without touching a .go file, and the cached binary would
# otherwise keep running with no sign that it is stale.
needs_build=0
if [[ ! -x "$BIN" ]]; then
    needs_build=1
elif [[ -n "$(find "$SRC_DIR" \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer "$BIN" -print -quit 2>/dev/null)" ]]; then
    needs_build=1
fi

# The hook runs synchronously on every prompt, so it must never compile: the
# first prompt after any source edit would block the turn on a full build, even
# for a session where TTS is off and the hook will emit nothing at all. It uses
# whatever binary exists and otherwise does nothing. The async SessionStart
# prune is what keeps the cache warm.
if (( needs_build )) && [[ "$SUBCOMMAND" == "hook" ]]; then
    [[ -x "$BIN" ]] || exit 0
    needs_build=0
fi

if (( needs_build )); then
    command -v go >/dev/null 2>&1 || give_up "go is not installed"
    mkdir -p "$CACHE_DIR" || give_up "cannot create ${CACHE_DIR}"

    # Build to a temporary name and rename over the target. Two hooks firing at
    # once must never see a half-written binary.
    tmp="$(mktemp "${CACHE_DIR}/ttsmode.XXXXXX")" || give_up "cannot write to ${CACHE_DIR}"
    if ! (cd "$SRC_DIR" && go build -o "$tmp" . >/dev/null 2>&1); then
        rm -f "$tmp"
        give_up "build failed, run 'go build ./...' in ${SRC_DIR} to see why"
    fi
    chmod +x "$tmp"
    mv -f "$tmp" "$BIN"
fi

# Hook events must never fail the turn they were only meant to observe, so
# their exit code is discarded. The user-facing commands are the opposite: if
# `on` fails, /tts must not print success-shaped nothing while TTS stays off.
# warm exists only to force the build-if-stale block above. The say wrapper
# calls it before taking the playback lock, so a first-run compile does not
# happen while every other session waits on that lock.
if [[ "$SUBCOMMAND" == "warm" ]]; then
    exit 0
fi

if (( USER_FACING )); then
    exec "$BIN" "$@"
fi
"$BIN" "$@" || exit 0
