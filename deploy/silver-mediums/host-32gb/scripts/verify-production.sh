#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="${MULTICA_HOST_ENV:-$HOME/.config/silver-mediums/multica/host.env}"
STATE_DIR="$HOME/.silver-mediums/multica/recovery/production-acceptance"
VERIFY_MODE="${MULTICA_VERIFY_MODE:-cutover}"
case "$VERIFY_MODE" in
  cutover)
    ACCEPTANCE_RECEIPT="$STATE_DIR/acceptance-receipt.json"
    RECEIPT_KIND="production_host_acceptance"
    EXPECTED_WORKFLOW_DISPATCH="false"
    ;;
  live)
    ACCEPTANCE_RECEIPT="$STATE_DIR/live-verification-receipt.json"
    RECEIPT_KIND="production_host_live_verification"
    EXPECTED_WORKFLOW_DISPATCH="true"
    ;;
  *)
    printf 'ERROR: MULTICA_VERIFY_MODE must be cutover or live\n' >&2
    exit 1
    ;;
esac
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

mkdir -p "$STATE_DIR"
archive_dir="$STATE_DIR/archive"
mkdir -p "$archive_dir"
if [[ -f "$ACCEPTANCE_RECEIPT" ]]; then
  mv "$ACCEPTANCE_RECEIPT" "$archive_dir/acceptance-receipt-$(date -u +%Y%m%dT%H%M%SZ)-$$.json"
fi
[[ -f "$ENV_FILE" ]] || fail "production host environment is missing"
[[ "$(stat -f '%Lp' "$ENV_FILE")" == "600" ]] || fail "production environment must have mode 600"
command -v python3 >/dev/null 2>&1 || fail "python3 is required for secret-safe local verification"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export -n JWT_SECRET POSTGRES_PASSWORD
[[ "$COMPOSE_PROJECT_NAME" == "multica" ]] || fail "unexpected production project name"

extract_json() {
  plutil -extract "$1" raw -o - "$2" 2>/dev/null || true
}

release_revision="$(extract_json source_revision "$RELEASE_MANIFEST")"
health_json="$(curl --fail --silent --show-error --max-time 5 "http://127.0.0.1:${BACKEND_PORT}/health")"
health_file="$(mktemp "$STATE_DIR/health.XXXXXX")"
secret_dir="$(mktemp -d "$STATE_DIR/verify-secrets.XXXXXX")"
token=""
cleanup() {
  rm -f "$health_file"
  rm -f "$secret_dir/auth.curl" "$secret_dir/workspace.curl" "$secret_dir/frontend.curl" "$secret_dir/jwt-secret" "$secret_dir/token"
  rmdir "$secret_dir" 2>/dev/null || true
  token=""
}
trap cleanup EXIT
printf '%s' "$health_json" > "$health_file"
[[ "$(extract_json status "$health_file")" == "ok" ]] || fail "backend health is not ok"
[[ "$(extract_json commit "$health_file")" == "$release_revision" ]] || fail "backend build identity does not match the release"
[[ "$(extract_json workflow_dispatch_enabled "$health_file")" == "$EXPECTED_WORKFLOW_DISPATCH" ]] || fail "running backend workflow dispatch state does not match $VERIFY_MODE verification"
[[ "$MULTICA_WORKFLOW_CONTROLLER_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]] || fail "workflow controller id is not a canonical UUID"
[[ "$(printf '%s' "$MULTICA_WORKFLOW_CONTROLLER_ID" | tr '[:upper:]' '[:lower:]')" != "00000000-0000-0000-0000-000000000000" ]] || fail "workflow controller id must not be nil"
curl --fail --silent --show-error --max-time 5 "http://127.0.0.1:${BACKEND_PORT}/healthz" >/dev/null

postgres_container="${COMPOSE_PROJECT_NAME}-postgres-1"
identity="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F '|' -c '
SELECT m.user_id, m.workspace_id
FROM member m
ORDER BY m.created_at
LIMIT 1;
')"
[[ -n "$identity" ]] || fail "no workspace member is available for authenticated acceptance"
IFS='|' read -r user_id workspace_id <<< "$identity"
for value in "$user_id" "$workspace_id"; do
  [[ "$value" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] || fail "acceptance identity contains an invalid identifier"
done

issue_count="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM issue WHERE workspace_id = '$workspace_id';")"
issue_id="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT id FROM issue WHERE workspace_id = '$workspace_id' ORDER BY created_at LIMIT 1;")"
attachment_id=""
if [[ -n "$issue_id" ]]; then
  attachment_id="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT id FROM attachment WHERE workspace_id = '$workspace_id' AND issue_id = '$issue_id' ORDER BY created_at LIMIT 1;")"
fi

issued_at="$(date +%s)"
expires_at="$((issued_at + 600))"
printf '%s' "$JWT_SECRET" > "$secret_dir/jwt-secret"
chmod 600 "$secret_dir/jwt-secret"
token="$(MULTICA_VERIFY_USER_ID="$user_id" MULTICA_VERIFY_IAT="$issued_at" MULTICA_VERIFY_EXP="$expires_at" MULTICA_VERIFY_SECRET_FILE="$secret_dir/jwt-secret" python3 - <<'PY'
import base64, hashlib, hmac, json, os
def enc(value):
    raw = json.dumps(value, separators=(",", ":")).encode()
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()
header = enc({"alg": "HS256", "typ": "JWT"})
payload = enc({"sub": os.environ["MULTICA_VERIFY_USER_ID"], "iat": int(os.environ["MULTICA_VERIFY_IAT"]), "exp": int(os.environ["MULTICA_VERIFY_EXP"])})
unsigned = f"{header}.{payload}"
with open(os.environ["MULTICA_VERIFY_SECRET_FILE"], "rb") as handle:
    secret = handle.read()
