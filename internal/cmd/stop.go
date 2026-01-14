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

	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	mgr := systemd.NewManager()

	for _, name := range args {
		fmt.Printf("Stopping service %s...\n", name)

		if err := mgr.Stop(name); err != nil {
			fmt.Printf("Error: Failed to stop %s: %v\n", name, err)
			return err
		}

		fmt.Printf("Service %s stopped successfully.\n", name)
	}

	return nil
}
