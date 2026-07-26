#!/bin/sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

banned_runtime='postgres|github\.com/jackc/pgx|github\.com/pgvector|riverpgxv5|libpq|postgres://|pg_dump|pg_restore|initdb|pg_ctl|pg_isready|createdb|POSTGRES_|LUMILIO_PG_|db_bootstrap_password'

if matches="$(
    git grep -n -I -i -E "$banned_runtime" -- \
        .devcontainer \
        .github/workflows \
        AGENTS.md \
        Makefile \
        README.md \
        README.zh-CN.md \
        'docker-compose*.yml' \
        desktop \
        server \
        site/docs/en \
        site/docs/zh-cn \
        site/docs/internal/agent/architecture.md \
        site/docs/internal/agent/BACKEND.md \
        site/docs/internal/agent/FRONTEND.md \
        web/e2e \
        web/src \
        ':!desktop/build/**' \
        ':!server/third_party/**'
)"; then
    printf '%s\n' "SQLite architecture guard found a legacy database reference:" "$matches" >&2
    exit 1
fi

if matches="$(
    git grep -n -E 'sqlc\.(arg|narg|slice|embed)|@[[:alpha:]_][[:alnum:]_]*' -- \
        'server/internal/db/repo/*.sql.go'
)"; then
    printf '%s\n' "SQLite architecture guard found an unresolved sqlc parameter macro in generated SQL:" "$matches" >&2
    exit 1
fi

if matches="$(
    git grep -n -E 'randomblob[[:space:]]*\(' -- \
        'server/internal/db/repo/queries/*.sql'
)"; then
    printf '%s\n' \
        "SQLite architecture guard found database-side randomness in an application query:" \
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
        "SQLite architecture guard found a generated sqlc slice query mixed with fixed parameters:" \
        "$mixed_slice_queries" >&2
    printf '%s\n' \
        "Use a JSON1 list parameter instead; sqlc numbered placeholders are invalid after dynamic slice expansion." >&2
    exit 1
fi

check_dependencies() {
    module_dir="$1"
    shift
    dependencies="$(cd "$module_dir" && go list -tags=sqlite_fts5 -deps "$@")"
    if printf '%s\n' "$dependencies" | grep -Eiq 'postgres|github\.com/jackc/pgx|github\.com/pgvector|riverpgx|libpq|github\.com/lib/pq|gorm\.io/driver/postgres'; then
        printf '%s\n' "SQLite architecture guard found a legacy compiled dependency in $module_dir:" >&2
        printf '%s\n' "$dependencies" | grep -Ei 'postgres|github\.com/jackc/pgx|github\.com/pgvector|riverpgx|libpq|github\.com/lib/pq|gorm\.io/driver/postgres' >&2
        exit 1
    fi
}

check_dependencies server ./...
# The Desktop root embeds ignored panel/dist output. Architecture checks run
# before that UI is built, so let go list report the dependency graph despite
# the expected missing-embed package error. Desktop test/build gates still
# compile the package normally after building the panel.
check_dependencies desktop -e ./...

printf '%s\n' "SQLite architecture guard passed"
