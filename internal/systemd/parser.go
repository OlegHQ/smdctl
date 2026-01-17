package systemd

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ListServices lists all smdctl-managed services
func (m *Manager) ListServices(all bool) ([]*ServiceInfo, error) {
	args := []string{"list-units", "--type=service", "--no-pager", "--no-legend"}
	if all {
		args = append(args, "--all")
	}
	args = append(args, "smdctl-*")

	output, err := m.systemctlOutput(args...)
	if err != nil {
		// If no services found, return empty list instead of error
		if strings.Contains(err.Error(), "No units found") {
			return []*ServiceInfo{}, nil
		}
		return nil, fmt.Errorf("list services: %w", err)
	}

	var services []*ServiceInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Parse service name (strip .service extension and smdctl- prefix)
		fullName := strings.TrimSuffix(fields[0], ".service")
		name := StripPrefix(fullName)

		// Get detailed info for this service
		info, err := m.GetServiceInfo(name)
		if err != nil {
			// If we can't get details, create basic info
			info = &ServiceInfo{
				Name:   name,
				Status: fields[2], // load status
			}
			if len(fields) >= 4 {
				info.SubState = fields[3] // active status
			}
		}

		services = append(services, info)
	}

	return services, nil
}

// GetServiceInfo gets detailed information about a service
func (m *Manager) GetServiceInfo(name string) (*ServiceInfo, error) {
	if !m.ServiceExists(name) {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	serviceName := ServiceName(name)

	// Get multiple properties in one call
	properties := []string{
		"MainPID",
		"ActiveState",
		"SubState",
		"Description",
		"ActiveEnterTimestamp",
		"MemoryCurrent",
	}

	info := &ServiceInfo{Name: name}

	for _, prop := range properties {
		value, err := m.getServiceProperty(serviceName, prop)
		if err != nil {
			continue
		}

		switch prop {
		case "MainPID":
			if pid, err := strconv.Atoi(value); err == nil {
				info.PID = pid
			}
		case "ActiveState":
			info.Status = value
		case "SubState":
			info.SubState = value
		case "Description":
			info.Description = value
		case "ActiveEnterTimestamp":
			if value != "" && value != "n/a" {
				if t, err := parseSystemdTimestamp(value); err == nil {
					info.Uptime = time.Since(t)
				}
			}
		case "MemoryCurrent":
			// MemoryCurrent may be "[not set]" or a number
			if value != "" && value != "[not set]" {
				if mem, err := strconv.ParseUint(value, 10, 64); err == nil {
					info.MemoryBytes = mem
				}
			}
		}
	}

	// Get listening ports for user services with active PID
	if m.mode == ModeUser && info.Status == "active" && info.PID > 0 {
		info.Ports = GetListeningPorts(info.PID)
	}

	return info, nil
}

// getServiceProperty gets a specific property from systemctl show
func (m *Manager) getServiceProperty(serviceName, property string) (string, error) {
	output, err := m.systemctlOutput("show", "-p", property, "--value", serviceName)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// parseSystemdTimestamp parses systemd timestamp format
func parseSystemdTimestamp(ts string) (time.Time, error) {
	// Systemd timestamp format: "Mon 2024-01-15 10:30:00 UTC"
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		time.RFC3339,
		time.RFC1123,
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}

// GetStatus returns the status output from systemctl status
func (m *Manager) GetStatus(name string) (string, error) {
	if !m.ServiceExists(name) {
		return "", fmt.Errorf("%w: %s", ErrServiceNotFound, name)
	}

	cmdArgs := m.buildSystemctlArgs("status", ServiceName(name), "--no-pager", "-l")
	cmd := exec.Command("systemctl", cmdArgs...)
	output, _ := cmd.CombinedOutput() // Don't treat non-zero exit as error (inactive services return non-zero)

	return string(output), nil
}
