#!/usr/bin/env bash
set -euo pipefail

SCRIPT_NAME="$(basename "$0")"

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
TOKEN="${TOKEN:-}"
REPOSITORY_ID="${REPOSITORY_ID:-}"
REPO_PATH="${REPO_PATH:-}"
DATABASE_URL="${DATABASE_URL:-}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-120}"
POLL_INTERVAL="${POLL_INTERVAL:-2}"

RUN_ID="$(date +%Y%m%d_%H%M%S)_$RANDOM"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/lumilio-acceptance.XXXXXX")"
DB_ENABLED=0
BASE64_DECODE_FLAG=""

CONFIG_FILE=""
CONFIG_BACKUP=""

log() {
  local level="$1"
  shift
  printf '[%s] %s\n' "$level" "$*"
}

info() {
  log INFO "$@"
}

pass() {
  log PASS "$@"
}

warn() {
  log WARN "$@"
}

fail() {
  log FAIL "$@"
  exit 1
}

cleanup() {
  if [[ -n "$CONFIG_BACKUP" && -f "$CONFIG_BACKUP" ]]; then
    if [[ -n "$CONFIG_FILE" && ! -f "$CONFIG_FILE" ]]; then
      mv "$CONFIG_BACKUP" "$CONFIG_FILE"
      log INFO "restored repository config: $CONFIG_FILE"
    fi
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

usage() {
  cat <<'EOF'
Manual acceptance script for M1/M2/M3.

Usage:
  scripts/manual_acceptance.sh <m1|m2|m3|all> [options]

Options:
  --api-base <url>         API base URL (default: http://localhost:8080/api/v1)
  --repo-path <path>       Repository absolute path (required)
  --repo-id <uuid>         Repository UUID (optional but recommended in multi-repo tests)
  --token <jwt>            Optional Bearer token
  --database-url <dsn>     Optional postgres DSN for stronger DB assertions
  --timeout <seconds>      Poll timeout (default: 120)
  --poll <seconds>         Poll interval (default: 2)
  -h, --help               Show help

Environment variables are also supported:
  API_BASE TOKEN REPOSITORY_ID REPO_PATH DATABASE_URL TIMEOUT_SECONDS POLL_INTERVAL
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    fail "missing command: $cmd"
  fi
}

check_base64_decode_flag() {
  if printf 'QQ==' | base64 -d >/dev/null 2>&1; then
    BASE64_DECODE_FLAG="-d"
    return
  fi
  if printf 'QQ==' | base64 -D >/dev/null 2>&1; then
    BASE64_DECODE_FLAG="-D"
    return
  fi
  fail "base64 decode flag not supported (-d/-D)"
}

require_repo_path() {
  if [[ -z "$REPO_PATH" ]]; then
    fail "REPO_PATH is required. pass --repo-path or set REPO_PATH"
  fi
  if [[ ! -d "$REPO_PATH" ]]; then
    fail "repository path does not exist: $REPO_PATH"
  fi
}

sql_escape() {
  printf "%s" "$1" | sed "s/'/''/g"
}

db_query_scalar() {
  local sql="$1"
  if [[ "$DB_ENABLED" -ne 1 ]]; then
    return 0
  fi
  psql "$DATABASE_URL" -At -c "$sql"
}

api_request() {
  local method="$1"
  local path="$2"
  shift 2

  local url="${API_BASE%/}${path}"
  local body_file="$TMP_DIR/api_body.json"
  local -a curl_args

  local status
  curl_args=(-sS -o "$body_file" -w "%{http_code}" -X "$method")
  if [[ -n "${TOKEN:-}" ]]; then
    curl_args+=(-H "Authorization: Bearer $TOKEN")
  fi
  if [[ "$#" -gt 0 ]]; then
    curl_args+=("$@")
  fi
  curl_args+=("$url")

  set +e
  status="$(curl "${curl_args[@]}")"
  local rc=$?
  set -e
  if [[ $rc -ne 0 ]]; then
    fail "curl failed: ${method} ${url}"
  fi

  API_STATUS="$status"
  API_BODY="$(cat "$body_file")"
}

assert_api_success() {
  local context="$1"
  if [[ "$API_STATUS" != "200" ]]; then
    fail "$context failed: http=$API_STATUS body=$API_BODY"
  fi
  local code
  code="$(jq -r '.code // empty' <<<"$API_BODY")"
  if [[ "$code" != "0" ]]; then
    fail "$context failed: code=$code body=$API_BODY"
  fi
}

health_check() {
  api_request GET "/health"
  assert_api_success "health check"
  pass "API health is ready"
}

write_tiny_gif() {
  local output="$1"
  printf 'R0lGODdhAQABAIABAP///wAAACwAAAAAAQABAAACAkQBADs=' | base64 "$BASE64_DECODE_FLAG" >"$output"
}

make_unique_hash() {
  local seed="cas-${RUN_ID}-${RANDOM}-$(date +%s)"
  printf '%s' "$seed" | shasum -a 256 | awk '{print tolower($1)}'
}

build_asset_list_payload() {
  local filename="$1"
  jq -n \
    --arg q "$filename" \
    --arg rid "$REPOSITORY_ID" \
    '{
      query: $q,
      search_type: "filename",
      filter: (if $rid == "" then {} else {repository_id: $rid} end),
      pagination: {limit: 100, offset: 0}
    }'
}

get_asset_by_filename_once() {
  local filename="$1"
  local payload
  payload="$(build_asset_list_payload "$filename")"
  api_request POST "/assets/list" -H "Content-Type: application/json" --data "$payload"
  assert_api_success "query assets by filename ${filename}"
  jq -c --arg name "$filename" '[.data.assets[]? | select(.original_filename == $name)][0] // empty' <<<"$API_BODY"
}

wait_for_asset_by_filename() {
  local filename="$1"
  local start
  start="$(date +%s)"

  while true; do
    local asset
    asset="$(get_asset_by_filename_once "$filename")"
    if [[ -n "$asset" && "$asset" != "null" ]]; then
      printf '%s' "$asset"
      return 0
    fi
    if (( "$(date +%s)" - start >= TIMEOUT_SECONDS )); then
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

wait_for_filename_absent() {
  local filename="$1"
  local start
  start="$(date +%s)"
  while true; do
    local asset
    asset="$(get_asset_by_filename_once "$filename")"
    if [[ -z "$asset" || "$asset" == "null" ]]; then
      return 0
    fi
    if (( "$(date +%s)" - start >= TIMEOUT_SECONDS )); then
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

get_asset_by_id_once() {
  local asset_id="$1"
  api_request GET "/assets/${asset_id}"
  if [[ "$API_STATUS" != "200" ]]; then
    return 1
  fi
  local code
  code="$(jq -r '.code // empty' <<<"$API_BODY")"
  if [[ "$code" != "0" ]]; then
    return 1
  fi
  jq -c '.data' <<<"$API_BODY"
}

wait_for_hash_change() {
  local asset_id="$1"
  local old_hash="$2"
  local start
  start="$(date +%s)"

  while true; do
    local asset
    asset="$(get_asset_by_id_once "$asset_id" || true)"
    if [[ -n "$asset" ]]; then
      local current_hash
      current_hash="$(jq -r '.hash // empty' <<<"$asset")"
      if [[ -n "$current_hash" && "$current_hash" != "$old_hash" ]]; then
        printf '%s' "$current_hash"
        return 0
      fi
    fi
    if (( "$(date +%s)" - start >= TIMEOUT_SECONDS )); then
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

upload_asset_file() {
  local file_path="$1"
  local file_name="$2"
  local hash_header="${3:-}"

  local args=("-F" "file=@${file_path};filename=${file_name};type=image/gif")
  if [[ -n "$REPOSITORY_ID" ]]; then
    args+=("-F" "repository_id=${REPOSITORY_ID}")
  fi
  if [[ -n "$hash_header" ]]; then
    args+=("-H" "X-Content-Hash: ${hash_header}")
  fi

  api_request POST "/assets" "${args[@]}"
  assert_api_success "upload asset ${file_name}"
}

find_trash_metadata_by_original_path() {
  local original_path="$1"
  local trash_dir="$REPO_PATH/.lumilio/trash"

  if [[ ! -d "$trash_dir" ]]; then
    return 1
  fi

  local meta
  for meta in "$trash_dir"/*.json; do
    [[ -f "$meta" ]] || continue
    local op
    op="$(jq -r '.original_path // empty' "$meta" 2>/dev/null || true)"
    if [[ "$op" == "$original_path" ]]; then
      printf '%s' "$meta"
      return 0
    fi
  done
  return 1
}

wait_for_failed_file() {
  local original_name="$1"
  local failed_dir="$REPO_PATH/.lumilio/staging/failed"
  local base ext pattern
  ext="${original_name##*.}"
  if [[ "$ext" == "$original_name" ]]; then
    base="$original_name"
    pattern="${base}_*"
  else
    base="${original_name%.*}"
    pattern="${base}_*.${ext}"
  fi

  local start
  start="$(date +%s)"
  while true; do
    if [[ -d "$failed_dir" ]]; then
      local found
      found="$(find "$failed_dir" -maxdepth 1 -type f -name "$pattern" | head -n 1)"
      if [[ -n "$found" ]]; then
        printf '%s' "$found"
        return 0
      fi
    fi
    if (( "$(date +%s)" - start >= TIMEOUT_SECONDS )); then
      return 1
    fi
    sleep "$POLL_INTERVAL"
  done
}

assert_storage_strategy_cas() {
  local cfg="$REPO_PATH/.lumiliorepo"
  if [[ ! -f "$cfg" ]]; then
    fail "missing repository config: $cfg"
  fi
  local strategy
  strategy="$(awk -F': *' '$1=="storage_strategy"{gsub(/[[:space:]]/, "", $2); print tolower($2)}' "$cfg" | tr -d '"')"
  if [[ "$strategy" != "cas" ]]; then
    fail "repository storage_strategy is '$strategy' (expect 'cas' for M2)"
  fi
  pass "repository storage_strategy=cas"
}

run_m1() {
  require_repo_path
  info "M1: watchman -> discover -> DB 变更闭环"

  local workspace="$REPO_PATH/manual_acceptance"
  mkdir -p "$workspace"

  local file_a="m1_${RUN_ID}_a.gif"
  local file_b="m1_${RUN_ID}_b.gif"
  local full_a="$workspace/$file_a"
  local full_b="$workspace/$file_b"
  local rel_a="manual_acceptance/$file_a"
  local rel_b="manual_acceptance/$file_b"

  write_tiny_gif "$full_a"
  info "M1.1 已创建测试文件: $rel_a"

  local asset_a
  asset_a="$(wait_for_asset_by_filename "$file_a" || true)"
  if [[ -z "$asset_a" ]]; then
    fail "M1.1 discover upsert 超时，未发现文件: $file_a"
  fi

  local asset_a_id asset_a_hash asset_repo_id storage_a
  asset_a_id="$(jq -r '.asset_id' <<<"$asset_a")"
  asset_a_hash="$(jq -r '.hash // empty' <<<"$asset_a")"
  asset_repo_id="$(jq -r '.repository_id // empty' <<<"$asset_a")"
  storage_a="$(jq -r '.storage_path // empty' <<<"$asset_a")"

  if [[ "$storage_a" != "$rel_a" ]]; then
    fail "M1.1 storage_path 不匹配: got=$storage_a expect=$rel_a"
  fi
  pass "M1.1 upsert 成功，asset_id=$asset_a_id"

  printf '\n# m1-modify-%s\n' "$(date +%s)" >>"$full_a"
  local new_hash
  new_hash="$(wait_for_hash_change "$asset_a_id" "$asset_a_hash" || true)"
  if [[ -z "$new_hash" ]]; then
    fail "M1.2 同路径修改未触发 hash 变化（可能被吞掉）"
  fi
  pass "M1.2 同路径修改生效，hash 已更新"

  mv "$full_a" "$full_b"
  info "M1.3 已重命名: $rel_a -> $rel_b"

  local asset_b
  asset_b="$(wait_for_asset_by_filename "$file_b" || true)"
  if [[ -z "$asset_b" ]]; then
    fail "M1.3 重命名后新路径未收敛: $file_b"
  fi
  if ! wait_for_filename_absent "$file_a"; then
    fail "M1.3 重命名后旧路径仍在活跃资产列表中: $file_a"
  fi
  pass "M1.3 重命名闭环通过（新路径 upsert + 旧路径删除）"

  rm -f "$full_b"
  info "M1.4 已删除文件: $rel_b"

  if ! wait_for_filename_absent "$file_b"; then
    fail "M1.4 删除事件未收敛: $file_b 仍在活跃资产列表中"
  fi
  pass "M1.4 删除闭环通过"

  if [[ "$DB_ENABLED" -eq 1 ]]; then
    local rid old_flag new_flag
    rid="$asset_repo_id"
    if [[ -z "$rid" ]]; then
      rid="$REPOSITORY_ID"
    fi
    if [[ -n "$rid" ]]; then
      old_flag="$(db_query_scalar "SELECT COALESCE((SELECT is_deleted::text FROM assets WHERE repository_id = '$(sql_escape "$rid")'::uuid AND storage_path = '$(sql_escape "$rel_a")' LIMIT 1), 'missing');")"
      new_flag="$(db_query_scalar "SELECT COALESCE((SELECT is_deleted::text FROM assets WHERE repository_id = '$(sql_escape "$rid")'::uuid AND storage_path = '$(sql_escape "$rel_b")' LIMIT 1), 'missing');")"
      if [[ "$old_flag" != "true" || "$new_flag" != "true" ]]; then
        fail "M1 DB 校验失败: old=$old_flag new=$new_flag (expect true/true)"
      fi
      pass "M1 DB 校验通过（旧/新路径均已软删）"
    else
      warn "M1 跳过 DB 强校验：未拿到 repository_id"
    fi
  else
    warn "M1 跳过 DB 强校验（未配置 DATABASE_URL）"
  fi
}

run_m2() {
  require_repo_path
  info "M2: CAS 接入 + 多仓路径解析检查"

  assert_storage_strategy_cas

  local upload_file="$TMP_DIR/m2_${RUN_ID}.gif"
  local file_name="m2_${RUN_ID}.gif"
  local expected_hash expected_rel

  write_tiny_gif "$upload_file"
  expected_hash="$(make_unique_hash)"

  upload_asset_file "$upload_file" "$file_name" "$expected_hash"
  local task_id
  task_id="$(jq -r '.data.task_id // empty' <<<"$API_BODY")"
  pass "M2 上传成功，task_id=${task_id:-unknown}"

  local asset
  asset="$(wait_for_asset_by_filename "$file_name" || true)"
  if [[ -z "$asset" ]]; then
    fail "M2 上传后未发现资产: $file_name"
  fi

  local asset_id asset_repo_id storage_path actual_hash
  asset_id="$(jq -r '.asset_id' <<<"$asset")"
  asset_repo_id="$(jq -r '.repository_id // empty' <<<"$asset")"
  storage_path="$(jq -r '.storage_path // empty' <<<"$asset")"
  actual_hash="$(jq -r '.hash // empty' <<<"$asset")"

  expected_rel="inbox/${expected_hash:0:2}/${expected_hash:2:2}/${expected_hash:4:2}/${expected_hash}.gif"
  if [[ "$storage_path" != "$expected_rel" ]]; then
    fail "M2 CAS 路径不匹配: got=$storage_path expect=$expected_rel"
  fi
  if [[ "$actual_hash" != "$expected_hash" ]]; then
    fail "M2 hash 不匹配: got=$actual_hash expect=$expected_hash"
  fi
  if [[ -n "$REPOSITORY_ID" && "$asset_repo_id" != "$REPOSITORY_ID" ]]; then
    fail "M2 repository_id 不匹配: got=$asset_repo_id expect=$REPOSITORY_ID"
  fi
  if [[ ! -f "$REPO_PATH/$storage_path" ]]; then
    fail "M2 CAS 文件未落盘: $REPO_PATH/$storage_path"
  fi
  pass "M2 CAS 路径与落盘校验通过"

  api_request HEAD "/assets/${asset_id}/original"
  if [[ "$API_STATUS" != "200" ]]; then
    fail "M2 original 访问失败: http=$API_STATUS"
  fi
  pass "M2 GetOriginalFile 校验通过"

  api_request HEAD "/assets/${asset_id}/thumbnail"
  if [[ "$API_STATUS" != "200" && "$API_STATUS" != "404" ]]; then
    fail "M2 thumbnail 返回异常: http=$API_STATUS (expect 200/404)"
  fi
  pass "M2 GetThumbnail 校验通过（200/404 可接受）"

  api_request HEAD "/assets/${asset_id}/video/web"
  if [[ "$API_STATUS" == "500" ]]; then
    fail "M2 web video 出现 500（路径解析异常）"
  fi
  pass "M2 GetWebVideo 无 500"

  api_request HEAD "/assets/${asset_id}/audio/web"
  if [[ "$API_STATUS" == "500" ]]; then
    fail "M2 web audio 出现 500（路径解析异常）"
  fi
  pass "M2 GetWebAudio 无 500"

  info "M2 备注：多仓场景建议显式传 --repo-id 指向非默认仓库，才能覆盖“第一个仓库”回归点。"
}

run_m3() {
  require_repo_path
  info "M3: Trash 与 Failed 路径集成验收"

  local del_file="$TMP_DIR/m3_delete_${RUN_ID}.gif"
  local del_name="m3_delete_${RUN_ID}.gif"
  write_tiny_gif "$del_file"

  upload_asset_file "$del_file" "$del_name"
  local del_asset
  del_asset="$(wait_for_asset_by_filename "$del_name" || true)"
  if [[ -z "$del_asset" ]]; then
    fail "M3.1 删除链路测试前未发现资产: $del_name"
  fi

  local del_asset_id del_storage del_repo_id
  del_asset_id="$(jq -r '.asset_id' <<<"$del_asset")"
  del_storage="$(jq -r '.storage_path // empty' <<<"$del_asset")"
  del_repo_id="$(jq -r '.repository_id // empty' <<<"$del_asset")"

  if [[ -n "$del_storage" && -f "$REPO_PATH/$del_storage" ]]; then
    pass "M3.1 删除前原文件存在"
  else
    warn "M3.1 删除前原文件不存在，继续执行（允许缺失文件软删）"
  fi

  api_request DELETE "/assets/${del_asset_id}"
  assert_api_success "delete asset ${del_asset_id}"

  if [[ -n "$del_storage" && -f "$REPO_PATH/$del_storage" ]]; then
    fail "M3.1 删除后原文件仍存在: $REPO_PATH/$del_storage"
  fi

  local trash_meta
  trash_meta="$(find_trash_metadata_by_original_path "$del_storage" || true)"
  if [[ -z "$trash_meta" ]]; then
    fail "M3.1 未在 trash 找到 original_path=$del_storage 的元数据"
  fi
  pass "M3.1 trash 文件已写入: $trash_meta"

  api_request GET "/assets/${del_asset_id}"
  if [[ "$API_STATUS" != "404" ]]; then
    fail "M3.1 删除后 GetAsset 应为 404，实际 http=$API_STATUS"
  fi
  pass "M3.1 DB 软删语义通过（API 层不可见）"

  if [[ "$DB_ENABLED" -eq 1 ]]; then
    local rid is_deleted
    rid="$del_repo_id"
    if [[ -z "$rid" ]]; then
      rid="$REPOSITORY_ID"
    fi
    if [[ -n "$rid" ]]; then
      is_deleted="$(db_query_scalar "SELECT COALESCE((SELECT is_deleted::text FROM assets WHERE asset_id = '$(sql_escape "$del_asset_id")'::uuid LIMIT 1), 'missing');")"
      if [[ "$is_deleted" != "true" ]]; then
        fail "M3.1 DB 强校验失败: is_deleted=$is_deleted"
      fi
      pass "M3.1 DB 强校验通过（is_deleted=true）"
    fi
  fi

  CONFIG_FILE="$REPO_PATH/.lumiliorepo"
  if [[ ! -f "$CONFIG_FILE" ]]; then
    fail "M3.2 缺少仓库配置文件，无法进行 ingest 失败注入: $CONFIG_FILE"
  fi
  CONFIG_BACKUP="$REPO_PATH/.lumiliorepo.acceptance_backup_${RUN_ID}"
  mv "$CONFIG_FILE" "$CONFIG_BACKUP"
  info "M3.2 已暂时移除仓库配置以触发 ingest commit 失败"

  local fail_file="$TMP_DIR/m3_fail_${RUN_ID}.gif"
  local fail_name="m3_fail_${RUN_ID}.gif"
  write_tiny_gif "$fail_file"
  upload_asset_file "$fail_file" "$fail_name"

  local failed_path
  failed_path="$(wait_for_failed_file "$fail_name" || true)"

  if [[ -f "$CONFIG_BACKUP" && ! -f "$CONFIG_FILE" ]]; then
    mv "$CONFIG_BACKUP" "$CONFIG_FILE"
  fi
  CONFIG_BACKUP=""

  if [[ -z "$failed_path" ]]; then
    fail "M3.2 未检测到 failed 目录落盘文件: $fail_name"
  fi
  pass "M3.2 failed 文件落盘成功: $failed_path"

  if [[ "$DB_ENABLED" -eq 1 ]]; then
    local where_clause status_state row
    where_clause="original_filename = '$(sql_escape "$fail_name")'"
    if [[ -n "$REPOSITORY_ID" ]]; then
      where_clause="$where_clause AND repository_id = '$(sql_escape "$REPOSITORY_ID")'::uuid"
    fi
    row="$(db_query_scalar "SELECT COALESCE(status->>'state',''), COALESCE(is_deleted::text,'') FROM assets WHERE ${where_clause} ORDER BY upload_time DESC LIMIT 1;")"
    status_state="$(printf '%s' "$row" | awk -F'|' '{print $1}')"
    if [[ "$status_state" != "failed" ]]; then
      fail "M3.2 失败状态校验失败: status.state=$status_state (expect failed)"
    fi
    pass "M3.2 DB 强校验通过（status.state=failed）"
  else
    warn "M3.2 跳过 failed 状态 DB 强校验（未配置 DATABASE_URL）"
  fi
}

main() {
  if [[ $# -lt 1 ]]; then
    usage
    exit 1
  fi

  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  local mode="$1"
  shift

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --api-base)
        API_BASE="$2"
        shift 2
        ;;
      --repo-path)
        REPO_PATH="$2"
        shift 2
        ;;
      --repo-id)
        REPOSITORY_ID="$2"
        shift 2
        ;;
      --token)
        TOKEN="$2"
        shift 2
        ;;
      --database-url)
        DATABASE_URL="$2"
        shift 2
        ;;
      --timeout)
        TIMEOUT_SECONDS="$2"
        shift 2
        ;;
      --poll)
        POLL_INTERVAL="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
  done

  case "$mode" in
    m1|m2|m3|all) ;;
    *)
      fail "invalid mode: $mode (use m1|m2|m3|all)"
      ;;
  esac

  require_cmd curl
  require_cmd jq
  require_cmd base64
  require_cmd shasum

  check_base64_decode_flag

  if [[ -n "$DATABASE_URL" ]]; then
    require_cmd psql
    DB_ENABLED=1
  fi

  info "run_id=$RUN_ID"
  info "api_base=$API_BASE"
  info "repo_path=${REPO_PATH:-<empty>}"
  info "repository_id=${REPOSITORY_ID:-<auto>}"
  info "db_checks=$([[ "$DB_ENABLED" -eq 1 ]] && echo enabled || echo disabled)"

  health_check

  case "$mode" in
    m1)
      run_m1
      ;;
    m2)
      run_m2
      ;;
    m3)
      run_m3
      ;;
    all)
      run_m1
      run_m2
      run_m3
      ;;
  esac

  pass "全部校验完成: mode=$mode"
}

main "$@"
