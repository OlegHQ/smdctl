package systemd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func HomeDir() (string, error) {
	home, ok := os.LookupEnv("HOME")
	if !ok {
		return "", errors.New("HOME environment variable is not set")
	}
	return home, nil
}

func ServiceFileName(name string) string { return ServicePrefix + name + ".service" }
func ServicePath(name string, mode Mode) (string, error) {
	if mode == System {
		return filepath.Join("/etc/systemd/system", ServiceFileName(name)), nil
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/systemd/user", ServiceFileName(name)), nil
}
func EnvFilePath(name string, mode Mode) (string, error) {
	if mode == System {
		return filepath.Join("/etc/smdctl/env", name+".env"), nil
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/smdctl/env", name+".env"), nil
}
func ConfigDir(mode Mode) (string, error) {
	if mode == System {
		return "/etc/smdctl/env", nil
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/smdctl/env"), nil
}
func LogDir(mode Mode) (string, error) {
	if mode == System {
		return "/var/log/smdctl", nil
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config/smdctl/logs"), nil
}
func LogFilePath(name string, mode Mode) string {
	if mode == System {
		return ""
	}
	home, ok := os.LookupEnv("HOME")
	if !ok {
		return ""
	}
	return filepath.Join(home, ".config/smdctl/logs", name+".log")
}
func UnitName(name string) string { return ServicePrefix + name }
func StripPrefix(name string) string {
	for strings.HasPrefix(name, ServicePrefix) {
		name = strings.TrimPrefix(name, ServicePrefix)
	}
	return name
}
func AbsCommandPath(command, workdir string) string {
	if filepath.IsAbs(command) {
		return command
	}
	if workdir != "" && workdir != "/" {
		if strings.HasSuffix(workdir, string(filepath.Separator)) {
			return workdir + command
		}
		return workdir + string(filepath.Separator) + command
	}
	return command
}
