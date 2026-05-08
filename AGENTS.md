<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (c) 2026 NVIDIA Corporation -->

# AGENTS.md - Guide for AI Coding Agents

You are contributing to **Karta**, an Apache 2.0 open-source Kubernetes project. Stack: Go 1.26, controller-runtime, Helm. Goal: a CRD that lets controllers and platforms inspect, modify, and manage workloads of any type without per-CRD adapters.

If you are an AI coding agent (Cursor, Codex, Claude Code, Aider, Continue, Copilot, etc.) opening this repo for the first time, read this file end to end before making changes. For Claude Code, the sibling `CLAUDE.md` imports this file via `@AGENTS.md`.

## Boundaries

**Always do:**
- Run `make check` before pushing.
- Sign off commits with `-s` (DCO).
- Add SPDX + copyright header to new source and Markdown files.
- Use Conventional Commits: `feat(scope):`, `fix(scope):`, `refactor(scope):`. Match an existing scope from `git log` when possible.
- Reference an open GitHub issue in the PR.
- Regenerate manifests after changing API types: `make generate && make manifests`.

**Discuss in the PR description:**
- Adding a new third-party Go module (license, alternatives considered).
- Breaking changes to public CRD fields or Helm values.
- Renaming or moving Go packages.
- Changes to `.github/workflows/`.

**Never do:**
- Hand-edit `charts/karta/crds/` — these are generated. Edit `pkg/api/optimization/v1alpha1/` and run `make manifests` instead.
- Disable a `golangci-lint` rule to silence a warning. Fix the underlying issue.
- Include URLs to private or login-gated resources (e.g., company Confluence, Jira, or wikis) in commits, code, or docs.
- Reference unannounced or non-public product features.
- Reference customer or partner names without their explicit permission.
- Push speculative commits to debug CI.

## Repo Layout

```
karta/
  cmd/                  Binaries (CLI entry points)
  pkg/                  Go library source
    api/                CRD types (optimization.nvidia.com/v1alpha1)
    resource/           Component extraction and update logic
    instructions/       Gang scheduling and pod manipulation
    jq/                 JQ path engine for spec/status traversal
  charts/karta/         Helm chart (CRDs auto-generated)
  docs/
    Technical Guide.md  CRD anatomy, JQ paths, spec definitions
    examples/           Pre-built Karta definitions
    ri-studio/          React + WASM authoring tool
  hack/                 Build and codegen scripts
  test/                 Test fixtures and integration tests
  .github/              CODEOWNERS, issue templates, PR template, CI workflows
```

If a directory has its own `AGENTS.md`, prefer that for subtree-specific rules.

## Build, Test, Lint

| Command | What it does |
|---------|--------------|
| `make check` | Umbrella: validates generated artifacts, runs tests, runs lint. Run before every PR. |
| `make test` | Run Go tests with mock regeneration. |
| `make lint` | `go fmt`, `go vet`, `golangci-lint`. |
| `make manifests` | Regenerate CRD YAML into `charts/karta/crds/`. |
| `make generate` | Regenerate DeepCopy methods. |
| `make generate-licenses` | Refresh `NOTICE` and `THIRD_PARTY_LICENSES`. |
| `make validate` | Run all generators and fail if the working tree drifts. |

Tools install into `./bin/` on first use. Versions are pinned in the Makefile.

## Code Style

- **Go formatting**: `go fmt` and `goimports` clean. `make fmt-go` runs locally.
- **Linter**: `golangci-lint` with `.golangci.yml`. Do not disable rules; fix the issue or discuss in the PR.
- **Naming and structure**: match the existing package layout and controller-runtime conventions.
- **Error handling**: wrap with `%w`. Do not log and return the same error.
- **Comments**: write self-documenting code. Add a comment only when the *why* is non-obvious.
- **Markdown**: short sentences, no markdown bold for emphasis, no emojis, no em-dash (U+2014), ASCII only.

## Commits and Pull Requests

Conventional Commits v1.0.0:

```
<type>(<scope>): <short description>

[optional body]
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`.

Sign off every commit with `-s` (DCO v1.1, real name required):

```bash
git commit -s -m "feat(cli): add tree alignment"
```

Use the PR template at `.github/PULL_REQUEST_TEMPLATE.md`. Required: link to an open issue, run `make check`, add or update tests (or explain why none).

## SPDX Headers

Every new source file and Markdown file:

```
// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation
```

Use the matching comment syntax for Markdown (`<!-- ... -->`), shell, and YAML.

## Third-Party Dependencies

Allowed licenses: MIT, BSD-2/3, Apache 2.0, ISC. Disallowed: GPL, LGPL, AGPL, any copyleft. After adding a module:

1. `make generate-licenses` to refresh `NOTICE` and `THIRD_PARTY_LICENSES`.
2. Name the dependency and version in the commit body.

`go-licenses` runs in `make validate` and CI. License drift fails the build.

## Tests

- Unit tests live next to the code they test (`*_test.go`).
- Mocks are generated via `go generate ./pkg/...` and refreshed by `make generate-mocks`.
- Integration tests and fixtures live under `test/`.

## Where to Ask

- Project owners: `OWNERS`, `MAINTAINERS.md`.
- Code-area owners: `.github/CODEOWNERS`.
- Security disclosures: `SECURITY.md`.
- Conduct concerns: `CODE_OF_CONDUCT.md`.

For feature discussion or bug reports, open a GitHub issue using the templates under `.github/ISSUE_TEMPLATE/`.

## Subtree AGENTS.md Files

When a subtree (for example `pkg/jq/`, `docs/ri-studio/`, `charts/karta/`) has non-obvious local conventions, add a nested `AGENTS.md`. Keep each under 200 lines.

To keep Claude Code agents on the same content, place a sibling `CLAUDE.md` containing only the import line:

```bash
printf '@AGENTS.md\n' > CLAUDE.md
```

Do not use a symlink, and do not put unique content in `CLAUDE.md`.
