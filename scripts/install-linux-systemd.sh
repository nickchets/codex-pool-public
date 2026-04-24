#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${CODEX_POOL_APP_DIR:-$HOME/.local/share/codex-pool}"
BIN_DIR="${CODEX_POOL_BIN_DIR:-$HOME/.local/bin}"
CONFIG_DIR="${CODEX_POOL_CONFIG_DIR:-$HOME/.config/codex-pool}"
SERVICE_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

mkdir -p "$APP_DIR" "$BIN_DIR" "$CONFIG_DIR" "$SERVICE_DIR"
go build -trimpath -o "$BIN_DIR/codex-pool" .

if [ ! -f "$CONFIG_DIR/config.toml" ]; then
  cp config.toml.example "$CONFIG_DIR/config.toml"
fi
mkdir -p "$APP_DIR/pool/codex" "$APP_DIR/pool/openai_api"

sed \
  -e "s#__CODEX_POOL_BIN__#$BIN_DIR/codex-pool#g" \
  -e "s#__CODEX_POOL_WORKDIR__#$APP_DIR#g" \
  -e "s#__CODEX_POOL_CONFIG__#$CONFIG_DIR/config.toml#g" \
  systemd/codex-pool.service > "$SERVICE_DIR/codex-pool.service"

systemctl --user daemon-reload
systemctl --user enable --now codex-pool.service
printf 'codex-pool installed. Status: systemctl --user status codex-pool.service --no-pager\n'
