# Best Practices Used In This Cut

This repo applies the public guidance below.

## Codex CLI

OpenAI documents Codex CLI as a local terminal coding agent that supports ChatGPT sign-in and API-key sign-in. The generated config keeps the standard Responses wire API and points Codex at this pool through `base_url` and `chatgpt_base_url`.

References:

- https://developers.openai.com/codex/cli
- https://developers.openai.com/codex/auth
- https://developers.openai.com/codex/agent-approvals-security
- https://developers.openai.com/codex/windows
- https://developers.openai.com/codex/noninteractive

## Secrets

Runtime credentials stay outside the repository. Docker's own docs warn against build-time secrets in build args or environment variables because they can persist in the final image. The Docker example therefore mounts `pool/` and `config.toml` at runtime.

The repo also ships `.dockerignore` so local credentials, local config, git history, and release artifacts are excluded from the Docker build context before `COPY . .` runs.

References:

- https://docs.docker.com/compose/how-tos/environment-variables/best-practices/
- https://docs.docker.com/build/building/secrets/
- https://docs.docker.com/compose/how-tos/use-secrets/

## Windows Scripts

PowerShell execution policies are Windows-only and can be controlled per process. The docs tell users to inspect saved scripts when policy blocks direct `iwr ... | iex` usage instead of telling them to weaken machine-wide policy.

Reference:

- https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_execution_policies

## Release Hygiene

GitHub recommends release automation with CI and tests. Homebrew tap guidance emphasizes local testing, dependency declarations, semantic versioning, and clear documentation. The repo includes a small CI workflow, release checklist, and build matrix.

References:

- https://docs.github.com/actions/how-tos/create-and-publish-actions/release-and-maintain-actions
- https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
