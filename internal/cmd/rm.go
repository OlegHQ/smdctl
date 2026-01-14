package cmd

import (
	"flag"
	"fmt"

	"github.com/snowbear/smdctl/internal/output"
	"github.com/snowbear/smdctl/internal/sudo"
	"github.com/snowbear/smdctl/internal/systemd"
)

// Remove removes one or more services
func Remove(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ExitOnError)
	force := fs.Bool("f", false, "Don't prompt for confirmation")
	fs.BoolVar(force, "force", false, "Don't prompt for confirmation")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: smdctl rm [OPTIONS] SERVICE [SERVICE...]")
	}

	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	mgr := systemd.NewManager()

	for _, name := range fs.Args() {
		// Verify service exists
		if !mgr.ServiceExists(name) {
			fmt.Printf("Service not found: %s\n", name)
			continue
		}

		// Check if running
		isActive := mgr.IsActive(name)
		statusMsg := "stopped"
		if isActive {
			statusMsg = "running"
		}

		// Prompt for confirmation unless -f flag
		if !*force {
			message := fmt.Sprintf("Remove service '%s' (%s)? This will stop and delete the service.", name, statusMsg)
			if !output.PromptYesNo(message, false) {
				fmt.Printf("Skipping %s\n", name)
				continue
			}
		}

		fmt.Printf("Removing service %s...\n", name)

		if err := mgr.Remove(name); err != nil {
			fmt.Printf("Error: Failed to remove %s: %v\n", name, err)
			return err
		}

		fmt.Printf("Service %s removed successfully.\n", name)
	}

	return nil
}
