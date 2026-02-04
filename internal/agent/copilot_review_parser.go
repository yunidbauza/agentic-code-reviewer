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
