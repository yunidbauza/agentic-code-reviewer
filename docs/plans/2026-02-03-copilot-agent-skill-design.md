# Copilot Agent & Skill Design for ACR

## Overview

This document describes the design for integrating ACR with GitHub Copilot in enterprise Windows environments where:
- MCP servers cannot be installed
- External API keys (OpenAI, Anthropic, Google) are unavailable
- GitHub Copilot and Copilot CLI are available
- Go is installed (can build from source)

## Goals

1. Add a new `copilot` agent type to ACR that uses `copilot -p` instead of external LLM CLIs
2. Create an Agent Skill that Copilot can discover and use to invoke ACR
3. Provide scripts to build ACR from source and run reviews
4. Fail gracefully when ACR cannot be built

## Architecture

```
User in VS Code
    │
    ▼
"@workspace review my changes"
    │
    ▼
Copilot loads Agent Skill (SKILL.md)
    │
    ▼
Skill script builds ACR (if needed) and runs it
    │
    ▼
ACR calls `copilot -p` for LLM analysis
    │
    ▼
Results returned to Copilot Chat
```

**Key benefit:** No API keys needed - leverages existing enterprise Copilot license for all LLM calls.

---

## Part 1: Copilot Agent (Go Code)

### New Files

#### `internal/agent/copilot.go`

Implements the `Agent` interface using GitHub Copilot CLI.

```go
package agent

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
)

var _ Agent = (*CopilotAgent)(nil)

type CopilotAgent struct{}

func NewCopilotAgent() *CopilotAgent {
    return &CopilotAgent{}
}

func (c *CopilotAgent) Name() string {
    return "copilot"
}

func (c *CopilotAgent) IsAvailable() error {
    _, err := exec.LookPath("copilot")
    if err != nil {
        return fmt.Errorf("copilot CLI not found in PATH: %w", err)
    }
    return nil
}

func (c *CopilotAgent) ExecuteReview(ctx context.Context, config *ReviewConfig) (*ExecutionResult, error) {
    if err := c.IsAvailable(); err != nil {
        return nil, err
    }

    diff, err := GetGitDiff(ctx, config.BaseRef, config.WorkDir)
    if err != nil {
        return nil, fmt.Errorf("failed to get diff for review: %w", err)
    }

    prompt := config.CustomPrompt
    if prompt == "" {
        prompt = DefaultCopilotPrompt
    }
    fullPrompt := BuildPromptWithDiff(prompt, diff)

    // Use -p for programmatic mode, --allow-tool for git access
    args := []string{"-p", fullPrompt, "--allow-tool", "shell(git)"}

    return executeCommand(ctx, executeOptions{
        Command: "copilot",
        Args:    args,
        WorkDir: config.WorkDir,
    })
}

func (c *CopilotAgent) ExecuteSummary(ctx context.Context, prompt string, input []byte) (*ExecutionResult, error) {
    if err := c.IsAvailable(); err != nil {
        return nil, err
    }

    fullPrompt := prompt + "\n\nINPUT JSON:\n" + string(input)
    args := []string{"-p", fullPrompt}

    return executeCommand(ctx, executeOptions{
        Command: "copilot",
        Args:    args,
    })
}
```

#### `internal/agent/copilot_review_parser.go`

Parses plain text output from Copilot CLI into findings.

```go
package agent

import (
    "bufio"
    "regexp"
    "strings"

    "github.com/richhaase/agentic-code-reviewer/internal/domain"
)

type CopilotReviewParser struct{}

func NewCopilotReviewParser() *CopilotReviewParser {
    return &CopilotReviewParser{}
}

// findingPattern matches "file:line: description" format
var findingPattern = regexp.MustCompile(`^([^:]+):(\d+):\s*(.+)$`)

func (p *CopilotReviewParser) Parse(output []byte, reviewerID int) ([]domain.Finding, error) {
    var findings []domain.Finding
    scanner := bufio.NewScanner(strings.NewReader(string(output)))

    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }

        matches := findingPattern.FindStringSubmatch(line)
        if matches != nil {
            findings = append(findings, domain.Finding{
                File:        matches[1],
                Line:        matches[2],
                Description: matches[3],
                ReviewerID:  reviewerID,
            })
        }
    }

    return findings, scanner.Err()
}
```

