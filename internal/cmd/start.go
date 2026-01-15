package cmd

import (
	"fmt"

	"github.com/nexo-tech/smdctl/internal/sudo"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Start starts one or more services
func Start(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl start SERVICE [SERVICE...]")
	}

	// Discover mode for first service to determine sudo needs
	// (In practice, users typically manage services of one mode at a time)
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

		fmt.Printf("Starting service %s...\n", name)

		if err := mgr.Start(name); err != nil {
			fmt.Printf("Error: Failed to start %s: %v\n", name, err)
			fmt.Printf("\nTroubleshooting:\n")
			fmt.Printf("  - Check status: smdctl status %s\n", name)
			fmt.Printf("  - View logs: smdctl logs -n 50 %s\n", name)
			return err
		}

		fmt.Printf("Service %s started successfully.\n", name)
	}

	if len(args) == 1 {
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  Check status: smdctl status %s\n", args[0])
		fmt.Printf("  View logs:    smdctl logs -f %s\n", args[0])
	}

	return nil
}
