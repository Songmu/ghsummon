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
      - uses: Songmu/ghsummon@v0
        with:
          token: ${{ secrets.GHSUMMON_TOKEN }}
```

### Prerequisites

- **Copilot license** (Pro, Pro+, Business, or Enterprise) — works with personal accounts, no Org required
- **Coding Agent enabled** (Settings > Copilot for personal accounts, or Org policy)
- **`.github/workflows/copilot-setup-steps.yml`** must exist in the repository ([docs](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-environment))
- **GitHub Actions** enabled

### Token Requirements

A **fine-grained PAT** is required for Copilot Coding Agent assignment. GitHub App installation tokens cannot assign Copilot to PRs ([cli/cli#11362](https://github.com/cli/cli/issues/11362)).

Create a fine-grained PAT with these repository permissions:
- `Contents`: Read and Write
- `Pull requests`: Read and Write

Store it as `secrets.GHSUMMON_TOKEN` in your repository settings.

> ⚠️ `GITHUB_TOKEN` cannot trigger Copilot due to recursive workflow prevention. GitHub App installation tokens cannot assign the Copilot agent as an assignee.

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

## Similar solutions and inspirations

Several tools and ideas share ghsummon's core theme of using natural-language directives or comments to automatically trigger AI agents that create GitHub Pull Requests.

### GitHub Agentic Workflows

The closest conceptual sibling is **GitHub Agentic Workflows** (technical preview, early 2026), a GitHub Next / Microsoft Research project. Like ghsummon, it lets you write automation intent in plain Markdown files rather than complex YAML. A YAML frontmatter section declares triggers, permissions, and "safe outputs", while the Markdown body is a natural-language prompt that a coding agent (Copilot, Claude, or Codex) interprets and executes—creating issues, comments, or PRs on your behalf.

- [GitHub Blog: Automate repository tasks with GitHub Agentic Workflows](https://github.blog/ai-and-ml/automate-repository-tasks-with-github-agentic-workflows/)
- [GitHub Agentic Workflows docs](https://github.github.com/gh-aw/)
- [Awesome Copilot – Agentic Workflows examples](https://awesome-copilot.github.com/learning-hub/agentic-workflows/)

### AI agents triggered by GitHub issues / comments

These tools are triggered by assigning a GitHub issue to the agent or mentioning it in a comment, rather than by a push with a Markdown directive, but they solve a similar problem—automating the issue → PR lifecycle:

| Tool | Trigger | Notes |
|------|---------|-------|
| **[Mentat](https://mentat.ai/docs)** | `@mentatbot` comment in issue or PR | GitHub App; iterates on review feedback |
| **[SWE-agent](https://swe-agent.com/)** (Princeton NLP, open source) | CLI / API given a GitHub issue URL | Turns any LLM (GPT-4, Claude, etc.) into an autonomous coder; ~12% resolution on SWE-bench |
| **[AutoCodeRover](https://github.com/AutoCodeRoverSG/auto-code-rover)** (open source) | CLI / API given a GitHub issue URL | Uses AST + fault localization for smart patching; ~16% on SWE-bench, ~$0.43/task |
| **[Devin](https://cognition.ai/blog/devin-101-automatic-pr-reviews-with-the-devin-api)** (Cognition Labs) | GitHub Action / API per PR | Commercial; focuses on PR review and implementation at scale |

### GitHub Action: Generate PR with AI

**[Generate PR with AI](https://github.com/marketplace/actions/generate-pr-with-ai)** is a GitHub Marketplace Action that converts GitHub issues into working PRs by invoking LLM-based coding agents (Aider, Codex CLI, Claude Code, Gemini CLI). Multiple agents can run in parallel and the best result is picked—complementary to ghsummon's single-agent, push-triggered flow.


## Author

[Songmu](https://github.com/Songmu)
