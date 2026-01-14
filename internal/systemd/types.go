package systemd

import "time"

// Service represents a systemd service configuration
type Service struct {
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
	CPUPercent  float64
	MemoryBytes uint64
}

// Stats represents resource usage statistics
type Stats struct {
	CPUPercent      float64
	MemoryBytes     uint64
	MemoryFormatted string
}
