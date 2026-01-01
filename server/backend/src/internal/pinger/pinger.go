package pinger

import (
	"context"
	"fmt"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	probing "github.com/prometheus-community/pro-bing"
	"go.uber.org/zap"
)

const (
	defaultPingTimeout = 3 * time.Second
	defaultPingCount   = 1
)

// Checker runs reachability checks against a host.
type Checker interface {
	Ping(ctx context.Context, host *model.HostConfig) (string, error)
}

// NewChecker builds a ping Checker using the pro-bing ICMP implementation.
func NewChecker(logger *zap.SugaredLogger) Checker {
	return &probingChecker{logger: logger}
}

type probingChecker struct {
	logger *zap.SugaredLogger
}

func (p *probingChecker) Ping(ctx context.Context, host *model.HostConfig) (string, error) {
	target := host.Hostname
	if host.ResolveTo != "" {
		target = host.ResolveTo
	}

	pinger, err := probing.NewPinger(target)
	if err != nil {
		return "", fmt.Errorf("create pinger: %w", err)
	}

	pinger.Count = defaultPingCount
	pinger.Timeout = defaultPingTimeout
	pinger.Interval = defaultPingTimeout
	pinger.SetPrivileged(false)

	runErr := make(chan error, 1)
	go func() {
		runErr <- pinger.Run()
	}()

	select {
	case err := <-runErr:
		if err != nil {
			return "", fmt.Errorf("ping failed: %w", err)
		}
	case <-ctx.Done():
		pinger.Stop()
		<-runErr
		return "", fmt.Errorf("ping canceled: %w", ctx.Err())
	}

	stats := pinger.Statistics()
	output := fmt.Sprintf("sent=%d recv=%d loss=%.1f%% avg_rtt=%s", stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss, stats.AvgRtt)
	if stats.PacketsRecv == 0 {
		return output, fmt.Errorf("ping packet loss %.1f%%", stats.PacketLoss)
	}
	p.logger.Debugf("ping ok host=%s output=%s", target, output)
	return output, nil
}
