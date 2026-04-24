# Connect Codex CLI

The pool exposes an OpenAI-compatible `/v1` Responses endpoint and a ChatGPT backend path for Codex account metadata.

Generated config:

```toml
model_provider = "codex-pool"
chatgpt_base_url = "http://127.0.0.1:8989/backend-api"

[model_providers.codex-pool]
name = "OpenAI via codex-pool"
base_url = "http://127.0.0.1:8989/v1"
wire_api = "responses"
requires_openai_auth = true
```

## Linux and macOS

```bash
curl -fsSL http://127.0.0.1:8989/setup/codex.sh | bash
```

For a remote pool:

```bash
CODEX_POOL_URL=https://pool.example.com curl -fsSL https://pool.example.com/setup/codex.sh | bash
```

## Windows PowerShell

```powershell
iwr http://127.0.0.1:8989/setup/codex.ps1 -UseBasicParsing | iex
```

For a remote pool:

```powershell
$env:CODEX_POOL_URL = "https://pool.example.com"
iwr "$env:CODEX_POOL_URL/setup/codex.ps1" -UseBasicParsing | iex
```

PowerShell execution policy is enforced on Windows. If your environment blocks downloaded scripts, save the script first, inspect it, and run it in a process with an execution policy allowed by your organization.

## Shared Proxy Token

If `shared_proxy_token` is configured, Codex CLI requests must include `X-Codex-Pool-Token`. Codex CLI config does not currently provide a generic static-header field for model providers, so this option is for reverse proxies or wrapper environments that can inject the header.

## Account Selection

- ChatGPT OAuth accounts can serve Codex backend and Responses traffic.
- OpenAI API keys can serve `/v1` and `/responses` traffic only.
- Conversation-like request identifiers are pinned to the same account when present.
- Disabled, dead, and temporarily rate-limited accounts are skipped.
