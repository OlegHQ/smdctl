package cmd

import (
	"flag"

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

	// Collect services from both user and system modes
	var allServices []*systemd.ServiceInfo

	// Get user services
	userMgr := systemd.NewManagerWithMode(systemd.ModeUser)
	userServices, err := userMgr.ListServices(*all)
	if err == nil {
		// Add mode info to each service
		for _, svc := range userServices {
			svc.Mode = systemd.ModeUser
			allServices = append(allServices, svc)
		}
	}

	// Get system services
	systemMgr := systemd.NewManagerWithMode(systemd.ModeSystem)
	systemServices, err := systemMgr.ListServices(*all)
	if err == nil {
		// Add mode info to each service
		for _, svc := range systemServices {
			svc.Mode = systemd.ModeSystem
			allServices = append(allServices, svc)
		}
	}

	if *quiet {
		output.FormatServicesQuiet(allServices)
	} else {
		output.FormatServicesTable(allServices)
	}

	return nil
}
