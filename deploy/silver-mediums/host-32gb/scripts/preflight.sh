#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

[[ "$(uname -s)" == "Darwin" ]] || fail "this host profile is only for macOS"
[[ "$(uname -m)" == "x86_64" ]] || fail "this release is pinned for the Intel 32GB Mac"

for command_name in colima docker curl stat lsof tailscale plutil; do
  require_command "$command_name"
done

load_host_env

runtime="$(docker info --format '{{.OSType}}/{{.Architecture}}' 2>/dev/null || true)"
case "$runtime" in
  linux/amd64|linux/x86_64) ;;
  *) fail "Docker must be a running Linux amd64 Colima runtime; found '${runtime:-unavailable}'" ;;
esac

docker compose version >/dev/null

[[ -d "$MULTICA_RECOVERY_STATE_PATH" ]] || fail "recovery state directory is missing"

for brain_file in \
  /Users/a32/.config/silver-mediums/multica/brain.json \
  /Users/a32/.config/silver-mediums/multica/brain.token; do
  [[ -f "$brain_file" ]] || fail "protected Brain configuration is missing: $brain_file"
  [[ "$(stat -f '%Lp' "$brain_file")" == "600" ]] || fail "protected Brain configuration must have mode 600: $brain_file"
done

for image in "$MULTICA_BACKEND_IMAGE" "$MULTICA_WEB_IMAGE" "$MULTICA_POSTGRES_IMAGE"; do
  docker image inspect "$image" >/dev/null 2>&1 || fail "pinned image is not loaded: $image"
done

if [[ -z "$(compose ps --status running -q 2>/dev/null)" ]]; then
  backend_port="${BACKEND_PORT:-8080}"
  frontend_port="${FRONTEND_PORT:-3000}"
  lsof -nP -iTCP:"$backend_port" -sTCP:LISTEN >/dev/null 2>&1 && fail "backend port $backend_port is already in use"
  lsof -nP -iTCP:"$frontend_port" -sTCP:LISTEN >/dev/null 2>&1 && fail "frontend port $frontend_port is already in use"
fi

compose config --quiet
release_revision="$(plutil -extract source_revision raw -o - "$PROFILE_DIR/release-manifest.json" 2>/dev/null || true)"
[[ -n "$release_revision" ]] || fail "release manifest source revision is missing"
printf 'Preflight passed for source %s.\n' "$release_revision"
