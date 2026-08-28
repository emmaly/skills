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

# Never fail the turn this hook was only meant to observe. A missing Go
# toolchain or a broken build disables spoken output rather than blocking work,
# and says so in the log the binary itself writes.
give_up() {
    exit 0
}

# Everything below runs under `set -e`, so any unguarded failure would exit
# non-zero and read as a broken hook. Catch those too.
trap give_up ERR

[[ -d "$SRC_DIR" ]] || give_up

# Rebuild when the binary is missing or any source file is newer than it.
needs_build=0
if [[ ! -x "$BIN" ]]; then
    needs_build=1
elif [[ -n "$(find "$SRC_DIR" -name '*.go' -newer "$BIN" -print -quit 2>/dev/null)" ]]; then
    needs_build=1
fi

if (( needs_build )); then
    command -v go >/dev/null 2>&1 || give_up
    mkdir -p "$CACHE_DIR" || give_up

    # Build to a temporary name and rename over the target. Two hooks firing at
    # once must never see a half-written binary.
    tmp="$(mktemp "${CACHE_DIR}/ttsmode.XXXXXX")" || give_up
    if ! (cd "$SRC_DIR" && go build -o "$tmp" . >/dev/null 2>&1); then
        rm -f "$tmp"
        give_up
    fi
    chmod +x "$tmp"
    mv -f "$tmp" "$BIN"
fi

# The binary decides its own exit codes: non-zero only for a usage error.
# Anything else it swallows on purpose, so passing the code through is safe.
"$BIN" "$@" || exit 0
