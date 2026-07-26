#!/usr/bin/env bash
# Resource sampler for the upload benchmark (Docker Compose release stack).
#
# Samples container CPU and memory at a fixed cadence via `docker stats`.
# Writes a CSV into the run output directory. Stop with Ctrl-C / SIGINT.
#
#   ./sample.sh <out_dir> [interval_seconds]
#
# Examples:
#   ./sample.sh benchruns/20260707-101500 1
set -euo pipefail

OUT_DIR="${1:?usage: sample.sh <out_dir> [interval_seconds]}"
INTERVAL="${2:-1}"

mkdir -p "$OUT_DIR"
DOCKER_CSV="$OUT_DIR/resource_samples.csv"

echo "ts,container,cpu_pct,mem_used,mem_limit,mem_pct,net_io,block_io,pids" > "$DOCKER_CSV"

echo "[sample] writing to $DOCKER_CSV (interval ${INTERVAL}s); Ctrl-C to stop" >&2
trap 'echo "[sample] stopped" >&2; exit 0' INT TERM

while true; do
    TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

    # docker stats: one row per running container.
    docker stats --no-stream \
        --format '{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.NetIO}},{{.BlockIO}},{{.PIDs}}' 2>/dev/null \
    | while IFS=',' read -r name cpu memusage mempct netio blockio pids; do
        mem_used="${memusage%% / *}"
        mem_limit="${memusage##* / }"
        # Quote net/block IO because they contain " / ".
        printf '%s,%s,%s,%s,%s,%s,"%s","%s",%s\n' \
            "$TS" "$name" "${cpu%\%}" "$mem_used" "$mem_limit" "${mempct%\%}" "$netio" "$blockio" "$pids" >> "$DOCKER_CSV"
    done

    sleep "$INTERVAL"
done
