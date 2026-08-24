#!/usr/bin/env bash
set -euo pipefail

# Builds hooks/plaincheck once, then runs it, passing stdin straight through.
#
# The checker is Go, like everything else here. Hooks need an executable to
# call, and a compiled binary cannot ship in the repo: it would be
# platform-specific and would go stale against its own source. So the binary is
# built on first use into the user's cache and reused after that. Steady-state
# cost is one exec of a small static binary, which is cheaper than starting an
# interpreter on every tool call.
#
# This wrapper is deliberately the only shell here, and it holds no logic beyond
# build-if-stale. Anything worth testing belongs in the Go code, where it can be.

PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SRC_DIR="${PLUGIN_ROOT}/hooks/plaincheck"
CACHE_DIR="${XDG_CACHE_HOME:-${HOME}/.cache}/emmaly-skills"
BIN="${CACHE_DIR}/plaincheck"

# Never fail the tool call this hook was only meant to inspect. A missing Go
# toolchain or a broken build disables the gate and says so on stderr, rather
# than blocking the work.
give_up() {
    echo "emmaly-skills: plain-language dash check disabled, $1" >&2
    exit 0
}

if [[ ! -d "$SRC_DIR" ]]; then
    give_up "no source at ${SRC_DIR}"
fi

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
    tmp="$(mktemp "${CACHE_DIR}/plaincheck.XXXXXX")" || give_up "cannot write to ${CACHE_DIR}"
    if ! (cd "$SRC_DIR" && go build -o "$tmp" . >/dev/null 2>&1); then
        rm -f "$tmp"
        give_up "build failed, run 'go build ./...' in ${SRC_DIR} to see why"
    fi
    chmod +x "$tmp"
    mv -f "$tmp" "$BIN"
fi

exec "$BIN"
