package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestParseInlineCommand(t *testing.T) {
	svc, e := parseInlineCommand([]string{"api", "/usr/bin/python3", "-m", "http.server", "8080"}, 1)
	if e != nil {
		t.Fatal(e)
	}
	if svc.Name != "api" || svc.Command != "/usr/bin/python3" || !reflect.DeepEqual(svc.Args, []string{"-m", "http.server", "8080"}) {
		t.Fatalf("service=%+v", svc)
	}
	if _, e = parseInlineCommand([]string{"only-name"}, 1); e == nil {
		t.Fatal("expected usage error")
	}
	svc, e = parseInlineCommand([]string{"discarded", "api", "/bin/server"}, 2)
	if e != nil || svc.Name != "api" || svc.Command != "/bin/server" {
		t.Fatalf("separator position was not preserved: service=%+v err=%v", svc, e)
	}
}

func TestDebugDurationFormat(t *testing.T) {
	for value, want := range map[time.Duration]string{0: "0ns", time.Second: "1s", 90 * time.Second: "90s", time.Hour: "3600s", 1500 * time.Millisecond: "1.5s", 500 * time.Microsecond: "500µs"} {
		if got := debugDuration(value); got != want {
			t.Errorf("debugDuration(%s)=%q want %q", value, got, want)
		}
	}
}

func TestSingleArgumentCommandRejectsExtras(t *testing.T) {
	cmd := singleCommand("status", "status", func([]string) error { return nil })
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"one", "two"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected extra argument error")
	}
}

func TestCLIParseFailuresUseUsageExitCode(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"start"}, "error: the following required arguments were not provided:\n  <NAMES>..."},
		{[]string{"status", "one", "two"}, "error: unexpected argument 'two' found"},
		{[]string{"ps", "--output", "invalid"}, "error: invalid value 'invalid' for '--output <OUTPUT>'"},
	} {
		root := newRoot()
		root.SilenceErrors = true
		root.SilenceUsage = true
		root.SetArgs(tc.args)
		err := root.Execute()
		if err == nil || ExitCode(err) != 2 || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("args %v: err=%v exit=%d", tc.args, err, ExitCode(err))
		}
	}
}

func TestRootHelpAndVersion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{{nil, "SMDCTL - Systemd Control CLI"}, {[]string{"version"}, "smdctl version 0.1.0"}, {[]string{"help", "run"}, "smdctl run - Create and start"}, {[]string{"--version"}, "smdctl 0.1.0"}} {
		root := newRoot()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(tc.args)
		if err := root.Execute(); err != nil {
			t.Fatalf("args %v: %v", tc.args, err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("args %v output missing %q: %q", tc.args, tc.want, out.String())
		}
	}
}

func TestCompatibleHelpPages(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"root", []string{"--help"}},
		{"run", []string{"run", "--help"}},
		{"ps", []string{"ps", "--help"}},
		{"start", []string{"start", "--help"}},
		{"explain", []string{"explain", "--help"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newRoot()
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			want, err := cliHelp.ReadFile("assets/cli_help_" + tc.name + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) || stderr.Len() != 0 {
				t.Fatalf("stdout=%q stderr=%q want=%q", out.String(), stderr.String(), want)
			}
		})
	}
}
func TestRunCobraParsesRepeatableFlagsAndTrailingArgs(t *testing.T) {
	cmd := runCommand()
	var got []string
	cmd.RunE = func(c *cobra.Command, args []string) error { got = append([]string(nil), args...); return nil }
	cmd.SetArgs([]string{"-e", "A=1", "-e", "B=2", "api", "--", "/usr/bin/server", "--listen", "80"})
	if e := cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, []string{"api", "/usr/bin/server", "--listen", "80"}) {
		t.Fatalf("args=%q", got)
	}
	if at := cmd.Flags().ArgsLenAtDash(); at != 1 {
		t.Fatalf("delimiter position=%d want 1", at)
	}
	envs, e := cmd.Flags().GetStringArray("env")
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(envs, []string{"A=1", "B=2"}) {
		t.Fatalf("env=%v", envs)
	}
}

func TestCobraCommandArgumentShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"start", []string{"start", "a", "b"}, []string{"a", "b"}},
		{"stop", []string{"stop", "a", "b"}, []string{"a", "b"}},
		{"restart", []string{"restart", "a", "b"}, []string{"a", "b"}},
		{"status", []string{"status", "api"}, []string{"api"}},
		{"env", []string{"env", "api"}, []string{"api"}},
		{"inspect", []string{"inspect", "api"}, []string{"api"}},
		{"regenerate", []string{"regenerate", "api"}, []string{"api"}},
		{"tasks", []string{"tasks", "api"}, []string{"api"}},
		{"explain", []string{"explain", "run", "api", "--", "/bin/true"}, []string{"run", "api", "--", "/bin/true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newRoot()
			var target *cobra.Command
			for _, candidate := range root.Commands() {
				if candidate.Name() == tc.name {
					target = candidate
					break
				}
			}
			if target == nil {
				t.Fatalf("command %s missing", tc.name)
			}
			var got []string
			target.RunE = func(_ *cobra.Command, args []string) error {
				got = append([]string(nil), args...)
				return nil
			}
			root.SilenceErrors = true
			root.SilenceUsage = true
			root.SetArgs(tc.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parsed args=%q want %q", got, tc.want)
			}
		})
	}
}

func TestRunRequiresCommandSeparatorWithoutConfig(t *testing.T) {
	cmd := runCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"web", "/bin/true"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Missing '--' separator") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunRequiresCommandAfterSeparator(t *testing.T) {
	cmd := runCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"web", "--"})
	err := cmd.Execute()
	if err == nil || err.Error() != "command is required after '--'" {
		t.Fatalf("error=%v", err)
	}
}

func TestRunSeparatorWithoutServiceShowsUsage(t *testing.T) {
	cmd := runCommand()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--", "/bin/true"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Usage: smdctl run [OPTIONS]") || !strings.Contains(err.Error(), "The '--' separator is required") {
		t.Fatalf("error=%v", err)
	}
}

func TestLogsAndPsFlags(t *testing.T) {
	logs := logsCommand()
	var logArgs []string
	logs.RunE = func(_ *cobra.Command, args []string) error { logArgs = args; return nil }
	logs.SetArgs([]string{"--follow", "--lines", "100", "api"})
	if err := logs.Execute(); err != nil {
		t.Fatal(err)
	}
	follow, _ := logs.Flags().GetBool("follow")
	lines, _ := logs.Flags().GetInt("lines")
	if !follow || lines != 100 || !reflect.DeepEqual(logArgs, []string{"api"}) {
		t.Fatalf("logs follow=%t lines=%d args=%v", follow, lines, logArgs)
	}
	ps := psCommand()
	ps.RunE = func(*cobra.Command, []string) error { return nil }
	ps.SetArgs([]string{"-a", "-q", "-o", "json"})
	if err := ps.Execute(); err != nil {
		t.Fatal(err)
	}
	all, _ := ps.Flags().GetBool("all")
	quiet, _ := ps.Flags().GetBool("quiet")
	format, _ := ps.Flags().GetString("output")
	if !all || !quiet || format != "json" {
		t.Fatalf("ps flags all=%t quiet=%t format=%q", all, quiet, format)
	}
}

func TestInheritIgnoresExitStatusButReportsSpawnFailure(t *testing.T) {
	if err := inherit("sh", "-c", "exit 7"); err != nil {
		t.Fatalf("launched command's exit status should be ignored: %v", err)
	}
	if err := inherit("/definitely/missing/smdctl-test-command"); err == nil {
		t.Fatal("expected command spawn error")
	}
}

func TestRunCreatesUserServiceAndEnvironmentFile(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"systemctl": "#!/bin/sh\nexit 0\n",
		"loginctl":  "#!/bin/sh\necho yes\n",
	} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USER", "smdctl-test")
	t.Setenv("PATH", bin)

	root := newRoot()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"run", "-e", "PORT=8080", "web", "--", "/bin/true"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	unitPath := filepath.Join(home, ".config/systemd/user/smdctl-web.service")
	unit, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(unit), "ExecStart=/bin/true\n") || !strings.Contains(string(unit), "EnvironmentFile=-"+filepath.Join(home, ".config/smdctl/env/web.env")) {
		t.Fatalf("service unit missing command or environment path:\n%s", unit)
	}
	env, err := os.ReadFile(filepath.Join(home, ".config/smdctl/env/web.env"))
	if err != nil || !strings.Contains(string(env), "PORT=8080\n") {
		t.Fatalf("environment file=%q err=%v", env, err)
	}
}
