package cmd

import (
	"fmt"
	"strings"

	"github.com/snowbear/smdctl/internal/systemd"
)

// Explain shows what a command will do without executing it
func Explain(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: smdctl explain COMMAND [ARGS...]")
	}

	command := args[0]

	switch command {
	case "run":
		return explainRun(args[1:])
	default:
		return fmt.Errorf("explain is currently only supported for the 'run' command")
	}
}

func explainRun(args []string) error {
	// Parse the run command arguments (simplified parsing for explanation)
	// This is a dry-run, so we'll be lenient with parsing

	if len(args) == 0 {
		fmt.Println("EXPLANATION: smdctl run")
		fmt.Println("\nThis command creates and starts a new systemd service.")
		fmt.Println("You must provide either:")
		fmt.Println("  1. A YAML config file with -f flag")
		fmt.Println("  2. Service name and command: smdctl run NAME -- COMMAND [ARGS...]")
		return nil
	}

	// Find the service name and command
	var serviceName string
	var command string
	var envVars []string

	// Find "--" separator and collect env vars
	var nonFlagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--" {
			// Everything after "--" is the command
			if i+1 < len(args) {
				command = strings.Join(args[i+1:], " ")
			}
			break
		} else if arg == "-e" && i+1 < len(args) {
			// Collect env var
			envVars = append(envVars, args[i+1])
			i++ // Skip next arg (the value)
		} else if !strings.HasPrefix(arg, "-") {
			// Non-flag argument
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}

	// Service name is the last non-flag arg before "--"
	if len(nonFlagArgs) > 0 {
		serviceName = nonFlagArgs[len(nonFlagArgs)-1]
	}

	if serviceName == "" {
		serviceName = "myapp"
	}
	if command == "" {
		command = "/usr/bin/mycommand"
	}

	fmt.Printf("EXPLANATION: smdctl run %s\n\n", strings.Join(args, " "))
	fmt.Println("This command will perform the following steps:")
	fmt.Println()
	fmt.Println("1. Check for sudo privileges")
	fmt.Println("   → If not root, re-execute with sudo")
	fmt.Println()

	if len(envVars) > 0 {
		fmt.Println("2. Create environment file")
		fmt.Printf("   → /etc/smdctl/env/%s.env\n", serviceName)
		fmt.Println("   → Contents:")
		for _, env := range envVars {
			fmt.Printf("     %s\n", env)
		}
		fmt.Println()
	}

	fmt.Println("3. Generate systemd service file")
	fmt.Printf("   → /etc/systemd/system/smdctl-%s.service\n", serviceName)
	fmt.Println()

	fmt.Println("4. Reload systemd daemon")
	fmt.Println("   → systemctl daemon-reload")
	fmt.Println()

	fmt.Println("5. Enable service (start at boot)")
	fmt.Printf("   → systemctl enable smdctl-%s\n", serviceName)
	fmt.Println()

	fmt.Println("6. Start service immediately")
	fmt.Printf("   → systemctl start smdctl-%s\n", serviceName)
	fmt.Println()

	// Generate and show service file
	svc := &systemd.Service{
		Name:         serviceName,
		Description:  fmt.Sprintf("smdctl managed service: %s", serviceName),
		Command:      command,
		Restart:      "always",
		Environment:  make(map[string]string),
		WorkDir:      "/",
		TimeoutStart: 90,
		TimeoutStop:  30,
		KillMode:     "control-group",
	}

	for _, env := range envVars {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			svc.Environment[parts[0]] = parts[1]
		}
	}

	serviceFile := systemd.GenerateServiceFile(svc)

	fmt.Println("Generated service file:")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Print(serviceFile)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	fmt.Println("To execute this command:")
	fmt.Printf("  smdctl run %s\n", strings.Join(args, " "))
	fmt.Println()
	fmt.Println("To see the result:")
	fmt.Printf("  smdctl status %s\n", serviceName)

	return nil
}