#### `internal/agent/copilot_summary_parser.go`

Parses summary output (can reuse existing text parsing logic).

```go
package agent

type CopilotSummaryParser struct{}

func NewCopilotSummaryParser() *CopilotSummaryParser {
    return &CopilotSummaryParser{}
}

func (p *CopilotSummaryParser) Parse(output []byte) ([]byte, error) {
    // Copilot returns plain text; extract JSON if wrapped
    // For now, assume output is already in expected format
    return output, nil
}
```

### Modified Files

#### `internal/agent/prompts.go`

Add Copilot-specific prompt (mirrors Codex structure):

```go
// DefaultCopilotPrompt is the default review prompt for Copilot CLI.
const DefaultCopilotPrompt = `You are a code reviewer. Review the provided code changes (git diff) and identify actionable issues.

Focus on:
- Bugs and logic errors
- Security vulnerabilities (SQL injection, XSS, authentication issues, etc.)
- Performance problems (inefficient algorithms, resource leaks, unnecessary operations)
- Maintainability issues (code clarity, error handling, edge cases)
- Best practices violations for the language/framework being used

Output format:
- One finding per line
- Format: file:line: description
- Be specific: include exact file paths, line numbers, and issue descriptions
- Keep findings concise but complete (1-3 sentences)
- Only report actual issues - do not output "looks good" or "no issues found" messages
- If there are genuinely no issues, output nothing

Example findings:
- auth/login.go:45: SQL injection vulnerability - user input not sanitized before query
- api/handler.go:123: Resource leak - HTTP response body not closed in error path
- utils/parser.go:67: Potential panic - missing nil check before dereferencing pointer

Review the changes now and output your findings.`
```

#### `internal/agent/factory.go`

Register the copilot agent:

```go
func NewAgent(name string) (Agent, error) {
    switch name {
    case "codex":
        return NewCodexAgent(), nil
    case "claude":
        return NewClaudeAgent(), nil
    case "gemini":
        return NewGeminiAgent(), nil
    case "copilot":
        return NewCopilotAgent(), nil
    default:
        return nil, fmt.Errorf("unknown agent: %s", name)
    }
}

func NewReviewParser(agentName string) ReviewParser {
    switch agentName {
    case "codex":
        return NewCodexReviewParser()
    case "claude":
        return NewClaudeReviewParser()
    case "gemini":
        return NewGeminiReviewParser()
    case "copilot":
        return NewCopilotReviewParser()
    default:
        return NewCodexReviewParser()
    }
}
```

---

## Part 2: Agent Skill

### Directory Structure

```
.github/skills/acr-review/
├── SKILL.md
└── scripts/
    ├── setup-acr.ps1
    ├── setup-acr.sh
    ├── review-changes.ps1
    ├── review-changes.sh
    ├── review-pr.ps1
    └── review-branch.ps1
```

### SKILL.md

```markdown
---
name: acr-review
description: Run ACR code reviews - review current changes, PRs, or branches for bugs and security issues using parallel AI reviewers
---

# ACR Code Review Skill

Use this skill to perform automated code reviews using ACR (Agentic Code Reviewer).

ACR spawns multiple parallel reviewers to analyze code changes, then aggregates and deduplicates findings.

## Setup (First Time Only)

Build ACR from source (requires Go):

**Windows:**
```powershell
.\.github\skills\acr-review\scripts\setup-acr.ps1
```

**Unix/Mac:**
```bash
./.github/skills/acr-review/scripts/setup-acr.sh
```

## Available Commands

### Review Current Changes

Review uncommitted changes against a base branch:

**Windows:**
```powershell
.\.github\skills\acr-review\scripts\review-changes.ps1 -BaseRef main
```

**Unix/Mac:**
```bash
./.github/skills/acr-review/scripts/review-changes.sh --base-ref main
```

### Review a Pull Request

Review a specific PR by number:

**Windows:**
```powershell
.\.github\skills\acr-review\scripts\review-pr.ps1 -PRNumber 123
```

### Review a Branch

Compare a feature branch against a base branch:

**Windows:**
```powershell
.\.github\skills\acr-review\scripts\review-branch.ps1 -Branch feature-x -BaseRef main
```

## Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| BaseRef | main | Base branch to compare against |
| Reviewers | 5 | Number of parallel reviewers |
| Agent | copilot | LLM agent to use (copilot, codex, claude, gemini) |

## Output

Returns markdown-formatted findings with file:line references, grouped by severity and deduplicated across reviewers.
```

