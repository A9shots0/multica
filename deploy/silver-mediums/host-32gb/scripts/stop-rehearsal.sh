#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$PROFILE_DIR/../../.." && pwd)"
REHEARSAL_ID="${1:-}"
[[ "$REHEARSAL_ID" =~ ^[a-z0-9]{6,24}$ ]] || { printf 'ERROR: rehearsal id must contain 6-24 lowercase letters or digits\n' >&2; exit 1; }
ENV_FILE="$HOME/.config/silver-mediums/multica/rehearsals/$REHEARSAL_ID.env"
[[ -f "$ENV_FILE" ]] || { printf 'ERROR: rehearsal environment is missing\n' >&2; exit 1; }
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
[[ "$COMPOSE_PROJECT_NAME" == "multica32_rehearsal_${REHEARSAL_ID}" ]] || { printf 'ERROR: unexpected rehearsal project name\n' >&2; exit 1; }
docker compose --project-name "$COMPOSE_PROJECT_NAME" --env-file "$ENV_FILE" \
  -f "$REPO_ROOT/docker-compose.selfhost.yml" -f "$PROFILE_DIR/compose.yml" down
printf 'Rehearsal containers stopped; isolated volumes were preserved.\n'
