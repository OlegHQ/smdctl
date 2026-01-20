package systemd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type TaskInfo struct {
	Service   string
	Task      string
	Mode      SystemdMode
	Next      string
	Last      string
	TimerUnit string
	Activates string
}

type listTimerRow struct {
	Next      *int64 `json:"next"`
	Last      *int64 `json:"last"`
	Unit      string `json:"unit"`
	Activates string `json:"activates"`
}

func (m *Manager) ListTasks(all bool, serviceFilter string) ([]*TaskInfo, error) {
	pattern := "smdctl-*-task-*.timer"
	if serviceFilter != "" {
		pattern = fmt.Sprintf("%s%s-task-*.timer", servicePrefix, serviceFilter)
	}

	args := []string{"list-timers", "--output=json", "--no-pager", "--no-legend"}
	if all {
		args = append(args, "--all")
	}
	args = append(args, pattern)

	output, err := m.systemctlOutput(args...)
	if err != nil {
		return nil, err
	}

	var rows []listTimerRow
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		return nil, fmt.Errorf("parse list-timers json: %w", err)
	}

	var tasks []*TaskInfo
	for _, row := range rows {
		if !strings.HasPrefix(row.Unit, servicePrefix) {
			continue
		}
		if !strings.HasSuffix(row.Unit, ".timer") {
			continue
		}

		base := strings.TrimSuffix(row.Unit, ".timer")
		trimmed := strings.TrimPrefix(base, servicePrefix)
		parts := strings.SplitN(trimmed, "-task-", 2)
		if len(parts) != 2 {
			continue
		}

		serviceName := parts[0]
		taskName := parts[1]
		if serviceFilter != "" && serviceName != serviceFilter {
			continue
		}

		tasks = append(tasks, &TaskInfo{
			Service:   serviceName,
			Task:      taskName,
			Mode:      m.mode,
			Next:      formatSystemdListTimerTS(row.Next),
			Last:      formatSystemdListTimerTS(row.Last),
			TimerUnit: row.Unit,
			Activates: row.Activates,
		})
	}

	return tasks, nil
}

func formatSystemdListTimerTS(v *int64) string {
	if v == nil || *v == 0 {
		return "-"
	}

	// systemd list-timers --output=json uses microseconds since epoch
	t := time.UnixMicro(*v).Local()
	return t.Format("2006-01-02 15:04")
}
