package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/nexo-tech/smdctl/internal/systemd"
)

// Logs displays service logs using journalctl
func Logs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := fs.Bool("f", false, "Follow logs (live tail)")
	fs.BoolVar(follow, "follow", false, "Follow logs (live tail)")
	lines := fs.Int("n", 50, "Number of lines to show")
	fs.IntVar(lines, "lines", 50, "Number of lines to show")
	since := fs.String("since", "", "Show logs since time (e.g., '1 hour ago')")
	until := fs.String("until", "", "Show logs until time")

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

	// Build journalctl command
	cmdArgs := []string{"-u", systemd.ServiceName(serviceName)}

	// Add --user flag if in user mode
	if mode == systemd.ModeUser {
		cmdArgs = append([]string{"--user"}, cmdArgs...)
	}

	if *follow {
		cmdArgs = append(cmdArgs, "-f")
	} else {
		cmdArgs = append(cmdArgs, "-n", fmt.Sprintf("%d", *lines))
	}

	if *since != "" {
		cmdArgs = append(cmdArgs, "--since", *since)
	}

	if *until != "" {
		cmdArgs = append(cmdArgs, "--until", *until)
	}

	cmdArgs = append(cmdArgs, "--no-pager")

	// Execute journalctl
	cmd := exec.Command("journalctl", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("journalctl failed: %w", err)
	}

	return nil
}
