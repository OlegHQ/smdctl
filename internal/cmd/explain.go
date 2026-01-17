package cmd

import (
	"fmt"
	"strings"

	"github.com/nexo-tech/smdctl/internal/systemd"
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
	// Detect whether this would run in userspace (default) or system mode.
	forceSystem := false
	for _, arg := range args {
		if arg == "--system" {
			forceSystem = true
			break
		}
	}

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

	mode := systemd.DetectMode(svc, forceSystem)

	fmt.Println("This command will perform the following steps:")
	fmt.Println()
	fmt.Printf("1. Select systemd mode (default: userspace)\n")
	fmt.Printf("   → %s mode\n", mode)
	fmt.Println()

	if mode == systemd.ModeSystem {
		fmt.Println("2. Check for sudo privileges")
		fmt.Println("   → If not root, re-execute with sudo")
		fmt.Println()
	}

	if len(envVars) > 0 {
		step := 2
		if mode == systemd.ModeSystem {
			step = 3
		}
		fmt.Printf("%d. Create environment file\n", step)
		fmt.Printf("   → %s\n", systemd.EnvFilePath(serviceName, mode))
		fmt.Println("   → Contents:")
		for _, env := range envVars {
			fmt.Printf("     %s\n", env)
		}
		fmt.Println()
	}

	stepBase := 2
	if mode == systemd.ModeSystem {
		stepBase = 3
	}
	if len(envVars) > 0 {
		stepBase++
	}

	fmt.Printf("%d. Generate systemd service file\n", stepBase)
	fmt.Printf("   → %s\n", systemd.ServicePath(serviceName, mode))
	fmt.Println()

	systemctlPrefix := "systemctl"
	journalctlPrefix := "journalctl"
	if mode == systemd.ModeUser {
		systemctlPrefix = "systemctl --user"
		journalctlPrefix = "journalctl --user"
	}

	fmt.Printf("%d. Reload systemd daemon\n", stepBase+1)
	fmt.Printf("   → %s daemon-reload\n", systemctlPrefix)
	fmt.Println()

	fmt.Printf("%d. Enable service (start at boot)\n", stepBase+2)
	fmt.Printf("   → %s enable smdctl-%s\n", systemctlPrefix, serviceName)
	fmt.Println()

	fmt.Printf("%d. Start service immediately\n", stepBase+3)
	fmt.Printf("   → %s start smdctl-%s\n", systemctlPrefix, serviceName)
	fmt.Println()

	// Generate and show service file (mode-aware)
	svc.Mode = mode

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
	fmt.Printf("  %s status smdctl-%s\n", systemctlPrefix, serviceName)
	fmt.Printf("  %s -u smdctl-%s -n 100\n", journalctlPrefix, serviceName)

	return nil
}
