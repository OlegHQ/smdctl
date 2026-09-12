package systemd

import (
	"errors"
	"os"
	"regexp"
)

type Manager struct {
	Mode Mode
	Exec Executor
}

func NewManager(mode Mode) *Manager { return &Manager{Mode: mode, Exec: RealExecutor{}} }
func (m *Manager) exec() Executor {
	if m.Exec == nil {
		return RealExecutor{}
	}
	return m.Exec
}

var serviceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func ValidateServiceName(name string) error {
	if name == "" {
		return errors.New("service name cannot be empty")
	}
	if !serviceNamePattern.MatchString(name) {
		return errors.New("invalid service name: must contain only alphanumeric characters, hyphens, and underscores")
	}
	return nil
}

func (m *Manager) ServiceExists(name string) bool {
	for _, mode := range []Mode{m.Mode, otherMode(m.Mode)} {
		if p, err := ServicePath(name, mode); err == nil {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		}
	}
	return false
}
func otherMode(mode Mode) Mode {
	if mode == User {
		return System
	}
	return User
}

func modeDebug(mode Mode) string {
	if mode == User {
		return "User"
	}
	return "System"
}
