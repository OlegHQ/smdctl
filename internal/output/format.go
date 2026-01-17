package output

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// FormatServicesTable formats services as a table
func FormatServicesTable(services []*systemd.ServiceInfo) {
	if len(services) == 0 {
		fmt.Println("No services found.")
		return
	}

	// Print header
	fmt.Printf("%-15s %-6s %-8s %-10s %-10s %-12s %-14s %s\n",
		"NAME", "MODE", "PID", "STATUS", "MEMORY", "UPTIME", "PORTS", "DESCRIPTION")

	// Print separator
	fmt.Println(strings.Repeat("-", 110))

	// Print services
	for _, svc := range services {
		pid := "-"
		if svc.PID > 0 {
			pid = fmt.Sprintf("%d", svc.PID)
		}

		memory := "-"
		if svc.MemoryBytes > 0 {
			memory = formatBytes(svc.MemoryBytes)
		}

		uptime := "-"
		if svc.Uptime > 0 {
			uptime = formatDuration(svc.Uptime)
		}

		status := svc.SubState
		if status == "" {
			status = svc.Status
		}

		ports := "-"
		if len(svc.Ports) > 0 {
			ports = formatPorts(svc.Ports)
		}

		desc := svc.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		fmt.Printf("%-15s %-6s %-8s %-10s %-10s %-12s %-14s %s\n",
			svc.Name, svc.Mode, pid, status, memory, uptime, ports, desc)
	}
}

// formatPorts formats a list of ports for display
func formatPorts(ports []int) string {
	if len(ports) == 0 {
		return "-"
	}

	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = fmt.Sprintf("%d", p)
	}

	result := strings.Join(strs, ",")
	// Truncate if too long
	if len(result) > 13 {
		return result[:10] + "..."
	}
	return result
}

// FormatServicesQuiet prints only service names
func FormatServicesQuiet(services []*systemd.ServiceInfo) {
	for _, svc := range services {
		fmt.Println(svc.Name)
	}
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div),
		"KMGTPE"[exp])
}

// formatDuration formats duration into human-readable format
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours%24)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", seconds)
}

// PromptYesNo prompts the user for a yes/no answer
func PromptYesNo(message string, defaultYes bool) bool {
	prompt := message
	if defaultYes {
		prompt += " [Y/n] "
	} else {
		prompt += " [y/N] "
	}

	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return defaultYes
	}

	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if answer == "" {
		return defaultYes
	}

	return answer == "y" || answer == "yes"
}
