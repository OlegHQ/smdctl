package systemd

import "time"

// SystemdMode represents the mode of systemd operation
type SystemdMode int

const (
	// ModeUser represents userspace systemd (--user)
	ModeUser SystemdMode = iota
	// ModeSystem represents system systemd (requires root)
	ModeSystem
)

// String returns the string representation of the mode
func (m SystemdMode) String() string {
	switch m {
	case ModeUser:
		return "user"
	case ModeSystem:
		return "system"
	default:
		return "unknown"
	}
}

// Service represents a systemd service configuration
type Service struct {
	Mode        SystemdMode
	Name        string
	Description string
	Command     string
	Args        []string
	WorkDir     string
	User        string
	Environment map[string]string
	Restart     string

	// Extended systemd options
	TimeoutStart    int
	TimeoutStop     int
	KillMode        string
	After           []string
	Wants           []string
	PrivateTmp      bool
	ProtectSystem   string
	NoNewPrivileges bool
	LimitNOFILE     int
	TasksMax        int
}

// ServiceInfo represents runtime information about a service
type ServiceInfo struct {
	Name        string
	PID         int
	Status      string
	SubState    string
	Uptime      time.Duration
	Description string
	MemoryBytes uint64
	Ports       []int // Listening ports (user services only)
	Mode        SystemdMode
}

// Stats represents resource usage statistics
type Stats struct {
	MemoryBytes     uint64
	MemoryFormatted string
}
