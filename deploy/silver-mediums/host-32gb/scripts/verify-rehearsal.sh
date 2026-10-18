#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REHEARSAL_ID="${1:-}"
[[ "$REHEARSAL_ID" =~ ^[a-z0-9]{6,24}$ ]] || { printf 'ERROR: rehearsal id must contain 6-24 lowercase letters or digits\n' >&2; exit 1; }

ENV_FILE="$HOME/.config/silver-mediums/multica/rehearsals/$REHEARSAL_ID.env"
STATE_DIR="$HOME/.silver-mediums/multica/rehearsals/$REHEARSAL_ID"
RESTORE_RECEIPT="$STATE_DIR/restore-receipt.json"
ACCEPTANCE_RECEIPT="$STATE_DIR/acceptance-receipt.json"
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
archive_dir="$STATE_DIR/archive"
if [[ -d "$STATE_DIR" ]]; then
  mkdir -p "$archive_dir"
  if [[ -f "$ACCEPTANCE_RECEIPT" ]]; then
    mv "$ACCEPTANCE_RECEIPT" "$archive_dir/acceptance-receipt-$(date -u +%Y%m%dT%H%M%SZ)-$$.json"
  fi
fi
[[ -f "$ENV_FILE" && -f "$RESTORE_RECEIPT" ]] || { printf 'ERROR: rehearsal restore state is incomplete\n' >&2; exit 1; }
[[ "$(stat -f '%Lp' "$ENV_FILE")" == "600" ]] || { printf 'ERROR: rehearsal environment must have mode 600\n' >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { printf 'ERROR: python3 is required for secret-safe local verification\n' >&2; exit 1; }

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export -n JWT_SECRET POSTGRES_PASSWORD
[[ "$COMPOSE_PROJECT_NAME" == "multica32_rehearsal_${REHEARSAL_ID}" ]] || { printf 'ERROR: unexpected rehearsal project name\n' >&2; exit 1; }

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
health_status="$(extract_json status "$health_file")"
health_revision="$(extract_json commit "$health_file")"
[[ "$health_status" == "ok" && "$health_revision" == "$release_revision" ]] || { printf 'ERROR: backend health identity does not match the release\n' >&2; exit 1; }
[[ "$(extract_json workflow_dispatch_enabled "$health_file")" == "false" ]] || { printf 'ERROR: running backend did not prove the workflow dispatch fence\n' >&2; exit 1; }
curl --fail --silent --show-error --max-time 5 "http://127.0.0.1:${BACKEND_PORT}/healthz" >/dev/null

postgres_container="${COMPOSE_PROJECT_NAME}-postgres-1"
selection="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F '|' -c '
SELECT m.user_id, a.workspace_id, a.issue_id, a.id
FROM attachment a
JOIN member m ON m.workspace_id = a.workspace_id
WHERE a.issue_id IS NOT NULL
ORDER BY a.created_at, m.created_at
LIMIT 1;
')"
[[ -n "$selection" ]] || { printf 'ERROR: no member-owned issue attachment is available for the restored-flow proof\n' >&2; exit 1; }
IFS='|' read -r user_id workspace_id issue_id attachment_id <<< "$selection"
for value in "$user_id" "$workspace_id" "$issue_id" "$attachment_id"; do
  [[ "$value" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] || { printf 'ERROR: restored-flow selection contains an invalid identifier\n' >&2; exit 1; }
done

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

[[ "$(curl --config "$secret_dir/auth.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/me")" == "200" ]] || { printf 'ERROR: authenticated API identity check failed\n' >&2; exit 1; }
[[ "$(curl --config "$secret_dir/workspace.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/issues/${issue_id}")" == "200" ]] || { printf 'ERROR: restored issue read failed\n' >&2; exit 1; }
[[ "$(curl --config "$secret_dir/workspace.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/issues/${issue_id}/timeline")" == "200" ]] || { printf 'ERROR: restored issue-history read failed\n' >&2; exit 1; }
[[ "$(curl --config "$secret_dir/workspace.curl" --silent --show-error --max-time 5 -o /dev/null -w '%{http_code}' "http://127.0.0.1:${BACKEND_PORT}/api/attachments/${attachment_id}")" == "200" ]] || { printf 'ERROR: restored attachment metadata read failed\n' >&2; exit 1; }
download_status="$(curl --config "$secret_dir/auth.curl" --silent --show-error --max-time 10 -o /dev/null -w '%{http_code}' -H 'Range: bytes=0-0' "http://127.0.0.1:${BACKEND_PORT}/api/attachments/${attachment_id}/download")"
[[ "$download_status" == "200" || "$download_status" == "206" ]] || { printf 'ERROR: restored attachment byte read failed\n' >&2; exit 1; }

frontend_status="$(curl --config "$secret_dir/frontend.curl" --silent --show-error --max-time 10 -L -o /dev/null -w '%{http_code}' "http://127.0.0.1:${FRONTEND_PORT}/")"
[[ "$frontend_status" == "200" ]] || { printf 'ERROR: frontend route did not load\n' >&2; exit 1; }

network="${COMPOSE_PROJECT_NAME}_default"
docker run --rm --network "$network" \
  -v "$secret_dir/token:/run/secrets/multica_verify_token:ro" \
  -e "MULTICA_TEST_WORKSPACE=$workspace_id" \
  --entrypoint node "$MULTICA_WEB_IMAGE" -e '
const fs = require("fs");
const token = fs.readFileSync("/run/secrets/multica_verify_token", "utf8");
const url = `ws://backend:8080/ws?workspace_id=${process.env.MULTICA_TEST_WORKSPACE}`;
const ws = new WebSocket(url);
const timer = setTimeout(() => { console.error("websocket timeout"); process.exit(1); }, 5000);
ws.onopen = () => ws.send(JSON.stringify({type: "auth", payload: {token}}));
ws.onmessage = (event) => {
  if (String(event.data).includes("auth_ack")) {
    clearTimeout(timer);
    ws.close();
    process.exit(0);
  }
};
ws.onerror = () => { clearTimeout(timer); process.exit(1); };
' >/dev/null

active_tasks="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM agent_task_queue WHERE status IN ('queued','dispatched','running','waiting_local_directory','deferred');")"
[[ "$active_tasks" == "0" ]] || { printf 'ERROR: rehearsal came up with active or queued agent work\n' >&2; exit 1; }
[[ "$(printf '%s' "$MULTICA_WORKFLOW_DRIVER" | tr '[:upper:]' '[:lower:]')" == "false" ]] || { printf 'ERROR: rehearsal workflow dispatch must remain disabled\n' >&2; exit 1; }

restore_backup_hash="$(extract_json source_backup_sha256 "$RESTORE_RECEIPT")"
restore_backup_run="$(extract_json source_backup_run_id "$RESTORE_RECEIPT")"
verified_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
umask 077
cat > "$ACCEPTANCE_RECEIPT" <<EOF
{
  "schema": 1,
  "kind": "isolated_rehearsal_acceptance",
  "rehearsal_id": "$REHEARSAL_ID",
  "source_revision": "$release_revision",
  "source_backup_run_id": "$restore_backup_run",
  "source_backup_sha256": "$restore_backup_hash",
  "health_identity_verified": true,
  "database_health_verified": true,
  "authenticated_api_verified": true,
  "frontend_route_verified": true,
  "authenticated_web_verified": false,
  "owner_login_pending": true,
  "websocket_verified": true,
  "issue_history_verified": true,
  "attachment_bytes_verified": true,
  "active_or_queued_tasks": 0,
  "workflow_driver_enabled": false,
  "production_routing_changed": false,
  "verified_at": "$verified_at"
}
EOF
chmod 600 "$ACCEPTANCE_RECEIPT"
printf 'Isolated rehearsal application accepted: %s\n' "$REHEARSAL_ID"
printf 'Acceptance receipt: %s\n' "$ACCEPTANCE_RECEIPT"
