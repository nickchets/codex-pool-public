# Security And Redaction

This repository must stay free of live credentials and private workstation material.

Never commit:

- `pool/`
- `config.toml`
- `~/.codex/auth.json`
- OpenAI API keys
- OAuth access tokens
- OAuth refresh tokens
- real account IDs tied to people
- dashboard screenshots that reveal account state
- local service logs or runtime databases

## Runtime Defaults

- The service binds to `127.0.0.1:8989` by default.
- Operator endpoints accept loopback requests when no `admin_token` is configured.
- Remote operator access requires `admin_token`.
- Generated account files are written with `0600` permissions.
- Docker examples mount secrets at runtime instead of copying them into the image.
- `.dockerignore` keeps `pool/`, `config.toml`, local databases, logs, git history, and build artifacts out of the Docker build context.

Docker documents that build args and environment variables are inappropriate for build-time secrets because they persist in images. Keep OpenAI credentials in runtime-mounted files or a secret manager.

## Pre-Push Checks

```bash
go test ./...
go build -o /tmp/codex-pool .
bash -n scripts/*.sh
test -f .dockerignore && rg -n 'pool|config.toml|\.git|dist' .dockerignore
rg -n "Claude|Gemini|OpenCode|Anthropic|GitLab|Kimi|MiniMax|Antigravity" .
rg -n 'sk-[A-Za-z0-9_-]{20,}|"(access_token|refresh_token|id_token)"[[:space:]]*:[[:space:]]*"[^"]+"' \
  --glob '!README.md' \
  --glob '!docs/security.md' \
  --glob '!docs/release.md' \
  --glob '!pool_test.go' .
```

The provider-name scan should return no production code or user-facing docs. Mentions inside changelog migration notes are acceptable only when they explain removal.
