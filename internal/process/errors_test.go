package process

import (
	"os/exec"
	"testing"
)

func TestNormalizeMissingExecutable(t *testing.T) {
	err := NormalizeError(&exec.Error{Name: "tail", Err: exec.ErrNotFound})
	if err == nil || err.Error() != "No such file or directory (os error 2)" {
		t.Fatalf("normalized error=%v", err)
	}
}
