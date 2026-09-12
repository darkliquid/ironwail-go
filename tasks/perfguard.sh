#!/bin/sh
# perfguard.sh — run a command under an RSS watchdog.
#
# Kills the whole process tree before the kernel OOM killer can (an OOM
# kill takes down the agent session too), then exits 124.
#
# Usage: perfguard.sh [max_rss_mib] [max_secs] -- cmd args...
# Defaults: 8192 MiB, 900 s.
set -u

MAX_RSS=${1:-8192}
MAX_SECS=${2:-900}
shift 2
[ "${1:-}" = "--" ] && shift

"$@" &
PID=$!

killed=""
elapsed=0
while kill -0 "$PID" 2>/dev/null; do
    if [ "$elapsed" -ge "$MAX_SECS" ]; then
        killed="time"
        break
    fi
    # RSS of the process tree (KiB): the leader plus any descendants.
    RSS=$(ps -o rss= --ppid "$PID" --pid "$PID" 2>/dev/null | awk '{s+=$1} END {print s+0}')
    if [ "$RSS" -gt $((MAX_RSS * 1024)) ]; then
        killed="rss:$RSS"
        break
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if [ -n "$killed" ]; then
    echo "perfguard: killed ($killed) after ${elapsed}s" >&2
    pkill -TERM -P "$PID" 2>/dev/null
    kill -TERM "$PID" 2>/dev/null
    sleep 2
    pkill -KILL -P "$PID" 2>/dev/null
    kill -KILL "$PID" 2>/dev/null
    exit 124
fi

wait "$PID"
exit $?
