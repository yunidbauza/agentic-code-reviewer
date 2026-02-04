//go:build !windows

package agent

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr configures process group handling for Unix systems.
// Setting Setpgid=true creates a new process group, allowing proper signal handling.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the entire process group on Unix systems.
// Using negative PID sends the signal to all processes in the group.
func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
