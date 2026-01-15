package env

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Edit opens the environment file for a service in an editor
func Edit(serviceName string, mode systemd.SystemdMode) error {
	envPath := systemd.EnvFilePath(serviceName, mode)

	// Create file if it doesn't exist
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// Create directory if needed
		envDir, err := systemd.GetConfigDir(mode)
		if err != nil {
			return fmt.Errorf("get config directory: %w", err)
		}
		if err := os.MkdirAll(envDir, 0755); err != nil {
			return fmt.Errorf("create env directory: %w", err)
		}

		// Create empty file with helpful comment
		content := fmt.Sprintf("# Environment variables for service: %s\n# Format: KEY=VALUE (one per line)\n\n", serviceName)
		if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("create env file: %w", err)
		}
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	// Open editor
	cmd := exec.Command(editor, envPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	fmt.Printf("\nEnvironment file updated: %s\n", envPath)
	fmt.Printf("\nTo apply changes, restart the service:\n")
	fmt.Printf("  smdctl restart %s\n", serviceName)

	return nil
}
