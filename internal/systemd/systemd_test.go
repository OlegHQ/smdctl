package systemd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeExecutor struct {
	calls    [][]string
	outputs  []string
	errors   []error
	statuses []bool
}

func (f *fakeExecutor) Run(_ Mode, args ...string) error {
	f.calls = append(f.calls, append([]string{"run"}, args...))
	if len(f.errors) > 0 {
		e := f.errors[0]
		f.errors = f.errors[1:]
		return e
	}
	return nil
}
func (f *fakeExecutor) Output(_ Mode, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{"output"}, args...))
	if len(f.errors) > 0 {
		e := f.errors[0]
		f.errors = f.errors[1:]
		if e != nil {
			return "", e
		}
	}
	if len(f.outputs) == 0 {
		return "", nil
	}
	s := f.outputs[0]
	f.outputs = f.outputs[1:]
	return s, nil
}
func (f *fakeExecutor) StatusOK(_ Mode, args ...string) bool {
	f.calls = append(f.calls, append([]string{"status"}, args...))
	if len(f.statuses) == 0 {
		return false
	}
	v := f.statuses[0]
	f.statuses = f.statuses[1:]
	return v
}

func TestPathsAndNames(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	p, e := ServicePath("foo", User)
	if e != nil || p != "/home/test/.config/systemd/user/smdctl-foo.service" {
		t.Fatalf("path=%q err=%v", p, e)
	}
	p, e = ServicePath("foo", System)
	if e != nil || p != "/etc/systemd/system/smdctl-foo.service" {
		t.Fatalf("path=%q err=%v", p, e)
	}
	p, e = EnvFilePath("svc", User)
	if e != nil || p != "/home/test/.config/smdctl/env/svc.env" {
		t.Fatalf("env path=%q err=%v", p, e)
	}
	if got := AbsCommandPath("./bin/app", "/srv"); got != "/srv/./bin/app" {
		t.Fatalf("command path %q", got)
	}
	if StripPrefix("smdctl-foo") != "foo" || StripPrefix("smdctl-smdctl-foo") != "foo" || StripPrefix("foo") != "foo" {
		t.Fatal("prefix handling")
	}
	if UnitName("foo") != "smdctl-foo" {
		t.Fatal("unit name")
	}
}
func TestServiceNameValidation(t *testing.T) {
	for _, tc := range []struct{ name, want string }{{"", "service name cannot be empty"}, {"bad name", "invalid service name"}, {"okay_1-x", ""}} {
		e := ValidateServiceName(tc.name)
		if tc.want == "" && e != nil {
			t.Fatalf("%q: %v", tc.name, e)
		}
		if tc.want != "" && (e == nil || !strings.Contains(e.Error(), tc.want)) {
			t.Fatalf("%q: %v", tc.name, e)
		}
	}
}
func TestGenerateAndParseService(t *testing.T) {
	t.Setenv("HOME", "/tmp/smdctl-test-home")
	svc := EmptyService()
	svc.Name = "api"
	svc.Mode = System
	svc.Description = "API"
	svc.Command = "/usr/local/bin/api"
	svc.UserName = "api"
	svc.After = []string{"network.target"}
	svc.Wants = []string{"network-online.target"}
	svc.PrivateTmp = true
	svc.ProtectSystem = "strict"
	svc.NoNewPrivileges = true
	svc.LimitNofile = 4096
	out, e := GenerateServiceFile(&svc)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"Description=API", "After=network.target", "Wants=network-online.target", "User=api", "PrivateTmp=true", "ProtectSystem=strict", "NoNewPrivileges=true", "LimitNOFILE=4096", "WantedBy=multi-user.target"} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q", s)
		}
	}
	got, e := ParseServiceFile(out, "api", System)
	if e != nil {
		t.Fatal(e)
	}
	if got.Command != svc.Command || got.UserName != svc.UserName || !reflect.DeepEqual(got.After, svc.After) || !got.NoNewPrivileges {
		t.Fatalf("parsed unexpected service: %+v", got)
	}
}
func TestParseServiceRejectsMissingExecStart(t *testing.T) {
	if _, e := ParseServiceFile("[Service]\nRestart=always\n", "x", User); e == nil || !strings.Contains(e.Error(), "ExecStart") {
		t.Fatalf("err=%v", e)
	}
}
func TestPortModeDetection(t *testing.T) {
	for _, tc := range []struct {
		env  map[string]string
		args []string
		want bool
	}{{map[string]string{"PORT": "80"}, nil, true}, {map[string]string{"HTTPS_PORT": "443"}, nil, true}, {map[string]string{"PORT": "8080"}, nil, false}, {nil, []string{"--port=80"}, true}, {nil, []string{"--bind", "0.0.0.0:443"}, true}, {nil, []string{"--port", "0"}, false}} {
		s := EmptyService()
		s.Environment = tc.env
		s.Command = "/bin/service"
		s.Args = tc.args
		if NeedsElevatedPort(&s) != tc.want {
			t.Errorf("env %v args %v got false?", tc.env, tc.args)
		}
	}
}
func TestManagerLifecycleAndCalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fakeExecutor{}
	m := &Manager{Mode: User, Exec: f}
	s := EmptyService()
	s.Name = "web"
	s.Command = "/bin/true"
	if e := m.Create(&s); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(os.Getenv("HOME"), ".config/systemd/user/smdctl-web.service")); e != nil {
		t.Fatal(e)
	}
	if len(f.calls) != 1 || !reflect.DeepEqual(f.calls[0], []string{"run", "daemon-reload"}) {
		t.Fatalf("calls=%v", f.calls)
	}
	if e := m.Enable("web"); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(f.calls[1], []string{"run", "enable", "smdctl-web"}) {
		t.Fatalf("calls=%v", f.calls)
	}
	if e := m.Restart("missing"); e == nil {
		t.Fatal("expected missing service error")
	}
}
func TestCreateRejectsModeMismatchAndExisting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := &Manager{Mode: User, Exec: &fakeExecutor{}}
	s := EmptyService()
	s.Name = "x"
	s.Mode = System
	if e := m.Create(&s); e == nil || e.Error() != "service mode System does not match manager mode User" {
		t.Fatalf("err=%v", e)
	}
	s.Mode = User
	s.Command = "/bin/true"
	p, _ := ServicePath(s.Name, User)
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, nil, 0644); e != nil {
		t.Fatal(e)
	}
	if e := m.Create(&s); e == nil || !strings.Contains(e.Error(), "already exists") {
		t.Fatalf("err=%v", e)
	}
}

