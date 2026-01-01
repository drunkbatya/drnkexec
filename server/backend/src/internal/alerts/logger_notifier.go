package alerts

import (
	"context"
	"strings"

	"go.uber.org/zap"
)

// LoggerNotifier writes alert messages into logs.
type LoggerNotifier struct {
	logger *zap.SugaredLogger
}

// NewLoggerNotifier builds a logger-backed notifier.
func NewLoggerNotifier(logger *zap.SugaredLogger) *LoggerNotifier {
	if logger == nil {
		return nil
	}
	return &LoggerNotifier{logger: logger}
}

// Send implements Notifier by logging at warn level for alerts and info for resolves.
func (l *LoggerNotifier) Send(_ context.Context, message string) error {
	if l == nil || l.logger == nil {
		return nil
	}
	if strings.HasPrefix(message, "[ALERT]") {
		l.logger.Warnf("%s", message)
	} else {
		l.logger.Infof("%s", message)
	}
	return nil
}
