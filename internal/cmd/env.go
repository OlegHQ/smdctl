package cmd

import (
	"fmt"

	"github.com/nexo-tech/smdctl/internal/env"
	"github.com/nexo-tech/smdctl/internal/sudo"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Env edits the environment file for a service
func Env(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl env SERVICE")
	}

	serviceName := args[0]

	// Discover mode
	mode, err := systemd.DiscoverServiceMode(serviceName)
	if err != nil {
		return err
	}

	// Verify service exists
	mgr := systemd.NewManagerWithMode(mode)
	if !mgr.ServiceExists(serviceName) {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	// Check for sudo only if system mode
	if mode == systemd.ModeSystem && sudo.NeedsSudoForSystem() {
		return sudo.ReExecWithSudo()
	}

	return env.Edit(serviceName, mode)
}