func TestRealExecutorFailureMatchesSystemctlContract(t *testing.T) {
	bin := t.TempDir()
	systemctl := filepath.Join(bin, "systemctl")
	if err := os.WriteFile(systemctl, []byte("#!/bin/sh\necho ignored-output\necho failure >&2\nexit 7\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	executor := RealExecutor{}
	want := "systemctl --user start smdctl-web: exit status: 7\nfailure\n"
	if err := executor.Run(User, "start", "smdctl-web"); err == nil || err.Error() != want {
		t.Fatalf("Run error=%q want %q", err, want)
	}
	if _, err := executor.Output(User, "show", "unit"); err == nil || err.Error() != "systemctl --user show unit: exit status: 7\nfailure\n" {
		t.Fatalf("Output error=%q", err)
	}
}

func TestRealExecutorReturnsSpawnErrorWithoutSystemctlContext(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := (RealExecutor{}).Run(User, "start", "smdctl-web")
	if err == nil || strings.Contains(err.Error(), "systemctl --user") {
		t.Fatalf("spawn error=%v", err)
	}
}
func TestServiceInfoJSONDurationShape(t *testing.T) {
	b, e := json.Marshal(ServiceInfo{Name: "a", Uptime: time.Hour + time.Second})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), `"uptime":{"secs":3601,"nanos":0}`) {
		t.Fatalf("json=%s", b)
	}
}

