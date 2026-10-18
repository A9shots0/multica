#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || fail "production restore is only supported on macOS"
[[ "$(uname -m)" == "x86_64" ]] || fail "production restore is pinned to the Intel 32GB Mac"
for command_name in docker plutil shasum openssl uuidgen; do
  command -v "$command_name" >/dev/null 2>&1 || fail "required command is missing: $command_name"
done
docker info >/dev/null 2>&1 || fail "docker runtime is not healthy"

BUNDLE_DIR="${1:-}"
WORKFLOW_CONTROLLER_ID="${2:-}"
[[ -n "$BUNDLE_DIR" && -n "$WORKFLOW_CONTROLLER_ID" ]] || \
  fail "usage: restore-production.sh BUNDLE_DIR WORKFLOW_CONTROLLER_ID"
BUNDLE_DIR="$(cd "$BUNDLE_DIR" && pwd)"
[[ "$WORKFLOW_CONTROLLER_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]] || \
  fail "workflow controller id is not a canonical UUID"
[[ "$(printf '%s' "$WORKFLOW_CONTROLLER_ID" | tr '[:upper:]' '[:lower:]')" != "00000000-0000-0000-0000-000000000000" ]] || \
  fail "workflow controller id must not be nil"

TRANSFER_MANIFEST="$BUNDLE_DIR/transfer-manifest.json"
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
ENV_DIR="$HOME/.config/silver-mediums/multica"
ENV_FILE="$ENV_DIR/host.env"
STATE_ROOT="$HOME/.silver-mediums/multica/recovery"
STATE_DIR="$STATE_ROOT/production-restore"
RECEIPT_DIR="$STATE_ROOT/receipts"
RECEIPT_FILE="$RECEIPT_DIR/production-data-ready.json"
RECOVERY_STATE_PATH="$STATE_ROOT/state"
DB_VOLUME="multica_pgdata"
UPLOADS_VOLUME="multica_backend_uploads"
DB_CONTAINER="multica32_production_restoredb"
LOCK_DIR="$STATE_ROOT/production-restore.lock"

umask 077
mkdir -p "$STATE_ROOT"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  fail "another production restore is active or left a lock at $LOCK_DIR"
fi
printf 'pid=%s started_at=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$LOCK_DIR/owner"

cleanup_failed=1
db_volume_created=0
uploads_volume_created=0
db_container_created=0
env_file_created=0
receipt_file_created=0
db_password=""
jwt_secret=""
cleanup() {
  db_password=""
  jwt_secret=""
  if [[ "$cleanup_failed" == "1" ]]; then
    if [[ "$db_container_created" == "1" ]]; then docker rm -f "$DB_CONTAINER" >/dev/null 2>&1 || true; fi
    if [[ "$uploads_volume_created" == "1" ]]; then docker volume rm "$UPLOADS_VOLUME" >/dev/null 2>&1 || true; fi
    if [[ "$db_volume_created" == "1" ]]; then docker volume rm "$DB_VOLUME" >/dev/null 2>&1 || true; fi
    if [[ "$env_file_created" == "1" ]]; then rm -f "$ENV_FILE"; fi
    if [[ "$receipt_file_created" == "1" ]]; then rm -f "$RECEIPT_FILE"; fi
  fi
  rm -f "$LOCK_DIR/owner"
  rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

[[ -f "$TRANSFER_MANIFEST" ]] || fail "transfer manifest is missing"
[[ ! -e "$ENV_FILE" ]] || fail "production host environment already exists"
[[ ! -e "$RECEIPT_FILE" ]] || fail "production data-ready receipt already exists"
for resource in "$DB_VOLUME" "$UPLOADS_VOLUME"; do
  [[ -z "$(docker volume ls --quiet --filter "name=^${resource}$")" ]] || fail "production volume already exists: $resource"
done
[[ -z "$(docker container ls -a --filter "name=^/${DB_CONTAINER}$" --format '{{.Names}}')" ]] || fail "restore container already exists"
[[ -z "$(docker container ls -a --filter 'label=com.docker.compose.project=multica' --format '{{.Names}}')" ]] || fail "a production Multica project already exists on this host"

extract_json() {
  plutil -extract "$1" raw -o - "$2" 2>/dev/null || true
}

transfer_revision="$(extract_json application_source_revision "$TRANSFER_MANIFEST")"
release_revision="$(extract_json source_revision "$RELEASE_MANIFEST")"
upgrade_from_revision="$(extract_json observed_runtime.backend_health_commit "$RELEASE_MANIFEST")"
[[ -n "$release_revision" ]] || fail "release source revision is missing"
if [[ "$transfer_revision" != "$release_revision" && "$transfer_revision" != "$upgrade_from_revision" ]]; then
  fail "data bundle source revision matches neither the release nor its recorded upgrade source"
fi

for component in database_dump database_verification uploads_archive uploads_verification; do
  path="$(extract_json "components.${component}.path" "$TRANSFER_MANIFEST")"
  bytes="$(extract_json "components.${component}.bytes" "$TRANSFER_MANIFEST")"
  expected_hash="$(extract_json "components.${component}.sha256" "$TRANSFER_MANIFEST")"
  [[ "$path" =~ ^[A-Za-z0-9._-]+$ ]] || fail "unsafe component path: $component"
  file="$BUNDLE_DIR/$path"
  [[ -f "$file" ]] || fail "component is missing: $component"
  [[ "$(stat -f '%z' "$file")" == "$bytes" ]] || fail "component size mismatch: $component"
  [[ "$(shasum -a 256 "$file" | awk '{print $1}')" == "$expected_hash" ]] || fail "component hash mismatch: $component"
  printf -v "${component}_file" '%s' "$file"
done

mkdir -p "$ENV_DIR" "$STATE_DIR" "$RECEIPT_DIR" "$RECOVERY_STATE_PATH"
db_password="$(openssl rand -hex 32)"
jwt_secret="$(openssl rand -hex 32)"
postgres_image="$(extract_json images.postgres "$RELEASE_MANIFEST")"

docker volume create "$DB_VOLUME" >/dev/null
db_volume_created=1
docker volume create "$UPLOADS_VOLUME" >/dev/null
uploads_volume_created=1
docker run -d --name "$DB_CONTAINER" \
  -e POSTGRES_DB=multica \
  -e POSTGRES_USER=multica \
  -e "POSTGRES_PASSWORD=$db_password" \
  -v "$DB_VOLUME:/var/lib/postgresql/data" \
  "$postgres_image" >/dev/null
db_container_created=1

ready=0
for _ in $(seq 1 60); do
  if docker exec "$DB_CONTAINER" pg_isready -U multica -d multica >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == "1" ]] || fail "production restore PostgreSQL did not become ready"

docker cp "$database_dump_file" "$DB_CONTAINER:/tmp/database.dump"
docker exec "$DB_CONTAINER" pg_restore -U multica -d multica --no-owner --no-privileges /tmp/database.dump
docker exec "$DB_CONTAINER" rm -f /tmp/database.dump
docker run --rm -v "$UPLOADS_VOLUME:/restore" -v "$uploads_archive_file:/bundle/uploads.tar.gz:ro" "$postgres_image" \
  sh -lc 'tar -xzf /bundle/uploads.tar.gz -C /restore'

expected_db="$(tr -d '\r' < "$database_verification_file" | sed -e '${/^$/d;}')"
actual_db="$(docker exec "$DB_CONTAINER" psql -U multica -d multica -At -c 'select coalesce(max(version), current_database()) from schema_migrations; select count(*) from "user"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;')"
[[ "$actual_db" == "$expected_db" ]] || fail "database migration identity or representative counts do not match"
active_tasks="$(docker exec "$DB_CONTAINER" psql -U multica -d multica -At -c "SELECT count(*) FROM agent_task_queue WHERE status IN ('queued','dispatched','running','waiting_local_directory','deferred');")"
[[ "$active_tasks" == "0" ]] || fail "source backup contains active or queued agent work"

expected_uploads="$(tr -d '\r' < "$uploads_verification_file" | sed -e '${/^$/d;}')"
actual_uploads="$(docker run --rm -v "$UPLOADS_VOLUME:/source:ro" "$postgres_image" sh -lc 'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"')"
[[ "$actual_uploads" == "$expected_uploads" ]] || fail "upload paths or hashes do not match"

llm_base_url="${MULTICA_MIGRATION_LLM_BASE_URL:-}"
llm_default_model="${MULTICA_MIGRATION_LLM_DEFAULT_MODEL:-}"
env_file_created=1
cat > "$ENV_FILE" <<EOF
COMPOSE_PROJECT_NAME=multica
MULTICA_BACKEND_IMAGE=$(extract_json images.backend "$RELEASE_MANIFEST")
MULTICA_WEB_IMAGE=$(extract_json images.frontend "$RELEASE_MANIFEST")
MULTICA_POSTGRES_IMAGE=$postgres_image
POSTGRES_DB=multica
POSTGRES_USER=multica
POSTGRES_PASSWORD=$db_password
JWT_SECRET=$jwt_secret
BACKEND_PORT=8080
FRONTEND_PORT=3000
MULTICA_PGDATA_VOLUME=$DB_VOLUME
MULTICA_UPLOADS_VOLUME=$UPLOADS_VOLUME
MULTICA_RECOVERY_STATE_PATH=$RECOVERY_STATE_PATH
MULTICA_DATA_READY_RECEIPT=$RECEIPT_FILE
FRONTEND_ORIGIN=https://32s-mac-mini.tailda9924.ts.net
CORS_ALLOWED_ORIGINS=https://32s-mac-mini.tailda9924.ts.net
MULTICA_APP_URL=https://32s-mac-mini.tailda9924.ts.net
MULTICA_DAEMON_SERVER_URL=https://32s-mac-mini.tailda9924.ts.net
LOCAL_UPLOAD_BASE_URL=https://32s-mac-mini.tailda9924.ts.net
ALLOW_SIGNUP=false
DISABLE_WORKSPACE_CREATION=true
FF_PLUGINS_V1=false
MULTICA_OWNER_SMS_MODE=off
MULTICA_LLM_MAX_RETRIES=0
MULTICA_LLM_BASE_URL=$llm_base_url
MULTICA_LLM_DEFAULT_MODEL=$llm_default_model
MULTICA_VCS_INTEGRATION_ENABLED=false
MULTICA_WORKFLOW_DRIVER=false
MULTICA_WORKFLOW_DRIVER_INTERVAL=60s
MULTICA_WORKFLOW_DRIVER_CAPACITY_BACKOFF=60s
MULTICA_WORKFLOW_CONTROLLER_ID=$WORKFLOW_CONTROLLER_ID
COLIMA_CPU=4
COLIMA_MEMORY_GIB=8
COLIMA_DISK_GIB=64
EOF
chmod 600 "$ENV_FILE"

docker rm -f "$DB_CONTAINER" >/dev/null
db_container_created=0
source_backup_hash="$(extract_json source_backup.sha256 "$TRANSFER_MANIFEST")"
source_backup_run="$(extract_json source_backup.run_id "$TRANSFER_MANIFEST")"
upload_count="$(printf '%s\n' "$actual_uploads" | sed '/^$/d' | wc -l | tr -d ' ')"
verified_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
receipt_file_created=1
cat > "$RECEIPT_FILE" <<EOF
{
  "schema": 1,
  "accepted": true,
  "kind": "production_restore",
  "source_revision": "$release_revision",
  "data_source_revision": "$transfer_revision",
  "release_source_revision": "$release_revision",
  "source_backup_run_id": "$source_backup_run",
  "source_backup_sha256": "$source_backup_hash",
  "database_restore_verified": true,
  "uploads_restore_verified": true,
  "upload_file_count": $upload_count,
  "active_or_queued_tasks": 0,
  "workflow_driver_enabled": false,
  "verified_at": "$verified_at"
}
EOF
chmod 600 "$RECEIPT_FILE"

cleanup_failed=0
printf 'Production data restored and verified, but not started.\n'
printf 'Workflow dispatch remains disabled until worker reconnection.\n'
