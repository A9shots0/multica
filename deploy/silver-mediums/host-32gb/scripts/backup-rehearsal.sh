#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REHEARSAL_ID="${1:-}"
[[ "$REHEARSAL_ID" =~ ^[a-z0-9]{6,24}$ ]] || { printf 'ERROR: rehearsal id must contain 6-24 lowercase letters or digits\n' >&2; exit 1; }

ENV_FILE="$HOME/.config/silver-mediums/multica/rehearsals/$REHEARSAL_ID.env"
STATE_DIR="$HOME/.silver-mediums/multica/rehearsals/$REHEARSAL_ID"
RESTORE_RECEIPT="$STATE_DIR/restore-receipt.json"
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
[[ -f "$ENV_FILE" && -f "$RESTORE_RECEIPT" ]] || { printf 'ERROR: rehearsal restore state is incomplete\n' >&2; exit 1; }

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
[[ "$COMPOSE_PROJECT_NAME" == "multica32_rehearsal_${REHEARSAL_ID}" ]] || { printf 'ERROR: unexpected rehearsal project name\n' >&2; exit 1; }

extract_json() {
  plutil -extract "$1" raw -o - "$2" 2>/dev/null || true
}

release_revision="$(extract_json source_revision "$RELEASE_MANIFEST")"
restore_revision="$(extract_json source_revision "$RESTORE_RECEIPT")"
[[ -n "$release_revision" && "$restore_revision" == "$release_revision" ]] || { printf 'ERROR: restore receipt source revision does not match the release\n' >&2; exit 1; }
[[ "$(extract_json database_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || { printf 'ERROR: rehearsal database restore is not verified\n' >&2; exit 1; }
[[ "$(extract_json uploads_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || { printf 'ERROR: rehearsal uploads restore is not verified\n' >&2; exit 1; }

run_id="$(date -u +%Y%m%dT%H%M%SZ)-$(openssl rand -hex 4)"
output="$STATE_DIR/backups/$run_id"
umask 077
mkdir -p "$output"
completed=0
cleanup() {
  if [[ "$completed" != "1" && -d "$output" ]]; then rm -rf "$output"; fi
}
trap cleanup EXIT

postgres_container="${COMPOSE_PROJECT_NAME}-postgres-1"
[[ "$(docker inspect -f '{{.State.Running}}' "$postgres_container" 2>/dev/null || true)" == "true" ]] || { printf 'ERROR: rehearsal PostgreSQL is not running\n' >&2; exit 1; }

docker exec "$postgres_container" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$output/database_dump.dump"
[[ -s "$output/database_dump.dump" ]] || { printf 'ERROR: 32GB database backup is empty\n' >&2; exit 1; }
docker exec "$postgres_container" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c \
  'select coalesce(max(version), current_database()) from schema_migrations; select count(*) from "user"; select count(*) from workspace; select count(*) from issue; select count(*) from comment; select count(*) from inbox_item;' \
  > "$output/database_verification.txt"
docker run --rm -v "$MULTICA_UPLOADS_VOLUME:/source:ro" -v "$output:/backup" "$MULTICA_POSTGRES_IMAGE" \
  sh -lc 'tar -czf /backup/uploads_archive.tar.gz -C /source .'
docker run --rm -v "$MULTICA_UPLOADS_VOLUME:/source:ro" "$MULTICA_POSTGRES_IMAGE" \
  sh -lc 'find /source -type f -print0 | sort -z | xargs -0 -r sha256sum | sed "s#  /source/#  #"' \
  > "$output/uploads_verification.sha256"

component_json=""
for spec in \
  'database_dump:database_dump.dump' \
  'database_verification:database_verification.txt' \
  'uploads_archive:uploads_archive.tar.gz' \
  'uploads_verification:uploads_verification.sha256'; do
  name="${spec%%:*}"
  file="${spec#*:}"
  bytes="$(stat -f '%z' "$output/$file")"
  hash="$(shasum -a 256 "$output/$file" | awk '{print $1}')"
  if [[ -n "$component_json" ]]; then component_json+=","; fi
  component_json+=$(printf '\n    "%s": {"path": "%s", "bytes": %s, "sha256": "%s"}' "$name" "$file" "$bytes" "$hash")
done

source_backup_hash="$(extract_json source_backup_sha256 "$RESTORE_RECEIPT")"
source_backup_run="$(extract_json source_backup_run_id "$RESTORE_RECEIPT")"
created_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat > "$output/transfer-manifest.json" <<EOF
{
  "schema": 1,
  "kind": "host32_rehearsal_backup",
  "created_at": "$created_at",
  "producer_host": "32gb-mac-mini",
  "producer_rehearsal_id": "$REHEARSAL_ID",
  "source_backup": {
    "run_id": "$source_backup_run",
    "sha256": "$source_backup_hash",
    "bytes": 0
  },
  "application_source_revision": "$release_revision",
  "components": {$component_json
  },
  "credentials_included": false,
  "production_environment_included": false
}
EOF

(
  cd "$output"
  for file in database_dump.dump database_verification.txt transfer-manifest.json uploads_archive.tar.gz uploads_verification.sha256; do
    shasum -a 256 "$file"
  done > SHA256SUMS
)
chmod 600 "$output"/*
completed=1
printf '32GB rehearsal backup verified: %s\n' "$run_id"
printf 'Data bundle: %s\n' "$output"
