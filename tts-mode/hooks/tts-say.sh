#!/usr/bin/env bash
# Speaks text without making the caller wait.
#
# Three jobs beyond calling the binary. Reporting, in the foreground, any
# failure an earlier line hit, since the background job that hit it has no
# stdout anyone reads. Detaching, so a turn never blocks on synthesis or
# audio. And a state directory the binary can queue in, so a progress line and
# a closing line play in order instead of talking over each other.
#
# Ordering and overlap are handled by the binary: each say takes a ticket,
# synthesizes its pieces concurrently, and whoever holds the player lock
# drains tickets in order. The queue is per user, not per session, and
# deliberately so: the audio device is machine-wide, so two enabled sessions
# speaking at once would overlap in the room. Serializing them is the point.
#
# Deliberately not `set -e`: this must return success no matter what, because
# the caller is a turn that should not fail over speech.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# The state directory is where the queue lives. It has to be the one path every
# session of this user agrees on, so it is the same directory the binary uses
# for sessions and the log, and it is refused unless absolute for the reasons
# run-ttsmode.sh spells out.
STATE_DIR="${TTSMODE_STATE_DIR:-}"
if [[ -z "$STATE_DIR" ]]; then
    if [[ -z "${HOME:-}" ]]; then
        exit 0
    fi
    STATE_DIR="${HOME}/.claude/tts-mode"
fi
if [[ "$STATE_DIR" != /* ]]; then
    exit 0
fi
if ! mkdir -p "$STATE_DIR" 2>/dev/null; then
    exit 0
fi

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
elif [[ -t 0 ]]; then
    # Every other guard here exits promptly, and cat on a terminal would hang
    # with no prompt and no message. Someone running this by hand with no
    # argument gets the old immediate exit back.
    exit 0
else
    TEXT="$(cat)"
fi

if [[ -z "${TEXT//[[:space:]]/}" ]]; then
    exit 0
fi

# Report what an earlier line could not do, before starting this one. This is
# the one synchronous step, and it only reads a small file.
"${HERE}/run-ttsmode.sh" failures 2>/dev/null || true

# Detach fully. stdin from /dev/null and both outputs discarded, so the tool
# that ran this sees no open descriptors and returns at once; setsid puts the
# job in its own session so nothing that cleans up this shell can stop the
# audio in the middle of a word. The compile, if the source changed, happens
# in the background too.
run_detached() {
    "${HERE}/run-ttsmode.sh" warm
    "${HERE}/run-ttsmode.sh" say "$TEXT"
}
if command -v setsid >/dev/null 2>&1; then
    setsid -f "${HERE}/tts-say-detached.sh" "$TEXT" </dev/null >/dev/null 2>&1
else
    run_detached </dev/null >/dev/null 2>&1 &
    disown 2>/dev/null || true
fi

exit 0
