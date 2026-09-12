package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TaskSchedule struct {
	OnCalendar         string `json:"on_calendar" yaml:"on_calendar"`
	OnBootSec          string `json:"on_boot_sec" yaml:"on_boot_sec"`
	OnStartupSec       string `json:"on_startup_sec" yaml:"on_startup_sec"`
	OnUnitActiveSec    string `json:"on_unit_active_sec" yaml:"on_unit_active_sec"`
	OnUnitInactiveSec  string `json:"on_unit_inactive_sec" yaml:"on_unit_inactive_sec"`
	Persistent         bool   `json:"persistent" yaml:"persistent"`
	RandomizedDelaySec string `json:"randomized_delay_sec" yaml:"randomized_delay_sec"`
	AccuracySec        string `json:"accuracy_sec" yaml:"accuracy_sec"`
}
type Task struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Command     string            `json:"command" yaml:"command"`
	Args        []string          `json:"args" yaml:"args"`
	Workdir     string            `json:"workdir" yaml:"workdir"`
	Environment map[string]string `json:"environment" yaml:"environment"`
	Schedule    TaskSchedule      `json:"schedule" yaml:"schedule"`
}
type TaskInfo struct {
	Service   string `json:"service"`
	Task      string `json:"task"`
	Mode      Mode   `json:"mode"`
	Next      string `json:"next"`
	Last      string `json:"last"`
	TimerUnit string `json:"timer_unit"`
	Activates string `json:"activates"`
}

func TaskServiceUnit(service, task string) string {
	return fmt.Sprintf("%s%s-task-%s.service", ServicePrefix, service, task)
}
func TaskTimerUnit(service, task string) string {
	return fmt.Sprintf("%s%s-task-%s.timer", ServicePrefix, service, task)
}
func TaskEnvFileName(service, task string) string {
	return fmt.Sprintf("%s-task-%s.env", service, task)
}
func TaskEnvFilePath(service, task string, mode Mode) (string, error) {
	dir, err := ConfigDir(mode)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, TaskEnvFileName(service, task)), nil
}
func TaskLogFilePath(service, task string, mode Mode) string {
	if mode == System {
		return ""
	}
	if home, ok := os.LookupEnv("HOME"); ok {
		return filepath.Join(home, ".config/smdctl/logs", fmt.Sprintf("%s-task-%s.log", service, task))
	}
	return ""
}
func taskUnitPath(unit string, mode Mode) (string, error) {
	if mode == System {
		return filepath.Join("/etc/systemd/system", unit), nil
	}
	home, e := HomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(home, ".config/systemd/user", unit), nil
}
func TaskServicePath(service, task string, mode Mode) (string, error) {
	return taskUnitPath(TaskServiceUnit(service, task), mode)
}
func TaskTimerPath(service, task string, mode Mode) (string, error) {
	return taskUnitPath(TaskTimerUnit(service, task), mode)
}

