package systemd

import (
	"fmt"
	"os"
)

// DiscoverServiceMode finds which mode a service is running in
func DiscoverServiceMode(name string) (SystemdMode, error) {
	// Check user mode first (new default)
	userPath := ServicePath(name, ModeUser)
	if _, err := os.Stat(userPath); err == nil {
		return ModeUser, nil
	}

	// Check system mode
	systemPath := ServicePath(name, ModeSystem)
	if _, err := os.Stat(systemPath); err == nil {
		return ModeSystem, nil
	}

	return 0, fmt.Errorf("service not found: %s", name)
}
