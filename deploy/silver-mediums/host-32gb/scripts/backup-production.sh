#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

for command_name in docker plutil shasum openssl stat scp ssh; do
  require_command "$command_name"
done
load_host_env

[[ "$COMPOSE_PROJECT_NAME" == "multica" ]] || fail "unexpected production project name"
[[ "$(stat -f '%Lp' "$ENV_FILE")" == "600" ]] || fail "production environment must have mode 600"

RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
RESTORE_RECEIPT="$HOME/.silver-mediums/multica/recovery/receipts/production-data-ready.json"
ACCEPTANCE_ROOT="$HOME/.silver-mediums/multica/recovery/production-acceptance"
CUTOVER_ACCEPTANCE_RECEIPT="$ACCEPTANCE_ROOT/acceptance-receipt.json"
LIVE_ACCEPTANCE_RECEIPT="$ACCEPTANCE_ROOT/live-verification-receipt.json"
BACKUP_ROOT="$HOME/.silver-mediums/multica/recovery/production-backups"
LOCK_DIR="$BACKUP_ROOT/.backup.lock"
OFFBOX_HOST="${MULTICA_BACKUP_OFFBOX_HOST:-drew@drews-mac-mini}"
OFFBOX_ROOT="${MULTICA_BACKUP_OFFBOX_ROOT:-/Users/drew/multica-offbox-backups}"
RCLONE_BIN="${MULTICA_BACKUP_RCLONE_BIN:-$HOME/.local/bin/rclone}"
RCLONE_TARGET="${MULTICA_BACKUP_RCLONE_TARGET:-r2:multica-control-plane-backups}"

extract_json() {
  plutil -extract "$1" raw -o - "$2" 2>/dev/null || true
}

