package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OlegHQ/smdctl/internal/systemd"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
)

func serviceRows(services []systemd.ServiceInfo) [][]string {
	rows := make([][]string, 0, len(services))
	for _, s := range services {
		pid, memory, uptime := "-", "-", "-"
		if s.PID > 0 {
			pid = strconv.Itoa(s.PID)
		}
		if s.MemoryBytes > 0 {
			memory = FormatBytes(s.MemoryBytes)
		}
		if s.Uptime/time.Second > 0 {
			uptime = FormatDuration(s.Uptime)
		}
		status := s.SubState
		if status == "" {
			status = s.Status
		}
		ports := "-"
		if len(s.Ports) > 0 {
			vals := make([]string, len(s.Ports))
			for i, p := range s.Ports {
				vals[i] = strconv.Itoa(p)
			}
			ports = strings.Join(vals, ",")
		}
		rows = append(rows, []string{s.Name, string(s.Mode), pid, status, memory, uptime, ports, s.Description})
	}
	return rows
}
func table(header []string, rows [][]string) {
	t := tablewriter.NewTable(os.Stdout,
		tablewriter.WithHeader(header),
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
		tablewriter.WithRendition(tw.Rendition{
			Borders:  tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off},
			Settings: tw.Settings{Separators: tw.SeparatorsNone, Lines: tw.LinesNone},
		}),
		tablewriter.WithPadding(tw.Padding{Left: " ", Right: " ", Overwrite: true}),
	)
	_ = t.Bulk(rows)
	_ = t.Render()
}
func FormatServicesTable(v []systemd.ServiceInfo) {
	if len(v) == 0 {
		fmt.Println("No services found.")
		return
	}
	table([]string{"NAME", "MODE", "PID", "STATUS", "MEMORY", "UPTIME", "PORTS", "DESCRIPTION"}, serviceRows(v))
}
func FormatServicesQuiet(v []systemd.ServiceInfo) {
	for _, s := range v {
		fmt.Println(s.Name)
	}
}
func PrintServicesJSON(v []systemd.ServiceInfo) error { return printJSON(v) }
func FormatTasksTable(v []systemd.TaskInfo) {
	if len(v) == 0 {
		fmt.Println("No tasks found.")
		return
	}
	rows := make([][]string, 0, len(v))
	for _, t := range v {
		rows = append(rows, []string{t.Service, t.Task, string(t.Mode), t.Next, t.Last, t.TimerUnit})
	}
	table([]string{"SERVICE", "TASK", "MODE", "NEXT", "LAST", "TIMER"}, rows)
}
func PrintTasksJSON(v []systemd.TaskInfo) error { return printJSON(v) }
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
func FormatBytes(n uint64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	div := uint64(1024)
	exp := 0
	for n/div >= 1024 {
		div *= 1024
		exp++
	}
	units := "KMGTPE"
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), units[exp])
}
func FormatDuration(d time.Duration) string {
	s := int64(d / time.Second)
	minutes := s / 60
	hours := minutes / 60
	days := hours / 24
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours%24)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", s)
}
func PromptYesNo(message string, defaultYes bool) bool {
	suffix := " [y/N] "
	if defaultYes {
		suffix = " [Y/n] "
	}
	fmt.Print(message + suffix)
	var line string
	if _, err := fmt.Scanln(&line); err != nil {
		return defaultYes
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "" {
		return defaultYes
	}
	return answer == "y" || answer == "yes"
}
func PrintYAML(v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err = yaml.Unmarshal(data, &document); err != nil {
		return err
	}
	quoteImplicitTypes(&document)
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	if err = enc.Encode(&document); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout)
	return err
}

func quoteImplicitTypes(node *yaml.Node) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		if strings.Contains(node.Value, "\n") {
			node.Style = yaml.LiteralStyle
		} else {
			var resolved any
			if err := yaml.Unmarshal([]byte(node.Value), &resolved); err == nil {
				if _, isString := resolved.(string); !isString {
					node.Style = yaml.SingleQuotedStyle
				}
			}
		}
	}
	for _, child := range node.Content {
		quoteImplicitTypes(child)
	}
}
