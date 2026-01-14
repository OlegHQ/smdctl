package systemd

import (
	"fmt"
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

	// After dependencies
	if len(svc.After) > 0 {
		sb.WriteString(fmt.Sprintf("After=%s\n", strings.Join(svc.After, " ")))
	} else {
		sb.WriteString("After=network-online.target\n")
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
	execStart := svc.Command
	if len(svc.Args) > 0 {
		execStart = fmt.Sprintf("%s %s", svc.Command, strings.Join(svc.Args, " "))
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

	// User
	if svc.User != "" {
		sb.WriteString(fmt.Sprintf("User=%s\n", svc.User))
	}

	// Environment file
	envFile := fmt.Sprintf("/etc/smdctl/env/%s.env", svc.Name)
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

	sb.WriteString("\n")

	// [Install] section
	sb.WriteString("[Install]\n")
	sb.WriteString("WantedBy=multi-user.target\n")

	return sb.String()
}

// ServiceFileName returns the full service file name with prefix and extension
func ServiceFileName(name string) string {
	return fmt.Sprintf("%s%s.service", servicePrefix, name)
}

// ServicePath returns the full path to the service file
func ServicePath(name string) string {
	return fmt.Sprintf("/etc/systemd/system/%s", ServiceFileName(name))
}

// ServiceName returns the full service name (with prefix)
func ServiceName(name string) string {
	return fmt.Sprintf("%s%s", servicePrefix, name)
}

// StripPrefix removes the smdctl- prefix from a service name
func StripPrefix(fullName string) string {
	return strings.TrimPrefix(fullName, servicePrefix)
}
