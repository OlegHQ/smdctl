package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GetServiceStats retrieves CPU and memory statistics for a service
func GetServiceStats(serviceName string) (*Stats, error) {
	cgroupPath := fmt.Sprintf(
		"/sys/fs/cgroup/system.slice/%s.service",
		ServiceName(serviceName),
	)

	// Check if cgroup exists
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		return &Stats{}, nil
	}

	stats := &Stats{}

	// Read memory usage
	memPath := filepath.Join(cgroupPath, "memory.current")
	if memBytes, err := os.ReadFile(memPath); err == nil {
		if mem, err := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64); err == nil {
			stats.MemoryBytes = mem
			stats.MemoryFormatted = formatBytes(mem)
		}
	}

	// CPU usage calculation would require tracking previous values
	// For now, we'll use a simplified approach or leave at 0
	stats.CPUPercent = 0.0

	return stats, nil
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

// FormatUptime formats a duration into human-readable uptime
func FormatUptime(d int64) string {
	if d == 0 {
		return "-"
	}

	seconds := d
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
		return fmt.Sprintf("%dm %ds", minutes, seconds%60)
	}
	return fmt.Sprintf("%ds", seconds)
}
