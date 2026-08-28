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

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
export CLAUDE_PLUGIN_ROOT="$PLUGIN_ROOT"
SRC_DIR="${PLUGIN_ROOT}/hooks/ttsmode"
CACHE_DIR="${XDG_CACHE_HOME:-${HOME}/.cache}/tts-mode"
BIN="${CACHE_DIR}/ttsmode"

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

LOG_DIR="${TTSMODE_STATE_DIR:-${HOME}/.claude/tts-mode}"

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
    if mkdir -p "$LOG_DIR" 2>/dev/null; then
        printf '%s cannot run, %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" >> "${LOG_DIR}/log" 2>/dev/null
    fi
    exit 0
}

# Everything below runs under `set -e`, so any unguarded failure would exit
# non-zero and read as a broken hook. Catch those too.
trap 'give_up "unexpected error in the wrapper"' ERR

[[ -d "$SRC_DIR" ]] || give_up "no source at ${SRC_DIR}"

# Rebuild when the binary is missing or any source file is newer than it.
needs_build=0
if [[ ! -x "$BIN" ]]; then
    needs_build=1
elif [[ -n "$(find "$SRC_DIR" -name '*.go' -newer "$BIN" -print -quit 2>/dev/null)" ]]; then
    needs_build=1
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
if [[ "${1:-}" == "warm" ]]; then
    exit 0
fi

if (( USER_FACING )); then
    exec "$BIN" "$@"
fi
"$BIN" "$@" || exit 0
