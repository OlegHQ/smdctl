package cmd

import (
	"fmt"
	"strings"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Migrate provides guidance on migrating from system to user mode
func Migrate(args []string) error {
	fmt.Println("Migration from system to user mode")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// List system services
	systemMgr := systemd.NewManagerWithMode(systemd.ModeSystem)
	services, err := systemMgr.ListServices(true)
	if err != nil || len(services) == 0 {
		fmt.Println("No system services found to migrate.")
		return nil
	}

	fmt.Printf("Found %d service(s) running in system mode:\n\n", len(services))
	for _, svc := range services {
		fmt.Printf("  - %s\n", svc.Name)
	}

	fmt.Println()
	fmt.Println("Migration Process:")
	fmt.Println("1. For each service, review if it needs privileged ports (< 1024)")
	fmt.Println("2. Services without privileged ports can be migrated to user mode")
	fmt.Println("3. Remove the system service: smdctl rm <service-name>")
	fmt.Println("4. Re-create in user mode: smdctl run ... (without --system flag)")
	fmt.Println()
	fmt.Println("Note: User mode services require lingering to persist after logout.")
	fmt.Printf("Enable lingering: sudo loginctl enable-linger $USER\n")
	fmt.Println()
	fmt.Println("For services requiring privileged ports, they must remain in system mode.")

	return nil
}
