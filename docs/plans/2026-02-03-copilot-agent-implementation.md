# Copilot Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `copilot` agent type to ACR that uses GitHub Copilot CLI (`copilot -p`) instead of external LLM CLIs.

**Architecture:** CopilotAgent implements the Agent interface, invoking `copilot -p "prompt"` for reviews and summaries. The parser handles plain text output in `file:line: description` format. Registration uses the existing factory registry pattern.

**Tech Stack:** Go, GitHub Copilot CLI, existing agent/parser patterns

---

## Task 1: Add Copilot Prompt

**Files:**
- Modify: `internal/agent/prompts.go`
- Test: `internal/agent/prompts_test.go`

**Step 1: Write the failing test**

Add test case to `internal/agent/prompts_test.go`:

```go
func TestDefaultCopilotPrompt(t *testing.T) {
	if DefaultCopilotPrompt == "" {
		t.Error("DefaultCopilotPrompt should not be empty")
	}
	if !strings.Contains(DefaultCopilotPrompt, "file:line:") {
		t.Error("DefaultCopilotPrompt should specify file:line: output format")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestDefaultCopilotPrompt -v`
Expected: FAIL with "undefined: DefaultCopilotPrompt"

**Step 3: Write minimal implementation**

Add to `internal/agent/prompts.go`:

```go
// DefaultCopilotPrompt is the default review prompt for GitHub Copilot CLI.
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

// DefaultCopilotRefFilePrompt is the review prompt used when the diff is passed via
// a reference file instead of being embedded in the prompt.
const DefaultCopilotRefFilePrompt = `You are a code reviewer. Review the code changes in the diff file and identify actionable issues.

The diff to review is in file: %s
Read the file contents to examine the changes.

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

Review the changes now and output your findings.`
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestDefaultCopilotPrompt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/prompts.go internal/agent/prompts_test.go
git commit -m "feat(agent): add DefaultCopilotPrompt for Copilot CLI"
```

---

## Task 2: Create Copilot Review Parser

**Files:**
- Create: `internal/agent/copilot_review_parser.go`
- Create: `internal/agent/copilot_parser_test.go`

**Step 1: Write the failing test**

Create `internal/agent/copilot_parser_test.go`:

```go
package agent

import (
	"bufio"
	"strings"
	"testing"
)

func TestCopilotOutputParser_ReadFinding(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		reviewerID int
		want       []string // Expected finding texts in order
	}{
		{
			name:       "single valid finding",
			reviewerID: 1,
			input:      "auth/login.go:45: SQL injection vulnerability",
			want:       []string{"auth/login.go:45: SQL injection vulnerability"},
		},
		{
			name:       "multiple valid findings",
			reviewerID: 2,
			input: `auth/login.go:45: SQL injection vulnerability
api/handler.go:123: Resource leak - HTTP response body not closed
utils/parser.go:67: Potential panic - missing nil check`,
			want: []string{
				"auth/login.go:45: SQL injection vulnerability",
				"api/handler.go:123: Resource leak - HTTP response body not closed",
				"utils/parser.go:67: Potential panic - missing nil check",
			},
		},
		{
			name:       "findings with empty lines",
			reviewerID: 1,
			input: `auth/login.go:45: Finding 1

api/handler.go:123: Finding 2
`,
			want: []string{
				"auth/login.go:45: Finding 1",
				"api/handler.go:123: Finding 2",
			},
		},
		{
			name:       "skip non-finding lines",
			reviewerID: 1,
			input: `Starting review...
