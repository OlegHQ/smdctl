package systemd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) DaemonReload() error           { return m.exec().Run(m.Mode, "daemon-reload") }
func (m *Manager) startUnit(unit string) error   { return m.exec().Run(m.Mode, "start", unit) }
func (m *Manager) stopUnit(unit string) error    { return m.exec().Run(m.Mode, "stop", unit) }
func (m *Manager) enableUnit(unit string) error  { return m.exec().Run(m.Mode, "enable", unit) }
func (m *Manager) disableUnit(unit string) error { return m.exec().Run(m.Mode, "disable", unit) }

func (m *Manager) Create(svc *Service) error {
	if err := ValidateServiceName(svc.Name); err != nil {
		return err
	}
	if svc.Mode != m.Mode {
		return fmt.Errorf("service mode %s does not match manager mode %s", modeDebug(svc.Mode), modeDebug(m.Mode))
	}
	if m.ServiceExists(svc.Name) {
		return fmt.Errorf("service already exists: %s", svc.Name)
	}
	content, err := GenerateServiceFile(svc)
	if err != nil {
		return err
	}
	path, err := ServicePath(svc.Name, svc.Mode)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}
	if svc.Mode == User {
		if dir, err := LogDir(svc.Mode); err == nil {
			_ = os.MkdirAll(dir, 0777)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0666); err != nil {
		return err
	}
	if len(svc.Environment) > 0 {
		if err := m.CreateEnvFile(svc.Name, svc.Environment); err != nil {
			return fmt.Errorf("create environment file: %w", err)
		}
	}
	return m.DaemonReload()
}
func (m *Manager) CreateEnvFile(name string, env map[string]string) error {
	dir, err := ConfigDir(m.Mode)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Environment variables for smdctl service: %s\n# Edit this file with: smdctl env %s\n\n", name, name)
	for k, v := range env {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	return os.WriteFile(filepath.Join(dir, name+".env"), []byte(b.String()), 0666)
}
func (m *Manager) Start(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	return m.startUnit(UnitName(name))
}
func (m *Manager) Stop(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	return m.stopUnit(UnitName(name))
}
func (m *Manager) Restart(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	return m.exec().Run(m.Mode, "restart", UnitName(name))
}
func (m *Manager) Enable(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	return m.enableUnit(UnitName(name))
}
func (m *Manager) Disable(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	return m.disableUnit(UnitName(name))
}
func (m *Manager) Remove(name string) error {
	if !m.ServiceExists(name) {
		return fmt.Errorf("service not found: %s", name)
	}
	_ = m.RemoveTasks(name)
	_ = m.Stop(name)
	_ = m.Disable(name)
	path, err := ServicePath(name, m.Mode)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil {
		return err
	}
	if p, e := EnvFilePath(name, m.Mode); e == nil {
		_ = os.Remove(p)
	}
	if p := LogFilePath(name, m.Mode); p != "" {
		_ = os.Remove(p)
	}
	return m.DaemonReload()
}
func (m *Manager) RemoveTasks(serviceName string) error {
	svcPath, err := ServicePath(serviceName, m.Mode)
	if err != nil {
		return err
	}
	unitDir := filepath.Dir(svcPath)
	prefix := fmt.Sprintf("%s%s-task-", ServicePrefix, serviceName)
	entries, err := os.ReadDir(unitDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".timer") {
			continue
		}
		base := strings.TrimSuffix(name, ".timer")
		taskName := strings.TrimPrefix(base, prefix)
		_ = m.stopUnit(name)
		_ = m.disableUnit(name)
		_ = os.Remove(filepath.Join(unitDir, name))
		oneShot := base + ".service"
		_ = m.stopUnit(oneShot)
		_ = os.Remove(filepath.Join(unitDir, oneShot))
		if taskName != "" {
			if p, e := TaskEnvFilePath(serviceName, taskName, m.Mode); e == nil {
				_ = os.Remove(p)
			}
			if p := TaskLogFilePath(serviceName, taskName, m.Mode); p != "" {
				_ = os.Remove(p)
			}
		}
	}
	return nil
}
