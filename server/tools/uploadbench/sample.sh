#!/usr/bin/env bash
# Resource sampler for the upload benchmark (Docker Compose release stack).
#
# Samples, at a fixed cadence, container CPU%/memory via `docker stats` and
# (optionally) a few PostgreSQL activity counters via `docker exec ... psql`.
# Writes two CSVs into the run output directory. Stop with Ctrl-C / SIGINT.
#
#   ./sample.sh <out_dir> [interval_seconds] [pg_container]
#
# Examples:
#   ./sample.sh benchruns/20260707-101500 1 lumilio-photos-db-1
#
# Container names default to whatever `docker stats` reports; pass a specific
# PostgreSQL container to enable the pg_stat sampler.
set -euo pipefail

OUT_DIR="${1:?usage: sample.sh <out_dir> [interval_seconds] [pg_container]}"
INTERVAL="${2:-1}"
PG_CONTAINER="${3:-}"

mkdir -p "$OUT_DIR"
DOCKER_CSV="$OUT_DIR/resource_samples.csv"
PG_CSV="$OUT_DIR/pg_samples.csv"

echo "ts,container,cpu_pct,mem_used,mem_limit,mem_pct,net_io,block_io,pids" > "$DOCKER_CSV"
if [[ -n "$PG_CONTAINER" ]]; then
    echo "ts,numbackends,xact_commit,tup_inserted,blks_read,blks_hit,active_queries" > "$PG_CSV"
fi

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

    if [[ -n "$PG_CONTAINER" ]]; then
        ROW="$(docker exec "$PG_CONTAINER" psql -U postgres -d lumiliophotos -At -F',' -c "
            SELECT
              (SELECT numbackends FROM pg_stat_database WHERE datname = 'lumiliophotos'),
              (SELECT xact_commit FROM pg_stat_database WHERE datname = 'lumiliophotos'),
              (SELECT tup_inserted FROM pg_stat_database WHERE datname = 'lumiliophotos'),
              (SELECT blks_read FROM pg_stat_database WHERE datname = 'lumiliophotos'),
              (SELECT blks_hit FROM pg_stat_database WHERE datname = 'lumiliophotos'),
              (SELECT count(*) FROM pg_stat_activity WHERE state = 'active');
        " 2>/dev/null | head -1 || true)"
        [[ -n "$ROW" ]] && echo "$TS,$ROW" >> "$PG_CSV"
    fi

    sleep "$INTERVAL"
done
