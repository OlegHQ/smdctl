package commands

import (
	"fmt"
	"os"
	"strings"

	envhelper "github.com/OlegHQ/smdctl/internal/env"
	"github.com/OlegHQ/smdctl/internal/output"
	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/spf13/cobra"
)

func cmdStart(names []string) error {
	return eachService(names, "Starting", "started", func(m *systemd.Manager, n string) error { return m.Start(n) }, true)
}
func cmdStop(names []string) error {
	return eachService(names, "Stopping", "stopped", func(m *systemd.Manager, n string) error { return m.Stop(n) }, false)
}
func cmdRestart(names []string) error {
	return eachService(names, "Restarting", "restarted", func(m *systemd.Manager, n string) error { return m.Restart(n) }, true)
}
func eachService(names []string, verb, past string, action func(*systemd.Manager, string) error, next bool) error {
	if len(names) > 0 {
		mode, err := systemd.DiscoverServiceMode(names[0])
		if err != nil {
			return err
		}
		if mode == systemd.System && envhelper.NeedsSudoForSystem() {
			return envhelper.ReexecWithSudo()
		}
	}
	for _, name := range names {
		mode, e := systemd.DiscoverServiceMode(name)
		if e != nil {
			return e
		}
		m := systemd.NewManager(mode)
		fmt.Printf("%s service %s...\n", verb, name)
		if e = action(m, name); e != nil {
			switch verb {
			case "Starting", "Restarting":
				operation := strings.ToLower(strings.TrimSuffix(verb, "ing"))
				fmt.Fprintf(os.Stderr, "Error: Failed to %s %s: %v\n", operation, name, e)
				fmt.Fprintln(os.Stderr, "\nTroubleshooting:")
				fmt.Fprintf(os.Stderr, "  - Check status: smdctl status %s\n", name)
				fmt.Fprintf(os.Stderr, "  - View logs: smdctl logs -n 50 %s\n", name)
				return fmt.Errorf("failed to %s %s", operation, name)
			default:
				return fmt.Errorf("Failed to stop %s: %w", name, e)
			}
		}
		fmt.Printf("Service %s %s successfully.\n", name, past)
	}
	if next && len(names) == 1 {
		fmt.Printf("\nNext steps:\n  Check status: smdctl status %s\n  View logs:    smdctl logs -f %s\n", names[0], names[0])
	}
	return nil
}

func rmCommand() *cobra.Command {
	var force bool
	c := &cobra.Command{Use: "rm [flags] SERVICE [SERVICE...]", Short: "Remove service(s)", Args: func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 1, -1, "<NAMES>...") }, RunE: func(_ *cobra.Command, args []string) error {
		mode, e := systemd.DiscoverServiceMode(args[0])
		if e != nil {
			return e
		}
		if mode == systemd.System && envhelper.NeedsSudoForSystem() {
			return envhelper.ReexecWithSudo()
		}
		for _, name := range args {
			mode, e := systemd.DiscoverServiceMode(name)
			if e != nil {
				fmt.Printf("Service not found: %s\n", name)
				continue
			}
			m := systemd.NewManager(mode)
			active := m.IsActive(name)
			state := "stopped"
			if active {
				state = "running"
			}
			if !force && !output.PromptYesNo(fmt.Sprintf("Remove service '%s' (%s)? This will stop and delete the service.", name, state), false) {
				fmt.Printf("Skipping %s\n", name)
				continue
			}
			fmt.Printf("Removing service %s...\n", name)
			if e = m.Remove(name); e != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to remove %s: %v\n", name, e)
				return fmt.Errorf("failed to remove %s", name)
			}
			fmt.Printf("Service %s removed successfully.\n", name)
		}
		return nil
	}}
	c.Flags().BoolVarP(&force, "force", "f", false, "Do not prompt")
	return c
}

func cmdEnv(args []string) error {
	name := args[0]
	mode, e := systemd.DiscoverServiceMode(name)
	if e != nil {
		return e
	}
	if mode == systemd.System && envhelper.NeedsSudoForSystem() {
		return envhelper.ReexecWithSudo()
	}
	path, e := systemd.EnvFilePath(name, mode)
	if e != nil {
		return e
	}
	dir, e := systemd.ConfigDir(mode)
	if e != nil {
		return e
	}
	return envhelper.EditEnvironment(name, path, dir)
}
