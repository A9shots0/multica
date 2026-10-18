#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROFILE_DIR/../../.." && pwd)"

fail() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || fail "rehearsal restore is only supported on macOS"
[[ "$(uname -m)" == "x86_64" ]] || fail "rehearsal restore is pinned to the Intel 32GB Mac"
command -v docker >/dev/null 2>&1 || fail "docker is missing"
command -v plutil >/dev/null 2>&1 || fail "plutil is missing"
command -v shasum >/dev/null 2>&1 || fail "shasum is missing"
docker info >/dev/null 2>&1 || fail "docker runtime is not healthy"

BUNDLE_DIR="${1:-}"
REHEARSAL_ID="${2:-$(date -u +%Y%m%d%H%M%S)}"
[[ -n "$BUNDLE_DIR" ]] || fail "usage: restore-rehearsal.sh BUNDLE_DIR [REHEARSAL_ID]"
BUNDLE_DIR="$(cd "$BUNDLE_DIR" && pwd)"
[[ "$REHEARSAL_ID" =~ ^[a-z0-9]{6,24}$ ]] || fail "rehearsal id must contain 6-24 lowercase letters or digits"

TRANSFER_MANIFEST="$BUNDLE_DIR/transfer-manifest.json"
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
[[ -f "$TRANSFER_MANIFEST" ]] || fail "transfer manifest is missing"

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
  actual_hash="$(shasum -a 256 "$file" | awk '{print $1}')"
  [[ "$actual_hash" == "$expected_hash" ]] || fail "component hash mismatch: $component"
  printf -v "${component}_file" '%s' "$file"
done

prefix="multica32_rehearsal_${REHEARSAL_ID}"
db_volume="${prefix}_pgdata"
uploads_volume="${prefix}_uploads"
db_container="${prefix}_restoredb"
env_dir="$HOME/.config/silver-mediums/multica/rehearsals"
state_root="$HOME/.silver-mediums/multica/rehearsals"
state_dir="$state_root/$REHEARSAL_ID"
lock_root="$state_root/.locks"
lock_dir="$lock_root/$REHEARSAL_ID.lock"
env_file="$env_dir/$REHEARSAL_ID.env"
receipt_file="$state_dir/restore-receipt.json"
postgres_image="$(extract_json images.postgres "$RELEASE_MANIFEST")"

umask 077
mkdir -p "$env_dir" "$lock_root"
if ! mkdir "$lock_dir" 2>/dev/null; then
  fail "another rehearsal restore is active or left a lock for $REHEARSAL_ID"
fi
printf 'pid=%s started_at=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$lock_dir/owner"

cleanup_failed=1
db_volume_created=0
uploads_volume_created=0
db_container_created=0
env_file_created=0
state_dir_created=0
db_password=""
jwt_secret=""
cleanup() {
  db_password=""
  jwt_secret=""
  if [[ "$cleanup_failed" == "1" ]]; then
    if [[ "$db_container_created" == "1" ]]; then docker rm -f "$db_container" >/dev/null 2>&1 || true; fi
    if [[ "$uploads_volume_created" == "1" ]]; then docker volume rm "$uploads_volume" >/dev/null 2>&1 || true; fi
    if [[ "$db_volume_created" == "1" ]]; then docker volume rm "$db_volume" >/dev/null 2>&1 || true; fi
    if [[ "$env_file_created" == "1" ]]; then rm -f "$env_file"; fi
    if [[ "$state_dir_created" == "1" ]]; then rm -rf "$state_dir"; fi
  fi
  rm -f "$lock_dir/owner"
  rmdir "$lock_dir" 2>/dev/null || true
}
trap cleanup EXIT

[[ ! -e "$env_file" ]] || fail "rehearsal environment already exists: $env_file"
[[ ! -e "$state_dir" ]] || fail "rehearsal state already exists: $state_dir"
for resource in "$db_volume" "$uploads_volume"; do
  [[ -z "$(docker volume ls --quiet --filter "name=^${resource}$")" ]] || fail "rehearsal volume already exists: $resource"
done
[[ -z "$(docker container ls -a --filter "name=^/${db_container}$" --format '{{.Names}}')" ]] || fail "rehearsal container already exists: $db_container"

mkdir -p "$env_dir" "$state_dir/recovery-state"
state_dir_created=1
db_password="$(openssl rand -hex 32)"
jwt_secret="$(openssl rand -hex 32)"
controller_id="$(uuidgen | tr '[:upper:]' '[:lower:]')"

docker volume create "$db_volume" >/dev/null
db_volume_created=1
docker volume create "$uploads_volume" >/dev/null
uploads_volume_created=1
docker run -d --name "$db_container" \
  -e POSTGRES_DB=multica \
  -e POSTGRES_USER=multica \
  -e "POSTGRES_PASSWORD=$db_password" \
  -v "$db_volume:/var/lib/postgresql/data" \
  "$postgres_image" >/dev/null
db_container_created=1

ready=0
for _ in $(seq 1 60); do
  if docker exec "$db_container" pg_isready -U multica -d multica >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == "1" ]] || fail "isolated rehearsal PostgreSQL did not become ready"

