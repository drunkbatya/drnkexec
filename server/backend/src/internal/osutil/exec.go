package osutil

import (
	"errors"
	"os/exec"
)

// ExitCodeFromError extracts the exit code from an exec error or returns the exec error itself.
func ExitCodeFromError(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}
