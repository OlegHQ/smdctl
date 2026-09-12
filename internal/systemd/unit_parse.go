package systemd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
)

func ParseServiceFile(content, name string, mode Mode) (*Service, error) {
	svc := EmptyService()
	svc.Name = name
	svc.Mode = mode
	options, err := unit.DeserializeOptions(strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parse systemd unit file: %w", err)
	}
	for _, option := range options {
		key, value := option.Name, strings.TrimSpace(option.Value)
		switch key {
		case "Description":
			svc.Description = value
		case "ExecStart":
			fields := strings.Fields(value)
			if len(fields) > 0 {
				svc.Command = fields[0]
				svc.Args = fields[1:]
			}
		case "WorkingDirectory":
			svc.Workdir = value
		case "User":
			svc.UserName = value
		case "Restart":
			svc.Restart = value
		case "TimeoutStartSec":
			if n, e := strconv.Atoi(value); e == nil {
				svc.TimeoutStart = n
			}
		case "TimeoutStopSec":
			if n, e := strconv.Atoi(value); e == nil {
				svc.TimeoutStop = n
			}
		case "KillMode":
			svc.KillMode = value
		case "PrivateTmp":
			svc.PrivateTmp = value == "true" || value == "yes"
		case "ProtectSystem":
			svc.ProtectSystem = value
		case "NoNewPrivileges":
			svc.NoNewPrivileges = value == "true" || value == "yes"
		case "LimitNOFILE":
			if n, e := strconv.Atoi(value); e == nil {
				svc.LimitNofile = n
			}
		case "TasksMax":
			if n, e := strconv.Atoi(value); e == nil {
				svc.TasksMax = n
			}
		case "After":
			svc.After = strings.Fields(value)
		case "Wants":
			svc.Wants = strings.Fields(value)
		}
	}
	if svc.Command == "" {
		return nil, errors.New("no ExecStart found in service file")
	}
	return &svc, nil
}
