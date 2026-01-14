package cmd

import (
	"fmt"

	"github.com/nexo-tech/smdctl/internal/sudo"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Restart restarts one or more services
func Restart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl restart SERVICE [SERVICE...]")
	}

	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	mgr := systemd.NewManager()

	for _, name := range args {
		fmt.Printf("Restarting service %s...\n", name)

		if err := mgr.Restart(name); err != nil {
			fmt.Printf("Error: Failed to restart %s: %v\n", name, err)
			fmt.Printf("\nTroubleshooting:\n")
			fmt.Printf("  - Check status: smdctl status %s\n", name)
			fmt.Printf("  - View logs: smdctl logs -n 50 %s\n", name)
			return err
		}

		fmt.Printf("Service %s restarted successfully.\n", name)
	}

	if len(args) == 1 {
		fmt.Printf("\nNext steps:\n")
		fmt.Printf("  Check status: smdctl status %s\n", args[0])
		fmt.Printf("  View logs:    smdctl logs -f %s\n", args[0])
	}

	return nil
}
