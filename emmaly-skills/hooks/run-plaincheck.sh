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

# Everything below runs under `set -e`, so any command failing without an
# explicit guard would exit non-zero and report a broken hook. Catch those too.
# Exit 2 must only ever come from plaincheck finding a dash, never from this
# wrapper falling over.
trap 'give_up "unexpected error in the wrapper"' ERR

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

# Not exec, so the exit code can be inspected before it escapes. Only 0 and 2
# are ours to return: 2 means plaincheck found a dash, 0 means it did not.
# Anything else came from a broken binary rather than from prose, and a broken
# binary must not read as a finding.
status=0
"$BIN" || status=$?
case "$status" in
    0 | 2) exit "$status" ;;
    *) give_up "checker exited ${status}, rebuild it with 'go build ./...' in ${SRC_DIR}" ;;
esac
