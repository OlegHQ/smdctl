package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/OlegHQ/smdctl/internal/config"
	envhelper "github.com/OlegHQ/smdctl/internal/env"
	"github.com/OlegHQ/smdctl/internal/output"
	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/spf13/cobra"
)

func runCommand() *cobra.Command {
	var file string
	var force bool
	var envs []string
	var restart, user, workdir, description, killMode, protect string
	var timeoutStart, timeoutStop, limitNofile int
	var privateTmp, noNew bool
	var after, wants []string
	c := &cobra.Command{Use: "run [OPTIONS] NAME -- COMMAND [ARGS...]", Short: "Create and start a new systemd service", Args: cobra.ArbitraryArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return runService(args, cmd.Flags().ArgsLenAtDash(), file, force, envs, restart, user, workdir, description, killMode, protect, timeoutStart, timeoutStop, limitNofile, privateTmp, noNew, after, wants)
	}}
	f := c.Flags()
	f.StringVarP(&file, "file", "f", "", "YAML config file")
	f.BoolVar(&force, "system", false, "Force system mode")
	f.StringArrayVarP(&envs, "env", "e", nil, "Set environment variable KEY=VALUE")
	f.StringVar(&restart, "restart", "", "Restart policy")
	f.StringVar(&user, "user", "", "Run as user")
	f.StringVar(&workdir, "workdir", "", "Working directory")
	f.StringVar(&description, "description", "", "Service description")
	f.IntVar(&timeoutStart, "timeout-start", 0, "Startup timeout in seconds")
	f.IntVar(&timeoutStop, "timeout-stop", 0, "Stop timeout in seconds")
	f.StringVar(&killMode, "kill-mode", "", "Kill mode")
	f.BoolVar(&privateTmp, "private-tmp", false, "Use private /tmp")
	f.StringVar(&protect, "protect-system", "", "Protect system level")
	f.BoolVar(&noNew, "no-new-privileges", false, "Prevent privilege escalation")
	f.IntVar(&limitNofile, "limit-nofile", 0, "File descriptor limit")
	f.StringArrayVar(&after, "after", nil, "systemd After= dependency")
	f.StringArrayVar(&wants, "wants", nil, "systemd Wants= dependency")
	return c
}
func runService(args []string, separatorAt int, file string, force bool, envs []string, restart, user, workdir, description, killMode, protect string, timeoutStart, timeoutStop, limitNofile int, privateTmp, noNew bool, after, wants []string) error {
	var cfg *config.Service
	var err error
	if separatorAt < 0 && file == "" {
		if _, autoConfigErr := config.FindConfigFile(); autoConfigErr != nil {
			return missingSeparatorError()
		}
	}
	if file != "" {
		cfg, err = config.Load(file)
	} else if auto, e := config.FindConfigFile(); e == nil {
		fmt.Printf("Found config file: %s\n", auto)
		cfg, err = config.Load(auto)
	}
	if err != nil {
		return err
	}
	var svc *systemd.Service
	if cfg != nil {
		svc = config.ToSystemd(cfg)
	} else {
		if separatorAt == 0 {
			return missingSeparatorError()
		}
		if separatorAt > 0 && len(args) <= separatorAt {
			return errors.New("command is required after '--'")
		}
		svc, err = parseInlineCommand(args, separatorAt)
		if err != nil {
			return err
		}
	}
	if restart != "" {
		svc.Restart = restart
	}
	if user != "" {
		svc.UserName = user
	}
	if workdir != "" {
		svc.Workdir = workdir
	}
	if description != "" {
		svc.Description = description
	}
	if timeoutStart != 0 {
		svc.TimeoutStart = timeoutStart
	}
	if timeoutStop != 0 {
		svc.TimeoutStop = timeoutStop
	}
	if killMode != "" {
		svc.KillMode = killMode
	}
	if privateTmp {
		svc.PrivateTmp = true
	}
	if protect != "" {
		svc.ProtectSystem = protect
	}
	if noNew {
		svc.NoNewPrivileges = true
	}
	if limitNofile != 0 {
		svc.LimitNofile = limitNofile
	}
	if len(after) > 0 {
		svc.After = after
	}
	if len(wants) > 0 {
		svc.Wants = wants
	}
	for _, raw := range envs {
		if k, v, ok := strings.Cut(raw, "="); ok {
			svc.Environment[k] = v
		}
	}
	if svc.Name == "" {
		return errors.New("service name is required")
	}
	if svc.Command == "" {
		return errors.New("command is required")
	}
	svc.Mode = systemd.DetectMode(svc, force || cfg != nil && cfg.SystemMode)
	if svc.Mode == systemd.System {
		if envhelper.NeedsSudoForSystem() {
			return envhelper.ReexecWithSudo()
		}
		fmt.Println("Running in system mode (elevated port or --system flag)")
	} else {
		fmt.Println("Running in user mode")
		warnLinger()
	}
	m := systemd.NewManager(svc.Mode)
	fmt.Printf("Creating service: %s\n", svc.Name)
	if err = m.Create(svc); err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	fmt.Println("Enabling service...")
	if err = m.Enable(svc.Name); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}
	fmt.Println("Starting service...")
	if err = m.Start(svc.Name); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: Failed to start service\n\n%s\n", err)
		if output.PromptYesNo("Service failed to start. View logs?", true) {
			showRecentLogs(svc.Name, 10, svc.Mode)
		}
		return errors.New("service failed to start")
	}
	fmt.Printf("\n%s\n\nService started successfully in %s mode.\n\n", systemd.UnitName(svc.Name), svc.Mode)
	if cfg != nil && len(cfg.Tasks) > 0 {
		fmt.Printf("\nCreating %d scheduled task(s)...\n\n", len(cfg.Tasks))
		for i := range cfg.Tasks {
			task := config.ToSystemdTask(&cfg.Tasks[i])
			if e := m.CreateTask(svc, task); e != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create task %s: %v\n", task.Name, e)
			}
		}
	}
	sp, _ := systemd.ServicePath(svc.Name, svc.Mode)
	ep, _ := systemd.EnvFilePath(svc.Name, svc.Mode)
	fmt.Printf("Service file: %s\nEnv file:     %s\n\nNext steps:\n  Check status:  smdctl status %s\n  View logs:     smdctl logs -f %s\n  List services: smdctl ps\n", sp, ep, svc.Name, svc.Name)
	return nil
}

