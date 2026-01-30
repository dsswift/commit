# commit

A git commit tool that connects to the LLM of your choice to add semantic intelligence to commit generation and diff analysis.

## What It Does

- **Smart commit splitting** — Groups changes into logical commits by type (feat, fix, docs, etc.)
- **Monorepo support** — Respects scopes defined in `.commit.json`
- **Commit cleanup** — Use `--reverse` to explode a commit and re-organize
- **Multi-provider** — Connect to Anthropic, OpenAI, Grok, Gemini, or Azure AI Foundry
- **Diff analysis** — Use `--diff` to get LLM explanations of file changes

## Installation

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/dsswift/commit/main/scripts/install.sh | sh
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/dsswift/commit/main/scripts/install.ps1 | iex
```

### From Source

```bash
go install github.com/dsswift/commit/cmd/commit@latest
```

### Verify Installation

```bash
commit --version
```

## Quick Start

1. **Configure your LLM provider:**

   **macOS / Linux:**
   ```bash
   nano ~/.commit-tool/.env
   ```

   **Windows (PowerShell):**
   ```powershell
   notepad $env:USERPROFILE\.commit-tool\.env
   ```

   Set `COMMIT_PROVIDER` and add your API key:
   ```bash
   COMMIT_PROVIDER=anthropic
   ANTHROPIC_API_KEY=sk-ant-...
   ```

2. **Make changes to your code, then run:**

   ```bash
   commit
   ```

The tool sends your changes to the configured LLM and creates semantic commits based on the analysis.

## Usage

```bash
# Analyze and commit all changes
commit

# Preview without committing
commit --dry-run

# Commit only staged files
commit --staged

# Verbose output
commit -v

# Reverse: explode HEAD commit into working changes
commit --reverse

# Analyze changes to a specific file
commit --diff src/main.go

# Analyze changes between refs
commit --diff src/main.go --from HEAD~5 --to HEAD

# Override provider for this run
commit --provider openai

# Self-update to latest version
commit --upgrade
```

## Configuration

### User Config

| Platform | Path |
|----------|------|
| macOS / Linux | `~/.commit-tool/.env` |
| Windows | `%USERPROFILE%\.commit-tool\.env` |

```bash
# Provider selection (required)
COMMIT_PROVIDER=anthropic  # anthropic | openai | grok | gemini | azure-foundry

# Public cloud API keys (use one)
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
GROK_API_KEY=xai-...
GEMINI_API_KEY=AIza...

# Azure AI Foundry (private cloud)
AZURE_FOUNDRY_ENDPOINT=https://your-instance.openai.azure.com
AZURE_FOUNDRY_API_KEY=...
AZURE_FOUNDRY_DEPLOYMENT=your-deployment-name

# Optional
COMMIT_MODEL=claude-3-5-sonnet  # Override default model
COMMIT_DRY_RUN=true             # Always preview
```

### Repo Config: `.commit.json` (Optional)

For monorepos, create a `.commit.json` at your repository root:

```json
{
  "scopes": [
    { "path": "backend/", "scope": "backend" },
    { "path": "frontend/", "scope": "frontend" },
    { "path": "infrastructure/", "scope": "infra" }
  ],
  "defaultScope": "repo"
}
```

Scope resolution uses longest-match-wins, so more specific paths take precedence.

### Commit Type Filtering

Whitelist specific commit types:

```json
{
  "commitTypes": {
    "mode": "whitelist",
    "types": ["docs"]
  }
}
```

Or blacklist types you don't want:

```json
{
  "commitTypes": {
    "mode": "blacklist",
    "types": ["refactor"]
  }
}
```

## Providers

| Provider | Env Var | Default Model |
|----------|---------|---------------|
| Anthropic | `ANTHROPIC_API_KEY` | claude-3-5-sonnet |
| OpenAI | `OPENAI_API_KEY` | gpt-4-turbo-preview |
| Grok (xAI) | `GROK_API_KEY` | grok-beta |
| Gemini | `GEMINI_API_KEY` | gemini-1.5-pro |
| Azure AI Foundry | `AZURE_FOUNDRY_*` | (deployment name) |

## The `--reverse` Flag

Explodes the current HEAD commit into uncommitted working changes. Useful for cleaning up messy commits:

```bash
# Squash messy commits
git rebase -i HEAD~10

# Explode the squashed commit
commit --reverse

# Let the tool re-organize properly
commit
```

**Safety rules:**
- Will not reverse if commit has been pushed to origin
- Requires `--force` flag to reverse pushed commits

## The `--diff` Flag

Analyzes changes to a specific file using the LLM:

```bash
# Analyze uncommitted changes
commit --diff src/auth/login.ts

# Analyze changes between refs
commit --diff src/auth/login.ts --from main --to feature-branch
```

## Logging

The tool maintains JSONL logs for debugging:

```
~/.commit-tool/logs/
├── tool_executions.jsonl     # Registry of all runs
└── executions/
    └── exec_*.jsonl          # Detailed per-execution logs
```

View recent executions:
```bash
tail -5 ~/.commit-tool/logs/tool_executions.jsonl | jq
```

## Building from Source

```bash
git clone https://github.com/dsswift/commit
cd commit
go build -o commit ./cmd/commit
```

## Running Tests

```bash
go test -v ./...
```

## License

MIT
