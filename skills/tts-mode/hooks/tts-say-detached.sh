#!/usr/bin/env bash
# The detached half of tts-say.sh: runs in its own session under setsid, so
# it survives whatever cleans up the shell that launched it. Text arrives as
# the one argument, already read from stdin by the wrapper, so no shell
# expansion happens on it here either.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"${HERE}/run-ttsmode.sh" warm
"${HERE}/run-ttsmode.sh" say "$1"
exit 0