docker cp "$database_dump_file" "$db_container:/tmp/database.dump"
docker exec "$db_container" pg_restore -U multica -d multica --no-owner --no-privileges /tmp/database.dump
docker exec "$db_container" rm -f /tmp/database.dump
docker run --rm -v "$uploads_volume:/restore" -v "$uploads_archive_file:/bundle/uploads.tar.gz:ro" "$postgres_image" \
  sh -lc 'tar -xzf /bundle/uploads.tar.gz -C /restore'

expected_db="$(tr -d '\r' < "$database_verification_file" | sed -e '${/^$/d;}')"
actual_db="$(docker exec "$db_container" psql -U multica -d multica -At -c 'select coalesce(max(version), current_database()) from schema_migrations; select count(*) from "user"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;')"
[[ "$actual_db" == "$expected_db" ]] || fail "database migration identity or representative counts do not match"
active_tasks="$(docker exec "$db_container" psql -U multica -d multica -At -c "SELECT count(*) FROM agent_task_queue WHERE status IN ('queued','dispatched','running','waiting_local_directory','deferred');")"
[[ "$active_tasks" == "0" ]] || fail "source backup contains active or queued agent work"

expected_uploads="$(tr -d '\r' < "$uploads_verification_file" | sed -e '${/^$/d;}')"
actual_uploads="$(docker run --rm -v "$uploads_volume:/source:ro" "$postgres_image" sh -lc 'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"')"
[[ "$actual_uploads" == "$expected_uploads" ]] || fail "upload paths or hashes do not match"

env_file_created=1
cat > "$env_file" <<EOF
COMPOSE_PROJECT_NAME=$prefix
MULTICA_BACKEND_IMAGE=$(extract_json images.backend "$RELEASE_MANIFEST")
MULTICA_WEB_IMAGE=$(extract_json images.frontend "$RELEASE_MANIFEST")
MULTICA_POSTGRES_IMAGE=$postgres_image
POSTGRES_DB=multica
POSTGRES_USER=multica
POSTGRES_PASSWORD=$db_password
JWT_SECRET=$jwt_secret
BACKEND_PORT=18080
FRONTEND_PORT=13000
MULTICA_PGDATA_VOLUME=$db_volume
MULTICA_UPLOADS_VOLUME=$uploads_volume
MULTICA_RECOVERY_STATE_PATH=$state_dir/recovery-state
MULTICA_DATA_READY_RECEIPT=$receipt_file
FRONTEND_ORIGIN=http://127.0.0.1:13000
CORS_ALLOWED_ORIGINS=http://127.0.0.1:13000
MULTICA_APP_URL=http://127.0.0.1:13000
MULTICA_DAEMON_SERVER_URL=http://127.0.0.1:18080
LOCAL_UPLOAD_BASE_URL=http://127.0.0.1:18080
ALLOW_SIGNUP=false
DISABLE_WORKSPACE_CREATION=true
FF_PLUGINS_V1=false
MULTICA_OWNER_SMS_MODE=off
MULTICA_LLM_MAX_RETRIES=0
MULTICA_VCS_INTEGRATION_ENABLED=false
MULTICA_WORKFLOW_DRIVER=false
MULTICA_WORKFLOW_DRIVER_INTERVAL=60s
MULTICA_WORKFLOW_DRIVER_CAPACITY_BACKOFF=60s
MULTICA_WORKFLOW_CONTROLLER_ID=$controller_id
EOF
chmod 600 "$env_file"

docker rm -f "$db_container" >/dev/null
db_container_created=0
source_backup_hash="$(extract_json source_backup.sha256 "$TRANSFER_MANIFEST")"
source_backup_run="$(extract_json source_backup.run_id "$TRANSFER_MANIFEST")"
upload_count="$(printf '%s\n' "$actual_uploads" | sed '/^$/d' | wc -l | tr -d ' ')"
schema_migration_identity="$(printf '%s\n' "$actual_db" | sed -n '1p')"
[[ "$schema_migration_identity" =~ ^[A-Za-z0-9._-]+$ ]] || fail "unsafe schema migration identity"
verified_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
receipt_file_created=1
cat > "$receipt_file" <<EOF
{
  "schema": 1,
  "accepted": true,
  "kind": "isolated_rehearsal_restore",
  "rehearsal_id": "$REHEARSAL_ID",
  "source_revision": "$release_revision",
  "data_source_revision": "$transfer_revision",
  "release_source_revision": "$release_revision",
  "source_backup_run_id": "$source_backup_run",
  "source_backup_sha256": "$source_backup_hash",
  "database_restore_verified": true,
  "uploads_restore_verified": true,
  "schema_migration_identity": "$schema_migration_identity",
  "upload_file_count": $upload_count,
  "active_or_queued_tasks": 0,
  "workflow_driver_enabled": false,
  "verified_at": "$verified_at"
}
EOF
chmod 600 "$receipt_file"

cleanup_failed=0
printf 'Isolated rehearsal restore verified: %s\n' "$REHEARSAL_ID"
printf 'Environment: %s\n' "$env_file"
printf 'Receipt: %s\n' "$receipt_file"
