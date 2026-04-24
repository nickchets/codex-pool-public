# codex-pool

Codex-only account pool for the Codex CLI.

`codex-pool` is a small local reverse proxy that rotates Codex traffic across:

- ChatGPT sign-in accounts from Codex CLI `auth.json`
- OpenAI API keys for usage-based automation

It intentionally does not include non-OpenAI providers, GitLab sidecars, shared friend pools, or IDE-specific compatibility layers. The public surface is only Codex CLI plus OpenAI account credentials.

## Quick Start

```bash
git clone https://github.com/nickchets/codex-pool-public.git
cd codex-pool-public
go build -o codex-pool .
mkdir -p pool/codex
cp ~/.codex/auth.json pool/codex/main.json
./codex-pool
```

In another terminal:

```bash
curl -fsSL http://127.0.0.1:8989/setup/codex.sh | bash
codex "Reply with exactly OK."
```

Windows PowerShell:

```powershell
iwr http://127.0.0.1:8989/setup/codex.ps1 -UseBasicParsing | iex
codex "Reply with exactly OK."
```

## Add Accounts

Import an existing Codex CLI login:

```bash
mkdir -p pool/codex
cp ~/.codex/auth.json pool/codex/work.json
```

Add an OpenAI API key:

```bash
mkdir -p pool/openai_api
cat > pool/openai_api/ci.json <<'JSON'
{"OPENAI_API_KEY":"sk-..."}
JSON
chmod 600 pool/openai_api/ci.json
```

Or use the local operator API:

```bash
curl -fsS -X POST http://127.0.0.1:8989/operator/codex/api-key-add \
  -H 'Content-Type: application/json' \
  --data '{"api_key":"sk-..."}'
```

Browser OAuth onboarding is available on loopback deployments:

```bash
curl -fsS -X POST http://127.0.0.1:8989/operator/codex/oauth-start | jq -r .oauth_url
```

Open the returned URL in the same desktop session. The OAuth callback listens on `127.0.0.1:1455`, matching the Codex CLI OAuth redirect.

## Codex CLI Configuration

The generated config writes:

```toml
model_provider = "codex-pool"
chatgpt_base_url = "http://127.0.0.1:8989/backend-api"

[model_providers.codex-pool]
name = "OpenAI via codex-pool"
base_url = "http://127.0.0.1:8989/v1"
wire_api = "responses"
requires_openai_auth = true
```

## Runtime Endpoints

- `GET /` dashboard
- `GET /status?format=json` machine-readable status
- `GET /healthz` liveness
- `GET /config/codex.toml` generated Codex CLI config
- `GET /setup/codex.sh` shell setup script
- `GET /setup/codex.ps1` PowerShell setup script
- `POST /operator/codex/oauth-start` local OAuth add flow
- `POST /operator/codex/api-key-add` local API key add flow

## Security Defaults

- Bind to `127.0.0.1:8989` by default.
- Store account files under `pool/`, ignored by git.
- Write generated account files with mode `0600`.
- Require loopback access for operator endpoints unless `CODEX_POOL_ADMIN_TOKEN` is set.
- Optional proxy request guard: set `shared_proxy_token` in `config.toml` or `CODEX_POOL_SHARED_PROXY_TOKEN`, then send `X-Codex-Pool-Token`.

Do not publish `pool/`, `config.toml`, `~/.codex/auth.json`, API keys, refresh tokens, account IDs tied to real users, or dashboard screenshots that reveal accounts.

## More Docs

- [Install](docs/install.md)
- [Codex CLI connection](docs/connect-codex.md)
- [Security and redaction](docs/security.md)
- [Best practices used](docs/best-practices.md)
- [Release process](docs/release.md)
