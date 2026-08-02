#!/bin/sh
set -eu

# Static regression guards for SQL generation and browse/search contracts.
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

# Browse/filter/search operate on logical media items. The retired RAW filter
# (`filter.raw` / `IsRaw` as a browse predicate) and the retired search
# `stack_mode` must not come back.
#
# Legitimate remaining uses of the `is_raw` name are whitelisted below:
#   * `dbtypes.PhotoSpecificMetadata.is_raw` — an EXIF metadata fact
#   * generated component-fact columns in the media/stack SQL
#   * legacyBrowseStateMigration — reads pre-existing persisted state whose old
#     `raw` key is deliberately dropped rather than migrated
if matches="$(
    git grep -n -E 'IsRaw|filter\.raw[^[:alnum:]_]|(^|[^[:alnum:]_])raw\?:[[:space:]]*boolean|(^|[^[:alnum:]_])raw:[[:space:]]*(true|false)|params\.(get|set|has|delete)\("raw"' -- \
        'server/internal/api' 'server/internal/service' 'server/internal/agent' \
        'web/src/features/assets' 'web/src/features/search' 'web/src/features/studio' \
    | grep -v -E 'PhotoSpecificMetadata|dbtypes|is_raw.*(metadata|exif)|legacyBrowseStateMigration' \
    || true
)"; then
    if [ -n "$matches" ]; then
        printf '%s\n' \
            "Browse architecture check found a retired RAW filter reference:" \
            "$matches" >&2
        printf '%s\n' \
            "Browse/filter/search work on media items: use media_item.composition, not a raw boolean." >&2
        exit 1
    fi
fi

# Search results are always flat by media item; stack_mode is browse-only
# presentation. SearchAssetsRequestDTO keeps a bindable StackMode field solely
# so the handler can reject it, so that declaration plus the rejection helper
# are the only permitted mentions.
if matches="$(
    git grep -n -E 'StackMode' -- 'server/internal/service/asset_search_fused.go' \
    || true
)"; then
    if [ -n "$matches" ]; then
        printf '%s\n' \
            "Browse architecture check found stack_mode in the search pipeline:" \
            "$matches" >&2
        printf '%s\n' "Search results are never stack-collapsed." >&2
        exit 1
    fi
fi

# The frontend must not send stack_mode to /assets/search either — the endpoint
# returns 400 for it. This needs a grep guard because the generated OpenAPI type
# does NOT catch it: the request literal is built inside `useMemo<T>(() => ({…}))`,
# where TypeScript performs no excess-property check, so an unknown key compiles
# clean and only fails against the real server. `useAssetsList.ts` is the one
# legitimate caller (browse collapse/expand presentation).
if matches="$(
    git grep -n -E 'stack_mode' -- 'web/src' \
    | grep -v -E 'useAssetsList\.ts|schema\.d\.ts' \
    || true
)"; then
    if [ -n "$matches" ]; then
        printf '%s\n' \
            "Browse architecture check found stack_mode outside the browse list request:" \
            "$matches" >&2
        printf '%s\n' \
            "Only useAssetsList.ts may send stack_mode; search rejects it with 400." >&2
        exit 1
    fi
fi

# Browse counts are total_visible / total_media_items / total_files; the
# asset-granular total_assets is gone. (Unrelated `TotalAssets` stats on face
# and OCR progress are a different concept and stay.)
if matches="$(
    git grep -n -E 'json:"(results_)?total_assets' -- 'server/internal/api/dto' \
    || true
)"; then
    if [ -n "$matches" ]; then
        printf '%s\n' \
            "Browse architecture check found a retired total_assets browse count:" \
            "$matches" >&2
        printf '%s\n' \
            "Use total_visible / total_media_items / total_files." >&2
        exit 1
    fi
fi

# Stack membership is media-item granular (BrowseStack.Members /
# MatchedMembers). The retired asset-id lists are not grepped for: the field
# names collide with legitimate album/folder/tag cover fields, and their
# absence is already enforced by the Go and TypeScript types.

printf '%s\n' "SQLite SQL checks passed"
printf '%s\n' "Browse architecture checks passed"

# Desktop is a separate Go module. Keep the new host boundary explicit: the
# server may not depend on Desktop, and only the runtime adapter may import the
# server application package. This is intentionally a small static guard so it
# works in local checkouts without a generated dependency graph.
desktop_server_imports="$({ rg -n '"server/' desktop --glob '*.go' || true; } | grep -vE '^desktop/internal/runtime/' || true)"
if [ -n "$desktop_server_imports" ]; then
    printf '%s\n' "Desktop architecture check found a server import outside internal/runtime:" "$desktop_server_imports" >&2
    exit 1
fi

if rg -n '^[[:space:]]*"desktop/' server --glob '*.go' >/dev/null 2>&1; then
    printf '%s\n' "Desktop architecture check found a server -> desktop import." >&2
    exit 1
fi

printf '%s\n' "Desktop architecture checks passed"