### Scripts

#### setup-acr.ps1

```powershell
# Setup ACR - one-time build from source
$ErrorActionPreference = "Stop"

$acrPath = "$env:USERPROFILE\.acr"
$acrExe = "$acrPath\acr.exe"

# Skip if already built
if (Test-Path $acrExe) {
    Write-Host "ACR already installed at $acrExe"
    exit 0
}

# Check if Go is available
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error @"
Go is not installed. ACR cannot be built.

To install Go:
1. Download from https://go.dev/dl/
2. Install and ensure 'go' is in your PATH
3. Run this script again
"@
    exit 1
}

# Clone and build
Write-Host "Cloning ACR repository..."
git clone https://github.com/richhaase/agentic-code-reviewer.git $acrPath

Write-Host "Building ACR..."
Push-Location $acrPath
go build -o acr.exe ./cmd/acr
Pop-Location

if (Test-Path $acrExe) {
    Write-Host "ACR built successfully at $acrExe"
} else {
    Write-Error "Build failed - acr.exe not created"
    exit 1
}
```

#### setup-acr.sh

```bash
#!/bin/bash
set -e

ACR_PATH="$HOME/.acr"
ACR_BIN="$ACR_PATH/acr"

# Skip if already built
if [ -f "$ACR_BIN" ]; then
    echo "ACR already installed at $ACR_BIN"
    exit 0
fi

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. ACR cannot be built."
    echo ""
    echo "To install Go:"
    echo "1. Download from https://go.dev/dl/"
    echo "2. Install and ensure 'go' is in your PATH"
    echo "3. Run this script again"
    exit 1
fi

# Clone and build
echo "Cloning ACR repository..."
git clone https://github.com/richhaase/agentic-code-reviewer.git "$ACR_PATH"

echo "Building ACR..."
cd "$ACR_PATH"
go build -o acr ./cmd/acr

if [ -f "$ACR_BIN" ]; then
    echo "ACR built successfully at $ACR_BIN"
else
    echo "Error: Build failed - acr binary not created"
    exit 1
fi
```

#### review-changes.ps1

```powershell
param(
    [string]$BaseRef = "main",
    [int]$Reviewers = 5,
    [string]$Agent = "copilot"
)

$ErrorActionPreference = "Stop"

$acr = "$env:USERPROFILE\.acr\acr.exe"

if (-not (Test-Path $acr)) {
    Write-Error @"
ACR not found at $acr

Run the setup script first to build ACR:
.\.github\skills\acr-review\scripts\setup-acr.ps1
"@
    exit 1
}

& $acr --base $BaseRef --reviewers $Reviewers --agent $Agent
```

#### review-changes.sh

```bash
#!/bin/bash
set -e

BASE_REF="main"
REVIEWERS=5
AGENT="copilot"

while [[ $# -gt 0 ]]; do
    case $1 in
        --base-ref) BASE_REF="$2"; shift 2 ;;
        --reviewers) REVIEWERS="$2"; shift 2 ;;
        --agent) AGENT="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

ACR_BIN="$HOME/.acr/acr"

if [ ! -f "$ACR_BIN" ]; then
    echo "Error: ACR not found at $ACR_BIN"
    echo ""
    echo "Run the setup script first to build ACR:"
    echo "./.github/skills/acr-review/scripts/setup-acr.sh"
    exit 1
fi

"$ACR_BIN" --base "$BASE_REF" --reviewers "$REVIEWERS" --agent "$AGENT"
```

