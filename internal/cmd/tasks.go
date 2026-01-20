package cmd

import (
	"flag"
	"fmt"

	"github.com/nexo-tech/smdctl/internal/output"
	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Tasks lists scheduled tasks (systemd timers) managed by smdctl.
func Tasks(args []string) error {
	fs := flag.NewFlagSet("tasks", flag.ExitOnError)
	all := fs.Bool("a", false, "Show all tasks (default: only active timers)")
	fs.BoolVar(all, "all", false, "Show all tasks (default: only active timers)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	serviceFilter := ""
	if fs.NArg() > 0 {
		serviceFilter = fs.Arg(0)
	}

	var tasks []*systemd.TaskInfo

	userMgr := systemd.NewManagerWithMode(systemd.ModeUser)
	userTasks, err := userMgr.ListTasks(*all, serviceFilter)
	if err == nil {
		tasks = append(tasks, userTasks...)
	}

	systemMgr := systemd.NewManagerWithMode(systemd.ModeSystem)
	systemTasks, err := systemMgr.ListTasks(*all, serviceFilter)
	if err == nil {
		tasks = append(tasks, systemTasks...)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return nil
	}

	output.FormatTasksTable(tasks)
	return nil
}
