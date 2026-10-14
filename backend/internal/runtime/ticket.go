package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type terminalTicket struct {
	containerID string
	expiresAt   time.Time
}

// TerminalTicketManager manages short-lived, single-use tickets for container WebSocket exec sessions.
type TerminalTicketManager struct {
	mu      sync.Mutex
	tickets map[string]terminalTicket
	ttl     time.Duration
}

// NewTerminalTicketManager creates a new TerminalTicketManager instance.
func NewTerminalTicketManager(ttl time.Duration) *TerminalTicketManager {
	tm := &TerminalTicketManager{
		tickets: make(map[string]terminalTicket),
		ttl:     ttl,
	}
	go tm.cleanupLoop()
	return tm
}

// IssueTicket issues a short-lived single-use ticket for the specified container.
func (m *TerminalTicketManager) IssueTicket(containerID string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := "tkt_" + hex.EncodeToString(b)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[ticket] = terminalTicket{
		containerID: containerID,
		expiresAt:   time.Now().Add(m.ttl),
	}
	return ticket, nil
}

// ValidateAndConsume validates the ticket and removes it immediately (single-use).
func (m *TerminalTicketManager) ValidateAndConsume(ticket, containerID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.tickets[ticket]
	if !exists {
		return false
	}
	delete(m.tickets, ticket) // single-use: consumed immediately

	if time.Now().After(t.expiresAt) {
		return false
	}
	return t.containerID == containerID || containerID == ""
}

func (m *TerminalTicketManager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.tickets {
			if now.After(v.expiresAt) {
				delete(m.tickets, k)
			}
		}
		m.mu.Unlock()
	}
}
