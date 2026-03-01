package alerts

import "context"

type Notifier interface {
	Send(ctx context.Context, message string) error
}

type NopNotifier struct{}

func (n *NopNotifier) Send(_ context.Context, _ string) error {
	return nil
}
