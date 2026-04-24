package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

func (s *server) statusPayload(r *http.Request) statusPayload {
	now := time.Now().UTC()
	rows := s.pool.summaries(now)
	oauth := 0
	apiKeys := 0
	eligible := 0
	for _, row := range rows {
		if row.Kind == accountKindCodexOAuth {
			oauth++
		}
		if row.Kind == accountKindOpenAIAPI {
			apiKeys++
		}
		if row.Eligible {
			eligible++
		}
	}
	base := publicBaseURL(s.cfg, r)
	return statusPayload{
		Version:         version,
		GeneratedAt:     now,
		UptimeSeconds:   int64(time.Since(s.startTime).Seconds()),
		ListenAddr:      s.cfg.ListenAddr,
		PoolDir:         s.cfg.PoolDir,
		AccountCount:    len(rows),
		OAuthCount:      oauth,
		APIKeyCount:     apiKeys,
		EligibleCount:   eligible,
		LocalOperator:   s.cfg.AdminToken == "",
		SharedProxyAuth: s.cfg.SharedProxyToken != "",
		Accounts:        rows,
		Setup: setupLinks{
			CodexConfig: base + "/config/codex.toml",
			ShellScript: base + "/setup/codex.sh",
			PowerShell:  base + "/setup/codex.ps1",
		},
	}
}

func (s *server) serveStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json") {
		respondJSON(w, s.statusPayload(r))
		return
	}
	s.serveHome(w, r)
}

func (s *server) serveHome(w http.ResponseWriter, r *http.Request) {
	payload := s.statusPayload(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homeTemplate.Execute(w, payload)
}

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>codex-pool</title>
  <style>
    :root { color-scheme: light dark; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f7f8fb; color: #15171a; }
    main { max-width: 1120px; margin: 0 auto; padding: 32px 20px 48px; }
    h1 { font-size: 32px; margin: 0 0 6px; letter-spacing: 0; }
    h2 { font-size: 18px; margin: 28px 0 12px; }
    p { margin: 0 0 14px; color: #4b5563; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin: 20px 0; }
    .metric, table, pre { background: #fff; border: 1px solid #dde2ea; border-radius: 8px; }
    .metric { padding: 14px 16px; }
    .metric b { display: block; font-size: 24px; margin-top: 4px; }
    table { width: 100%; border-collapse: collapse; overflow: hidden; }
    th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #edf0f5; font-size: 14px; }
    th { color: #526070; font-weight: 650; background: #fbfcfe; }
    tr:last-child td { border-bottom: 0; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    pre { padding: 14px; overflow: auto; }
    .ok { color: #12773d; }
    .bad { color: #a12b2b; }
    @media (prefers-color-scheme: dark) {
      body { background: #111418; color: #f5f7fa; }
      p { color: #aeb7c2; }
      .metric, table, pre { background: #171b21; border-color: #29313a; }
      th, td { border-bottom-color: #29313a; }
      th { color: #b7c1ce; background: #1b2027; }
    }
  </style>
</head>
<body>
<main>
  <h1>codex-pool</h1>
  <p>Codex CLI proxy for ChatGPT sign-in accounts and OpenAI API keys.</p>
  <div class="grid">
    <div class="metric">Accounts<b>{{.AccountCount}}</b></div>
    <div class="metric">Eligible<b>{{.EligibleCount}}</b></div>
    <div class="metric">ChatGPT OAuth<b>{{.OAuthCount}}</b></div>
    <div class="metric">OpenAI API keys<b>{{.APIKeyCount}}</b></div>
  </div>
  <h2>Connect Codex CLI</h2>
  <pre><code>curl -fsSL {{.Setup.ShellScript}} | bash</code></pre>
  <pre><code>iwr {{.Setup.PowerShell}} -UseBasicParsing | iex</code></pre>
  <h2>Accounts</h2>
  <table>
    <thead><tr><th>ID</th><th>Kind</th><th>Plan</th><th>Status</th><th>Inflight</th><th>Last used</th></tr></thead>
    <tbody>
    {{range .Accounts}}
      <tr>
        <td><code>{{.ID}}</code></td>
        <td>{{.Kind}}</td>
        <td>{{.PlanType}}</td>
        <td>{{if .Eligible}}<span class="ok">eligible</span>{{else}}<span class="bad">{{.BlockReason}}</span>{{end}}</td>
        <td>{{.Inflight}}</td>
        <td>{{.LastUsed}}</td>
      </tr>
    {{else}}
      <tr><td colspan="6">No accounts loaded. Add <code>pool/codex/*.json</code> or <code>pool/openai_api/*.json</code>.</td></tr>
    {{end}}
    </tbody>
  </table>
</main>
</body>
</html>`))

func (s *server) serveCodexConfig(w http.ResponseWriter, r *http.Request) {
	base := publicBaseURL(s.cfg, r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, `model_provider = "codex-pool"
chatgpt_base_url = "%s/backend-api"

[model_providers.codex-pool]
name = "OpenAI via codex-pool"
base_url = "%s/v1"
wire_api = "responses"
requires_openai_auth = true
`, base, base)
}

func (s *server) serveShellSetup(w http.ResponseWriter, r *http.Request) {
	base := publicBaseURL(s.cfg, r)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	fmt.Fprintf(w, `#!/usr/bin/env bash
set -euo pipefail

POOL_URL="${CODEX_POOL_URL:-%s}"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
CONFIG_FILE="$CODEX_HOME/config.toml"
mkdir -p "$CODEX_HOME"
if [ -f "$CONFIG_FILE" ]; then
  cp "$CONFIG_FILE" "$CONFIG_FILE.bak.$(date +%%Y%%m%%d%%H%%M%%S)"
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
printf 'Codex CLI now points at %%s\n' "$POOL_URL"
`, base)
}

func (s *server) servePowerShellSetup(w http.ResponseWriter, r *http.Request) {
	base := publicBaseURL(s.cfg, r)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, `$PoolUrl = if ($env:CODEX_POOL_URL) { $env:CODEX_POOL_URL.TrimEnd('/') } else { "%s" }
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
`, base)
}
