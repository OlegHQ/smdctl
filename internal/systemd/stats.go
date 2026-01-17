package systemd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

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

// pidPattern matches pid=N in ss output (e.g., pid=12345)
var pidPattern = regexp.MustCompile(`pid=(\d+)`)

// GetListeningPorts returns listening TCP/UDP ports for a given PID.
// It uses `ss` to find sockets owned by the process.
func GetListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}

	pidStr := strconv.Itoa(pid)
	var ports []int
	seen := make(map[int]bool)

	// Run ss for TCP listening sockets
	tcpOut, _ := exec.Command("ss", "-H", "-lntp").Output()
	ports = appendPortsForPID(ports, seen, string(tcpOut), pidStr)

	// Run ss for UDP listening sockets
	udpOut, _ := exec.Command("ss", "-H", "-lnup").Output()
	ports = appendPortsForPID(ports, seen, string(udpOut), pidStr)

	return ports
}

// appendPortsForPID parses ss output lines and extracts ports owned by pidStr.
// ss output example:
// LISTEN  0  128  0.0.0.0:8080  0.0.0.0:*  users:(("myapp",pid=12345,fd=3))
func appendPortsForPID(ports []int, seen map[int]bool, output, pidStr string) []int {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if line contains our PID
		matches := pidPattern.FindAllStringSubmatch(line, -1)
		ownerMatch := false
		for _, m := range matches {
			if len(m) > 1 && m[1] == pidStr {
				ownerMatch = true
				break
			}
		}
		if !ownerMatch {
			continue
		}

		// Extract local address:port (4th field typically)
		// Format: State Recv-Q Send-Q Local-Address:Port Peer-Address:Port ...
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		localAddr := fields[3]
		// Handle IPv6 bracket notation [::]:port or plain addr:port
		port := extractPort(localAddr)
		if port > 0 && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports
}

// extractPort extracts port number from address string like "0.0.0.0:8080" or "[::]:8080"
func extractPort(addr string) int {
	// Handle IPv6 [::]:port
	if idx := strings.LastIndex(addr, "]:"); idx != -1 {
		portStr := addr[idx+2:]
		if p, err := strconv.Atoi(portStr); err == nil {
			return p
		}
		return 0
	}

	// Handle IPv4 or hostname:port
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		portStr := addr[idx+1:]
		if p, err := strconv.Atoi(portStr); err == nil {
			return p
		}
	}
	return 0
}