#### review-pr.ps1

```powershell
param(
    [Parameter(Mandatory=$true)]
    [int]$PRNumber,
    [int]$Reviewers = 5,
    [string]$Agent = "copilot"
)

$ErrorActionPreference = "Stop"

$acr = "$env:USERPROFILE\.acr\acr.exe"

if (-not (Test-Path $acr)) {
    Write-Error @"
ACR not found at $acr

Run the setup script first to build ACR:
.\.github\skills\acr-review\scripts\setup-acr.ps1
"@
    exit 1
}

& $acr --pr $PRNumber --reviewers $Reviewers --agent $Agent
```

#### review-branch.ps1

```powershell
param(
    [Parameter(Mandatory=$true)]
    [string]$Branch,
    [string]$BaseRef = "main",
    [int]$Reviewers = 5,
    [string]$Agent = "copilot"
)

$ErrorActionPreference = "Stop"

$acr = "$env:USERPROFILE\.acr\acr.exe"

if (-not (Test-Path $acr)) {
    Write-Error @"
ACR not found at $acr

Run the setup script first to build ACR:
.\.github\skills\acr-review\scripts\setup-acr.ps1
"@
    exit 1
}

& $acr --branch $Branch --base $BaseRef --reviewers $Reviewers --agent $Agent
```

---

## Configuration Defaults

| Setting | Value | Rationale |
|---------|-------|-----------|
| Default agent | copilot | Uses enterprise Copilot license, no external API keys |
| Default reviewers | 5 | Matches other agents |
| Default concurrency | 5 | Matches other agents |
| Rate limit handling | Graceful retry | Handle Copilot rate limits with exponential backoff |

---

## Files Changed Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/agent/copilot.go` | CopilotAgent implementation |
| `internal/agent/copilot_review_parser.go` | Parse plain text findings |
| `internal/agent/copilot_summary_parser.go` | Parse summary output |
| `.github/skills/acr-review/SKILL.md` | Agent Skill definition |
| `.github/skills/acr-review/scripts/setup-acr.ps1` | Windows setup script |
| `.github/skills/acr-review/scripts/setup-acr.sh` | Unix setup script |
| `.github/skills/acr-review/scripts/review-changes.ps1` | Windows review script |
| `.github/skills/acr-review/scripts/review-changes.sh` | Unix review script |
| `.github/skills/acr-review/scripts/review-pr.ps1` | PR review script |
| `.github/skills/acr-review/scripts/review-branch.ps1` | Branch review script |

### Modified Files

| File | Change |
|------|--------|
| `internal/agent/prompts.go` | Add `DefaultCopilotPrompt` |
| `internal/agent/factory.go` | Register "copilot" agent type |

---

## Testing Plan

1. **Unit tests** for CopilotAgent and parsers
2. **Integration test** with mock Copilot CLI
3. **Manual test** in VS Code with Copilot Chat
4. **Manual test** in enterprise Windows environment

---

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Copilot rate limits | Graceful retry with exponential backoff |
| Large diffs exceed prompt limits | Use ref-file mode (write diff to temp file) |
| Copilot CLI not available | Clear error message with installation instructions |
| Go not available for build | Clear error message, fail gracefully |

---

## Sources

- [GitHub Copilot CLI Documentation](https://docs.github.com/en/copilot/concepts/agents/about-copilot-cli)
- [Agent Skills in VS Code](https://code.visualstudio.com/docs/copilot/customization/agent-skills)
- [Copilot CLI Rate Limits](https://docs.github.com/en/copilot/concepts/rate-limits)
- [Copilot CLI January 2026 Updates](https://github.blog/changelog/2026-01-14-github-copilot-cli-enhanced-agents-context-management-and-new-ways-to-install/)
