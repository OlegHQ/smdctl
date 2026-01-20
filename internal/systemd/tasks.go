package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Task represents a scheduled oneshot job associated with a service.
type Task struct {
	Name        string
	Description string
	Command     string
	Args        []string
	WorkDir     string
	Environment map[string]string
	Schedule    TaskSchedule
}

type TaskSchedule struct {
	OnCalendar        string
	OnBootSec         string
	OnStartupSec      string
	OnUnitActiveSec   string
	OnUnitInactiveSec string

	Persistent         bool
	RandomizedDelaySec string
	AccuracySec        string
}

func TaskUnitBase(serviceName, taskName string) string {
	return fmt.Sprintf("%s%s-task-%s", servicePrefix, serviceName, taskName)
}

func TaskServiceUnit(serviceName, taskName string) string {
	return TaskUnitBase(serviceName, taskName) + ".service"
}

func TaskTimerUnit(serviceName, taskName string) string {
	return TaskUnitBase(serviceName, taskName) + ".timer"
}

func TaskEnvFileName(serviceName, taskName string) string {
	return fmt.Sprintf("%s-task-%s.env", serviceName, taskName)
}

func TaskEnvFilePath(serviceName, taskName string, mode SystemdMode) string {
	if mode == ModeSystem {
		return fmt.Sprintf("/etc/smdctl/env/%s", TaskEnvFileName(serviceName, taskName))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("/etc/smdctl/env/%s", TaskEnvFileName(serviceName, taskName))
	}
	return fmt.Sprintf("%s/.config/smdctl/env/%s", homeDir, TaskEnvFileName(serviceName, taskName))
}

func TaskLogFilePath(serviceName, taskName string, mode SystemdMode) string {
	if mode == ModeSystem {
		return "" // system timers use journal
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config/smdctl/logs", fmt.Sprintf("%s-task-%s.log", serviceName, taskName))
}

func TaskServicePath(serviceName, taskName string, mode SystemdMode) string {
	return unitPath(TaskServiceUnit(serviceName, taskName), mode)
}

func TaskTimerPath(serviceName, taskName string, mode SystemdMode) string {
	return unitPath(TaskTimerUnit(serviceName, taskName), mode)
}

func unitPath(unitFile string, mode SystemdMode) string {
	if mode == ModeSystem {
		return fmt.Sprintf("/etc/systemd/system/%s", unitFile)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("/etc/systemd/system/%s", unitFile)
	}
	return fmt.Sprintf("%s/.config/systemd/user/%s", homeDir, unitFile)
}

func GenerateTaskServiceFile(parent *Service, task *Task) string {
	var sb strings.Builder

	sb.WriteString("[Unit]\n")
	if task.Description != "" {
		sb.WriteString(fmt.Sprintf("Description=%s\n", task.Description))
	} else {
		sb.WriteString(fmt.Sprintf("Description=smdctl task: %s/%s\n", parent.Name, task.Name))
	}

	// Match parent dependency defaults
	if len(parent.After) > 0 {
		sb.WriteString(fmt.Sprintf("After=%s\n", strings.Join(parent.After, " ")))
	} else {
		if parent.Mode == ModeUser {
			sb.WriteString("After=default.target\n")
		} else {
			sb.WriteString("After=network-online.target\n")
		}
	}
	if len(parent.Wants) > 0 {
		sb.WriteString(fmt.Sprintf("Wants=%s\n", strings.Join(parent.Wants, " ")))
	}

	sb.WriteString("\n")

	sb.WriteString("[Service]\n")
	sb.WriteString("Type=oneshot\n")

	command := task.Command
	if !filepath.IsAbs(command) && task.WorkDir != "" {
		command = filepath.Join(task.WorkDir, command)
	}
	execStart := command
	if len(task.Args) > 0 {
		execStart = fmt.Sprintf("%s %s", command, strings.Join(task.Args, " "))
	}
	sb.WriteString(fmt.Sprintf("ExecStart=%s\n", execStart))

	if task.WorkDir != "" {
		sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", task.WorkDir))
	}

	if parent.Mode == ModeSystem && parent.User != "" {
		sb.WriteString(fmt.Sprintf("User=%s\n", parent.User))
	}

	sb.WriteString(fmt.Sprintf("EnvironmentFile=-%s\n", TaskEnvFilePath(parent.Name, task.Name, parent.Mode)))

	if parent.Mode == ModeUser {
		logFile := TaskLogFilePath(parent.Name, task.Name, parent.Mode)
		if logFile != "" {
			sb.WriteString(fmt.Sprintf("StandardOutput=append:%s\n", logFile))
			sb.WriteString(fmt.Sprintf("StandardError=append:%s\n", logFile))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

func GenerateTaskTimerFile(serviceName, taskName string, schedule TaskSchedule) string {
	var sb strings.Builder

	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=smdctl timer: %s/%s\n\n", serviceName, taskName))

	sb.WriteString("[Timer]\n")
	if schedule.OnCalendar != "" {
		sb.WriteString(fmt.Sprintf("OnCalendar=%s\n", schedule.OnCalendar))
	}
	if schedule.OnBootSec != "" {
		sb.WriteString(fmt.Sprintf("OnBootSec=%s\n", schedule.OnBootSec))
	}
	if schedule.OnStartupSec != "" {
		sb.WriteString(fmt.Sprintf("OnStartupSec=%s\n", schedule.OnStartupSec))
	}
	if schedule.OnUnitActiveSec != "" {
		sb.WriteString(fmt.Sprintf("OnUnitActiveSec=%s\n", schedule.OnUnitActiveSec))
	}
	if schedule.OnUnitInactiveSec != "" {
		sb.WriteString(fmt.Sprintf("OnUnitInactiveSec=%s\n", schedule.OnUnitInactiveSec))
	}

	sb.WriteString(fmt.Sprintf("Unit=%s\n", TaskServiceUnit(serviceName, taskName)))
	sb.WriteString(fmt.Sprintf("Persistent=%t\n", schedule.Persistent))
	if schedule.RandomizedDelaySec != "" {
		sb.WriteString(fmt.Sprintf("RandomizedDelaySec=%s\n", schedule.RandomizedDelaySec))
	}
	if schedule.AccuracySec != "" {
		sb.WriteString(fmt.Sprintf("AccuracySec=%s\n", schedule.AccuracySec))
	}

	sb.WriteString("\n")

	sb.WriteString("[Install]\n")
	sb.WriteString("WantedBy=timers.target\n")

	return sb.String()
}
