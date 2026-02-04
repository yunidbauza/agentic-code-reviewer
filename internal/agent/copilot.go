package agent

import (
	"context"
	"fmt"
	"os/exec"
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
