#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

# Bare Compose must fail closed instead of using the checkout directory as the
# project-name fallback. Production is selected only by the release script.
for compose_file in docker-compose.yml docker-compose.selfhost.yml; do
  grep -Eq '^name: \$\{COMPOSE_PROJECT_NAME(:\?|:-)' "$ROOT_DIR/$compose_file" ||
    fail "$compose_file must define a non-basename Compose project identity"
done

# Host PostgreSQL belongs only to the local development override. The base
# file is also the file a stray `docker compose up` discovers automatically.
if grep -Fq '127.0.0.1:5432:5432' "$ROOT_DIR/docker-compose.yml"; then
  fail "docker-compose.yml must not publish the host PostgreSQL port"
fi
grep -Fq '127.0.0.1:${POSTGRES_PORT:-5432}:5432' "$ROOT_DIR/docker-compose.local.yml" ||
  fail "docker-compose.local.yml must own the local PostgreSQL port"
grep -Fq 'docker-compose.local.yml' "$ROOT_DIR/scripts/ensure-postgres.sh" ||
  fail "local database startup must opt into the port override"

# Production retains the deliberate identity and adds a checkout-path gate;
# this is the static regression proof for the incident's release boundary.
grep -Fq -- "--project-name', 'multica'" "$ROOT_DIR/scripts/adoption-release.ps1" ||
  fail "adoption release must keep the explicit production project name"
grep -Fq "[IO.Path]::GetFullPath('C:/Dev/multica')" "$ROOT_DIR/scripts/adoption-release.ps1" ||
  fail "adoption release must guard the canonical owner checkout"
grep -Fq "C:/Dev/SMK-multica-selfhost-pilot" "$ROOT_DIR/scripts/adoption-release.ps1" ||
  fail "adoption release must preserve the operational environment/state root"
grep -Fq 'Production release refused' "$ROOT_DIR/scripts/adoption-release.ps1" ||
  fail "adoption release must reject non-owner checkouts"

echo "compose isolation checks passed"
