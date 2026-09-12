package systemd

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/OlegHQ/smdctl/internal/process"
)

type Executor interface {
	Run(mode Mode, args ...string) error
	Output(mode Mode, args ...string) (string, error)
	StatusOK(mode Mode, args ...string) bool
}

type RealExecutor struct{}

func systemctlArgs(mode Mode, args []string) []string {
	out := make([]string, 0, len(args)+1)
	if mode == User {
		out = append(out, "--user")
	}
	return append(out, args...)
}
func (RealExecutor) Run(mode Mode, args ...string) error {
	argv := systemctlArgs(mode, args)
	cmd := exec.Command("systemctl", argv...)
	stderr := new(strings.Builder)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return process.NormalizeError(err)
		}
		return fmt.Errorf("systemctl %s: %s\n%s", strings.Join(argv, " "), processExitStatus(exitErr), strings.ToValidUTF8(stderr.String(), "\uFFFD"))
	}
	return nil
}
func (RealExecutor) Output(mode Mode, args ...string) (string, error) {
	argv := systemctlArgs(mode, args)
	cmd := exec.Command("systemctl", argv...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("systemctl %s: %s\n%s", strings.Join(argv, " "), processExitStatus(ee), strings.ToValidUTF8(string(ee.Stderr), "\uFFFD"))
		}
		return "", process.NormalizeError(err)
	}
	return strings.ToValidUTF8(string(out), "\uFFFD"), nil
}

func processExitStatus(err *exec.ExitError) string {
	if code := err.ExitCode(); code >= 0 {
		return fmt.Sprintf("exit status: %d", code)
	}
	return err.Error()
}
func (RealExecutor) StatusOK(mode Mode, args ...string) bool {
	cmd := exec.Command("systemctl", systemctlArgs(mode, args)...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
}
