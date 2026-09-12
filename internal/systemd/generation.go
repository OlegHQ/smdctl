package systemd

import (
	"fmt"
	"strings"
)

func GenerateServiceFile(svc *Service) (string, error) {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	if svc.Description != "" {
		fmt.Fprintf(&b, "Description=%s\n", svc.Description)
	} else {
		fmt.Fprintf(&b, "Description=smdctl managed service: %s\n", svc.Name)
	}
	if len(svc.After) > 0 {
		fmt.Fprintf(&b, "After=%s\n", strings.Join(svc.After, " "))
	} else if svc.Mode == User {
		b.WriteString("After=default.target\n")
	} else {
		b.WriteString("After=network-online.target\n")
	}
	if len(svc.Wants) > 0 {
		fmt.Fprintf(&b, "Wants=%s\n", strings.Join(svc.Wants, " "))
	}
	b.WriteString("\n[Service]\nType=simple\n")
	command := AbsCommandPath(svc.Command, svc.Workdir)
	execStart := command
	if len(svc.Args) > 0 {
		execStart += " " + strings.Join(svc.Args, " ")
	}
	fmt.Fprintf(&b, "ExecStart=%s\n", execStart)
	if svc.Restart != "" {
		fmt.Fprintf(&b, "Restart=%s\n", svc.Restart)
	}
	b.WriteString("RestartSec=5s\n")
	if svc.Workdir != "" {
		fmt.Fprintf(&b, "WorkingDirectory=%s\n", svc.Workdir)
	}
	if svc.Mode == System && svc.UserName != "" {
		fmt.Fprintf(&b, "User=%s\n", svc.UserName)
	}
	env, err := EnvFilePath(svc.Name, svc.Mode)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "EnvironmentFile=-%s\n", env)
	if svc.TimeoutStart > 0 {
		fmt.Fprintf(&b, "TimeoutStartSec=%d\n", svc.TimeoutStart)
	}
	if svc.TimeoutStop > 0 {
		fmt.Fprintf(&b, "TimeoutStopSec=%d\n", svc.TimeoutStop)
	}
	if svc.KillMode != "" {
		fmt.Fprintf(&b, "KillMode=%s\n", svc.KillMode)
	}
	if svc.PrivateTmp {
		b.WriteString("PrivateTmp=true\n")
	}
	if svc.ProtectSystem != "" {
		fmt.Fprintf(&b, "ProtectSystem=%s\n", svc.ProtectSystem)
	}
	if svc.NoNewPrivileges {
		b.WriteString("NoNewPrivileges=true\n")
	}
	if svc.LimitNofile > 0 {
		fmt.Fprintf(&b, "LimitNOFILE=%d\n", svc.LimitNofile)
	}
	if svc.TasksMax > 0 {
		fmt.Fprintf(&b, "TasksMax=%d\n", svc.TasksMax)
	}
	if log := LogFilePath(svc.Name, svc.Mode); log != "" {
		fmt.Fprintf(&b, "StandardOutput=append:%s\nStandardError=append:%s\n", log, log)
	}
	b.WriteString("\n[Install]\n")
	if svc.Mode == User {
		b.WriteString("WantedBy=default.target\n")
	} else {
		b.WriteString("WantedBy=multi-user.target\n")
	}
	return b.String(), nil
}
