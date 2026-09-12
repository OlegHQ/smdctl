package systemd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) ListServices(all bool) ([]ServiceInfo, error) {
	args := []string{"list-units", "--type=service", "--no-pager", "--no-legend"}
	if all {
		args = append(args, "--all")
	}
	args = append(args, "smdctl-*")
	out, err := m.exec().Output(m.Mode, args...)
	if err != nil {
		if strings.Contains(err.Error(), "No units found") {
			return []ServiceInfo{}, nil
		}
		return nil, fmt.Errorf("list services: %w", err)
	}
	result := make([]ServiceInfo, 0)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 4 {
			continue
		}
		name := StripPrefix(strings.TrimSuffix(f[0], ".service"))
		info, e := m.GetServiceInfo(name)
		if e != nil {
			info = ServiceInfo{Name: name, Status: f[2], SubState: f[3], Mode: m.Mode, Ports: []int{}}
		}
		result = append(result, info)
	}
	return result, nil
}
func (m *Manager) GetServiceInfo(name string) (ServiceInfo, error) {
	if !m.ServiceExists(name) {
		return ServiceInfo{}, fmt.Errorf("service not found: %s", name)
	}
	info := ServiceInfo{Name: name, Mode: m.Mode, Ports: []int{}}
	unit := UnitName(name)
	props := []string{"MainPID", "ActiveState", "SubState", "Description", "ActiveEnterTimestamp", "MemoryCurrent"}
	for _, p := range props {
		v, e := m.exec().Output(m.Mode, "show", "-p", p, "--value", unit)
		if e != nil {
			continue
		}
		v = strings.TrimSpace(v)
		switch p {
		case "MainPID":
			info.PID, _ = strconv.Atoi(v)
		case "ActiveState":
			info.Status = v
		case "SubState":
			info.SubState = v
		case "Description":
			info.Description = v
		case "ActiveEnterTimestamp":
			if t, ok := parseSystemdTimestamp(v); ok {
				if d := time.Since(t); d > 0 {
					info.Uptime = d
				}
			}
		case "MemoryCurrent":
			if v != "[not set]" {
				info.MemoryBytes, _ = strconv.ParseUint(v, 10, 64)
			}
		}
	}
	if m.Mode == User && info.Status == "active" && info.PID > 0 {
		info.Ports = getListeningPorts(info.PID)
	}
	return info, nil
}
func (m *Manager) GetStatusText(name string) (string, error) {
	if !m.ServiceExists(name) {
		return "", fmt.Errorf("service not found: %s", name)
	}
	out, err := m.exec().Output(m.Mode, "status", UnitName(name), "--no-pager", "-l")
	if err != nil {
		return err.Error(), nil
	}
	return out, nil
}
func (m *Manager) IsActive(name string) bool {
	return m.exec().StatusOK(m.Mode, "is-active", UnitName(name))
}
func parseSystemdTimestamp(ts string) (time.Time, bool) {
	ts = strings.TrimSpace(strings.TrimSuffix(ts, " UTC"))
	for _, layout := range []string{"Mon 2006-01-02 15:04:05", "Mon Jan 2 15:04:05 MST 2006", time.RFC3339} {
		if t, e := time.ParseInLocation(layout, ts, time.UTC); e == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

var pidPattern = regexp.MustCompile(`pid=(\d+)`)

func getListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}
	var ports []int
	seen := map[int]bool{}
	for _, args := range [][]string{{"-H", "-lntp"}, {"-H", "-lnup"}} {
		out, e := exec.Command("ss", args...).Output()
		if e != nil {
			continue
		}
		appendPortsForPID(&ports, seen, string(out), strconv.Itoa(pid))
	}
	sort.Ints(ports)
	return ports
}
func appendPortsForPID(ports *[]int, seen map[int]bool, out, pid string) {
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		owned := false
		for _, match := range pidPattern.FindAllStringSubmatch(line, -1) {
			if match[1] == pid {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		port := extractPort(fields[3])
		if port > 0 && !seen[port] {
			seen[port] = true
			*ports = append(*ports, port)
		}
	}
}
func extractPort(addr string) int {
	i := strings.LastIndex(addr, "]:")
	if i >= 0 {
		addr = addr[i+2:]
	} else if i = strings.LastIndex(addr, ":"); i >= 0 {
		addr = addr[i+1:]
	} else {
		return 0
	}
	n, _ := strconv.Atoi(addr)
	return n
}

func (m *Manager) ListTasks(all bool, serviceFilter string) ([]TaskInfo, error) {
	pattern := "smdctl-*-task-*.timer"
	if serviceFilter != "" {
		pattern = fmt.Sprintf("%s%s-task-*.timer", ServicePrefix, serviceFilter)
	}
	args := []string{"list-timers", "--output=json", "--no-pager", "--no-legend"}
	if all {
		args = append(args, "--all")
	}
	args = append(args, pattern)
	out, err := m.exec().Output(m.Mode, args...)
	if err != nil {
		return nil, err
	}
	var rows []map[string]json.RawMessage
	if err = json.Unmarshal([]byte(out), &rows); err != nil {
		return nil, err
	}
	result := make([]TaskInfo, 0)
	for _, row := range rows {
		var unit, activates string
		_ = json.Unmarshal(row["unit"], &unit)
		_ = json.Unmarshal(row["activates"], &activates)
		if !strings.HasPrefix(unit, ServicePrefix) || !strings.HasSuffix(unit, ".timer") {
			continue
		}
		base := strings.TrimSuffix(strings.TrimPrefix(unit, ServicePrefix), ".timer")
		parts := strings.SplitN(base, "-task-", 2)
		if len(parts) != 2 || (serviceFilter != "" && parts[0] != serviceFilter) {
			continue
		}
		result = append(result, TaskInfo{Service: parts[0], Task: parts[1], Mode: m.Mode, Next: formatTimerTimestamp(row["next"]), Last: formatTimerTimestamp(row["last"]), TimerUnit: unit, Activates: activates})
	}
	return result, nil
}
func formatTimerTimestamp(raw json.RawMessage) string {
	var n float64
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil || n == 0 {
		return "-"
	}
	return time.UnixMicro(int64(n)).Local().Format("2006-01-02 15:04")
}

func (m *Manager) GetServiceFile(name string) (string, error) {
	if !m.ServiceExists(name) {
		return "", fmt.Errorf("service not found: %s", name)
	}
	path, err := ServicePath(name, m.Mode)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	return string(b), err
}
func DiscoverServiceMode(name string) (Mode, error) {
	for _, mode := range []Mode{User, System} {
		if p, err := ServicePath(name, mode); err == nil {
			if _, err = os.Stat(p); err == nil {
				return mode, nil
			}
		}
	}
	return "", fmt.Errorf("service not found: %s", name)
}
