package cmd

import (
	"fmt"

	"github.com/snowbear/smdctl/internal/sudo"
	"github.com/snowbear/smdctl/internal/systemd"
)

// Start starts one or more services
func Start(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl start SERVICE [SERVICE...]")
	}

	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	mgr := systemd.NewManager()

	for _, name := range args {
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
