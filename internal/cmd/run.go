package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/snowbear/smdctl/internal/config"
	"github.com/snowbear/smdctl/internal/output"
	"github.com/snowbear/smdctl/internal/sudo"
	"github.com/snowbear/smdctl/internal/systemd"
)

// Run creates and starts a new systemd service
func Run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	// File-based config
	configFile := fs.String("f", "", "YAML config file")
	fs.StringVar(configFile, "file", "", "YAML config file")

	// Service options
	var envVars []string
	fs.Func("e", "Environment variable (repeatable)", func(s string) error {
		envVars = append(envVars, s)
		return nil
	})
	fs.Func("env", "Environment variable (repeatable)", func(s string) error {
		envVars = append(envVars, s)
		return nil
	})

	restart := fs.String("restart", "always", "Restart policy: no|on-failure|always")
	user := fs.String("user", "", "Run as specific user")
	workdir := fs.String("workdir", "/", "Working directory")
	description := fs.String("description", "", "Service description")
	timeoutStart := fs.Int("timeout-start", 90, "Startup timeout in seconds")
	timeoutStop := fs.Int("timeout-stop", 30, "Stop timeout in seconds")
	killMode := fs.String("kill-mode", "control-group", "Kill mode")
	privateTmp := fs.Bool("private-tmp", false, "Use private /tmp")
	protectSystem := fs.String("protect-system", "", "Protect system directories")
	noNewPrivileges := fs.Bool("no-new-privileges", false, "Prevent privilege escalation")
	limitNOFILE := fs.Int("limit-nofile", 0, "File descriptor limit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check for sudo
	if sudo.NeedsSudo() {
		return sudo.ReExecWithSudo()
	}

	var svc *systemd.Service

	// Load from YAML config if specified
	if *configFile != "" {
		cfg, err := config.LoadFromFile(*configFile)
		if err != nil {
			return fmt.Errorf("load config file: %w", err)
		}

		svc = configToService(cfg)
	} else {
		// Try to auto-discover config file
		if autoFile, err := config.FindConfigFile(); err == nil {
			fmt.Printf("Found config file: %s\n", autoFile)
			cfg, err := config.LoadFromFile(autoFile)
			if err != nil {
				return fmt.Errorf("load config file: %w", err)
			}
			svc = configToService(cfg)
		} else {
			// Parse from command line args
			svc = parseCommandLine(fs)
		}
	}

	// Override with CLI flags
	applyOverrides(svc, fs, envVars, *restart, *user, *workdir, *description,
		*timeoutStart, *timeoutStop, *killMode, *privateTmp, *protectSystem,
		*noNewPrivileges, *limitNOFILE)

	// Validate
	if svc.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if svc.Command == "" {
		return fmt.Errorf("command is required")
	}

	// Create the service
	mgr := systemd.NewManager()

	fmt.Printf("Creating service: %s\n", svc.Name)

	if err := mgr.Create(svc); err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	// Enable the service
	fmt.Printf("Enabling service...\n")
	if err := mgr.Enable(svc.Name); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	// Start the service
	fmt.Printf("Starting service...\n")
	if err := mgr.Start(svc.Name); err != nil {
		fmt.Printf("\nError: Failed to start service\n")
		fmt.Printf("\n%v\n", err)

		// Prompt to view logs
		if output.PromptYesNo("Service failed to start. View logs?", true) {
			showRecentLogs(svc.Name, 10)
		}

		return fmt.Errorf("failed to start service")
	}

	// Success!
	fmt.Printf("\n%s\n\n", systemd.ServiceName(svc.Name))
	fmt.Printf("Service started successfully.\n\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("  Check status:  smdctl status %s\n", svc.Name)
	fmt.Printf("  View logs:     smdctl logs -f %s\n", svc.Name)
	fmt.Printf("  List services: smdctl ps\n")

	return nil
}

func configToService(cfg *config.ServiceConfig) *systemd.Service {
	return &systemd.Service{
		Name:            cfg.Name,
		Description:     cfg.Description,
		Command:         cfg.Command,
		Args:            cfg.Args,
		WorkDir:         cfg.WorkDir,
		User:            cfg.User,
		Environment:     cfg.Environment,
		Restart:         cfg.Restart,
		TimeoutStart:    cfg.TimeoutStart,
		TimeoutStop:     cfg.TimeoutStop,
		KillMode:        cfg.KillMode,
		After:           cfg.After,
		Wants:           cfg.Wants,
		PrivateTmp:      cfg.PrivateTmp,
		ProtectSystem:   cfg.ProtectSystem,
		NoNewPrivileges: cfg.NoNewPrivileges,
		LimitNOFILE:     cfg.LimitNOFILE,
		TasksMax:        cfg.TasksMax,
	}
}

func parseCommandLine(fs *flag.FlagSet) *systemd.Service {
	args := fs.Args()

	// Find the "--" separator
	separatorIdx := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIdx = i
			break
		}
	}

	if separatorIdx == -1 || separatorIdx == 0 {
		fmt.Fprintf(os.Stderr, "Error: Missing '--' separator before command\n\n")
		fmt.Fprintf(os.Stderr, "Usage: smdctl run [OPTIONS] NAME -- COMMAND [ARGS...]\n\n")
		fmt.Fprintf(os.Stderr, "Example:\n")
		fmt.Fprintf(os.Stderr, "  smdctl run myapp -- /usr/bin/python3 server.py\n\n")
		fmt.Fprintf(os.Stderr, "The '--' separator is required to separate smdctl options from the command.\n")
		os.Exit(1)
	}

	serviceName := args[separatorIdx-1]
	command := args[separatorIdx+1]
	cmdArgs := []string{}
	if len(args) > separatorIdx+2 {
		cmdArgs = args[separatorIdx+2:]
	}

	return &systemd.Service{
		Name:         serviceName,
		Command:      command,
		Args:         cmdArgs,
		Restart:      "always",
		Environment:  make(map[string]string),
		WorkDir:      "/",
		TimeoutStart: 90,
		TimeoutStop:  30,
		KillMode:     "control-group",
	}
}

