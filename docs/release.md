# Release Process

The public release process is intentionally small.

1. Run tests and build locally.
2. Run the provider-removal and secret scans from `docs/security.md`.
3. Commit a coherent slice.
4. Push to `main`.
5. Tag a semantic version, for example `v0.11.0`.
6. Publish binaries and checksums from GitHub Actions or a trusted local machine.

GitHub recommends release automation with CI, tests, and semantically versioned tags. Homebrew tap guidance also calls out local testing, dependency declarations, semantic versioning, and clear documentation.

## Local Build Matrix

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o dist/codex-pool-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -trimpath -o dist/codex-pool-linux-arm64 .
GOOS=darwin GOARCH=amd64 go build -trimpath -o dist/codex-pool-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -trimpath -o dist/codex-pool-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -trimpath -o dist/codex-pool-windows-amd64.exe .
(cd dist && sha256sum * > SHA256SUMS)
```

## GitHub Release

```bash
git tag v$(cat VERSION)
git push origin main --tags
```
