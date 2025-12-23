package alerts

import "context"

// Notifier abstracts a downstream notification system.
type Notifier interface {
	Send(ctx context.Context, message string) error
}

// NopNotifier drops every message; useful when integration is not configured.
type NopNotifier struct{}

// Send implements Notifier.
func (n *NopNotifier) Send(_ context.Context, _ string) error {
	return nil
}
