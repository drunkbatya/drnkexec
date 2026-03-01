package alerts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/drunkbatya/drnkexec/internal/model"
	"github.com/drunkbatya/drnkexec/internal/nrpeclient"
	"go.uber.org/zap"
)

type Manager struct {
	notifiers      []Notifier
	logger         *zap.SugaredLogger
	repeatInterval time.Duration

	mu        sync.Mutex
	lastAlert map[string]time.Time
	active    map[string]bool
}

func NewManager(logger *zap.SugaredLogger, notifiers []Notifier, repeatInterval time.Duration) *Manager {
	filtered := notifiers
	if len(filtered) == 0 {
		filtered = []Notifier{&NopNotifier{}}
	}
	return &Manager{
		notifiers:      filtered,
		logger:         logger,
		repeatInterval: repeatInterval,
		lastAlert:      make(map[string]time.Time),
		active:         make(map[string]bool),
	}
}

func (m *Manager) Alert(ctx context.Context, assignment model.CheckAssignment, output string, result nrpeclient.Result) {
	key := m.keyFor(assignment)
	if !m.shouldSendAlert(key) {
		return
	}
	text := fmt.Sprintf("%s for %s is %s: %s",
		assignment.Check.Name, assignment.Host.Hostname, result.Status, output)
	m.dispatch(ctx, assignment, text)
}

func (m *Manager) Resolve(ctx context.Context, assignment model.CheckAssignment, output string, result nrpeclient.Result) {
	key := m.keyFor(assignment)

	m.mu.Lock()
	if !m.active[key] {
		m.mu.Unlock()
		return
	}
	delete(m.active, key)
	m.mu.Unlock()

	text := fmt.Sprintf("[RESOLVED] host=%s (%s) check=%s command=%s output=%s",
		assignment.Host.Name, assignment.Host.Hostname, assignment.Check.Name, assignment.Check.Command, output)
	m.dispatch(ctx, assignment, text)
}

func (m *Manager) dispatch(ctx context.Context, assignment model.CheckAssignment, text string) {
	for _, notifier := range m.notifiers {
		if notifier == nil {
			continue
		}
		if err := notifier.Send(ctx, text); err != nil {
			m.logger.Warnf("notifier send failed host=%s check=%s error=%v", assignment.Host.Hostname, assignment.Check.Name, err)
		}
	}
	m.logger.Debugf("dispatched notification host=%s check=%s", assignment.Host.Hostname, assignment.Check.Name)
}

func (m *Manager) shouldSendAlert(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active[key] {
		m.active[key] = true
	}
	now := time.Now()
	last := m.lastAlert[key]
	if now.Sub(last) < m.repeatInterval {
		return false
	}
	m.lastAlert[key] = now
	return true
}

func (m *Manager) keyFor(assignment model.CheckAssignment) string {
	return fmt.Sprintf("%s::%s", assignment.Host.Hostname, assignment.Check.Name)
}
