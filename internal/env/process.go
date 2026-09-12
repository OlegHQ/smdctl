package env

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/OlegHQ/smdctl/internal/process"
)

func NeedsSudoForSystem() bool {
	if os.Geteuid() == 0 {
		return false
	}
	f, e := os.Create("/etc/systemd/system/.smdctl-permission-test")
	if e != nil {
		return true
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return false
}
func ReexecWithSudo() error {
	if os.Geteuid() == 0 {
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return fmt.Errorf("could not read executable for sudo re-exec: %w", e)
	}
	cmd := exec.Command("sudo", append([]string{exe}, os.Args[1:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e = cmd.Run(); e != nil {
		if x, ok := e.(*exec.ExitError); ok {
			os.Exit(x.ExitCode())
		}
		return process.NormalizeError(e)
	}
	return nil
}
func EditEnvironment(service string, modePath, configDir string) error {
	if _, e := os.Stat(modePath); e != nil {
		if e = os.MkdirAll(configDir, 0777); e != nil {
			return e
		}
		if e = os.WriteFile(modePath, []byte(fmt.Sprintf("# Environment variables for service: %s\n# Format: KEY=VALUE (one per line)\n\n", service)), 0666); e != nil {
			return e
		}
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}
	cmd := exec.Command(editor, modePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e := cmd.Run(); e != nil {
		var exitErr *exec.ExitError
		if !errors.As(e, &exitErr) {
			return fmt.Errorf("editor failed: %w", process.NormalizeError(e))
		}
	}
	fmt.Printf("\nEnvironment file updated: %s\n\nTo apply changes, restart the service:\n  smdctl restart %s\n", modePath, service)
	return nil
}
