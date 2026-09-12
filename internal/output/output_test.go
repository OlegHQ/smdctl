package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OlegHQ/smdctl/internal/systemd"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	b, e := io.ReadAll(r)
	if e != nil {
		t.Fatal(e)
	}
	_ = r.Close()
	return string(b)
}
func TestFormatBoundaries(t *testing.T) {
	for n, want := range map[uint64]string{0: "0 B", 512: "512 B", 1023: "1023 B", 1024: "1.0 KB", 1024 * 1024: "1.0 MB", 1024 * 1024 * 1024: "1.0 GB"} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d)=%q want %q", n, got, want)
		}
	}
	for d, want := range map[time.Duration]string{5 * time.Second: "5s", 90 * time.Second: "1m", 3700 * time.Second: "1h 1m", 90000 * time.Second: "1d 1h"} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%s)=%q want %q", d, got, want)
		}
	}
}
func TestServicesJSONAndTable(t *testing.T) {
	services := []systemd.ServiceInfo{{Name: "web", Mode: systemd.User, PID: 12, Status: "active", SubState: "running", Uptime: time.Hour, MemoryBytes: 1024, Ports: []int{80}, Description: "web service"}}
	b, e := json.Marshal(services)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b, []byte(`"name":"web"`)) || !bytes.Contains(b, []byte(`"secs":3600`)) {
		t.Fatalf("json=%s", b)
	}
	tableText := captureStdout(t, func() { FormatServicesTable(services) })
	for _, s := range []string{"web", "user", "running", "1.0 KB", "80", "web service"} {
		if !strings.Contains(tableText, s) {
			t.Errorf("table missing %q: %s", s, tableText)
		}
	}
	empty := captureStdout(t, func() { FormatServicesTable(nil) })
	if !strings.Contains(empty, "No services found.") {
		t.Fatalf("empty output=%q", empty)
	}
}

func TestPrintYAMLUsesTwoSpaceNestingWithoutDocumentMarker(t *testing.T) {
	got := captureStdout(t, func() {
		if err := PrintYAML(struct {
			Name        string            `yaml:"name"`
			Environment map[string]string `yaml:"environment"`
			Details     string            `yaml:"details"`
		}{"example", map[string]string{"PORT": "8080", "PLAIN": "alpha", "ENABLED": "true"}, "first line\nsecond line\n"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.HasPrefix(got, "---\n") || !strings.Contains(got, "name: example") || !strings.Contains(got, "  PORT: '8080'") || !strings.Contains(got, "  ENABLED: 'true'") || !strings.Contains(got, "  PLAIN: alpha") || !strings.Contains(got, "details: |\n  first line\n  second line") || !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("yaml=%q", got)
	}
}