auth/login.go:45: Valid finding
Debug info here
api/handler.go:123: Another finding
Review complete.`,
			want: []string{
				"auth/login.go:45: Valid finding",
				"api/handler.go:123: Another finding",
			},
		},
		{
			name:       "empty input",
			reviewerID: 1,
			input:      "",
			want:       []string{},
		},
		{
			name:       "only whitespace",
			reviewerID: 1,
			input:      "\n\n\n",
			want:       []string{},
		},
		{
			name:       "windows line endings",
			reviewerID: 1,
			input:      "file.go:10: Finding 1\r\nfile.go:20: Finding 2\r\n",
			want: []string{
				"file.go:10: Finding 1",
				"file.go:20: Finding 2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewCopilotOutputParser(tt.reviewerID)
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			ConfigureScanner(scanner)

			var got []string
			for {
				finding, err := parser.ReadFinding(scanner)
				if err != nil {
					if IsRecoverable(err) {
						continue
					}
					t.Fatalf("ReadFinding() error = %v", err)
				}
				if finding == nil {
					break
				}
				got = append(got, finding.Text)
				if finding.ReviewerID != tt.reviewerID {
					t.Errorf("ReviewerID = %d, want %d", finding.ReviewerID, tt.reviewerID)
				}
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d findings, want %d\nGot: %v\nWant: %v", len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("finding[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCopilotOutputParser_ParseErrors(t *testing.T) {
	parser := NewCopilotOutputParser(1)
	scanner := bufio.NewScanner(strings.NewReader("file.go:10: Valid finding"))
	ConfigureScanner(scanner)

	for {
		finding, err := parser.ReadFinding(scanner)
		if err != nil && !IsRecoverable(err) {
			t.Fatalf("unexpected error: %v", err)
		}
		if finding == nil {
			break
		}
	}

	// Copilot parser doesn't have parse errors - it just skips non-matching lines
	if parser.ParseErrors() != 0 {
		t.Errorf("ParseErrors() = %d, want 0", parser.ParseErrors())
	}
}

func TestNewCopilotOutputParser(t *testing.T) {
	parser := NewCopilotOutputParser(42)
	if parser == nil {
		t.Fatal("NewCopilotOutputParser() returned nil")
	}
	if parser.reviewerID != 42 {
		t.Errorf("reviewerID = %d, want 42", parser.reviewerID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestCopilotOutputParser -v`
Expected: FAIL with "undefined: NewCopilotOutputParser"

**Step 3: Write minimal implementation**

Create `internal/agent/copilot_review_parser.go`:

```go
package agent

import (
	"bufio"
	"regexp"
	"strings"

	"github.com/richhaase/agentic-code-reviewer/internal/domain"
)

// findingLinePattern matches "file:line: description" format.
// Captures: file path, line number, and description.
var findingLinePattern = regexp.MustCompile(`^([^:]+):(\d+):\s*(.+)$`)

// CopilotOutputParser parses plain text output from the Copilot CLI.
// Expects findings in "file:line: description" format, one per line.
type CopilotOutputParser struct {
	reviewerID int
}

// NewCopilotOutputParser creates a new parser for Copilot output.
func NewCopilotOutputParser(reviewerID int) *CopilotOutputParser {
	return &CopilotOutputParser{
		reviewerID: reviewerID,
	}
}

// ReadFinding reads and parses the next finding from the Copilot output stream.
// Copilot outputs plain text with findings in "file:line: description" format.
//
// Returns a finding when one is found.
// Returns (nil, nil) when no more findings are available (end of stream).
// Lines not matching the finding pattern are silently skipped.
func (p *CopilotOutputParser) ReadFinding(scanner *bufio.Scanner) (*domain.Finding, error) {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Try to match finding pattern
		matches := findingLinePattern.FindStringSubmatch(line)
		if matches != nil {
			// Reconstruct the full finding text for consistency
			return &domain.Finding{
				Text:       line,
				ReviewerID: p.reviewerID,
			}, nil
		}
		// Line doesn't match pattern - skip it (not a finding)
	}

	// Check for scanner error
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// No more findings
	return nil, nil
}

// ParseErrors returns the number of recoverable parse errors encountered.
// Copilot parser doesn't track parse errors - non-matching lines are expected.
func (p *CopilotOutputParser) ParseErrors() int {
	return 0
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestCopilotOutputParser -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/copilot_review_parser.go internal/agent/copilot_parser_test.go
git commit -m "feat(agent): add CopilotOutputParser for plain text findings"
```

---

## Task 3: Create Copilot Summary Parser

**Files:**
- Create: `internal/agent/copilot_summary_parser.go`
- Create: `internal/agent/copilot_summary_parser_test.go`

**Step 1: Write the failing test**

Create `internal/agent/copilot_summary_parser_test.go`:

```go
package agent

import (
	"testing"
)

func TestCopilotSummaryParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int // Number of groups expected
		wantErr bool
	}{
		{
			name: "valid JSON output",
			input: `{"groups":[{"title":"Security Issues","findings":[{"file":"auth.go","line":"45","description":"SQL injection"}]}]}`,
			want: 1,
		},
		{
			name: "JSON with markdown fence",
			input: "```json\n{\"groups\":[{\"title\":\"Bugs\",\"findings\":[{\"file\":\"main.go\",\"line\":\"10\",\"description\":\"Nil pointer\"}]}]}\n```",
			want: 1,
		},
		{
			name: "multiple groups",
			input: `{"groups":[
				{"title":"Security","findings":[{"file":"a.go","line":"1","description":"Issue 1"}]},
				{"title":"Bugs","findings":[{"file":"b.go","line":"2","description":"Issue 2"}]}
			]}`,
			want: 2,
		},
		{
			name:    "invalid JSON",
			input:   "not valid json",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewCopilotSummaryParser()
			result, err := parser.Parse([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Groups) != tt.want {
				t.Errorf("got %d groups, want %d", len(result.Groups), tt.want)
			}
		})
	}
}

