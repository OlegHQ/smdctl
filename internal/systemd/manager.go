package systemd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles systemd operations
type Manager struct{}

// NewManager creates a new systemd manager
func NewManager() *Manager {
	return &Manager{}
}

// Create creates a new systemd service
func (m *Manager) Create(svc *Service) error {
	if err := ValidateServiceName(svc.Name); err != nil {
		return err
	}

	// Check if service already exists
	if m.ServiceExists(svc.Name) {
		return fmt.Errorf("%w: %s", ErrServiceAlreadyExists, svc.Name)
	}

	// Generate service file content
	content := GenerateServiceFile(svc)
	path := ServicePath(svc.Name)

	// Write service file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Create environment file if needed
	if len(svc.Environment) > 0 {
		if err := m.CreateEnvFile(svc.Name, svc.Environment); err != nil {
			return fmt.Errorf("create environment file: %w", err)
		}
	}

	// Reload systemd daemon
	if err := m.DaemonReload(); err != nil {
		return fmt.Errorf("daemon reload: %w", err)
	}

	return nil
}

// CreateEnvFile creates an environment file for a service
func (m *Manager) CreateEnvFile(serviceName string, env map[string]string) error {
	// Create directory if it doesn't exist
	envDir := "/etc/smdctl/env"
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("create env directory: %w", err)
	}

	// Build environment file content
	var content strings.Builder
	content.WriteString("# Environment variables for smdctl service: " + serviceName + "\n")
	content.WriteString("# Edit this file with: smdctl env " + serviceName + "\n\n")

	for key, value := range env {
		content.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	// Write environment file
	envPath := filepath.Join(envDir, serviceName+".env")
	if err := os.WriteFile(envPath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	return nil
}

// ServiceExists checks if a service exists
func (m *Manager) ServiceExists(name string) bool {
	path := ServicePath(name)
	_, err := os.Stat(path)
	return err == nil
}

// Start starts a service
func (m *Manager) Start(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return m.systemctl("start", ServiceName(name))
}

// Stop stops a service
func (m *Manager) Stop(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return m.systemctl("stop", ServiceName(name))
}

// Restart restarts a service
func (m *Manager) Restart(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return m.systemctl("restart", ServiceName(name))
}

// Enable enables a service to start at boot
func (m *Manager) Enable(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return m.systemctl("enable", ServiceName(name))
}

// Disable disables a service from starting at boot
func (m *Manager) Disable(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	return m.systemctl("disable", ServiceName(name))
}

// Remove removes a service
func (m *Manager) Remove(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	// Stop service if running
	_ = m.Stop(name)

	// Disable service
	_ = m.Disable(name)

	// Remove service file
	path := ServicePath(name)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove service file: %w", err)
	}

	// Remove environment file if exists
	envPath := fmt.Sprintf("/etc/smdctl/env/%s.env", name)
	_ = os.Remove(envPath)

	// Reload systemd daemon
	if err := m.DaemonReload(); err != nil {
		return fmt.Errorf("daemon reload: %w", err)
	}

	return nil
}

// DaemonReload reloads the systemd daemon
func (m *Manager) DaemonReload() error {
	return m.systemctl("daemon-reload")
}

// systemctl executes a systemctl command
func (m *Manager) systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return nil
}

// systemctlOutput executes a systemctl command and returns output
func (m *Manager) systemctlOutput(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("systemctl %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// IsActive checks if a service is active
func (m *Manager) IsActive(name string) bool {
	cmd := exec.Command("systemctl", "is-active", ServiceName(name))
	return cmd.Run() == nil
}

// GetServiceFile returns the content of a service file
func (m *Manager) GetServiceFile(name string) (string, error) {
	if !m.ServiceExists(name) {
		return "", fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	content, err := os.ReadFile(ServicePath(name))
	if err != nil {
		return "", fmt.Errorf("read service file: %w", err)
	}

	return string(content), nil
}
