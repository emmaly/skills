#!/usr/bin/env bash
# Speaks one line without making the caller wait.
#
# Two jobs beyond calling the binary. Backgrounding, so a turn never blocks on
# audio. And a lock, so a progress line and a closing line queue instead of
# talking over each other.
#
# Deliberately not `set -e`: this must return success no matter what, because
# the caller is a turn that should not fail over speech.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCK="${XDG_RUNTIME_DIR:-/tmp}/tts-mode.lock"

if [[ $# -lt 1 || -z "${1//[[:space:]]/}" ]]; then
    exit 0
fi

(
    flock 9
    "${HERE}/run-ttsmode.sh" say "$1"
) 9>"$LOCK" >/dev/null 2>&1 &

exit 0
