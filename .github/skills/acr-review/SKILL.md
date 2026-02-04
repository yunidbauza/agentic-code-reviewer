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
