package alerts

import (
	"context"
	"strings"

	"go.uber.org/zap"
)

type LoggerNotifier struct {
	logger *zap.SugaredLogger
}

func NewLoggerNotifier(logger *zap.SugaredLogger) *LoggerNotifier {
	if logger == nil {
		return nil
	}
	return &LoggerNotifier{logger: logger}
}

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
