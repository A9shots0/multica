#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

load_host_env
compose ps

backend_url="http://127.0.0.1:${BACKEND_PORT:-8080}/health"
frontend_url="http://127.0.0.1:${FRONTEND_PORT:-3000}/login"

printf '\nBackend identity:\n'
curl --fail --silent --show-error --max-time 5 "$backend_url" || true
printf '\n\nFrontend: '
if curl --fail --silent --show-error --max-time 5 "$frontend_url" >/dev/null; then
  printf 'reachable\n'
else
  printf 'unreachable\n'
fi

printf '\nTailscale Serve:\n'
tailscale serve status || true
