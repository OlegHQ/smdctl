package systemd

import "testing"

func TestGenerateTaskTimerFile_OnCalendar(t *testing.T) {
	content := GenerateTaskTimerFile("svc", "cleanup", TaskSchedule{OnCalendar: "daily", Persistent: true})
	if !contains(content, "OnCalendar=daily\n") {
		t.Fatalf("expected OnCalendar, got:\n%s", content)
	}
	if !contains(content, "WantedBy=timers.target\n") {
		t.Fatalf("expected WantedBy=timers.target, got:\n%s", content)
	}
	if !contains(content, "Unit=smdctl-svc-task-cleanup.service\n") {
		t.Fatalf("expected Unit=...service, got:\n%s", content)
	}
}

func TestGenerateTaskServiceFile_UserLogging(t *testing.T) {
	parent := &Service{Name: "svc", Mode: ModeUser, WorkDir: "/opt/svc"}
	task := &Task{Name: "cleanup", Command: "/bin/true", WorkDir: "/opt/svc"}
	content := GenerateTaskServiceFile(parent, task)
	if !contains(content, "Type=oneshot\n") {
		t.Fatalf("expected Type=oneshot, got:\n%s", content)
	}
	if !contains(content, "StandardOutput=append:") {
		t.Fatalf("expected file logging for user mode, got:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (stringIndex(s, substr) >= 0)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
