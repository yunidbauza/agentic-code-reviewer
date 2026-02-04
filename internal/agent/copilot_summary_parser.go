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
