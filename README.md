ghsummon
=======

[![Test Status](https://github.com/Songmu/ghsummon/actions/workflows/test.yaml/badge.svg?branch=main)][actions]
[![Coverage Status](https://codecov.io/gh/Songmu/ghsummon/branch/main/graph/badge.svg)][codecov]
[![MIT License](https://img.shields.io/github/license/Songmu/ghsummon)][license]
[![PkgGoDev](https://pkg.go.dev/badge/github.com/Songmu/ghsummon)][PkgGoDev]

[actions]: https://github.com/Songmu/ghsummon/actions?workflow=test
[codecov]: https://codecov.io/gh/Songmu/ghsummon
[license]: https://github.com/Songmu/ghsummon/blob/main/LICENSE
[PkgGoDev]: https://pkg.go.dev/github.com/Songmu/ghsummon

**ghsummon** detects `@copilot <prompt>` directives in files (primarily Markdown), then automatically creates a Pull Request and summons [GitHub Copilot Coding Agent](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent) to work on it — all driven by a simple push to your default branch.

## How It Works

1. Write `@copilot <prompt>` in a Markdown file and push to the default branch
2. GitHub Actions runs ghsummon
3. ghsummon detects the `@copilot` directive in the git diff
4. A new branch and PR are created automatically
5. Copilot Coding Agent is assigned and triggered via `@copilot` comment
6. Copilot replaces the `@copilot` prompt with research results directly on the PR branch
7. Review and merge the PR — the branch is deleted, allowing re-use

## Usage

### Writing `@copilot` Prompts

Add an `@copilot` directive at the beginning of a line in a Markdown file:

```markdown
@copilot Investigate the latest best practices for Go error handling
```

Multi-line prompts are supported using indentation (tab or 2+ spaces):

```markdown
@copilot Research GitHub Copilot Coding Agent trigger methods
  Compare REST API, GraphQL, and GitHub CLI approaches
  Include best practices and recommendations
```

### GitHub Actions Setup

```yaml
name: ghsummon
on:
  push:
    branches: [main]
    paths: ['**.md']

permissions:
  contents: write
  pull-requests: write
  issues: write

jobs:
  research:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/create-github-app-token@v2
        id: app-token
        with:
          app-id: ${{ vars.APP_ID }}
          private-key: ${{ secrets.APP_PRIVATE_KEY }}
      - uses: Songmu/ghsummon@v0
        with:
          token: ${{ steps.app-token.outputs.token }}
```

### Prerequisites

- **Copilot license** (Pro, Pro+, Business, or Enterprise) — works with personal accounts, no Org required
- **Coding Agent enabled** (Settings > Copilot for personal accounts, or Org policy)
- **`.github/workflows/copilot-setup-steps.yml`** must exist in the repository ([docs](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-environment))
- **GitHub Actions** enabled

### Token Requirements

| Method | Required Permissions |
|--------|---------------------|
| **GitHub App Token** (recommended) | `issues: write`, `pull_requests: write`, `contents: write` |
| **PAT (fine-grained)** | `issues:write`, `pull_requests:write`, `contents:write` |
| **`GITHUB_TOKEN`** | ⚠️ May not trigger Copilot due to recursive workflow prevention |

> **Recommendation**: Use a [GitHub App token](https://github.com/marketplace/actions/create-github-app-token) via `actions/create-github-app-token` for reliable Copilot triggering.

### Minimal `copilot-setup-steps.yml`

```yaml
name: "Copilot Setup Steps"
on:
  workflow_dispatch:
  push:
    paths:
      - .github/workflows/copilot-setup-steps.yml
  pull_request:
    paths:
      - .github/workflows/copilot-setup-steps.yml

jobs:
  copilot-setup-steps:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v5
```

### Action Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `token` | No | `github.token` | GitHub token used for API calls and git operations |
| `version` | No | `v0.0.0` | Version of ghsummon to install |

### Action Outputs

| Output | Description |
|--------|-------------|
| `pr_count` | Number of PRs created |
| `pr_numbers` | Comma-separated list of created PR numbers |

## Exclusion Control

ghsummon uses branch-name-based exclusion. For each file with an `@copilot` directive, a branch named `ghsummon-<filepath>` is created. If the branch already exists, the file is skipped (work already in progress). Once the PR is merged and the branch is deleted, the same file can be processed again.

## Installation

```console
# Install the latest version. (Install it into ./bin/ by default).
% curl -sfL https://raw.githubusercontent.com/Songmu/ghsummon/main/install.sh | sh -s

# Specify installation directory ($(go env GOPATH)/bin/) and version.
% curl -sfL https://raw.githubusercontent.com/Songmu/ghsummon/main/install.sh | sh -s -- -b $(go env GOPATH)/bin [vX.Y.Z]

# In alpine linux (as it does not come with curl by default)
% wget -O - -q https://raw.githubusercontent.com/Songmu/ghsummon/main/install.sh | sh -s [vX.Y.Z]

# go install
% go install github.com/Songmu/ghsummon/cmd/ghsummon@latest
```

## Author

[Songmu](https://github.com/Songmu)
