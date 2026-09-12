package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/OlegHQ/smdctl/internal/output"
	"github.com/OlegHQ/smdctl/internal/process"
	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/spf13/cobra"
)

func logsCommand() *cobra.Command {
	var follow bool
	var lines int
	var since, until string
	c := &cobra.Command{Use: "logs [flags] SERVICE", Short: "View service logs", Args: func(cmd *cobra.Command, args []string) error { return requireArguments(cmd, args, 1, 1, "<SERVICE>") }, RunE: func(_ *cobra.Command, args []string) error {
		mode, e := systemd.DiscoverServiceMode(args[0])
		if e != nil {
			return e
		}
		if mode == systemd.User {
			p := systemd.LogFilePath(args[0], mode)
			if p == "" {
				return errors.New("user log path is unavailable (is HOME set?)")
			}
			if _, e = os.Stat(p); e != nil {
				return fmt.Errorf("log file not found: %s\n\nThe service may not have produced any output yet, or it may need to be regenerated.\nTry: smdctl regenerate %s", p, args[0])
			}
			a := []string{"-n", strconv.Itoa(lines), p}
			if follow {
				a = []string{"-f", p}
			}
			if e = inherit("tail", a...); e != nil {
				return fmt.Errorf("failed to read log file: %w", e)
			}
			return nil
		}
		a := []string{"-u", systemd.UnitName(args[0])}
		if follow {
			a = append(a, "-f")
		} else {
			a = append(a, "-n", strconv.Itoa(lines))
		}
		if since != "" {
			a = append(a, "--since", since)
		}
		if until != "" {
			a = append(a, "--until", until)
		}
		a = append(a, "--no-pager")
		if e = inherit("journalctl", a...); e != nil {
			return fmt.Errorf("journalctl failed: %w", e)
		}
		return nil
	}}
	c.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	c.Flags().IntVarP(&lines, "lines", "n", 50, "Number of lines")
	c.Flags().StringVar(&since, "since", "", "Show entries since time")
	c.Flags().StringVar(&until, "until", "", "Show entries until time")
	return c
}
func inherit(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if e := c.Run(); e != nil {
		var exitErr *exec.ExitError
		if errors.As(e, &exitErr) {
			return nil
		}
		return process.NormalizeError(e)
	}
	return nil
}

func cmdStatus(args []string) error {
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
	fmt.Printf("Service: %s (%s)\nMode:    %s\nStatus:  %s (%s)\n", info.Name, systemd.UnitName(name), mode, info.Status, displayOrDash(info.SubState))
	if info.PID > 0 {
		fmt.Printf("PID:     %d\n", info.PID)
	}
	if info.Uptime > 0 {
		fmt.Printf("Uptime:  %s\n", formatStatusUptime(int64(info.Uptime/time.Second)))
	}
	if info.MemoryBytes > 0 {
		fmt.Printf("Memory:  %s\n", output.FormatBytes(info.MemoryBytes))
	}
	if len(info.Ports) > 0 {
		fmt.Printf("Ports:   %v\n", info.Ports)
	}
	sp, _ := systemd.ServicePath(name, mode)
	ep, _ := systemd.EnvFilePath(name, mode)
	fmt.Printf("\nService File: %s\nEnv File:     %s\n\n--- systemctl status output ---\n", sp, ep)
	statusArgs := []string{"status", systemd.UnitName(name), "--no-pager", "-l", "-n", "10"}
	if mode == systemd.User {
		statusArgs = append([]string{"--user"}, statusArgs...)
	}
	_ = inherit("systemctl", statusArgs...)
	fmt.Printf("\n--- Next steps ---\n  View full logs:       smdctl logs -f %s\n  Restart service:      smdctl restart %s\n  Edit environment:     smdctl env %s\n  Inspect config:       smdctl inspect %s\n", name, name, name, name)
	return nil
}
func displayOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
func formatStatusUptime(s int64) string {
	m := s / 60
	h := m / 60
	d := h / 24
	if d > 0 {
		return fmt.Sprintf("%d days %d hours", d, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("%d hours %d minutes", h, m%60)
	}
	if m > 0 {
		return fmt.Sprintf("%d minutes", m)
	}
	return fmt.Sprintf("%d seconds", s)
}
