package agent

import (
	"strings"
	"testing"

	"github.com/richhaase/agentic-code-reviewer/internal/domain"
)

func TestNewCopilotSummaryParser(t *testing.T) {
	parser := NewCopilotSummaryParser()
	if parser == nil {
		t.Fatal("NewCopilotSummaryParser() returned nil")
	}
}

func TestCopilotSummaryParser_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    *domain.GroupedFindings
		wantErr bool
	}{
		{
			name:  "valid JSON with findings and info",
			input: []byte(`{"findings": [{"title": "Security Issue", "summary": "SQL injection vulnerability", "messages": ["msg1"], "reviewer_count": 2, "sources": [1, 2]}], "info": [{"title": "Note", "summary": "A note", "messages": ["info1"], "reviewer_count": 1, "sources": [3]}]}`),
			want: &domain.GroupedFindings{
				Findings: []domain.FindingGroup{
					{Title: "Security Issue", Summary: "SQL injection vulnerability", Messages: []string{"msg1"}, ReviewerCount: 2, Sources: []int{1, 2}},
				},
				Info: []domain.FindingGroup{
					{Title: "Note", Summary: "A note", Messages: []string{"info1"}, ReviewerCount: 1, Sources: []int{3}},
				},
			},
			wantErr: false,
		},
		{
			name:  "JSON with markdown fence",
			input: []byte("```json\n{\"findings\": [{\"title\": \"Bug\", \"summary\": \"Nil pointer\", \"messages\": [], \"reviewer_count\": 1, \"sources\": [1]}], \"info\": []}\n```"),
			want: &domain.GroupedFindings{
				Findings: []domain.FindingGroup{
					{Title: "Bug", Summary: "Nil pointer", Messages: []string{}, ReviewerCount: 1, Sources: []int{1}},
				},
				Info: []domain.FindingGroup{},
			},
			wantErr: false,
		},
		{
			name:  "multiple findings",
			input: []byte(`{"findings": [{"title": "F1", "summary": "S1", "messages": [], "reviewer_count": 1, "sources": []}, {"title": "F2", "summary": "S2", "messages": ["a", "b"], "reviewer_count": 2, "sources": [1, 2]}], "info": []}`),
			want: &domain.GroupedFindings{
				Findings: []domain.FindingGroup{
					{Title: "F1", Summary: "S1", Messages: []string{}, ReviewerCount: 1, Sources: []int{}},
					{Title: "F2", Summary: "S2", Messages: []string{"a", "b"}, ReviewerCount: 2, Sources: []int{1, 2}},
				},
				Info: []domain.FindingGroup{},
			},
			wantErr: false,
		},
		{
			name:  "empty findings and info",
			input: []byte(`{"findings": [], "info": []}`),
			want: &domain.GroupedFindings{
				Findings: []domain.FindingGroup{},
				Info:     []domain.FindingGroup{},
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   []byte("not valid json"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   []byte(""),
			want:    nil,
			wantErr: true,
		},
	}

	parser := NewCopilotSummaryParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(got.Findings) != len(tt.want.Findings) {
				t.Errorf("Parse() got %d findings, want %d", len(got.Findings), len(tt.want.Findings))
			}

			if len(got.Info) != len(tt.want.Info) {
				t.Errorf("Parse() got %d info, want %d", len(got.Info), len(tt.want.Info))
			}

			for i := range got.Findings {
				if got.Findings[i].Title != tt.want.Findings[i].Title {
					t.Errorf("Finding[%d].Title = %q, want %q", i, got.Findings[i].Title, tt.want.Findings[i].Title)
				}
				if got.Findings[i].Summary != tt.want.Findings[i].Summary {
					t.Errorf("Finding[%d].Summary = %q, want %q", i, got.Findings[i].Summary, tt.want.Findings[i].Summary)
				}
				if got.Findings[i].ReviewerCount != tt.want.Findings[i].ReviewerCount {
					t.Errorf("Finding[%d].ReviewerCount = %d, want %d", i, got.Findings[i].ReviewerCount, tt.want.Findings[i].ReviewerCount)
				}
			}
		})
	}
}

func TestCopilotSummaryParser_SummaryParserInterface(t *testing.T) {
	var _ SummaryParser = (*CopilotSummaryParser)(nil)
}

func TestCopilotSummaryParser_ParseErrorIncludesContent(t *testing.T) {
	parser := NewCopilotSummaryParser()

	// Malformed JSON that will cause a parse error
	input := []byte(`{invalid json}`)

	_, err := parser.Parse(input)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	// Error should include content preview for debugging
	if !strings.Contains(err.Error(), "failed to parse summary JSON") {
		t.Errorf("error should include parse failure details, got: %v", err)
	}
}