func applyOverrides(svc *systemd.Service, fs *flag.FlagSet, envVars []string,
	restart, user, workdir, description string,
	timeoutStart, timeoutStop int, killMode string,
	privateTmp bool, protectSystem string, noNewPrivileges bool, limitNOFILE int) {

	// Only override if flag was explicitly set
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "restart":
			svc.Restart = restart
		case "user":
			svc.User = user
		case "workdir":
			svc.WorkDir = workdir
		case "description":
			svc.Description = description
		case "timeout-start":
			svc.TimeoutStart = timeoutStart
		case "timeout-stop":
			svc.TimeoutStop = timeoutStop
		case "kill-mode":
			svc.KillMode = killMode
		case "private-tmp":
			svc.PrivateTmp = privateTmp
		case "protect-system":
			svc.ProtectSystem = protectSystem
		case "no-new-privileges":
			svc.NoNewPrivileges = noNewPrivileges
		case "limit-nofile":
			svc.LimitNOFILE = limitNOFILE
		}
	})

	// Apply environment variables
	if len(envVars) > 0 {
		if svc.Environment == nil {
			svc.Environment = make(map[string]string)
		}
		for _, env := range envVars {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				svc.Environment[parts[0]] = parts[1]
			}
		}
	}
}

func showRecentLogs(serviceName string, lines int) {
	fmt.Printf("\nRecent logs (last %d lines):\n", lines)
	fmt.Println(strings.Repeat("─", 60))

	cmd := exec.Command("journalctl", "-u", systemd.ServiceName(serviceName),
		"-n", fmt.Sprintf("%d", lines), "--no-pager")
	output, _ := cmd.CombinedOutput()
	fmt.Print(string(output))

	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("\nFor full logs, run: smdctl logs %s\n", serviceName)
}
