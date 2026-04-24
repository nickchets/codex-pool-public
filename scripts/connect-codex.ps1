$ErrorActionPreference = "Stop"

$PoolUrl = if ($env:CODEX_POOL_URL) { $env:CODEX_POOL_URL.TrimEnd("/") } else { "http://127.0.0.1:8989" }
$CodexHome = if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME ".codex" }
$ConfigFile = Join-Path $CodexHome "config.toml"

New-Item -ItemType Directory -Force -Path $CodexHome | Out-Null
if (Test-Path $ConfigFile) {
  $stamp = Get-Date -Format "yyyyMMddHHmmss"
  Copy-Item $ConfigFile "$ConfigFile.bak.$stamp"
}

@"
model_provider = "codex-pool"
chatgpt_base_url = "$PoolUrl/backend-api"

[model_providers.codex-pool]
name = "OpenAI via codex-pool"
base_url = "$PoolUrl/v1"
wire_api = "responses"
requires_openai_auth = true
"@ | Set-Content -Path $ConfigFile -Encoding utf8

Write-Host "Codex CLI now points at $PoolUrl"
