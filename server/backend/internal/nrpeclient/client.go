package nrpeclient

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpe/protocol"
	"github.com/drunkbatya/drnkexec/internal/nrpe/tlsdial"
	"go.uber.org/zap"
)

type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusCritical
	StatusUnknown
)

type Result struct {
	Output string
	Status Status
}

type Client interface {
	Execute(ctx context.Context, host *model.HostConfig, check *model.CheckConfig) (Result, error)
}

type client struct {
	logger *zap.SugaredLogger
	dialer *net.Dialer
}

func NewClient(logger *zap.SugaredLogger) Client {
	return &client{logger: logger, dialer: &net.Dialer{}}
}

func (c *client) Execute(ctx context.Context, host *model.HostConfig, check *model.CheckConfig) (Result, error) {
	target := host.Hostname
	if host.ResolveTo != "" {
		target = host.ResolveTo
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		target = net.JoinHostPort(target, "5666")
	}
	conn, err := c.dialNRPE(ctx, host, target)
	if err != nil {
		return Result{}, err
	}
	defer conn.Close()
	timeout := time.Duration(check.ExecutionTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return Result{}, err
	}
	raw, err := protocol.BuildRequest(check.Command, check.Arguments)
	if err != nil {
		return Result{}, err
	}
	if _, err := conn.Write(raw); err != nil {
		return Result{}, err
	}
	parsed, err := c.readResponse(conn)
	if err != nil {
		return Result{}, err
	}
	output := protocol.PayloadString(parsed)
	status := mapStatus(parsed.Result)
	c.logger.Debugf("nrpe executed host=%s check=%s status=%d output=%s", host.Hostname, check.Name, status, output)
	return Result{Output: output, Status: status}, nil
}

func (c *client) dialNRPE(ctx context.Context, host *model.HostConfig, target string) (net.Conn, error) {
	if !tlsEnabled(host) {
		return c.dialer.DialContext(ctx, "tcp", target)
	}
	conn, err := tlsdial.Dial(ctx, target, 0)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func tlsEnabled(host *model.HostConfig) bool {
	if host != nil && host.Nrpe.TLS.Enabled != nil {
		return *host.Nrpe.TLS.Enabled
	}
	return true
}

func (c *client) readResponse(conn net.Conn) (protocol.Packet, error) {
	header := make([]byte, protocol.HeaderLength())
	if _, err := io.ReadFull(conn, header); err != nil {
		return protocol.Packet{}, err
	}
	bufferLen := int(binary.BigEndian.Uint32(header[12:16]))
	if bufferLen <= 0 {
		return protocol.Packet{}, errors.New("invalid buffer length")
	}
	rest := make([]byte, bufferLen)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return protocol.Packet{}, err
	}
	packetBytes := append(header, rest...)
	return protocol.ParseResponse(packetBytes)
}

func mapStatus(s protocol.Status) Status {
	switch s {
	case protocol.StatusOK:
		return StatusOK
	case protocol.StatusWarning:
		return StatusWarning
	case protocol.StatusCritical:
		return StatusCritical
	default:
		return StatusUnknown
	}
}
