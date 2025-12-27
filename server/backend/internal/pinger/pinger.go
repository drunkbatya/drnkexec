package pinger

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/osutil"
	"go.uber.org/zap"
)

const (
	defaultPingBinary = "ping"
	pingBinaryEnv     = "DRNKEXEC_PING_PATH"
)

// Checker runs reachability checks against a host.
type Checker interface {
	Ping(ctx context.Context, host *model.HostConfig) (string, error)
}

// NewChecker builds a ping Checker using the system ping binary.
func NewChecker(logger *zap.SugaredLogger) Checker {
	binary := os.Getenv(pingBinaryEnv)
	if binary == "" {
		binary = defaultPingBinary
	}
	return &execChecker{logger: logger, binary: binary}
}

type execChecker struct {
	logger *zap.SugaredLogger
	binary string
}

func (e *execChecker) Ping(ctx context.Context, host *model.HostConfig) (string, error) {
	target := host.Hostname
	if host.ResolveTo != "" {
		target = host.ResolveTo
	}

	args := []string{"-c", "1", "-W", "2", "-n", target}
	cmd := exec.CommandContext(ctx, e.binary, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode, execErr := osutil.ExitCodeFromError(err)
	output := strings.TrimSpace(stdoutBuf.String())
	if output == "" {
		output = strings.TrimSpace(stderrBuf.String())
	}

	if execErr != nil {
		return output, fmt.Errorf("ping exec failed: %w", execErr)
	}
	if exitCode != 0 {
		return output, fmt.Errorf("ping exit code %d", exitCode)
	}

	e.logger.Debugf("ping ok host=%s output=%s", target, output)
	return output, nil
}
