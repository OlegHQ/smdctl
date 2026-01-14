package cmd

import (
	"fmt"

	"github.com/snowbear/smdctl/internal/env"
	"github.com/snowbear/smdctl/internal/sudo"
	"github.com/snowbear/smdctl/internal/systemd"
)

// Env edits the environment file for a service
func Env(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl env SERVICE")
	}

	serviceName := args[0]

	// Verify service exists
	mgr := systemd.NewManager()
	if !mgr.ServiceExists(serviceName) {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	return env.Edit(serviceName)
}
