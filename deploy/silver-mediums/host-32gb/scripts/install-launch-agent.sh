#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "$SCRIPT_DIR/common.sh"

require_command plutil
load_host_env

agent_dir="$HOME/Library/LaunchAgents"
log_dir="$HOME/Library/Logs/SilverMediums/Multica"
agent_path="$agent_dir/com.silvermediums.multica.plist"
template="$PROFILE_DIR/launchd/com.silvermediums.multica.plist.template"

mkdir -p "$agent_dir" "$log_dir"
sed \
  -e "s|__START_SCRIPT__|$SCRIPT_DIR/start.sh|g" \
  -e "s|__ENV_FILE__|$ENV_FILE|g" \
  -e "s|__LOG_DIR__|$log_dir|g" \
  "$template" >"$agent_path"
chmod 600 "$agent_path"
plutil -lint "$agent_path"

launchctl bootout "gui/$UID/com.silvermediums.multica" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$UID" "$agent_path"
launchctl enable "gui/$UID/com.silvermediums.multica"

printf 'Installed user LaunchAgent at %s. Reboot recovery still requires a logged-in user session and must be tested.\n' "$agent_path"
