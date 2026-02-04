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
