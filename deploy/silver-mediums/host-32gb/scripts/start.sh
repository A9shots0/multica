#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

require_command colima
load_host_env
if ! colima status >/dev/null 2>&1; then
  colima start \
    --cpu "${COLIMA_CPU:-4}" \
    --memory "${COLIMA_MEMORY_GIB:-8}" \
    --disk "${COLIMA_DISK_GIB:-64}"
fi

bash "$SCRIPT_DIR/preflight.sh"
assert_data_ready

docker volume inspect "${MULTICA_PGDATA_VOLUME:-multica_pgdata}" >/dev/null 2>&1 || \
  fail "restored PostgreSQL volume is missing"
docker volume inspect "${MULTICA_UPLOADS_VOLUME:-multica_backend_uploads}" >/dev/null 2>&1 || \
  fail "restored uploads volume is missing"

compose up -d --no-build --pull never
wait_for_url "http://127.0.0.1:${BACKEND_PORT:-8080}/healthz"
wait_for_url "http://127.0.0.1:${FRONTEND_PORT:-3000}/login"

printf 'Multica is healthy on loopback. Private Tailscale routing is verified separately.\n'