func TestNewCopilotSummaryParser(t *testing.T) {
	parser := NewCopilotSummaryParser()
	if parser == nil {
		t.Fatal("NewCopilotSummaryParser() returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestCopilotSummaryParser -v`
Expected: FAIL with "undefined: NewCopilotSummaryParser"

**Step 3: Write minimal implementation**

Create `internal/agent/copilot_summary_parser.go`:

```go
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/richhaase/agentic-code-reviewer/internal/domain"
)

// CopilotSummaryParser parses summary output from the Copilot CLI.
// Expects JSON output (possibly wrapped in markdown code fences).
type CopilotSummaryParser struct{}

// NewCopilotSummaryParser creates a new CopilotSummaryParser.
func NewCopilotSummaryParser() *CopilotSummaryParser {
	return &CopilotSummaryParser{}
}

// Parse parses the summary output and returns grouped findings.
// Handles plain JSON or JSON wrapped in markdown code fences.
func (p *CopilotSummaryParser) Parse(data []byte) (*domain.GroupedFindings, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty summary output")
	}

	// Strip markdown code fences if present
	cleaned := StripMarkdownCodeFence(string(data))

	// Parse the JSON
	var grouped domain.GroupedFindings
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	if err := decoder.Decode(&grouped); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w (content: %s)", err, truncate(cleaned, 200))
	}

	return &grouped, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestCopilotSummaryParser -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/copilot_summary_parser.go internal/agent/copilot_summary_parser_test.go
git commit -m "feat(agent): add CopilotSummaryParser for JSON output"
```

---

## Task 4: Create Copilot Agent

**Files:**
- Create: `internal/agent/copilot.go`
- Create: `internal/agent/copilot_test.go`

**Step 1: Write the failing test**

Create `internal/agent/copilot_test.go`:

```go
package agent

import (
	"testing"
)

func TestCopilotAgent_Name(t *testing.T) {
	agent := NewCopilotAgent()
	if got := agent.Name(); got != "copilot" {
		t.Errorf("Name() = %q, want %q", got, "copilot")
	}
}

func TestNewCopilotAgent(t *testing.T) {
	agent := NewCopilotAgent()
	if agent == nil {
		t.Fatal("NewCopilotAgent() returned nil")
	}
}

func TestCopilotAgent_IsAvailable(t *testing.T) {
	agent := NewCopilotAgent()
	// This test will pass or fail depending on whether copilot CLI is installed
	// We just verify it doesn't panic
	_ = agent.IsAvailable()
}

func TestBuildCopilotRefFilePrompt(t *testing.T) {
	tests := []struct {
		name         string
		customPrompt string
		diffPath     string
		wantContains []string
	}{
		{
			name:         "default prompt with path",
			customPrompt: "",
			diffPath:     "/tmp/diff.txt",
			wantContains: []string{"/tmp/diff.txt", "file:line:"},
		},
		{
			name:         "custom prompt with path",
			customPrompt: "Review this code carefully",
			diffPath:     "/tmp/diff.txt",
			wantContains: []string{"Review this code carefully", "/tmp/diff.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCopilotRefFilePrompt(tt.customPrompt, tt.diffPath)
			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("BuildCopilotRefFilePrompt() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestCopilotAgent -v`
Expected: FAIL with "undefined: NewCopilotAgent"

**Step 3: Write minimal implementation**

Create `internal/agent/copilot.go`:

```go
package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Compile-time interface check
var _ Agent = (*CopilotAgent)(nil)

// BuildCopilotRefFilePrompt constructs the review prompt for ref-file mode.
// If customPrompt is provided, it appends ref-file instructions to it.
// Otherwise, it uses DefaultCopilotRefFilePrompt.
func BuildCopilotRefFilePrompt(customPrompt, diffPath string) string {
	if customPrompt != "" {
		return fmt.Sprintf("%s\n\nThe diff to review is in file: %s\nRead the file contents to examine the changes.", customPrompt, diffPath)
	}
	return fmt.Sprintf(DefaultCopilotRefFilePrompt, diffPath)
}

// CopilotAgent implements the Agent interface for the GitHub Copilot CLI backend.
type CopilotAgent struct{}

// NewCopilotAgent creates a new CopilotAgent instance.
func NewCopilotAgent() *CopilotAgent {
	return &CopilotAgent{}
}

// Name returns the agent's identifier.
func (c *CopilotAgent) Name() string {
	return "copilot"
}

// IsAvailable checks if the copilot CLI is installed and accessible.
func (c *CopilotAgent) IsAvailable() error {
	_, err := exec.LookPath("copilot")
	if err != nil {
		return fmt.Errorf("copilot CLI not found in PATH: %w", err)
	}
	return nil
}

// ExecuteReview runs a code review using the GitHub Copilot CLI.
// Uses 'copilot -p "prompt"' for programmatic mode.
//
// The git diff is either appended to the prompt (default) or written to a
// reference file when the diff is large or UseRefFile is set.
func (c *CopilotAgent) ExecuteReview(ctx context.Context, config *ReviewConfig) (*ExecutionResult, error) {
	if err := c.IsAvailable(); err != nil {
		return nil, err
	}

	diff, err := GetGitDiff(ctx, config.BaseRef, config.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff for review: %w", err)
	}

	var prompt string
	var tempFilePath string

	// Determine if we should use ref-file mode
	useRefFile := config.UseRefFile || len(diff) > RefFileSizeThreshold

	customPrompt := config.CustomPrompt
	if customPrompt == "" {
		customPrompt = DefaultCopilotPrompt
	}

	if useRefFile && diff != "" {
		// Write diff to a temp file in the working directory
		absPath, err := WriteDiffToTempFile(config.WorkDir, diff)
		if err != nil {
			return nil, err
		}
		tempFilePath = absPath
		prompt = BuildCopilotRefFilePrompt(config.CustomPrompt, absPath)
	} else {
		prompt = BuildPromptWithDiff(customPrompt, diff)
	}

	// Use -p for programmatic mode, --allow-tool for file access
	args := []string{"-p", prompt, "--allow-tool", "read"}

	return executeCommand(ctx, executeOptions{
		Command:      "copilot",
		Args:         args,
		WorkDir:      config.WorkDir,
		TempFilePath: tempFilePath,
	})
}

// ExecuteSummary runs a summarization task using the GitHub Copilot CLI.
// Uses 'copilot -p "prompt"' with the input embedded in the prompt.
func (c *CopilotAgent) ExecuteSummary(ctx context.Context, prompt string, input []byte) (*ExecutionResult, error) {
	if err := c.IsAvailable(); err != nil {
		return nil, err
	}

	// Combine prompt and input
	fullPrompt := prompt + "\n\nINPUT JSON:\n" + string(input)

	args := []string{"-p", fullPrompt}

	return executeCommand(ctx, executeOptions{
		Command: "copilot",
		Args:    args,
	})
}

// buildCopilotPromptWithDiff combines the prompt and diff for Copilot CLI.
// Helper function to ensure consistent prompt construction.
func buildCopilotPromptWithDiff(prompt, diff string) string {
	if diff == "" {
		return prompt + "\n\nNo changes detected."
	}
	return prompt + "\n\nGIT DIFF:\n" + diff
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestCopilotAgent -v`
Run: `go test ./internal/agent -run TestBuildCopilotRefFilePrompt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/agent/copilot.go internal/agent/copilot_test.go
git commit -m "feat(agent): add CopilotAgent for GitHub Copilot CLI"
```

---

## Task 5: Register Copilot in Factory

**Files:**
- Modify: `internal/agent/factory.go`
- Modify: `internal/agent/factory_test.go`

**Step 1: Write the failing test**

Add test cases to `internal/agent/factory_test.go`:

In `TestNewAgent`, add:
```go
{
	name:      "copilot agent",
	agentName: "copilot",
	wantName:  "copilot",
	wantErr:   false,
},
```

In `TestNewReviewParser`, add:
```go
{
	name:       "copilot parser",
	agentName:  "copilot",
	reviewerID: 4,
	wantErr:    false,
},
```

In `TestSupportedAgents`, update expected:
```go
expected := []string{"claude", "codex", "copilot", "gemini"}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent -run TestNewAgent -v`
Run: `go test ./internal/agent -run TestSupportedAgents -v`
Expected: FAIL with "copilot" not found

**Step 3: Write minimal implementation**

Add to `internal/agent/factory.go` registry map:

```go
"copilot": {
	newAgent:         func() Agent { return NewCopilotAgent() },
	newReviewParser:  func(id int) ReviewParser { return NewCopilotOutputParser(id) },
	newSummaryParser: func() SummaryParser { return NewCopilotSummaryParser() },
},
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent -run TestNewAgent -v`
Run: `go test ./internal/agent -run TestNewReviewParser -v`
Run: `go test ./internal/agent -run TestSupportedAgents -v`
Expected: PASS

**Step 5: Run all agent tests**

Run: `go test ./internal/agent/... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/agent/factory.go internal/agent/factory_test.go
git commit -m "feat(agent): register copilot agent in factory"
```

---

## Task 6: Run Full Test Suite

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Build**

Run: `make build`
Expected: Success

**Step 4: Commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address lint issues"
```

---

## Task 7: Create Agent Skill Directory Structure

**Files:**
- Create: `.github/skills/acr-review/SKILL.md`

**Step 1: Create skill directory**

Run: `mkdir -p .github/skills/acr-review/scripts`

**Step 2: Create SKILL.md**

Create `.github/skills/acr-review/SKILL.md`:

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

**Step 3: Commit**

```bash
git add .github/skills/acr-review/SKILL.md
git commit -m "feat(skill): add ACR Agent Skill definition"
```

---

## Task 8: Create Setup Scripts

**Files:**
- Create: `.github/skills/acr-review/scripts/setup-acr.ps1`
- Create: `.github/skills/acr-review/scripts/setup-acr.sh`

**Step 1: Create Windows setup script**

Create `.github/skills/acr-review/scripts/setup-acr.ps1`:

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

**Step 2: Create Unix setup script**

Create `.github/skills/acr-review/scripts/setup-acr.sh`:

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

**Step 3: Make Unix script executable**

Run: `chmod +x .github/skills/acr-review/scripts/setup-acr.sh`

**Step 4: Commit**

```bash
git add .github/skills/acr-review/scripts/setup-acr.ps1 .github/skills/acr-review/scripts/setup-acr.sh
git commit -m "feat(skill): add ACR setup scripts"
```

---

## Task 9: Create Review Scripts

**Files:**
- Create: `.github/skills/acr-review/scripts/review-changes.ps1`
- Create: `.github/skills/acr-review/scripts/review-changes.sh`
- Create: `.github/skills/acr-review/scripts/review-pr.ps1`
- Create: `.github/skills/acr-review/scripts/review-branch.ps1`

**Step 1: Create review-changes.ps1**

Create `.github/skills/acr-review/scripts/review-changes.ps1`:

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

**Step 2: Create review-changes.sh**

Create `.github/skills/acr-review/scripts/review-changes.sh`:

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

**Step 3: Create review-pr.ps1**

Create `.github/skills/acr-review/scripts/review-pr.ps1`:

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

**Step 4: Create review-branch.ps1**

Create `.github/skills/acr-review/scripts/review-branch.ps1`:

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

**Step 5: Make Unix script executable**

Run: `chmod +x .github/skills/acr-review/scripts/review-changes.sh`

**Step 6: Commit**

```bash
git add .github/skills/acr-review/scripts/
git commit -m "feat(skill): add ACR review scripts"
```

---

## Task 10: Final Verification

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Run linter**

Run: `make lint`
Expected: No errors

**Step 3: Run staticcheck**

Run: `make staticcheck`
Expected: No errors

**Step 4: Build**

Run: `make build`
Expected: Success

**Step 5: Verify agent help shows copilot**

Run: `./bin/acr --help`
Expected: Help text shows "copilot" in supported agents list

**Step 6: Final commit if needed**

```bash
git add -A
git commit -m "chore: final cleanup"
```

---

## Summary

After completing all tasks, the following files will be created/modified:

**New files:**
- `internal/agent/copilot.go` - CopilotAgent implementation
- `internal/agent/copilot_test.go` - CopilotAgent tests
- `internal/agent/copilot_review_parser.go` - Plain text parser
- `internal/agent/copilot_parser_test.go` - Parser tests
- `internal/agent/copilot_summary_parser.go` - Summary JSON parser
- `internal/agent/copilot_summary_parser_test.go` - Summary parser tests
- `.github/skills/acr-review/SKILL.md` - Agent Skill definition
- `.github/skills/acr-review/scripts/setup-acr.ps1` - Windows setup
- `.github/skills/acr-review/scripts/setup-acr.sh` - Unix setup
- `.github/skills/acr-review/scripts/review-changes.ps1` - Windows review
- `.github/skills/acr-review/scripts/review-changes.sh` - Unix review
- `.github/skills/acr-review/scripts/review-pr.ps1` - PR review
- `.github/skills/acr-review/scripts/review-branch.ps1` - Branch review

**Modified files:**
- `internal/agent/prompts.go` - Add DefaultCopilotPrompt
- `internal/agent/prompts_test.go` - Add prompt test
- `internal/agent/factory.go` - Register copilot in registry
- `internal/agent/factory_test.go` - Update tests for copilot
