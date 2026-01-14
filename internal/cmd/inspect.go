package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/snowbear/smdctl/internal/output"
	"github.com/snowbear/smdctl/internal/systemd"
)

// Inspect shows service configuration in YAML format
func Inspect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl inspect SERVICE")
	}

	serviceName := args[0]

	mgr := systemd.NewManager()

	// Verify service exists
	if !mgr.ServiceExists(serviceName) {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	// Get service info
	info, err := mgr.GetServiceInfo(serviceName)
	if err != nil {
		return fmt.Errorf("get service info: %w", err)
	}

	// Get service file content
	serviceFile, err := mgr.GetServiceFile(serviceName)
	if err != nil {
		return fmt.Errorf("get service file: %w", err)
	}

	// Parse environment file if exists
	envPath := fmt.Sprintf("/etc/smdctl/env/%s.env", serviceName)
	envVars := make(map[string]string)
	if envContent, err := os.ReadFile(envPath); err == nil {
		lines := strings.Split(string(envContent), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			}
		}
	}

	// Build inspection data
	inspection := map[string]interface{}{
		"name":        info.Name,
		"description": info.Description,
		"status":      info.Status,
		"substate":    info.SubState,
		"pid":         info.PID,
		"uptime":      info.Uptime.String(),
		"environment": envVars,
		"files": map[string]string{
			"service_file": systemd.ServicePath(serviceName),
			"env_file":     envPath,
		},
		"resources": map[string]interface{}{
			"memory_bytes": info.MemoryBytes,
			"cpu_percent":  info.CPUPercent,
		},
		"service_file_content": serviceFile,
	}

	return output.FormatYAML(inspection)
}