func missingSeparatorError() error {
	return errors.New("Missing '--' separator before command\n\nUsage: smdctl run [OPTIONS] NAME -- COMMAND [ARGS...]\n\nExample:\n  smdctl run myapp -- /usr/bin/python3 server.py\n\nThe '--' separator is required to separate smdctl options from the command.")
}

func showRecentLogs(name string, lines int, mode systemd.Mode) {
	fmt.Printf("\nRecent logs (last %d lines):\n%s\n", lines, strings.Repeat("─", 60))
	if path := systemd.LogFilePath(name, mode); path != "" {
		_ = inherit("tail", "-n", strconv.Itoa(lines), path)
	} else {
		_ = inherit("journalctl", "-u", systemd.UnitName(name), "-n", strconv.Itoa(lines), "--no-pager")
	}
	fmt.Printf("%s\n\nFor full logs, run: smdctl logs %s\n", strings.Repeat("─", 60), name)
}

func parseInlineCommand(args []string, separatorAt int) (*systemd.Service, error) {
	if separatorAt < 1 || len(args) <= separatorAt {
		return nil, errors.New("usage: smdctl run [OPTIONS] NAME -- COMMAND [ARGS...]")
	}
	svc := systemd.EmptyService()
	svc.Name, svc.Command = args[separatorAt-1], args[separatorAt]
	svc.Args = append([]string(nil), args[separatorAt+1:]...)
	return &svc, nil
}
func warnLinger() {
	user := os.Getenv("USER")
	if user == "" {
		return
	}
	out, _ := exec.Command("loginctl", "show-user", user, "--property=Linger", "--value").Output()
	if strings.TrimSpace(string(out)) == "yes" {
		return
	}
	fmt.Printf("\n⚠️  WARNING: User lingering is not enabled for %s\n   Your services will stop when you log out.\n   To enable lingering (services persist after logout):\n   sudo loginctl enable-linger %s\n\n", user, user)
}
