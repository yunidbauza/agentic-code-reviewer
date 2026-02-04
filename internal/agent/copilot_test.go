package agent

import (
	"context"
	"os"
	"os/exec"
	"strings"
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
	err := agent.IsAvailable()

	// Check if copilot is in PATH
	_, lookPathErr := exec.LookPath("copilot")

	if lookPathErr != nil {
		// Copilot not in PATH - should return error
		if err == nil {
			t.Error("IsAvailable() should return error when copilot is not in PATH")
		}
		if !strings.Contains(err.Error(), "copilot CLI not found") {
			t.Errorf("IsAvailable() error = %v, want error containing 'copilot CLI not found'", err)
		}
	} else {
		// Copilot is in PATH - should return nil
		if err != nil {
			t.Errorf("IsAvailable() unexpected error = %v", err)
		}
	}
}

func TestCopilotAgent_ExecuteReview_CopilotNotAvailable(t *testing.T) {
	// Temporarily remove PATH to ensure copilot is not available
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", "")

	agent := NewCopilotAgent()
	ctx := context.Background()
	config := &ReviewConfig{
		BaseRef: "main",
		WorkDir: ".",
	}

	result, err := agent.ExecuteReview(ctx, config)
	if err == nil {
		if result != nil {
			result.Close()
		}
		t.Error("ExecuteReview() should return error when copilot is not available")
	}

	if !strings.Contains(err.Error(), "copilot CLI not found") {
		t.Errorf("ExecuteReview() error = %v, want error containing 'copilot CLI not found'", err)
	}
}

func TestCopilotAgent_ExecuteSummary_CopilotNotAvailable(t *testing.T) {
	// Temporarily remove PATH to ensure copilot is not available
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", "")

	agent := NewCopilotAgent()
	ctx := context.Background()

	result, err := agent.ExecuteSummary(ctx, "test prompt", []byte(`{"findings":[]}`))
	if err == nil {
		if result != nil {
			result.Close()
		}
		t.Error("ExecuteSummary() should return error when copilot is not available")
	}

	if !strings.Contains(err.Error(), "copilot CLI not found") {
		t.Errorf("ExecuteSummary() error = %v, want error containing 'copilot CLI not found'", err)
	}
}

func TestCopilotAgentInterface(t *testing.T) {
	var _ Agent = (*CopilotAgent)(nil)
}

func TestBuildCopilotRefFilePrompt(t *testing.T) {
	tests := []struct {
		name         string
		customPrompt string
		diffPath     string
		wantContains []string
		wantMissing  []string
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
		{
			name:         "custom prompt is preserved in ref-file mode",
			customPrompt: "My custom security review prompt with special instructions",
			diffPath:     "/tmp/test.patch",
			wantContains: []string{
				"My custom security review prompt with special instructions",
				"/tmp/test.patch",
				"Read the file contents",
			},
			wantMissing: []string{},
		},
		{
			name:         "empty custom prompt uses default ref-file prompt",
			customPrompt: "",
			diffPath:     "/tmp/test.patch",
			wantContains: []string{
				"/tmp/test.patch",
				"Read the file contents",
				"You are a code reviewer", // from DefaultCopilotRefFilePrompt
			},
			wantMissing: []string{},
		},
		{
			name:         "custom prompt not overwritten by default",
			customPrompt: "CUSTOM_MARKER_12345",
			diffPath:     "/path/to/diff.patch",
			wantContains: []string{
				"CUSTOM_MARKER_12345",
			},
			wantMissing: []string{
				"You are a code reviewer", // should NOT contain default prompt
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCopilotRefFilePrompt(tt.customPrompt, tt.diffPath)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildCopilotRefFilePrompt() missing %q\nGot: %s", want, got)
				}
			}
			for _, notWant := range tt.wantMissing {
				if strings.Contains(got, notWant) {
					t.Errorf("BuildCopilotRefFilePrompt() should not contain %q\nGot: %s", notWant, got)
				}
			}
		})
	}
}
