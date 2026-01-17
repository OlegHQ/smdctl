package cmd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Regenerate regenerates a service file with the current template
// This is useful for applying new features (like file-based logging) to existing services
func Regenerate(args []string) error {
	fs := flag.NewFlagSet("regenerate", flag.ExitOnError)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: smdctl regenerate SERVICE")
	}

	serviceName := fs.Arg(0)

	// Discover mode
	mode, err := systemd.DiscoverServiceMode(serviceName)
	if err != nil {
		return err
	}

	// Read existing service file
	servicePath := systemd.ServicePath(serviceName, mode)
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return fmt.Errorf("read service file: %w", err)
	}

	// Parse existing service file
	svc, err := parseServiceFile(string(content), serviceName, mode)
	if err != nil {
		return fmt.Errorf("parse service file: %w", err)
	}

	// For user services, ensure log directory exists
	if mode == systemd.ModeUser {
		logDir, err := systemd.GetLogDir(mode)
		if err == nil {
			_ = os.MkdirAll(logDir, 0755)
		}
	}

	// Generate new service file with current template
	newContent := systemd.GenerateServiceFile(svc)

	// Write updated service file
	if err := os.WriteFile(servicePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Reload systemd daemon
	mgr := systemd.NewManagerWithMode(mode)
	if err := mgr.DaemonReload(); err != nil {
		return fmt.Errorf("daemon reload: %w", err)
	}

	fmt.Printf("Regenerated service file: %s\n", servicePath)

	// Show what changed for user services
	if mode == systemd.ModeUser {
		logFile := systemd.LogFilePath(serviceName, mode)
		fmt.Printf("Log file: %s\n", logFile)
	}

	fmt.Printf("\nRestart the service to apply changes:\n")
	fmt.Printf("  smdctl restart %s\n", serviceName)

	return nil
}

// parseServiceFile parses a systemd service file and extracts configuration
func parseServiceFile(content string, name string, mode systemd.SystemdMode) (*systemd.Service, error) {
	svc := &systemd.Service{
		Name:        name,
		Mode:        mode,
		Environment: make(map[string]string),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Description":
			svc.Description = value
		case "ExecStart":
			// Parse command and args
			cmdParts := strings.Fields(value)
			if len(cmdParts) > 0 {
				svc.Command = cmdParts[0]
				if len(cmdParts) > 1 {
					svc.Args = cmdParts[1:]
				}
			}
		case "WorkingDirectory":
			svc.WorkDir = value
		case "User":
			svc.User = value
		case "Restart":
			svc.Restart = value
		case "TimeoutStartSec":
			fmt.Sscanf(value, "%d", &svc.TimeoutStart)
		case "TimeoutStopSec":
			fmt.Sscanf(value, "%d", &svc.TimeoutStop)
		case "KillMode":
			svc.KillMode = value
		case "PrivateTmp":
			svc.PrivateTmp = value == "true" || value == "yes"
		case "ProtectSystem":
			svc.ProtectSystem = value
		case "NoNewPrivileges":
			svc.NoNewPrivileges = value == "true" || value == "yes"
		case "LimitNOFILE":
			fmt.Sscanf(value, "%d", &svc.LimitNOFILE)
		case "TasksMax":
			fmt.Sscanf(value, "%d", &svc.TasksMax)
		case "After":
			svc.After = strings.Fields(value)
		case "Wants":
			svc.Wants = strings.Fields(value)
		}
	}

	if svc.Command == "" {
		return nil, fmt.Errorf("no ExecStart found in service file")
	}

	return svc, nil
}