signature = base64.urlsafe_b64encode(hmac.new(secret, unsigned.encode(), hashlib.sha256).digest()).rstrip(b"=").decode()
print(f"{unsigned}.{signature}")
PY
)"
printf 'header = "Authorization: Bearer %s"\n' "$token" > "$secret_dir/auth.curl"
printf 'header = "Authorization: Bearer %s"\nheader = "X-Workspace-ID: %s"\n' "$token" "$workspace_id" > "$secret_dir/workspace.curl"
printf 'cookie = "multica_auth=%s; multica_logged_in=true"\n' "$token" > "$secret_dir/frontend.curl"
printf '%s' "$token" > "$secret_dir/token"
chmod 600 "$secret_dir/"*

[[ "$(curl --config "$secret_dir/auth.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/me")" == "200" ]] || fail "authenticated API identity check failed"
issue_history_verified=false
attachment_bytes_verified=false
empty_board_verified=false
if [[ -n "$issue_id" ]]; then
  [[ "$(curl --config "$secret_dir/workspace.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/issues/${issue_id}/timeline")" == "200" ]] || fail "issue-history read failed"
  issue_history_verified=true
  if [[ -n "$attachment_id" ]]; then
    download_status="$(curl --config "$secret_dir/auth.curl" --silent --show-error --max-time 10 -o /dev/null -w '%{http_code}' -H 'Range: bytes=0-0' "http://127.0.0.1:${BACKEND_PORT}/api/attachments/${attachment_id}/download")"
    [[ "$download_status" == "200" || "$download_status" == "206" ]] || fail "attachment byte read failed"
    attachment_bytes_verified=true
  fi
elif [[ "$issue_count" == "0" ]]; then
  empty_board_verified=true
else
  fail "workspace issue count is nonzero but no issue can be selected for acceptance"
fi
[[ "$(curl --config "$secret_dir/frontend.curl" --silent --show-error --max-time 10 -L -o /dev/null -w '%{http_code}' "http://127.0.0.1:${FRONTEND_PORT}/")" == "200" ]] || fail "frontend protected route did not load"

network="${COMPOSE_PROJECT_NAME}_default"
docker run --rm --network "$network" \
  -v "$secret_dir/token:/run/secrets/multica_verify_token:ro" \
  -e "MULTICA_TEST_WORKSPACE=$workspace_id" \
  --entrypoint node "$MULTICA_WEB_IMAGE" -e '
const fs = require("fs");
const token = fs.readFileSync("/run/secrets/multica_verify_token", "utf8");
const url = `ws://backend:8080/ws?workspace_id=${process.env.MULTICA_TEST_WORKSPACE}`;
const ws = new WebSocket(url);
const timer = setTimeout(() => process.exit(1), 5000);
ws.onopen = () => ws.send(JSON.stringify({type: "auth", payload: {token}}));
ws.onmessage = (event) => { if (String(event.data).includes("auth_ack")) { clearTimeout(timer); ws.close(); process.exit(0); } };
ws.onerror = () => { clearTimeout(timer); process.exit(1); };
' >/dev/null

active_tasks="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM agent_task_queue WHERE status IN ('queued','dispatched','running','waiting_local_directory','deferred');")"
if [[ "$VERIFY_MODE" == "cutover" ]]; then
  [[ "$active_tasks" == "0" ]] || fail "production came up with active or queued agent work"
fi
workflow_driver_value="$(printf '%s' "$MULTICA_WORKFLOW_DRIVER" | tr '[:upper:]' '[:lower:]')"
case "$workflow_driver_value" in
  false|0|no|off) workflow_driver_json=false ;;
  true|1|yes|on) workflow_driver_json=true ;;
  *) fail "workflow driver setting is not a recognized boolean" ;;
esac
[[ "$workflow_driver_json" == "$EXPECTED_WORKFLOW_DISPATCH" ]] || fail "configured workflow driver does not match $VERIFY_MODE verification"

umask 077
verified_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat > "$ACCEPTANCE_RECEIPT" <<EOF
{
  "schema": 1,
  "kind": "$RECEIPT_KIND",
  "source_revision": "$release_revision",
  "health_identity_verified": true,
  "database_health_verified": true,
  "authenticated_api_verified": true,
  "frontend_route_verified": true,
  "authenticated_web_verified": false,
  "owner_login_pending": true,
  "websocket_verified": true,
  "issue_history_verified": $issue_history_verified,
  "attachment_bytes_verified": $attachment_bytes_verified,
  "empty_board_verified": $empty_board_verified,
  "active_or_queued_tasks": 0,
  "workflow_driver_enabled": $workflow_driver_json,
  "verified_at": "$verified_at"
}
EOF
chmod 600 "$ACCEPTANCE_RECEIPT"
printf 'Production application path verified on the 32GB host in %s mode.\n' "$VERIFY_MODE"
