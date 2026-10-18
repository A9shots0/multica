#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROFILE_DIR/../../.." && pwd)"
REHEARSAL_ID="${1:-}"
[[ "$REHEARSAL_ID" =~ ^[a-z0-9]{6,24}$ ]] || { printf 'ERROR: rehearsal id must contain 6-24 lowercase letters or digits\n' >&2; exit 1; }
ENV_FILE="$HOME/.config/silver-mediums/multica/rehearsals/$REHEARSAL_ID.env"
STATE_DIR="$HOME/.silver-mediums/multica/rehearsals/$REHEARSAL_ID"
RESTORE_RECEIPT="$STATE_DIR/restore-receipt.json"
RELEASE_MANIFEST="$PROFILE_DIR/release-manifest.json"
[[ -f "$ENV_FILE" ]] || { printf 'ERROR: rehearsal environment is missing\n' >&2; exit 1; }
[[ "$(stat -f '%Lp' "$ENV_FILE")" == "600" ]] || { printf 'ERROR: rehearsal environment must have mode 600\n' >&2; exit 1; }
[[ -f "$RESTORE_RECEIPT" ]] || { printf 'ERROR: verified rehearsal restore receipt is missing\n' >&2; exit 1; }
[[ "$(stat -f '%Lp' "$RESTORE_RECEIPT")" == "600" ]] || { printf 'ERROR: rehearsal restore receipt must have mode 600\n' >&2; exit 1; }

extract_json() {
  plutil -extract "$1" raw -o - "$2" 2>/dev/null || true
}

release_revision="$(extract_json source_revision "$RELEASE_MANIFEST")"
[[ "$(extract_json accepted "$RESTORE_RECEIPT")" == "true" ]] || { printf 'ERROR: rehearsal restore is not accepted\n' >&2; exit 1; }
[[ "$(extract_json rehearsal_id "$RESTORE_RECEIPT")" == "$REHEARSAL_ID" ]] || { printf 'ERROR: rehearsal restore receipt identity mismatch\n' >&2; exit 1; }
[[ -n "$release_revision" && "$(extract_json source_revision "$RESTORE_RECEIPT")" == "$release_revision" ]] || { printf 'ERROR: rehearsal restore receipt revision mismatch\n' >&2; exit 1; }
[[ "$(extract_json database_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || { printf 'ERROR: rehearsal database restore is not verified\n' >&2; exit 1; }
[[ "$(extract_json uploads_restore_verified "$RESTORE_RECEIPT")" == "true" ]] || { printf 'ERROR: rehearsal uploads restore is not verified\n' >&2; exit 1; }
[[ "$(extract_json active_or_queued_tasks "$RESTORE_RECEIPT")" == "0" ]] || { printf 'ERROR: rehearsal restore contains active or queued work\n' >&2; exit 1; }
[[ "$(extract_json workflow_driver_enabled "$RESTORE_RECEIPT")" == "false" ]] || { printf 'ERROR: rehearsal restore did not preserve the dispatch fence\n' >&2; exit 1; }

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
[[ "$COMPOSE_PROJECT_NAME" == "multica32_rehearsal_${REHEARSAL_ID}" ]] || { printf 'ERROR: unexpected rehearsal project name\n' >&2; exit 1; }
[[ "$MULTICA_PGDATA_VOLUME" == "${COMPOSE_PROJECT_NAME}_pgdata" ]] || { printf 'ERROR: unexpected rehearsal database volume\n' >&2; exit 1; }
[[ "$MULTICA_UPLOADS_VOLUME" == "${COMPOSE_PROJECT_NAME}_uploads" ]] || { printf 'ERROR: unexpected rehearsal uploads volume\n' >&2; exit 1; }
[[ "$(printf '%s' "$MULTICA_WORKFLOW_DRIVER" | tr '[:upper:]' '[:lower:]')" == "false" ]] || { printf 'ERROR: rehearsal workflow dispatch must remain disabled\n' >&2; exit 1; }
[[ -n "$(docker volume ls --quiet --filter "name=^${MULTICA_PGDATA_VOLUME}$")" ]] || { printf 'ERROR: rehearsal database volume is missing\n' >&2; exit 1; }
[[ -n "$(docker volume ls --quiet --filter "name=^${MULTICA_UPLOADS_VOLUME}$")" ]] || { printf 'ERROR: rehearsal uploads volume is missing\n' >&2; exit 1; }

docker compose --project-name "$COMPOSE_PROJECT_NAME" --env-file "$ENV_FILE" \
  -f "$REPO_ROOT/docker-compose.selfhost.yml" -f "$PROFILE_DIR/compose.yml" up -d

for _ in $(seq 1 120); do
  if curl --fail --silent --max-time 3 "http://127.0.0.1:${BACKEND_PORT}/health" >/dev/null 2>&1 && \
     curl --fail --silent --max-time 3 "http://127.0.0.1:${FRONTEND_PORT}/login" >/dev/null 2>&1; then
    printf 'Rehearsal application is healthy on loopback ports %s and %s.\n' "$BACKEND_PORT" "$FRONTEND_PORT"
    exit 0
  fi
  sleep 1
done

docker compose --project-name "$COMPOSE_PROJECT_NAME" --env-file "$ENV_FILE" \
  -f "$REPO_ROOT/docker-compose.selfhost.yml" -f "$PROFILE_DIR/compose.yml" ps
printf 'ERROR: rehearsal application did not become healthy\n' >&2
exit 1
