package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string]sessionData
	ttl      time.Duration
}

type sessionData struct {
	username  string
	expiresAt time.Time
}

func NewManager(ttl time.Duration) *Manager {
	return &Manager{
		sessions: make(map[string]sessionData),
		ttl:      ttl,
	}
}

func (m *Manager) Create(username string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	m.sessions[token] = sessionData{username: username, expiresAt: time.Now().Add(m.ttl)}
	return token, nil
}

func (m *Manager) Validate(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(data.expiresAt) {
		delete(m.sessions, token)
		return false
	}
	data.expiresAt = time.Now().Add(m.ttl)
	m.sessions[token] = data
	return true
}

func (m *Manager) Delete(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *Manager) cleanupLocked() {
	now := time.Now()
	for token, data := range m.sessions {
		if now.After(data.expiresAt) {
			delete(m.sessions, token)
		}
	}
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
