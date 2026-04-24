#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${CODEX_POOL_APP_DIR:-$HOME/Library/Application Support/codex-pool}"
BIN_DIR="$APP_DIR/bin"
CONFIG_DIR="$APP_DIR"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST="$PLIST_DIR/com.codex-pool.plist"

mkdir -p "$BIN_DIR" "$CONFIG_DIR/pool/codex" "$CONFIG_DIR/pool/openai_api" "$PLIST_DIR"
go build -trimpath -o "$BIN_DIR/codex-pool" .

if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  cp config.toml.example "$CONFIG_DIR/config.toml"
fi

sed \
  -e "s#__CODEX_POOL_BIN__#$BIN_DIR/codex-pool#g" \
  -e "s#__CODEX_POOL_WORKDIR__#$APP_DIR#g" \
  launchd/com.codex-pool.plist > "$PLIST"

launchctl bootout "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
launchctl enable "gui/$(id -u)/com.codex-pool"
printf 'codex-pool installed. Status: launchctl print gui/%s/com.codex-pool\n' "$(id -u)"
