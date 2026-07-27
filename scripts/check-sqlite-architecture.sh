#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

if matches="$(
    git grep -n -E 'sqlc\.(arg|narg|slice|embed)|@[[:alpha:]_][[:alnum:]_]*' -- \
        'server/internal/db/repo/*.sql.go'
)"; then
    printf '%s\n' "SQLite SQL check found an unresolved sqlc parameter macro in generated SQL:" "$matches" >&2
    exit 1
fi

if matches="$(
    git grep -n -E 'randomblob[[:space:]]*\(' -- \
        'server/internal/db/repo/queries/*.sql'
)"; then
    printf '%s\n' \
        "SQLite SQL check found database-side randomness in an application query:" \
        "$matches" >&2
    printf '%s\n' "Generate UUIDs and security-sensitive random values in Go and pass them explicitly." >&2
    exit 1
fi

mixed_slice_queries="$(
    awk '
        /^const [[:alnum:]_]+ = `/ {
            in_query = 1
            query_start = FNR
            has_slice = 0
            has_numbered_parameter = 0
        }
        in_query && /\/\*SLICE:[^*]+\*\// { has_slice = 1 }
        in_query && /\?[[:digit:]]+/ { has_numbered_parameter = 1 }
        in_query && /^`$/ {
            if (has_slice && has_numbered_parameter) {
                print FILENAME ":" query_start
            }
            in_query = 0
        }
    ' server/internal/db/repo/*.sql.go
)"
if [ -n "$mixed_slice_queries" ]; then
    printf '%s\n' \
        "SQLite SQL check found a generated sqlc slice query mixed with fixed parameters:" \
        "$mixed_slice_queries" >&2
    printf '%s\n' \
        "Use a JSON1 list parameter instead; sqlc numbered placeholders are invalid after dynamic slice expansion." >&2
    exit 1
fi

printf '%s\n' "SQLite SQL checks passed"
