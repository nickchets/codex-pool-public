# Install

`codex-pool` is a single Go binary. It should usually run on the same machine as Codex CLI and bind to loopback.

## Requirements

- Go 1.24 or newer
- Codex CLI installed separately
- At least one ChatGPT Codex login or one OpenAI API key

OpenAI documents Codex CLI installation through `npm i -g @openai/codex` and notes that Codex CLI can authenticate with either ChatGPT sign-in or an API key. The pool supports those same two OpenAI credential types.

## Linux

```bash
git clone https://github.com/nickchets/codex-pool-public.git
cd codex-pool-public
bash scripts/install-linux-systemd.sh
systemctl --user status codex-pool.service --no-pager
curl -fsS http://127.0.0.1:8989/healthz
```

The installer builds `~/.local/bin/codex-pool`, creates `~/.config/codex-pool/config.toml` if missing, installs a user systemd unit, and starts it.

## macOS

```bash
git clone https://github.com/nickchets/codex-pool-public.git
cd codex-pool-public
bash scripts/install-macos-launchd.sh
launchctl print gui/$(id -u)/com.codex-pool
curl -fsS http://127.0.0.1:8989/healthz
```

The installer builds `~/Library/Application Support/codex-pool/bin/codex-pool`, writes a LaunchAgent plist, and starts it with `launchctl bootstrap`.

## Windows

Run from PowerShell in the repository:

```powershell
.\scripts\install-windows-task.ps1
Invoke-RestMethod http://127.0.0.1:8989/healthz
```

The installer builds `%LOCALAPPDATA%\codex-pool\codex-pool.exe`, writes `config.toml`, and registers a per-user scheduled task.

## Docker

Docker is useful for isolated testing. Do not bake secrets into images. Mount `pool/` and `config.toml` at runtime.

```bash
cp config.toml.example config.toml
mkdir -p pool/codex pool/openai_api
docker compose up --build
```

## Add Credentials

ChatGPT Codex login:

```bash
mkdir -p pool/codex
cp ~/.codex/auth.json pool/codex/main.json
chmod 600 pool/codex/main.json
```

OpenAI API key:

```bash
mkdir -p pool/openai_api
printf '{"OPENAI_API_KEY":"%s"}\n' "$OPENAI_API_KEY" > pool/openai_api/main.json
chmod 600 pool/openai_api/main.json
```

## Verify

```bash
go test ./...
go build -o /tmp/codex-pool .
curl -fsS http://127.0.0.1:8989/status?format=json
codex "Reply with exactly OK."
```
