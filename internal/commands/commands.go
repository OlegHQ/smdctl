package commands

import (
	"embed"
	"fmt"

	"github.com/OlegHQ/smdctl/internal/output"
	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

//go:embed assets/help_main.txt
var helpMain string

//go:embed assets/help_run.txt
var helpRun string

//go:embed assets/cli_help_*.txt
var cliHelp embed.FS

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "smdctl",
		Short:         "Docker-like systemd service CLI (Linux)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		RunE: func(c *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(c.OutOrStdout(), helpMain)
			return err
		},
	}
	var printVersion bool
	root.Flags().BoolVarP(&printVersion, "version", "V", false, "Print version")
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpFunc(func(c *cobra.Command, args []string) {
		name := c.Name()
		if c == root {
			name = "root"
		}
		if content, err := cliHelp.ReadFile("assets/cli_help_" + name + ".txt"); err == nil {
			fmt.Fprint(c.OutOrStdout(), string(content))
			return
		}
		fmt.Fprint(c.OutOrStdout(), c.UsageString())
	})
	root.AddCommand(
		runCommand(),
		psCommand(),
		tasksCommand(),
		namedCommand("start", "Start one or more services", cmdStart),
		namedCommand("stop", "Stop one or more services", cmdStop),
		namedCommand("restart", "Restart one or more services", cmdRestart),
		logsCommand(),
		singleCommand("status", "Show detailed status of a service", cmdStatus),
		rmCommand(),
		singleCommand("env", "Edit environment variables for a service", cmdEnv),
		singleCommand("inspect", "Show service configuration in YAML format", cmdInspect),
		explainCommand(),
		&cobra.Command{
			Use:   "migrate",
			Short: "Explain migration from system to user mode",
			Args:  func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 0, 0, "") },
			RunE:  func(*cobra.Command, []string) error { return cmdMigrate() },
		},
		singleCommand("regenerate", "Regenerate a unit file", cmdRegenerate),
		&cobra.Command{
			Use:   "version",
			Short: "Print version",
			Args:  func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 0, 0, "") },
			Run:   func(c *cobra.Command, _ []string) { fmt.Fprintf(c.OutOrStdout(), "smdctl version %s\n", version) },
		},
	)
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Run: func(c *cobra.Command, args []string) {
		if len(args) > 0 && args[0] == "run" {
			fmt.Fprint(c.OutOrStdout(), helpRun)
			return
		}
		if len(args) > 0 {
			fmt.Fprintf(c.ErrOrStderr(), "No detailed help available for command: %s\nRun 'smdctl help' for general usage.\n", args[0])
			return
		}
		fmt.Fprint(c.OutOrStdout(), helpMain)
	}})
	return root
}
func namedCommand(name, short string, run func([]string) error) *cobra.Command {
	return &cobra.Command{Use: name + " NAME [NAME...]", Short: short, Args: func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 1, -1, "<NAMES>...") }, RunE: func(_ *cobra.Command, args []string) error { return run(args) }}
}
func singleCommand(name, short string, run func([]string) error) *cobra.Command {
	return &cobra.Command{Use: name + " NAME", Short: short, Args: func(cmd *cobra.Command, args []string) error {
		return requireArguments(cmd, args, 1, 1, "<NAME>")
	}, RunE: func(_ *cobra.Command, args []string) error { return run(args) }}
}

func psCommand() *cobra.Command {
	var all, quiet bool
	var format string
	c := &cobra.Command{Use: "ps", Short: "List managed services", Args: func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 0, 0, "") }, RunE: func(cmd *cobra.Command, _ []string) error {
		if format != "table" && format != "json" {
			return invalidOutputFormat(format)
		}
		services := make([]systemd.ServiceInfo, 0)
		for _, mode := range []systemd.Mode{systemd.User, systemd.System} {
			v, e := systemd.NewManager(mode).ListServices(all)
			if e == nil {
				services = append(services, v...)
			}
		}
		if quiet {
			output.FormatServicesQuiet(services)
		} else if format == "json" {
			return output.PrintServicesJSON(services)
		} else {
			output.FormatServicesTable(services)
		}
		return nil
	}}
	c.Flags().BoolVarP(&all, "all", "a", false, "Show all services, including stopped")
	c.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only print service names")
	c.Flags().StringVarP(&format, "output", "o", "table", "Output format: table or json")
	return c
}
func tasksCommand() *cobra.Command {
	var all bool
	var format string
	c := &cobra.Command{Use: "tasks [SERVICE]", Short: "List scheduled tasks", Args: func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 0, 1, "") }, RunE: func(_ *cobra.Command, args []string) error {
		if format != "table" && format != "json" {
			return invalidOutputFormat(format)
		}
		filter := ""
		if len(args) > 0 {
			filter = args[0]
		}
		tasks := make([]systemd.TaskInfo, 0)
		for _, mode := range []systemd.Mode{systemd.User, systemd.System} {
			v, e := systemd.NewManager(mode).ListTasks(all, filter)
			if e == nil {
				tasks = append(tasks, v...)
			}
		}
		if format == "json" {
			return output.PrintTasksJSON(tasks)
		}
		output.FormatTasksTable(tasks)
		return nil
	}}
	c.Flags().BoolVarP(&all, "all", "a", false, "Show inactive timers too")
	c.Flags().StringVarP(&format, "output", "o", "table", "Output format: table or json")
	return c
}