[[ -f "$RESTORE_RECEIPT" ]] || fail "production restore receipt is missing"
release_revision="$(extract_json source_revision "$RELEASE_MANIFEST")"
[[ -n "$release_revision" ]] || fail "release source revision is missing"
restore_revision="$(extract_json source_revision "$RESTORE_RECEIPT")"
[[ -n "$restore_revision" ]] || fail "restore receipt source revision is missing"
[[ "$(extract_json accepted "$RESTORE_RECEIPT")" == "true" ]] || fail "production restore receipt is not accepted"
[[ "$(extract_json database_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || fail "production database restore is not verified"
[[ "$(extract_json uploads_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || fail "production uploads restore is not verified"

ACCEPTANCE_RECEIPT=""
for candidate in "$LIVE_ACCEPTANCE_RECEIPT" "$CUTOVER_ACCEPTANCE_RECEIPT"; do
  if [[ -f "$candidate" && "$(extract_json source_revision "$candidate")" == "$release_revision" ]]; then
    ACCEPTANCE_RECEIPT="$candidate"
    break
  fi
done
[[ -n "$ACCEPTANCE_RECEIPT" ]] || fail "no production acceptance receipt matches the running release"

umask 077
mkdir -p "$BACKUP_ROOT"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  fail "another production backup is active or left a lock at $LOCK_DIR"
fi
printf 'pid=%s started_at=%s\n' "$$" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$LOCK_DIR/owner"

run_id="$(date -u +%Y%m%dT%H%M%SZ)-$(openssl rand -hex 4)"
output="$BACKUP_ROOT/$run_id"
partial="$BACKUP_ROOT/.${run_id}.partial"
completed=0
cleanup() {
  if [[ "$completed" != "1" && -d "$partial" ]]; then rm -rf "$partial"; fi
  rm -f "$LOCK_DIR/owner"
  rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT
mkdir "$partial"

postgres_container="${COMPOSE_PROJECT_NAME}-postgres-1"
backend_container="${COMPOSE_PROJECT_NAME}-backend-1"
[[ "$(docker inspect -f '{{.State.Running}}' "$postgres_container" 2>/dev/null || true)" == "true" ]] || fail "production PostgreSQL is not running"
[[ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$backend_container" 2>/dev/null || true)" == "healthy" ]] || fail "production backend is not healthy"

active_tasks="$(docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM agent_task_queue WHERE status IN ('queued','dispatched','running','waiting_local_directory','deferred');")"

docker exec "$postgres_container" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$partial/database_dump.dump"
[[ -s "$partial/database_dump.dump" ]] || fail "production database backup is empty"
docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c \
  'select coalesce(max(version), current_database()) from schema_migrations; select count(*) from "user"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;' \
  > "$partial/database_verification.txt"
docker run --rm -v "$MULTICA_UPLOADS_VOLUME:/source:ro" -v "$partial:/backup" "$MULTICA_POSTGRES_IMAGE" \
  sh -lc 'tar -czf /backup/uploads_archive.tar.gz -C /source .'
docker run --rm -v "$MULTICA_UPLOADS_VOLUME:/source:ro" "$MULTICA_POSTGRES_IMAGE" \
  sh -lc 'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"' \
  > "$partial/uploads_verification.sha256"

component_json=""
for spec in \
  'database_dump:database_dump.dump' \
  'database_verification:database_verification.txt' \
  'uploads_archive:uploads_archive.tar.gz' \
  'uploads_verification:uploads_verification.sha256'; do
  name="${spec%%:*}"
  file="${spec#*:}"
  bytes="$(stat -f '%z' "$partial/$file")"
  hash="$(shasum -a 256 "$partial/$file" | awk '{print $1}')"
  if [[ -n "$component_json" ]]; then component_json+=","; fi
  component_json+=$(printf '\n    "%s": {"path": "%s", "bytes": %s, "sha256": "%s"}' "$name" "$file" "$bytes" "$hash")
done

source_backup_hash="$(extract_json source_backup_sha256 "$RESTORE_RECEIPT")"
source_backup_run="$(extract_json source_backup_run_id "$RESTORE_RECEIPT")"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat > "$partial/transfer-manifest.json" <<EOF
{
  "schema": 1,
  "kind": "host32_production_backup",
  "created_at": "$created_at",
  "producer_host": "32gb-mac-mini",
  "source_backup": {
    "run_id": "$source_backup_run",
    "sha256": "$source_backup_hash",
    "bytes": 0
  },
  "application_source_revision": "$release_revision",
  "active_or_queued_tasks_at_backup": $active_tasks,
  "components": {$component_json
  },
  "credentials_included": false,
  "production_environment_included": false
}
EOF

(
  cd "$partial"
  for file in database_dump.dump database_verification.txt transfer-manifest.json uploads_archive.tar.gz uploads_verification.sha256; do
    shasum -a 256 "$file"
  done > SHA256SUMS
  shasum -a 256 -c SHA256SUMS >/dev/null
)
chmod 600 "$partial"/*
mv "$partial" "$output"

offbox_status="failed"
if ssh -o BatchMode=yes -o ConnectTimeout=15 "$OFFBOX_HOST" "mkdir -p '$OFFBOX_ROOT/$run_id'" 2>/dev/null \
  && scp -q -o BatchMode=yes -o ConnectTimeout=15 "$output"/* "$OFFBOX_HOST:$OFFBOX_ROOT/$run_id/" 2>/dev/null; then
  offbox_status="ok"
fi

offsite_status="skipped"
if [[ -x "$RCLONE_BIN" && -f "$HOME/.config/rclone/rclone.conf" ]] \
  && "$RCLONE_BIN" listremotes 2>/dev/null | grep -q '^r2:$'; then
  offsite_status="failed"
  if "$RCLONE_BIN" copy "$output" "$RCLONE_TARGET/$run_id/" 2>/dev/null; then
    offsite_status="ok"
  fi
fi

cat > "$BACKUP_ROOT/latest.json" <<EOF
{
  "schema": 1,
  "run_id": "$run_id",
  "application_source_revision": "$release_revision",
  "active_or_queued_tasks_at_backup": $active_tasks,
  "local_status": "ok",
  "offbox_status": "$offbox_status",
  "offsite_status": "$offsite_status",
  "created_at": "$created_at"
}
EOF
chmod 600 "$BACKUP_ROOT/latest.json"

completed=1
printf '32GB production backup verified: %s\n' "$run_id"
printf 'Local bundle: %s\n' "$output"
printf 'Off-box copy: %s\n' "$offbox_status"
printf 'Private R2 copy: %s\n' "$offsite_status"
[[ "$offbox_status" == "ok" ]] || fail "off-box backup copy failed"
[[ "$offsite_status" == "ok" ]] || fail "private R2 backup copy failed"
