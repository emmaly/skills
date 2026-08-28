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

# The lock path carries the user id. A fixed /tmp/tts-mode.lock is shared on a
# multi-user host, where another user's file makes the redirect fail and speech
# dies with nothing logged, and a symlink planted there gets truncated.
LOCK_DIR="${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}"
LOCK="${LOCK_DIR}/tts-mode-$(id -u).lock"

# Text is joined rather than taken as "$1". The instruction shows it quoted,
# but a model that drops the quotes would otherwise have its line truncated at
# the first space with the rest silently discarded.
TEXT="$*"

if [[ -z "${TEXT//[[:space:]]/}" ]]; then
    exit 0
fi

(
    # Bounded wait. Without it, one player hung on a busy audio device holds
    # this lock forever and every later line in every session queues behind it,
    # silently. Ninety seconds is longer than the player's own timeout, so a
    # normal slow line still gets its turn and only a truly stuck lock is
    # abandoned.
    flock -w 90 9 || exit 0
    "${HERE}/run-ttsmode.sh" say "$TEXT"
) 9>"$LOCK" >/dev/null 2>&1 &

exit 0
