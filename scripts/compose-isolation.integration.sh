#!/usr/bin/env bash
set -euo pipefail

# Docker-enabled incident-shape check. It intentionally uses a temporary
# checkout whose basename is `multica`, the collision shape that caused the
# incident. The corrected compose contract must refuse before it can inspect or
# mutate a stack. The production-like fixture is snapshotted before and after
# so a regression that reaches Docker is visible immediately. A disposable
# Docker context is mandatory; this test refuses to target the active context.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/multica-compose-isolation.XXXXXX")"
SECOND_ROOT="$TEMP_ROOT/multica"
cleanup() {
  rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

command -v docker >/dev/null 2>&1 || { echo "docker CLI is required" >&2; exit 77; }
TEST_CONTEXT="${COMPOSE_ISOLATION_DOCKER_CONTEXT:-}"
if [ -z "$TEST_CONTEXT" ] || [ "$TEST_CONTEXT" = "default" ]; then
  echo "COMPOSE_ISOLATION_DOCKER_CONTEXT must name a disposable Docker context" >&2
  exit 77
fi
docker --context "$TEST_CONTEXT" info >/dev/null 2>&1 || {
  echo "Disposable Docker context is unavailable: $TEST_CONTEXT" >&2
  exit 77
}
export DOCKER_CONTEXT="$TEST_CONTEXT"

production_ids=()
while IFS= read -r id; do
  [ -n "$id" ] && production_ids+=("$id")
done < <(docker ps -aq --filter label=com.docker.compose.project=multica | sort)
if [ "${#production_ids[@]}" -eq 0 ]; then
  echo "production fixture is required: no containers found for Compose project multica" >&2
  exit 1
fi

snapshot() {
  local id
  for id in "${production_ids[@]}"; do
  docker --context "$TEST_CONTEXT" inspect "$id" --format '{{.Id}}|{{.State.Status}}|{{index .Config.Labels "com.docker.compose.project"}}|{{index .Config.Labels "com.docker.compose.project.config_files"}}|{{json .NetworkSettings.Ports}}'
  done | sort
}

before="$(snapshot)"
mkdir -p "$SECOND_ROOT"
cp "$ROOT_DIR/docker-compose.yml" "$SECOND_ROOT/docker-compose.yml"

set +e
output="$(
  cd "$SECOND_ROOT"
  unset COMPOSE_PROJECT_NAME
  docker compose up -d 2>&1
)"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "bare compose unexpectedly started from a non-production checkout:" >&2
  echo "$output" >&2
  exit 1
fi
grep -Fq 'COMPOSE_PROJECT_NAME must be set' <<<"$output" || {
  echo "bare compose failed for an unexpected reason:" >&2
  echo "$output" >&2
  exit 1
}

after="$(snapshot)"
if [ "$before" != "$after" ]; then
  echo "production Compose fixture changed after the second-checkout attempt" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$after") >&2 || true
  exit 1
fi

echo "compose isolation integration check passed: second checkout refused and production fixture was unchanged"
