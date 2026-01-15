package sudo

import (
	"fmt"
	"os"
	"os/exec"
)

// NeedsSudoForSystem checks if we need sudo for system mode operations
func NeedsSudoForSystem() bool {
	// Already running as root
	if os.Getuid() == 0 {
		return false
	}

	// Try to write test file to /etc/systemd/system
	testFile := "/etc/systemd/system/.smdctl-permission-test"
	f, err := os.Create(testFile)
	if err != nil {
		return true // Need sudo
	}
	f.Close()
	os.Remove(testFile)
	return false // Already have permissions
}

// NeedsSudoForUser returns false - user mode doesn't need sudo
func NeedsSudoForUser() bool {
	return false
}

// ReExecWithSudo re-executes the current command with sudo
func ReExecWithSudo() error {
	if os.Getuid() == 0 {
		return nil // Already root
	}

	// Build sudo command with original args
	args := append([]string{os.Args[0]}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo execution failed: %w", err)
	}

	// Exit with same code as sudo process
	if cmd.ProcessState != nil {
		os.Exit(cmd.ProcessState.ExitCode())
	}
	os.Exit(0)
	return nil
}
