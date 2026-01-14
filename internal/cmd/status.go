package cmd

import (
	"fmt"
	"os/exec"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Status shows detailed status of a service
func Status(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl status SERVICE")
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

	// Print structured status
	fmt.Printf("Service: %s (%s)\n", info.Name, systemd.ServiceName(info.Name))
	fmt.Printf("Status:  %s (%s)\n", info.Status, info.SubState)

	if info.PID > 0 {
		fmt.Printf("PID:     %d\n", info.PID)
	}

	if info.Uptime > 0 {
		fmt.Printf("Uptime:  %s\n", formatUptime(int64(info.Uptime.Seconds())))
	}

	if info.MemoryBytes > 0 {
		fmt.Printf("Memory:  %s\n", formatBytes(info.MemoryBytes))
	}

	if info.CPUPercent > 0 {
		fmt.Printf("CPU:     %.1f%%\n", info.CPUPercent)
	}

	fmt.Printf("\nService File: %s\n", systemd.ServicePath(serviceName))
	fmt.Printf("Env File:     /etc/smdctl/env/%s.env\n", serviceName)

	// Show systemctl status output
	fmt.Printf("\n--- systemctl status output ---\n")
	cmd := exec.Command("systemctl", "status", systemd.ServiceName(serviceName), "--no-pager", "-l", "-n", "10")
	output, _ := cmd.CombinedOutput()
	fmt.Print(string(output))

	// Show next steps
	fmt.Printf("\n--- Next steps ---\n")
	fmt.Printf("  View full logs:       smdctl logs -f %s\n", serviceName)
	fmt.Printf("  Restart service:      smdctl restart %s\n", serviceName)
	fmt.Printf("  Edit environment:     smdctl env %s\n", serviceName)
	fmt.Printf("  Inspect config:       smdctl inspect %s\n", serviceName)

	return nil
}

func formatUptime(seconds int64) string {
	if seconds == 0 {
		return "-"
	}

	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if days > 0 {
		return fmt.Sprintf("%d days %d hours", days, hours%24)
	}
	if hours > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, minutes%60)
	}
	if minutes > 0 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

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
