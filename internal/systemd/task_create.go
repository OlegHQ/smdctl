package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) CreateTask(parent *Service, task *Task) error {
	if err := ValidateServiceName(parent.Name); err != nil {
		return err
	}
	if err := ValidateServiceName(task.Name); err != nil {
		return err
	}
	if task.Command == "" {
		return fmt.Errorf("task command is required: %s/%s", parent.Name, task.Name)
	}
	if task.Schedule.OnCalendar == "" && task.Schedule.OnBootSec == "" && task.Schedule.OnStartupSec == "" && task.Schedule.OnUnitActiveSec == "" && task.Schedule.OnUnitInactiveSec == "" {
		return fmt.Errorf("task schedule is required: %s/%s", parent.Name, task.Name)
	}

	// Update manager mode to match service
	m.SetMode(parent.Mode)

	servicePath := TaskServicePath(parent.Name, task.Name, parent.Mode)
	timerPath := TaskTimerPath(parent.Name, task.Name, parent.Mode)

	// Ensure unit directory exists
	if err := os.MkdirAll(filepath.Dir(servicePath), 0755); err != nil {
		return fmt.Errorf("create unit directory: %w", err)
	}

	// Ensure env/log directories exist
	if err := os.MkdirAll(filepath.Dir(TaskEnvFilePath(parent.Name, task.Name, parent.Mode)), 0755); err != nil {
		return fmt.Errorf("create env directory: %w", err)
	}
	if parent.Mode == ModeUser {
		logDir, err := GetLogDir(parent.Mode)
		if err == nil {
			_ = os.MkdirAll(logDir, 0755)
		}
	}

	// Write oneshot unit
	oneShot := GenerateTaskServiceFile(parent, task)
	if err := os.WriteFile(servicePath, []byte(oneShot), 0644); err != nil {
		return fmt.Errorf("write task service file: %w", err)
	}

	// Write timer unit
	timer := GenerateTaskTimerFile(parent.Name, task.Name, task.Schedule)
	if err := os.WriteFile(timerPath, []byte(timer), 0644); err != nil {
		return fmt.Errorf("write task timer file: %w", err)
	}

	// Write env file (even if empty, so overrides are consistent)
	if err := m.writeEnvFile(TaskEnvFilePath(parent.Name, task.Name, parent.Mode), fmt.Sprintf("%s/%s", parent.Name, task.Name), task.Environment); err != nil {
		return fmt.Errorf("write task env file: %w", err)
	}

	if err := m.DaemonReload(); err != nil {
		return fmt.Errorf("daemon reload: %w", err)
	}

	// Enable + start timer
	timerUnit := TaskTimerUnit(parent.Name, task.Name)
	if err := m.EnableUnit(timerUnit); err != nil {
		return fmt.Errorf("enable task timer: %w", err)
	}
	if err := m.StartUnit(timerUnit); err != nil {
		return fmt.Errorf("start task timer: %w", err)
	}

	return nil
}

func (m *Manager) writeEnvFile(path string, label string, env map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	var content strings.Builder
	content.WriteString("# Environment variables for smdctl: " + label + "\n")
	content.WriteString("# Edit this file with your editor\n\n")

	if env != nil {
		for key, value := range env {
			content.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	return os.WriteFile(path, []byte(content.String()), 0644)
}
