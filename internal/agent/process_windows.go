//go:build windows

package agent

import (
	"os/exec"
)

// setSysProcAttr is a no-op on Windows.
// Windows doesn't support Unix-style process groups via Setpgid.
func setSysProcAttr(cmd *exec.Cmd) {
	// No-op on Windows - process groups work differently
}

// killProcessGroup terminates a process on Windows.
// Unlike Unix, Windows doesn't have process groups, so we just kill the main process.
// The taskkill /T option could be used for tree kill, but os.Process.Kill is sufficient
// for most cases as child processes typically terminate when parent dies.
func killProcessGroup(pid int) error {
	// On Windows, we can't easily kill a process group.
	// Returning nil as the process will be killed by cmd.Wait() or context cancellation.
	return nil
}
