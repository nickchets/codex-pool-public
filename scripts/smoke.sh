#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${CODEX_POOL_URL:-http://127.0.0.1:8989}"
BASE_URL="${BASE_URL%/}"

curl -fsS "$BASE_URL/healthz" >/dev/null
curl -fsS "$BASE_URL/status?format=json" >/dev/null
curl -fsS "$BASE_URL/config/codex.toml" | grep -q 'model_provider = "codex-pool"'
printf 'codex-pool smoke passed for %s\n' "$BASE_URL"
