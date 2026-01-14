package cmd

import (
	"flag"
	"fmt"

	"github.com/nexo-tech/smdctl/internal/output"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// PS lists all smdctl-managed services
func PS(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ExitOnError)
	all := fs.Bool("a", false, "Show all services (default: only running)")
	fs.BoolVar(all, "all", false, "Show all services (default: only running)")
	quiet := fs.Bool("q", false, "Only show service names")
	fs.BoolVar(quiet, "quiet", false, "Only show service names")

	if err := fs.Parse(args); err != nil {
		return err
	}

	mgr := systemd.NewManager()

	services, err := mgr.ListServices(*all)
	if err != nil {
		return fmt.Errorf("list services: %w", err)
	}

	if *quiet {
		output.FormatServicesQuiet(services)
	} else {
		output.FormatServicesTable(services)
	}

	return nil
}
