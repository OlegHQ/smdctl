package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LingerChecker handles user lingering checks
type LingerChecker struct{}

// NewLingerChecker creates a new linger checker
func NewLingerChecker() *LingerChecker {
	return &LingerChecker{}
}

// IsEnabled checks if lingering is enabled for current user
func (lc *LingerChecker) IsEnabled() (bool, error) {
	username := os.Getenv("USER")
	if username == "" {
		return false, fmt.Errorf("USER environment variable not set")
	}

	cmd := exec.Command("loginctl", "show-user", username, "--property=Linger", "--value")
	output, err := cmd.Output()
	if err != nil {
		// User might not be logged in via loginctl (e.g., in container)
		return false, nil
	}

	return strings.TrimSpace(string(output)) == "yes", nil
}

// WarnIfDisabled prints a warning if lingering is disabled
func (lc *LingerChecker) WarnIfDisabled() {
	enabled, err := lc.IsEnabled()
	if err != nil || enabled {
		return
	}

	username := os.Getenv("USER")
	fmt.Printf("\n⚠️  WARNING: User lingering is not enabled for %s\n", username)
	fmt.Printf("   Your services will stop when you log out.\n")
	fmt.Printf("   To enable lingering (services persist after logout):\n")
	fmt.Printf("   sudo loginctl enable-linger %s\n\n", username)
}
