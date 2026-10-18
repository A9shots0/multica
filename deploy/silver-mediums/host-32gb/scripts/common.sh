#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROFILE_DIR/../../.." && pwd)"
ENV_FILE="${MULTICA_HOST_ENV:-$HOME/.config/silver-mediums/multica/host.env}"
BASE_COMPOSE="$REPO_ROOT/docker-compose.selfhost.yml"
HOST_COMPOSE="$PROFILE_DIR/compose.yml"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

load_host_env() {
  [[ -f "$ENV_FILE" ]] || fail "protected host environment is missing: $ENV_FILE"
  local mode
  mode="$(stat -f '%Lp' "$ENV_FILE")"
  [[ "$mode" == "600" ]] || fail "host environment must have mode 600; found $mode"
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a

  local name
  for name in \
    MULTICA_BACKEND_IMAGE MULTICA_WEB_IMAGE MULTICA_POSTGRES_IMAGE \
    POSTGRES_DB POSTGRES_USER POSTGRES_PASSWORD JWT_SECRET \
    MULTICA_RECOVERY_STATE_PATH MULTICA_DATA_READY_RECEIPT \
    FRONTEND_ORIGIN CORS_ALLOWED_ORIGINS MULTICA_APP_URL \
    MULTICA_DAEMON_SERVER_URL LOCAL_UPLOAD_BASE_URL \
    MULTICA_WORKFLOW_CONTROLLER_ID; do
    [[ -n "${!name:-}" ]] || fail "required host setting is empty: $name"
  done

  case "$POSTGRES_PASSWORD $JWT_SECRET $MULTICA_WORKFLOW_CONTROLLER_ID" in
    *replace-with-*) fail "host environment still contains placeholder values" ;;
  esac
}

assert_data_ready() {
  [[ -f "$MULTICA_DATA_READY_RECEIPT" ]] || fail "verified data-ready receipt is missing"
  local receipt_mode accepted receipt_revision database_verified uploads_verified
  receipt_mode="$(stat -f '%Lp' "$MULTICA_DATA_READY_RECEIPT")"
  [[ "$receipt_mode" == "600" ]] || fail "data-ready receipt must have mode 600; found $receipt_mode"
  accepted="$(plutil -extract accepted raw -o - "$MULTICA_DATA_READY_RECEIPT" 2>/dev/null || true)"
  receipt_revision="$(plutil -extract source_revision raw -o - "$MULTICA_DATA_READY_RECEIPT" 2>/dev/null || true)"
  database_verified="$(plutil -extract database_restore_verified raw -o - "$MULTICA_DATA_READY_RECEIPT" 2>/dev/null || true)"
  uploads_verified="$(plutil -extract uploads_restore_verified raw -o - "$MULTICA_DATA_READY_RECEIPT" 2>/dev/null || true)"
  [[ "$accepted" == "true" ]] || fail "data-ready receipt has not been accepted"
  # This receipt proves the persistent volumes were restored and verified. Its
  # source revision is the release that performed that restore, not a pin that
  # should block every later application upgrade from using the same volumes.
  [[ -n "$receipt_revision" ]] || fail "data-ready receipt source revision is missing"
  [[ "$database_verified" == "true" ]] || fail "database restore is not verified"
  [[ "$uploads_verified" == "true" ]] || fail "uploads restore is not verified"
}

compose() {
  docker compose \
    --env-file "$ENV_FILE" \
    -f "$BASE_COMPOSE" \
    -f "$HOST_COMPOSE" \
    "$@"
}

wait_for_url() {
  local url="$1"
  local attempts="${2:-120}"
  local i
  for ((i = 1; i <= attempts; i++)); do
    if curl --fail --silent --show-error --max-time 3 "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  fail "timed out waiting for $url"
}
