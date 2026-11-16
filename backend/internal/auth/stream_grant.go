package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type streamGrant struct {
	userID    string
	expiresAt time.Time
}

// StreamGrantManager issues short-lived single-use grants for streaming endpoints (SSE / WS).
type StreamGrantManager struct {
	mu     sync.Mutex
	grants map[string]streamGrant
	ttl    time.Duration
}

// NewStreamGrantManager creates a new StreamGrantManager.
func NewStreamGrantManager(ttl time.Duration) *StreamGrantManager {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	m := &StreamGrantManager{
		grants: make(map[string]streamGrant),
		ttl:    ttl,
	}
	go m.cleanupLoop()
	return m
}

// IssueGrant creates a short-lived random single-use grant token for the user.
func (m *StreamGrantManager) IssueGrant(userID string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	grant := "grant_" + hex.EncodeToString(b)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.grants[grant] = streamGrant{
		userID:    userID,
		expiresAt: time.Now().Add(m.ttl),
	}
	return grant, nil
}

// ValidateAndConsume validates the grant and removes it immediately (single-use).
func (m *StreamGrantManager) ValidateAndConsume(grant string) (string, bool) {
	if grant == "" {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.grants[grant]
	if !ok {
		return "", false
	}
	delete(m.grants, grant)
	if time.Now().After(g.expiresAt) {
		return "", false
	}
	return g.userID, true
}

func (m *StreamGrantManager) cleanupLoop() {
	ticker := time.NewTicker(m.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for k, v := range m.grants {
			if now.After(v.expiresAt) {
				delete(m.grants, k)
			}
		}
		m.mu.Unlock()
	}
}
