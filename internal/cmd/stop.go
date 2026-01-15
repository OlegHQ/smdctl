package cmd

import (
	"fmt"

	"github.com/nexo-tech/smdctl/internal/sudo"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Stop stops one or more services
func Stop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl stop SERVICE [SERVICE...]")
	}

	// Discover mode for first service to determine sudo needs
	mode, err := systemd.DiscoverServiceMode(args[0])
	if err != nil {
		return err
	}

	// Check for sudo only if system mode
	if mode == systemd.ModeSystem && sudo.NeedsSudoForSystem() {
		return sudo.ReExecWithSudo()
	}

	mgr := systemd.NewManagerWithMode(mode)

	for _, name := range args {
		// Update mode for each service (in case of mixed modes)
		currentMode, err := systemd.DiscoverServiceMode(name)
		if err != nil {
			return err
		}
		mgr.SetMode(currentMode)

		fmt.Printf("Stopping service %s...\n", name)

		if err := mgr.Stop(name); err != nil {
			fmt.Printf("Error: Failed to stop %s: %v\n", name, err)
			return err
		}

		fmt.Printf("Service %s stopped successfully.\n", name)
	}

	return nil
}
