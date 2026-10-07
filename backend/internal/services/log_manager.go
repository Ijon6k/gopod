package services

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogManager handles deployment log file storage and real-time streaming subscriptions.
type LogManager struct {
	baseDir     string
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

// NewLogManager creates a new log manager storing logs in baseDir/deployments.
func NewLogManager(baseDir string) *LogManager {
	dir := filepath.Join(baseDir, "deployments")
	_ = os.MkdirAll(dir, 0755)
	return &LogManager{
		baseDir:     dir,
		subscribers: make(map[string][]chan string),
	}
}

// LogLine writes a single log line to disk with timestamp and broadcasts to live listeners.
func (m *LogManager) LogLine(deploymentID, line string) {
	timeStr := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("[%s] %s\n", timeStr, line)

	logFilePath := filepath.Join(m.baseDir, fmt.Sprintf("%s.log", deploymentID))
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString(formatted)
		_ = f.Close()
	}

	m.mu.RLock()
	subs, exists := m.subscribers[deploymentID]
	if exists {
		for _, ch := range subs {
			select {
			case ch <- formatted:
			default:
			}
		}
	}
	m.mu.RUnlock()
}

// GetLogs reads complete recorded logs for a given deployment.
func (m *LogManager) GetLogs(deploymentID string) (string, error) {
	logFilePath := filepath.Join(m.baseDir, fmt.Sprintf("%s.log", deploymentID))
	data, err := os.ReadFile(logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// Subscribe returns a channel that receives live log lines and an unsubscribe function.
func (m *LogManager) Subscribe(deploymentID string) (<-chan string, func()) {
	ch := make(chan string, 100)
	m.mu.Lock()
	m.subscribers[deploymentID] = append(m.subscribers[deploymentID], ch)
	m.mu.Unlock()

	unsubscribe := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		subs := m.subscribers[deploymentID]
		for i, sub := range subs {
			if sub == ch {
				m.subscribers[deploymentID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, unsubscribe
}

// PipeReader reads lines from an io.Reader and sends each to LogLine.
func (m *LogManager) PipeReader(deploymentID string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		m.LogLine(deploymentID, scanner.Text())
	}
}
