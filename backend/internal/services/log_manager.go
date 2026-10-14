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
	filesMu     sync.Mutex
	writers     map[string]*logWriter
	subscribers map[string][]chan string
}

type logWriter struct {
	file  *os.File
	timer *time.Timer
}

// NewLogManager creates a new log manager storing logs in baseDir/deployments.
func NewLogManager(baseDir string) *LogManager {
	dir := filepath.Join(baseDir, "deployments")
	_ = os.MkdirAll(dir, 0755)
	return &LogManager{
		baseDir:     dir,
		writers:     make(map[string]*logWriter),
		subscribers: make(map[string][]chan string),
	}
}

func (m *LogManager) getWriter(deploymentID string) (*os.File, error) {
	m.filesMu.Lock()
	defer m.filesMu.Unlock()

	if lw, ok := m.writers[deploymentID]; ok {
		if lw.timer != nil {
			lw.timer.Stop()
		}
		lw.timer = time.AfterFunc(5*time.Second, func() {
			m.CloseLog(deploymentID)
		})
		return lw.file, nil
	}

	logFilePath := filepath.Join(m.baseDir, fmt.Sprintf("%s.log", deploymentID))
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	lw := &logWriter{file: f}
	lw.timer = time.AfterFunc(5*time.Second, func() {
		m.CloseLog(deploymentID)
	})
	m.writers[deploymentID] = lw
	return f, nil
}

// CloseLog safely flushes and closes open file handles for a deployment.
func (m *LogManager) CloseLog(deploymentID string) {
	m.filesMu.Lock()
	defer m.filesMu.Unlock()

	if lw, ok := m.writers[deploymentID]; ok {
		if lw.timer != nil {
			lw.timer.Stop()
		}
		_ = lw.file.Sync()
		_ = lw.file.Close()
		delete(m.writers, deploymentID)
	}
}

// LogLine writes a single log line to disk with timestamp and broadcasts to live listeners.
func (m *LogManager) LogLine(deploymentID, line string) {
	timeStr := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("[%s] %s\n", timeStr, line)

	if f, err := m.getWriter(deploymentID); err == nil {
		_, _ = f.WriteString(formatted)
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
	if err := scanner.Err(); err != nil && err != io.EOF {
		m.LogLine(deploymentID, "[log error] "+err.Error())
	}
}
