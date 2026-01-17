package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Logs displays service logs
func Logs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow logs (live tail)")
	fs.BoolVar(follow, "follow", false, "Follow logs (live tail)")
	lines := fs.Int("n", 50, "Number of lines to show")
	fs.IntVar(lines, "lines", 50, "Number of lines to show")
	since := fs.String("since", "", "Show logs since time (e.g., '1 hour ago') - system services only")
	until := fs.String("until", "", "Show logs until time - system services only")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: smdctl logs [OPTIONS] SERVICE")
	}

	serviceName := fs.Arg(0)

	// Discover mode
	mode, err := systemd.DiscoverServiceMode(serviceName)
	if err != nil {
		return err
	}

	// Verify service exists
	mgr := systemd.NewManagerWithMode(mode)
	if !mgr.ServiceExists(serviceName) {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	// User services use file-based logging
	if mode == systemd.ModeUser {
		return logsFromFile(serviceName, *follow, *lines)
	}

	// System services use journalctl
	return logsFromJournal(serviceName, *follow, *lines, *since, *until)
}

// logsFromFile reads logs from the service's log file using tail
func logsFromFile(serviceName string, follow bool, lines int) error {
	logFile := systemd.LogFilePath(serviceName, systemd.ModeUser)
	if logFile == "" {
		return fmt.Errorf("unable to determine log file path")
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("log file not found: %s\n\n"+
			"The service may not have produced any output yet, or it may need to be regenerated.\n"+
			"Try: smdctl regenerate %s", logFile, serviceName)
	}

	// Build tail command
	var cmdArgs []string
	if follow {
		cmdArgs = []string{"-f", logFile}
	} else {
		cmdArgs = []string{"-n", fmt.Sprintf("%d", lines), logFile}
	}

	cmd := exec.Command("tail", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	return nil
}

// logsFromJournal reads logs using journalctl (for system services)
func logsFromJournal(serviceName string, follow bool, lines int, since, until string) error {
	cmdArgs := []string{"-u", systemd.ServiceName(serviceName)}

	if follow {
		cmdArgs = append(cmdArgs, "-f")
	} else {
		cmdArgs = append(cmdArgs, "-n", fmt.Sprintf("%d", lines))
	}

	if since != "" {
		cmdArgs = append(cmdArgs, "--since", since)
	}

	if until != "" {
		cmdArgs = append(cmdArgs, "--until", until)
	}

	cmdArgs = append(cmdArgs, "--no-pager")

	cmd := exec.Command("journalctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("journalctl failed: %w", err)
	}

	return nil
}
