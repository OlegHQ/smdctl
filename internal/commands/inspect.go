package commands

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OlegHQ/smdctl/internal/output"
	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/spf13/cobra"
)

func debugDuration(d time.Duration) string {
	n := int64(d)
	if n == 0 {
		return "0ns"
	}
	if n >= int64(time.Second) {
		if n%int64(time.Second) == 0 {
			return fmt.Sprintf("%ds", n/int64(time.Second))
		}
		return strconv.FormatFloat(float64(n)/float64(time.Second), 'f', -1, 64) + "s"
	}
	if n%int64(time.Millisecond) == 0 {
		return fmt.Sprintf("%dms", n/int64(time.Millisecond))
	}
	if n%int64(time.Microsecond) == 0 {
		return fmt.Sprintf("%dµs", n/int64(time.Microsecond))
	}
	return fmt.Sprintf("%dns", n)
}

func cmdInspect(args []string) error {
	name := args[0]
	mode, e := systemd.DiscoverServiceMode(name)
	if e != nil {
		return e
	}
	m := systemd.NewManager(mode)
	info, e := m.GetServiceInfo(name)
	if e != nil {
		return fmt.Errorf("get service info: %w", e)
	}
	file, e := m.GetServiceFile(name)
	if e != nil {
		return fmt.Errorf("get service file: %w", e)
	}
	envPath, _ := systemd.EnvFilePath(name, mode)
	envs := map[string]string{}
	if b, e := os.ReadFile(envPath); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok {
				envs[k] = v
			}
		}
	}
	sp, _ := systemd.ServicePath(name, mode)
	v := struct {
		Name        string            `yaml:"name"`
		Description string            `yaml:"description"`
		Status      string            `yaml:"status"`
		Substate    string            `yaml:"substate"`
		PID         int               `yaml:"pid"`
		Uptime      string            `yaml:"uptime"`
		Environment map[string]string `yaml:"environment"`
		Files       map[string]string `yaml:"files"`
		Resources   struct {
			MemoryBytes    uint64 `yaml:"memory_bytes"`
			ListeningPorts []int  `yaml:"listening_ports"`
		} `yaml:"resources"`
		ServiceFileContent string `yaml:"service_file_content"`
	}{info.Name, info.Description, info.Status, info.SubState, info.PID, debugDuration(info.Uptime), envs, map[string]string{"service_file": sp, "env_file": envPath}, struct {
		MemoryBytes    uint64 `yaml:"memory_bytes"`
		ListeningPorts []int  `yaml:"listening_ports"`
	}{info.MemoryBytes, info.Ports}, file}
	return output.PrintYAML(v)
}

func cmdMigrate() error {
	fmt.Print("Migration from system to user mode\n==================================================\n\n")
	services, _ := systemd.NewManager(systemd.System).ListServices(true)
	if len(services) == 0 {
		fmt.Println("No system services found to migrate.")
		return nil
	}
	fmt.Printf("Found %d service(s) running in system mode:\n\n", len(services))
	for _, s := range services {
		fmt.Printf("  - %s\n", s.Name)
	}
	fmt.Println("\nMigration Process:\n1. For each service, review if it needs privileged ports (< 1024)\n2. Services without privileged ports can be migrated to user mode\n3. Remove the system service: smdctl rm <service-name>\n4. Re-create in user mode: smdctl run ... (without --system flag)\n\nNote: User mode services require lingering to persist after logout.\nEnable lingering: sudo loginctl enable-linger $USER\n\nFor services requiring privileged ports, they must remain in system mode.")
	return nil
}

func cmdRegenerate(args []string) error {
	name := args[0]
	mode, e := systemd.DiscoverServiceMode(name)
	if e != nil {
		return e
	}
	path, e := systemd.ServicePath(name, mode)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return fmt.Errorf("read service file: %w", e)
	}
	svc, e := systemd.ParseServiceFile(string(b), name, mode)
	if e != nil {
		return e
	}
	if mode == systemd.User {
		if d, e := systemd.LogDir(mode); e == nil {
			_ = os.MkdirAll(d, 0777)
		}
	}
	content, e := systemd.GenerateServiceFile(svc)
	if e != nil {
		return e
	}
	if e = os.WriteFile(path, []byte(content), 0666); e != nil {
		return fmt.Errorf("write service file: %w", e)
	}
	if e = systemd.NewManager(mode).DaemonReload(); e != nil {
		return fmt.Errorf("daemon reload: %w", e)
	}
	fmt.Printf("Regenerated service file: %s\n", path)
	if p := systemd.LogFilePath(name, mode); p != "" {
		fmt.Printf("Log file: %s\n", p)
	}
	fmt.Printf("\nRestart the service to apply changes:\n  smdctl restart %s\n", name)
	return nil
}

