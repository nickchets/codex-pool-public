#!/usr/bin/env bash
set -euo pipefail

POOL_URL="${CODEX_POOL_URL:-http://127.0.0.1:8989}"
POOL_URL="${POOL_URL%/}"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
CONFIG_FILE="$CODEX_HOME/config.toml"

mkdir -p "$CODEX_HOME"
if [ -f "$CONFIG_FILE" ]; then
  cp "$CONFIG_FILE" "$CONFIG_FILE.bak.$(date +%Y%m%d%H%M%S)"
fi

cat > "$CONFIG_FILE" <<EOF
model_provider = "codex-pool"
chatgpt_base_url = "$POOL_URL/backend-api"

[model_providers.codex-pool]
name = "OpenAI via codex-pool"
base_url = "$POOL_URL/v1"
wire_api = "responses"
requires_openai_auth = true
EOF

chmod 600 "$CONFIG_FILE"
printf 'Codex CLI now points at %s\n' "$POOL_URL"