func GenerateTaskServiceFile(parent *Service, task *Task) (string, error) {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	if task.Description != "" {
		fmt.Fprintf(&b, "Description=%s\n", task.Description)
	} else {
		fmt.Fprintf(&b, "Description=smdctl task: %s/%s\n", parent.Name, task.Name)
	}
	if len(parent.After) > 0 {
		fmt.Fprintf(&b, "After=%s\n", strings.Join(parent.After, " "))
	} else if parent.Mode == User {
		b.WriteString("After=default.target\n")
	} else {
		b.WriteString("After=network-online.target\n")
	}
	if len(parent.Wants) > 0 {
		fmt.Fprintf(&b, "Wants=%s\n", strings.Join(parent.Wants, " "))
	}
	b.WriteString("\n[Service]\nType=oneshot\n")
	command := AbsCommandPath(task.Command, task.Workdir)
	if len(task.Args) > 0 {
		command += " " + strings.Join(task.Args, " ")
	}
	fmt.Fprintf(&b, "ExecStart=%s\n", command)
	if task.Workdir != "" {
		fmt.Fprintf(&b, "WorkingDirectory=%s\n", task.Workdir)
	}
	if parent.Mode == System && parent.UserName != "" {
		fmt.Fprintf(&b, "User=%s\n", parent.UserName)
	}
	env, e := TaskEnvFilePath(parent.Name, task.Name, parent.Mode)
	if e != nil {
		return "", e
	}
	fmt.Fprintf(&b, "EnvironmentFile=-%s\n", env)
	if log := TaskLogFilePath(parent.Name, task.Name, parent.Mode); log != "" {
		fmt.Fprintf(&b, "StandardOutput=append:%s\nStandardError=append:%s\n", log, log)
	}
	b.WriteByte('\n')
	return b.String(), nil
}
func GenerateTaskTimerFile(service, task string, s TaskSchedule) string {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	fmt.Fprintf(&b, "Description=smdctl timer: %s/%s\n\n[Timer]\n", service, task)
	for _, entry := range []struct{ k, v string }{{"OnCalendar", s.OnCalendar}, {"OnBootSec", s.OnBootSec}, {"OnStartupSec", s.OnStartupSec}, {"OnUnitActiveSec", s.OnUnitActiveSec}, {"OnUnitInactiveSec", s.OnUnitInactiveSec}} {
		if entry.v != "" {
			fmt.Fprintf(&b, "%s=%s\n", entry.k, entry.v)
		}
	}
	fmt.Fprintf(&b, "Unit=%s\nPersistent=%t\n", TaskServiceUnit(service, task), s.Persistent)
	if s.RandomizedDelaySec != "" {
		fmt.Fprintf(&b, "RandomizedDelaySec=%s\n", s.RandomizedDelaySec)
	}
	if s.AccuracySec != "" {
		fmt.Fprintf(&b, "AccuracySec=%s\n", s.AccuracySec)
	}
	b.WriteString("\n[Install]\nWantedBy=timers.target\n")
	return b.String()
}

func (m *Manager) CreateTask(parent *Service, task *Task) error {
	if err := ValidateServiceName(parent.Name); err != nil {
		return err
	}
	if err := ValidateServiceName(task.Name); err != nil {
		return err
	}
	if parent.Mode != m.Mode {
		return fmt.Errorf("parent service mode %s does not match manager mode %s", modeDebug(parent.Mode), modeDebug(m.Mode))
	}
	if task.Command == "" {
		return fmt.Errorf("task command is required: %s/%s", parent.Name, task.Name)
	}
	s := task.Schedule
	if s.OnCalendar == "" && s.OnBootSec == "" && s.OnStartupSec == "" && s.OnUnitActiveSec == "" && s.OnUnitInactiveSec == "" {
		return fmt.Errorf("task schedule is required: %s/%s", parent.Name, task.Name)
	}
	sp, err := TaskServicePath(parent.Name, task.Name, parent.Mode)
	if err != nil {
		return err
	}
	tp, err := TaskTimerPath(parent.Name, task.Name, parent.Mode)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(sp), 0777); err != nil {
		return err
	}
	if parent.Mode == User {
		if d, e := LogDir(parent.Mode); e == nil {
			_ = os.MkdirAll(d, 0777)
		}
	}
	svc, err := GenerateTaskServiceFile(parent, task)
	if err != nil {
		return err
	}
	if err = os.WriteFile(sp, []byte(svc), 0666); err != nil {
		return err
	}
	timer := GenerateTaskTimerFile(parent.Name, task.Name, task.Schedule)
	if err = os.WriteFile(tp, []byte(timer), 0666); err != nil {
		return err
	}
	p, err := TaskEnvFilePath(parent.Name, task.Name, parent.Mode)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0777); err != nil {
		return err
	}
	var env strings.Builder
	fmt.Fprintf(&env, "# Environment variables for smdctl: %s/%s\n# Edit this file with your editor\n\n", parent.Name, task.Name)
	for k, v := range task.Environment {
		fmt.Fprintf(&env, "%s=%s\n", k, v)
	}
	if err = os.WriteFile(p, []byte(env.String()), 0666); err != nil {
		return err
	}
	if err = m.DaemonReload(); err != nil {
		return err
	}
	unit := filepath.Base(tp)
	if err = m.enableUnit(unit); err != nil {
		return err
	}
	return m.startUnit(unit)
}
