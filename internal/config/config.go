package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/OlegHQ/smdctl/internal/systemd"
	"gopkg.in/yaml.v3"
)

type Service struct {
	Name            string            `yaml:"name" json:"name"`
	Description     string            `yaml:"description" json:"description"`
	Command         string            `yaml:"command" json:"command"`
	Args            []string          `yaml:"args" json:"args"`
	Workdir         string            `yaml:"workdir" json:"workdir"`
	User            string            `yaml:"user" json:"user"`
	Environment     map[string]string `yaml:"environment" json:"environment"`
	Restart         string            `yaml:"restart" json:"restart"`
	SystemMode      bool              `yaml:"system_mode" json:"system_mode"`
	TimeoutStart    int               `yaml:"timeout_start" json:"timeout_start"`
	TimeoutStop     int               `yaml:"timeout_stop" json:"timeout_stop"`
	KillMode        string            `yaml:"kill_mode" json:"kill_mode"`
	After           []string          `yaml:"after" json:"after"`
	Wants           []string          `yaml:"wants" json:"wants"`
	PrivateTmp      bool              `yaml:"private_tmp" json:"private_tmp"`
	ProtectSystem   string            `yaml:"protect_system" json:"protect_system"`
	NoNewPrivileges bool              `yaml:"no_new_privileges" json:"no_new_privileges"`
	LimitNofile     int               `yaml:"limit_nofile" json:"limit_nofile"`
	TasksMax        int               `yaml:"tasks_max" json:"tasks_max"`
	Tasks           []Task            `yaml:"tasks" json:"tasks"`
}
type Task struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args" json:"args"`
	Workdir     string            `yaml:"workdir" json:"workdir"`
	Environment map[string]string `yaml:"environment" json:"environment"`
	Schedule    Schedule          `yaml:"schedule" json:"schedule"`
}
type Schedule struct {
	OnCalendar         string `yaml:"on_calendar" json:"on_calendar"`
	OnBootSec          string `yaml:"on_boot_sec" json:"on_boot_sec"`
	OnStartupSec       string `yaml:"on_startup_sec" json:"on_startup_sec"`
	OnUnitActiveSec    string `yaml:"on_unit_active_sec" json:"on_unit_active_sec"`
	OnUnitInactiveSec  string `yaml:"on_unit_inactive_sec" json:"on_unit_inactive_sec"`
	Persistent         *bool  `yaml:"persistent" json:"persistent"`
	RandomizedDelaySec string `yaml:"randomized_delay_sec" json:"randomized_delay_sec"`
	AccuracySec        string `yaml:"accuracy_sec" json:"accuracy_sec"`
}

func Load(path string) (*Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, err
	}
	ExpandEnvVars(cfg)
	return cfg, nil
}
func Parse(data []byte) (*Service, error) {
	var required struct {
		Name    *string `yaml:"name"`
		Command *string `yaml:"command"`
	}
	if err := yaml.Unmarshal(data, &required); err != nil {
		return nil, err
	}
	if required.Name == nil {
		return nil, fmt.Errorf("missing field `name`")
	}
	if required.Command == nil {
		return nil, fmt.Errorf("missing field `command`")
	}
	var cfg Service
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Restart == "" {
		cfg.Restart = "always"
	}
	if cfg.TimeoutStart == 0 {
		cfg.TimeoutStart = 90
	}
	if cfg.TimeoutStop == 0 {
		cfg.TimeoutStop = 30
	}
	if cfg.KillMode == "" {
		cfg.KillMode = "control-group"
	}
	if cfg.Workdir == "" {
		cfg.Workdir = "/"
	}
	if cfg.Environment == nil {
		cfg.Environment = map[string]string{}
	}
	for i := range cfg.Tasks {
		t := &cfg.Tasks[i]
		if t.Workdir == "" {
			t.Workdir = cfg.Workdir
		}
		if t.Environment == nil {
			t.Environment = map[string]string{}
		}
		for k, v := range cfg.Environment {
			if _, exists := t.Environment[k]; !exists {
				t.Environment[k] = v
			}
		}
		if t.Schedule.Persistent == nil {
			v := true
			t.Schedule.Persistent = &v
		}
	}
	return &cfg, nil
}
func FindConfigFile() (string, error) {
	for _, p := range []string{"smdctl.yml", "smdctl.yaml"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("config file not found (looked for: smdctl.yml, smdctl.yaml)")
}

var envPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

func expand(s string, missing *[]string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		m := envPattern.FindStringSubmatch(match)
		key := m[1]
		if key == "" {
			key = m[2]
		}
		v, ok := os.LookupEnv(key)
		if !ok {
			*missing = append(*missing, key)
			return ""
		}
		return v
	})
}
func ExpandEnvVars(c *Service) {
	missing := make([]string, 0)
	replace := func(v *string) { *v = expand(*v, &missing) }
	replace(&c.Name)
	replace(&c.Description)
	replace(&c.Command)
	replace(&c.Workdir)
	replace(&c.User)
	replace(&c.Restart)
	replace(&c.KillMode)
	replace(&c.ProtectSystem)
	for i := range c.Args {
		replace(&c.Args[i])
	}
	for i := range c.After {
		replace(&c.After[i])
	}
	for i := range c.Wants {
		replace(&c.Wants[i])
	}
	for k, v := range c.Environment {
		c.Environment[k] = expand(v, &missing)
	}
	for i := range c.Tasks {
		t := &c.Tasks[i]
		replace(&t.Name)
		replace(&t.Description)
		replace(&t.Command)
		replace(&t.Workdir)
		for j := range t.Args {
			replace(&t.Args[j])
		}
		for k, v := range t.Environment {
			t.Environment[k] = expand(v, &missing)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Printf("Warning: environment variables not set (using empty value): %s\n", strings.Join(missing, ", "))
	}
}
func ToSystemd(c *Service) *systemd.Service {
	return &systemd.Service{Mode: systemd.User, Name: c.Name, Description: c.Description, Command: c.Command, Args: c.Args, Workdir: c.Workdir, UserName: c.User, Environment: c.Environment, Restart: c.Restart, TimeoutStart: c.TimeoutStart, TimeoutStop: c.TimeoutStop, KillMode: c.KillMode, After: c.After, Wants: c.Wants, PrivateTmp: c.PrivateTmp, ProtectSystem: c.ProtectSystem, NoNewPrivileges: c.NoNewPrivileges, LimitNofile: c.LimitNofile, TasksMax: c.TasksMax}
}
func ToSystemdTask(t *Task) *systemd.Task {
	persistent := true
	if t.Schedule.Persistent != nil {
		persistent = *t.Schedule.Persistent
	}
	return &systemd.Task{Name: t.Name, Description: t.Description, Command: t.Command, Args: t.Args, Workdir: t.Workdir, Environment: t.Environment, Schedule: systemd.TaskSchedule{OnCalendar: t.Schedule.OnCalendar, OnBootSec: t.Schedule.OnBootSec, OnStartupSec: t.Schedule.OnStartupSec, OnUnitActiveSec: t.Schedule.OnUnitActiveSec, OnUnitInactiveSec: t.Schedule.OnUnitInactiveSec, Persistent: persistent, RandomizedDelaySec: t.Schedule.RandomizedDelaySec, AccuracySec: t.Schedule.AccuracySec}}
}
func ResolvePath(path string) string { return filepath.Clean(path) }
