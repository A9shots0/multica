#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

[[ "$(uname -s)" == "Darwin" ]] || fail "runtime bootstrap must run on macOS"
[[ "$(uname -m)" == "x86_64" ]] || fail "runtime bootstrap is pinned to the Intel 32GB Mac"
[[ -t 0 ]] || fail "run this script through an interactive SSH TTY so sudo can prompt securely"

package_root="$(cd "$REPO_ROOT/.." && pwd)"
[[ -f "$package_root/SHA256SUMS" ]] || fail "release package checksum manifest is missing"
(
  cd "$package_root"
  shasum -a 256 -c SHA256SUMS >/dev/null
) || fail "release package checksum verification failed"

if ! sudo -n true >/dev/null 2>&1; then
  printf 'macOS will now request the Mac login password. Input is hidden.\n'
  sudo -v
fi

macports_version="2.12.6"
macports_package="MacPorts-${macports_version}-15-Sequoia.pkg"
macports_sha256="2d6d58ff3ff60e70f8dc05cb2df9df3137e3d7c0c6d6f6236d68c8ba03343e34"
macports_url="https://github.com/macports/macports-base/releases/download/v${macports_version}/${macports_package}"

if [[ ! -x /opt/local/bin/port ]]; then
  cache_dir="$HOME/.silver-mediums/multica/cache"
  package_path="$cache_dir/$macports_package"
  mkdir -p "$cache_dir"
  chmod 700 "$cache_dir"
  if [[ ! -f "$package_path" ]]; then
    partial_path="$package_path.partial.$$"
    curl --fail --location --retry 3 --output "$partial_path" "$macports_url"
    chmod 600 "$partial_path"
    partial_sha256="$(shasum -a 256 "$partial_path" | awk '{print $1}')"
    [[ "$partial_sha256" == "$macports_sha256" ]] || fail "downloaded MacPorts package checksum mismatch"
    mv "$partial_path" "$package_path"
  fi
  actual_sha256="$(shasum -a 256 "$package_path" | awk '{print $1}')"
  [[ "$actual_sha256" == "$macports_sha256" ]] || fail "MacPorts package checksum mismatch"
  pkgutil --check-signature "$package_path" >/dev/null
  sudo /usr/sbin/installer -pkg "$package_path" -target /
fi

export PATH="/opt/local/bin:/opt/local/sbin:$PATH"
require_command port
sudo port -N selfupdate
sudo port -N install colima docker docker-compose-plugin

require_command colima
require_command docker
docker compose version >/dev/null

if ! colima status >/dev/null 2>&1; then
  colima start --cpu 4 --memory 8 --disk 64
fi

docker info >/dev/null
docker compose version

image_archive="${MULTICA_IMAGE_ARCHIVE:-$REPO_ROOT/../deployment-images.tar}"
[[ -f "$image_archive" ]] || fail "deployment image archive is missing: $image_archive"
docker image load --input "$image_archive" >/dev/null

printf 'Runtime bootstrap passed. Multica was not started and no production data was restored.\n'
