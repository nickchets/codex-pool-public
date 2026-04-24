# Versioning

This repository uses semantic versioning once public releases are tagged.

- Patch: bug fixes and documentation corrections.
- Minor: compatible setup, routing, or operator additions.
- Major: config, API, account-file, or CLI setup changes that require user migration.

Before tagging a release, run:

```bash
go test ./...
go build -o /tmp/codex-pool .
bash -n scripts/*.sh
test -f .dockerignore && rg -n 'pool|config.toml|\.git|dist' .dockerignore
rg -n "Claude|Gemini|OpenCode|Anthropic|GitLab|Kimi|MiniMax|Antigravity" .
rg -n 'sk-[A-Za-z0-9_-]{20,}|"(access_token|refresh_token|id_token)"[[:space:]]*:[[:space:]]*"[^"]+"' \
  --glob '!docs/security.md' \
  --glob '!README.md' \
  --glob '!pool_test.go' .
```
