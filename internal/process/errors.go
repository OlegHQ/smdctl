// Package process contains small adapters for OS process errors.
package process

import (
	"errors"
	"os/exec"
)

// NormalizeError preserves the OS-level message used for a failed exec.
func NormalizeError(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errors.New("No such file or directory (os error 2)")
	}
	return err
}