func explainCommand() *cobra.Command {
	return &cobra.Command{Use: "explain COMMAND [ARGS...]", Short: "Explain an operation without executing it", DisableFlagParsing: true, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			content, err := cliHelp.ReadFile("assets/cli_help_explain.txt")
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), string(content))
			return err
		}
		if len(args) == 0 {
			return errors.New("usage: smdctl explain COMMAND [ARGS...]")
		}
		if args[0] != "run" {
			return errors.New("explain is currently only supported for the 'run' command")
		}
		return explainRun(args[1:])
	}}
}

func explainRun(args []string) error {
	if len(args) == 0 {
		fmt.Print("EXPLANATION: smdctl run\n\n")
		fmt.Println("\nThis command creates and starts a new systemd service.")
		fmt.Println("You must provide either:")
		fmt.Println("  1. A YAML config file with -f flag")
		fmt.Println("  2. Service name and command: smdctl run NAME -- COMMAND [ARGS...]")
		return nil
	}
	forceSystem := false
	for _, a := range args {
		if a == "--system" {
			forceSystem = true
		}
	}
	serviceName, commandLine := "myapp", "/usr/bin/mycommand"
	envVars := make([]string, 0)
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep >= 0 {
		if sep+1 < len(args) {
			commandLine = strings.Join(args[sep+1:], " ")
		}
		nonFlag := make([]string, 0)
		for i := 0; i < sep; {
			if args[i] == "-e" || args[i] == "--env" {
				if i+1 < sep {
					envVars = append(envVars, args[i+1])
					i += 2
					continue
				}
			} else if strings.HasPrefix(args[i], "-") {
				i++
				continue
			} else {
				nonFlag = append(nonFlag, args[i])
			}
			i++
		}
		if len(nonFlag) > 0 {
			serviceName = nonFlag[len(nonFlag)-1]
		}
	}
	service := systemd.EmptyService()
	service.Name = serviceName
	service.Description = fmt.Sprintf("smdctl managed service: %s", serviceName)
	parts := strings.Fields(commandLine)
	if len(parts) > 0 {
		service.Command = parts[0]
		service.Args = parts[1:]
	}
	for _, raw := range envVars {
		if key, value, ok := strings.Cut(raw, "="); ok {
			service.Environment[key] = value
		}
	}
	mode := systemd.DetectMode(&service, forceSystem)
	fmt.Printf("EXPLANATION: smdctl run %s\n\n", strings.Join(args, " "))
	fmt.Print("This command will perform the following steps:\n\n")
	fmt.Println("1. Select systemd mode (default: userspace)")
	fmt.Printf("   → %s mode\n\n", mode)
	if mode == systemd.System {
		fmt.Println("2. Check for sudo privileges")
		fmt.Print("   → If not root, re-execute with sudo\n\n")
	}
	step := 2
	if mode == systemd.System {
		step = 3
	}
	if len(envVars) > 0 {
		fmt.Printf("%d. Create environment file\n", step)
		envPath, err := systemd.EnvFilePath(serviceName, mode)
		if err != nil {
			return err
		}
		fmt.Printf("   → %s\n", envPath)
		fmt.Println("   → Contents:")
		for _, value := range envVars {
			fmt.Printf("     %s\n", value)
		}
		fmt.Println()
		step++
	}
	servicePath, err := systemd.ServicePath(serviceName, mode)
	if err != nil {
		return err
	}
	fmt.Printf("%d. Generate systemd service file\n", step)
	fmt.Printf("   → %s\n\n", servicePath)
	systemctlPrefix, journalctlPrefix := "systemctl", "journalctl"
	if mode == systemd.User {
		systemctlPrefix += " --user"
		journalctlPrefix += " --user"
	}
	fmt.Printf("%d. Reload systemd daemon\n   → %s daemon-reload\n\n", step+1, systemctlPrefix)
	fmt.Printf("%d. Enable service (start at boot)\n   → %s enable %s\n\n", step+2, systemctlPrefix, systemd.UnitName(serviceName))
	fmt.Printf("%d. Start service immediately\n   → %s start %s\n\n", step+3, systemctlPrefix, systemd.UnitName(serviceName))
	service.Mode = mode
	fmt.Println("Generated service file:")
	fmt.Println(strings.Repeat("─", 60))
	render, e := systemd.GenerateServiceFile(&service)
	if e != nil {
		return e
	}
	fmt.Print(render)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()
	fmt.Println("To execute this command:")
	fmt.Printf("  smdctl run %s\n\n", strings.Join(args, " "))
	fmt.Println("To see the result:")
	fmt.Printf("  smdctl status %s\n", serviceName)
	fmt.Printf("  %s status %s\n", systemctlPrefix, systemd.UnitName(serviceName))
	fmt.Printf("  %s -u %s -n 100\n", journalctlPrefix, systemd.UnitName(serviceName))
	return nil
}
