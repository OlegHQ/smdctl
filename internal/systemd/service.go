package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const servicePrefix = "smdctl-"

// ValidateServiceName checks if a service name is valid
func ValidateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Only allow alphanumeric, hyphens, and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	if !matched {
		return ErrInvalidServiceName
	}

	return nil
}

// GenerateServiceFile generates a systemd service file content
func GenerateServiceFile(svc *Service) string {
	var sb strings.Builder

	// [Unit] section
	sb.WriteString("[Unit]\n")
	if svc.Description != "" {
		sb.WriteString(fmt.Sprintf("Description=%s\n", svc.Description))
	} else {
		sb.WriteString(fmt.Sprintf("Description=smdctl managed service: %s\n", svc.Name))
	}

	// After dependencies - different defaults for user vs system
	if len(svc.After) > 0 {
		sb.WriteString(fmt.Sprintf("After=%s\n", strings.Join(svc.After, " ")))
	} else {
		if svc.Mode == ModeUser {
			// User services don't need network-online.target
			sb.WriteString("After=default.target\n")
		} else {
			sb.WriteString("After=network-online.target\n")
		}
	}

	// Wants dependencies
	if len(svc.Wants) > 0 {
		sb.WriteString(fmt.Sprintf("Wants=%s\n", strings.Join(svc.Wants, " ")))
	}

	sb.WriteString("\n")

	// [Service] section
	sb.WriteString("[Service]\n")
	sb.WriteString("Type=simple\n")

	// ExecStart - build command with args
	// Systemd requires absolute paths for ExecStart
	command := svc.Command
	if !filepath.IsAbs(command) && svc.WorkDir != "" {
		// Convert relative path to absolute using WorkDir
		command = filepath.Join(svc.WorkDir, command)
	}
	execStart := command
	if len(svc.Args) > 0 {
		execStart = fmt.Sprintf("%s %s", command, strings.Join(svc.Args, " "))
	}
	sb.WriteString(fmt.Sprintf("ExecStart=%s\n", execStart))

	// Restart policy
	if svc.Restart != "" {
		sb.WriteString(fmt.Sprintf("Restart=%s\n", svc.Restart))
	}
	sb.WriteString("RestartSec=5s\n")

	// Working directory
	if svc.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", svc.WorkDir))
	}

	// User - only for system mode services
	if svc.Mode == ModeSystem && svc.User != "" {
		sb.WriteString(fmt.Sprintf("User=%s\n", svc.User))
	}

	// Environment file
	envFile := EnvFilePath(svc.Name, svc.Mode)
	sb.WriteString(fmt.Sprintf("EnvironmentFile=-%s\n", envFile))

	// Timeouts
	if svc.TimeoutStart > 0 {
		sb.WriteString(fmt.Sprintf("TimeoutStartSec=%d\n", svc.TimeoutStart))
	}
	if svc.TimeoutStop > 0 {
		sb.WriteString(fmt.Sprintf("TimeoutStopSec=%d\n", svc.TimeoutStop))
	}

	// Kill mode
	if svc.KillMode != "" {
		sb.WriteString(fmt.Sprintf("KillMode=%s\n", svc.KillMode))
	}

	// Security options
	if svc.PrivateTmp {
		sb.WriteString("PrivateTmp=true\n")
	}
	if svc.ProtectSystem != "" {
		sb.WriteString(fmt.Sprintf("ProtectSystem=%s\n", svc.ProtectSystem))
	}
	if svc.NoNewPrivileges {
		sb.WriteString("NoNewPrivileges=true\n")
	}

	// Resource limits
	if svc.LimitNOFILE > 0 {
		sb.WriteString(fmt.Sprintf("LimitNOFILE=%d\n", svc.LimitNOFILE))
	}
	if svc.TasksMax > 0 {
		sb.WriteString(fmt.Sprintf("TasksMax=%d\n", svc.TasksMax))
	}

	// Logging - user services use file-based logging for reliable access
	if svc.Mode == ModeUser {
		logFile := LogFilePath(svc.Name, svc.Mode)
		if logFile != "" {
			sb.WriteString(fmt.Sprintf("StandardOutput=append:%s\n", logFile))
			sb.WriteString(fmt.Sprintf("StandardError=append:%s\n", logFile))
		}
	}

	sb.WriteString("\n")

	// [Install] section - different for user vs system
	sb.WriteString("[Install]\n")
	if svc.Mode == ModeUser {
		sb.WriteString("WantedBy=default.target\n")
	} else {
		sb.WriteString("WantedBy=multi-user.target\n")
	}

	return sb.String()
}

// ServiceFileName returns the full service file name with prefix and extension
func ServiceFileName(name string) string {
	return fmt.Sprintf("%s%s.service", servicePrefix, name)
}

// ServicePath returns the full path to the service file based on mode
func ServicePath(name string, mode SystemdMode) string {
	if mode == ModeSystem {
		return fmt.Sprintf("/etc/systemd/system/%s", ServiceFileName(name))
	}

	// User mode: ~/.config/systemd/user/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to /etc if home dir unavailable
		return fmt.Sprintf("/etc/systemd/system/%s", ServiceFileName(name))
	}
	return fmt.Sprintf("%s/.config/systemd/user/%s", homeDir, ServiceFileName(name))
}

// EnvFilePath returns the path to environment file based on mode
func EnvFilePath(name string, mode SystemdMode) string {
	if mode == ModeSystem {
		return fmt.Sprintf("/etc/smdctl/env/%s.env", name)
	}

	// User mode: ~/.config/smdctl/env/
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("/etc/smdctl/env/%s.env", name)
	}
	return fmt.Sprintf("%s/.config/smdctl/env/%s.env", homeDir, name)
}

// GetConfigDir returns the config directory for the given mode
func GetConfigDir(mode SystemdMode) (string, error) {
	if mode == ModeSystem {
		return "/etc/smdctl/env", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config/smdctl/env"), nil
}

// GetLogDir returns the log directory for the given mode
func GetLogDir(mode SystemdMode) (string, error) {
	if mode == ModeSystem {
		return "/var/log/smdctl", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config/smdctl/logs"), nil
}

// LogFilePath returns the path to log file based on mode
func LogFilePath(name string, mode SystemdMode) string {
	if mode == ModeSystem {
		return "" // System services use journal
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config/smdctl/logs", name+".log")
}

// ServiceName returns the full service name (with prefix)
func ServiceName(name string) string {
	return fmt.Sprintf("%s%s", servicePrefix, name)
}

// StripPrefix removes the smdctl- prefix from a service name
func StripPrefix(fullName string) string {
	return strings.TrimPrefix(fullName, servicePrefix)
}
