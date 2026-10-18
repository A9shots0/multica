#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROFILE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

command -v plutil >/dev/null 2>&1 || { printf 'ERROR: plutil is required\n' >&2; exit 1; }

agent_dir="$HOME/Library/LaunchAgents"
log_dir="$HOME/Library/Logs/SilverMediums/Multica"
agent_path="$agent_dir/com.silvermediums.multica.backup.plist"
template="$PROFILE_DIR/launchd/com.silvermediums.multica.backup.plist.template"

mkdir -p "$agent_dir" "$log_dir"
sed \
  -e "s|__BACKUP_SCRIPT__|$SCRIPT_DIR/backup-production.sh|g" \
  -e "s|__LOG_DIR__|$log_dir|g" \
  "$template" > "$agent_path"
chmod 600 "$agent_path"
plutil -lint "$agent_path"

launchctl bootout "gui/$UID/com.silvermediums.multica.backup" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$UID" "$agent_path"
launchctl enable "gui/$UID/com.silvermediums.multica.backup"

printf 'Installed nightly Multica backup LaunchAgent at %s.\n' "$agent_path"
