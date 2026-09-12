package systemd

import (
	"encoding/json"
	"time"
)

const ServicePrefix = "smdctl-"

type Mode string

const (
	User   Mode = "user"
	System Mode = "system"
)

type Service struct {
	Mode            Mode              `json:"mode" yaml:"-"`
	Name            string            `json:"name" yaml:"name"`
	Description     string            `json:"description" yaml:"description"`
	Command         string            `json:"command" yaml:"command"`
	Args            []string          `json:"args" yaml:"args"`
	Workdir         string            `json:"workdir" yaml:"workdir"`
	UserName        string            `json:"user" yaml:"user"`
	Environment     map[string]string `json:"environment" yaml:"environment"`
	Restart         string            `json:"restart" yaml:"restart"`
	TimeoutStart    int               `json:"timeout_start" yaml:"timeout_start"`
	TimeoutStop     int               `json:"timeout_stop" yaml:"timeout_stop"`
	KillMode        string            `json:"kill_mode" yaml:"kill_mode"`
	After           []string          `json:"after" yaml:"after"`
	Wants           []string          `json:"wants" yaml:"wants"`
	PrivateTmp      bool              `json:"private_tmp" yaml:"private_tmp"`
	ProtectSystem   string            `json:"protect_system" yaml:"protect_system"`
	NoNewPrivileges bool              `json:"no_new_privileges" yaml:"no_new_privileges"`
	LimitNofile     int               `json:"limit_nofile" yaml:"limit_nofile"`
	TasksMax        int               `json:"tasks_max" yaml:"tasks_max"`
}

func EmptyService() Service {
	return Service{Mode: User, Workdir: "/", Environment: map[string]string{}, Restart: "always", TimeoutStart: 90, TimeoutStop: 30, KillMode: "control-group"}
}

type ServiceInfo struct {
	Name        string        `json:"name"`
	PID         int           `json:"pid"`
	Status      string        `json:"status"`
	SubState    string        `json:"sub_state"`
	Uptime      time.Duration `json:"uptime"`
	Description string        `json:"description"`
	MemoryBytes uint64        `json:"memory_bytes"`
	Ports       []int         `json:"ports"`
	Mode        Mode          `json:"mode"`
}

func (s ServiceInfo) MarshalJSON() ([]byte, error) {
	ports := s.Ports
	if ports == nil {
		ports = []int{}
	}
	return json.Marshal(struct {
		Name     string `json:"name"`
		PID      int    `json:"pid"`
		Status   string `json:"status"`
		SubState string `json:"sub_state"`
		Uptime   struct {
			Seconds     uint64 `json:"secs"`
			Nanoseconds uint32 `json:"nanos"`
		} `json:"uptime"`
		Description string `json:"description"`
		MemoryBytes uint64 `json:"memory_bytes"`
		Ports       []int  `json:"ports"`
		Mode        Mode   `json:"mode"`
	}{s.Name, s.PID, s.Status, s.SubState, struct {
		Seconds     uint64 `json:"secs"`
		Nanoseconds uint32 `json:"nanos"`
	}{uint64(s.Uptime / time.Second), uint32(s.Uptime % time.Second)}, s.Description, s.MemoryBytes, ports, s.Mode})
}
