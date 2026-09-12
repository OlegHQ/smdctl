package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDefaultsAndOverrides(t *testing.T) {
	cfg, e := Parse([]byte("name: web\ncommand: /bin/x\n"))
	if e != nil {
		t.Fatal(e)
	}
	if cfg.Restart != "always" || cfg.TimeoutStart != 90 || cfg.TimeoutStop != 30 || cfg.KillMode != "control-group" || cfg.Workdir != "/" {
		t.Fatalf("defaults: %+v", cfg)
	}
	cfg, e = Parse([]byte("name: web\ncommand: /bin/x\nrestart: no\ntimeout_start: 10\n"))
	if e != nil {
		t.Fatal(e)
	}
	if cfg.Restart != "no" || cfg.TimeoutStart != 10 {
		t.Fatalf("explicit values lost: %+v", cfg)
	}
}

func TestParseRequiresNameAndCommandFields(t *testing.T) {
	for _, tc := range []struct{ yaml, want string }{{"command: /bin/x\n", "name"}, {"name: web\n", "command"}} {
		if _, err := Parse([]byte(tc.yaml)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Parse(%q) error=%v", tc.yaml, err)
		}
	}
}
func TestTaskDefaultsAndEnvironmentInheritance(t *testing.T) {
	cfg, e := Parse([]byte("name: web\ncommand: /bin/x\nworkdir: /app\nenvironment:\n  A: parent\n  B: inherited\ntasks:\n  - name: task\n    command: /bin/y\n    environment:\n      A: child\n    schedule:\n      on_calendar: daily\n"))
	if e != nil {
		t.Fatal(e)
	}
	task := cfg.Tasks[0]
	if task.Workdir != "/app" || task.Environment["A"] != "child" || task.Environment["B"] != "inherited" || task.Schedule.Persistent == nil || !*task.Schedule.Persistent {
		t.Fatalf("task defaults: %+v", task)
	}
}
func TestExpandEnvironmentVariables(t *testing.T) {
	t.Setenv("SMDCTL_CFG_TEST", "set")
	cfg, e := Parse([]byte("name: ${SMDCTL_CFG_TEST}\ncommand: /bin/x\ndescription: $SMDCTL_CFG_TEST\nenvironment:\n  VALUE: '${SMDCTL_CFG_TEST}'\n"))
	if e != nil {
		t.Fatal(e)
	}
	ExpandEnvVars(cfg)
	if cfg.Name != "set" || cfg.Description != "set" || cfg.Environment["VALUE"] != "set" {
		t.Fatalf("expanded: %+v", cfg)
	}
}
func TestExamplesParse(t *testing.T) {
	for _, name := range []string{"webapp.yml", "nodeapp.yml", "worker.yml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "examples", name)
			b, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = Parse(b); e != nil {
				t.Fatal(e)
			}
		})
	}
}