func TestTaskGenerationAndCreate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	parent := EmptyService()
	parent.Name = "worker"
	parent.Command = "/bin/true"
	task := &Task{Name: "cleanup", Command: "/usr/bin/python3", Args: []string{"-m", "cleanup"}, Workdir: "/srv", Environment: map[string]string{"A": "b"}, Schedule: TaskSchedule{OnCalendar: "daily", Persistent: true}}
	unit, err := GenerateTaskServiceFile(&parent, task)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"Type=oneshot", "ExecStart=/usr/bin/python3 -m cleanup", "WorkingDirectory=/srv"} {
		if !strings.Contains(unit, needle) {
			t.Fatalf("missing service content %q", needle)
		}
	}
	if strings.Contains(unit, "OnCalendar") {
		t.Fatal("timer schedule leaked into service unit")
	}
	if !strings.Contains(unit, "StandardOutput=append:") {
		t.Fatal("user task should log to file")
	}
	timer := GenerateTaskTimerFile(parent.Name, task.Name, task.Schedule)
	for _, needle := range []string{"OnCalendar=daily", "Persistent=true", "Unit=smdctl-worker-task-cleanup.service", "WantedBy=timers.target"} {
		if !strings.Contains(timer, needle) {
			t.Errorf("missing %q", needle)
		}
	}
	f := &fakeExecutor{}
	m := &Manager{Mode: User, Exec: f}
	if err = m.CreateTask(&parent, task); err != nil {
		t.Fatal(err)
	}
	for _, pathFn := range []func(string, string, Mode) (string, error){TaskServicePath, TaskTimerPath, TaskEnvFilePath} {
		p, e := pathFn("worker", "cleanup", User)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = os.Stat(p); e != nil {
			t.Errorf("missing %s: %v", p, e)
		}
	}
	if len(f.calls) != 3 {
		t.Fatalf("systemctl calls=%v", f.calls)
	}
	if !reflect.DeepEqual(f.calls[0], []string{"run", "daemon-reload"}) || !reflect.DeepEqual(f.calls[1], []string{"run", "enable", "smdctl-worker-task-cleanup.timer"}) || !reflect.DeepEqual(f.calls[2], []string{"run", "start", "smdctl-worker-task-cleanup.timer"}) {
		t.Fatalf("calls=%v", f.calls)
	}
}

func TestListServicesAndTasks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path, _ := ServicePath("api", User)
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0666); err != nil {
		t.Fatal(err)
	}
	f := &fakeExecutor{outputs: []string{"smdctl-api.service loaded active running API\n", "42\n", "active\n", "running\n", "API\n", "Sat 2026-05-16 21:58:55 UTC\n", "1024\n"}}
	m := &Manager{Mode: User, Exec: f}
	services, err := m.ListServices(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || services[0].Name != "api" || services[0].PID != 42 || services[0].Status != "active" || services[0].MemoryBytes != 1024 {
		t.Fatalf("services=%+v", services)
	}
	f.outputs = []string{`[{"unit":"smdctl-api-task-cleanup.timer","next":1778976000000000,"last":0,"activates":"smdctl-api-task-cleanup.service"}]`}
	tasks, err := m.ListTasks(true, "api")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Service != "api" || tasks[0].Task != "cleanup" || tasks[0].Last != "-" || tasks[0].Activates != "smdctl-api-task-cleanup.service" {
		t.Fatalf("tasks=%+v", tasks)
	}
}

func TestRemoveDeletesServiceAndAssociatedFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := &Manager{Mode: User, Exec: &fakeExecutor{}}
	service, _ := ServicePath("gone", User)
	env, _ := EnvFilePath("gone", User)
	log := LogFilePath("gone", User)
	for _, p := range []string{service, env, log} {
		if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0666); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Remove("gone"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{service, env, log} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected deleted %s, stat err=%v", p, err)
		}
	}
}
