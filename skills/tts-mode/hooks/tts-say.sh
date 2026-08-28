#!/usr/bin/env bash
# Speaks one line without making the caller wait.
#
# Two jobs beyond calling the binary. Backgrounding, so a turn never blocks on
# audio. And a lock, so a progress line and a closing line queue instead of
# talking over each other.
#
# The lock is per user, not per session, and deliberately so: the audio device
# is machine-wide, so two enabled sessions speaking at once would overlap in
# the room. Serializing them is the point.
#
# Deliberately not `set -e`: this must return success no matter what, because
# the caller is a turn that should not fail over speech.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The lock path carries the user id. A fixed /tmp/tts-mode.lock is shared on a
# multi-user host, where another user's file makes the redirect fail and speech
# dies with nothing logged, and a symlink planted there gets truncated.
#
# The lock lives with the state, which is the one path every session of this
# user already agrees on.
#
# Choosing it from the environment does not work. A desktop terminal has
# XDG_RUNTIME_DIR and a cron job or docker exec does not, so the two would pick
# different files, each take its own lock, and talk over each other on the one
# audio device. That is the divergence serializing exists to prevent, and it is
# why a fallback chain is the wrong shape here even though a stale
# XDG_RUNTIME_DIR is a real problem.
STATE_DIR="${TTSMODE_STATE_DIR:-}"
if [[ -z "$STATE_DIR" ]]; then
    # No /tmp fallback. A fixed path under a world-writable directory is the
    # same predictable target for every user on the host: plant a symlink at
    # lock and the redirect below truncates whatever it points at, with this
    # hook's privileges. Creating the directory is not a check, because
    # mkdir -p succeeds on one someone else already owns. With no HOME there
    # is nowhere safe to lock, so say nothing and exit successfully.
    if [[ -z "${HOME:-}" ]]; then
        exit 0
    fi
    STATE_DIR="${HOME}/.claude/tts-mode"
fi

# Only absolute paths, for the same reason: a relative one resolves against
# whatever directory this ran in, which is a tree someone else can pre-create.
if [[ "$STATE_DIR" != /* ]]; then
    exit 0
fi

if ! mkdir -p "$STATE_DIR" 2>/dev/null; then
    exit 0
fi
LOCK="${STATE_DIR}/lock"

# Text comes from stdin when no argument is given, and the injected
# instruction always uses that form.
#
# Putting the summary in the command line made it shell source. Claude's Bash
# executor performs substitution, and double quotes do not disable $(...),
# backticks, or variable expansion, so a summary quoting a filename or a log
# line containing those would run before this script ever saw the argument.
# The summary is written by a model that has been reading files, so its content
# is not trustworthy input to a shell. A heredoc with a quoted delimiter is not
# expanded at all.
#
# Arguments still work, for calling this by hand.
if (( $# )); then
    TEXT="$*"
else
    TEXT="$(cat)"
fi

if [[ -z "${TEXT//[[:space:]]/}" ]]; then
    exit 0
fi

(
    # Compile first, inside the background job but before the lock. The first
    # call after a source change builds, and doing that while holding the lock
    # would make every other session queue behind a compile. Backgrounding this
    # separately did not work: both jobs started at once and say built inside
    # the lock anyway.
    "${HERE}/run-ttsmode.sh" warm

    if command -v flock >/dev/null 2>&1; then
        # Bounded wait. Without it, one player hung on a busy audio device
        # holds the lock forever and every later line queues behind it. On
        # timeout the line is dropped, but it is recorded first: a line that
        # vanishes with nothing in the log is the failure the README rules out.
        if ! flock -w 90 9; then
            "${HERE}/run-ttsmode.sh" log "dropped a line, waited 90s for the playback lock"
            exit 0
        fi
    fi
    # No flock, as on macOS: speak unlocked. Two lines may overlap, which beats
    # total silence on a machine that meets every documented requirement. It is
    # not logged, because it would be logged on every single line.

    "${HERE}/run-ttsmode.sh" say "$TEXT"
) 9>"$LOCK" >/dev/null 2>&1 &

exit 0
