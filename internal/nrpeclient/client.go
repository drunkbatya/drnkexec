package nrpeclient

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/osutil"
	"go.uber.org/zap"
)

// Status represents NRPE status codes.
type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusCritical
	StatusUnknown
)

const (
	defaultCheckNRPEBinary = "check_nrpe"
	checkNRPEBinaryEnv     = "DRNKEXEC_CHECK_NRPE_PATH"
)

// Result describes an execution result.
type Result struct {
	Output string
	Status Status
}

// Client executes remote checks.
type Client interface {
	Execute(ctx context.Context, host *model.HostConfig, check *model.CheckConfig) (Result, error)
}

// NewClient returns an NRPE client that shells out to check_nrpe binary.
func NewClient(logger *zap.SugaredLogger) Client {
	binary := os.Getenv(checkNRPEBinaryEnv)
	if binary == "" {
		binary = defaultCheckNRPEBinary
	}
	return &client{logger: logger, binary: binary}
}

type client struct {
	logger *zap.SugaredLogger
	binary string
}

// Execute runs check_nrpe with parameters derived from host and check config.
func (c *client) Execute(ctx context.Context, host *model.HostConfig, check *model.CheckConfig) (Result, error) {
	targetHost, targetPort := normalizeAddress(host)

	args := []string{"-H", targetHost, "-p", targetPort, "-c", check.Command}
	if check.ExecutionTimeoutSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", check.ExecutionTimeoutSec))
	}
	if len(check.Arguments) > 0 {
		args = append(args, "-a")
		args = append(args, check.Arguments...)
	}

	cmd := exec.CommandContext(ctx, c.binary, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode, execErr := osutil.ExitCodeFromError(err)
	if execErr != nil {

		return Result{}, fmt.Errorf("check_nrpe failed: %w; stderr: %s", execErr, strings.TrimSpace(stderrBuf.String()))
	}

	status := convertStatus(exitCode)
	output := strings.TrimSpace(stdoutBuf.String())
	if output == "" {
		output = strings.TrimSpace(stderrBuf.String())
	}

	c.logger.Debugf("nrpe executed host=%s check=%s status=%d output=%s", host.Hostname, check.Name, status, output)
	return Result{Output: output, Status: status}, nil
}

func normalizeAddress(host *model.HostConfig) (string, string) {
	target := host.Hostname
	if host.ResolveTo != "" {
		target = host.ResolveTo
	}
	port := "5666"
	if h, p, err := net.SplitHostPort(target); err == nil {
		target = h
		port = p
	}
	return target, port
}

func convertStatus(code int) Status {
	switch code {
	case 0:
		return StatusOK
	case 1:
		return StatusWarning
	case 2:
		return StatusCritical
	case 3:
		return StatusUnknown
	default:
		return StatusUnknown
	}
}
