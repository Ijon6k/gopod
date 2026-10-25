package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogManager handles deployment log file storage and real-time streaming subscriptions.
type LogManager struct {
	baseDir string
	mu      sync.RWMutex
	filesMu sync.Mutex
	writers map[string]*logWriter
	// subscribers maps a deployment to its live SSE channels. Entries exist
	// only while the stream runs; CloseSubscribers removes and closes them
	// when the deployment ends.
	subscribers map[string][]chan string
	// closedDeployments remembers deployments whose subscriber fan-out has
	// already been terminated. Without this, an SSE client connecting right
	// after a deployment finished would register a brand new channel that no
	// one ever closes again (a goroutine leak for every streaming consumer).
	closedDeployments map[string]struct{}
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
		baseDir:           dir,
		writers:           make(map[string]*logWriter),
		subscribers:       make(map[string][]chan string),
		closedDeployments: make(map[string]struct{}),
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
	// Backstop: the handler validates ids too, but the file path must never be
	// reachable with a crafted id even if a new caller forgets.
	if !ValidDeploymentLogID(deploymentID) {
		return "", fmt.Errorf("invalid deployment id")
	}
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

// CloseSubscribers terminates every live log stream for a deployment: each
// subscriber channel is closed (SSE handlers observe the `!ok` receive and
// return) and the map entries are dropped. Called when a deployment reaches a
// terminal state, so no goroutine parks on a channel that will never carry
// another line.
//
// It is double-close-safe: unlike unsubscribe it removes the entries it
// closes, so a returning handler's deferred cleanup cannot close them again.
// LogLine sends stay race-free because senders hold the read lock while
// CloseSubscribers takes the write lock.
func (m *LogManager) CloseSubscribers(deploymentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if subs, ok := m.subscribers[deploymentID]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(m.subscribers, deploymentID)
	}
	// Tombstone even when nobody was connected: an SSE client connecting right
	// after the fan-out ended must not register a channel that no one closes.
	m.closedDeployments[deploymentID] = struct{}{}
}

// Subscribe returns a channel that receives live log lines and an
// unsubscribe function. Subscribing to a deployment whose stream already
// ended returns a pre-closed channel; the SSE handler treats the first
// `!ok` receive as end-of-stream.
func (m *LogManager) Subscribe(deploymentID string) (<-chan string, func()) {
	m.mu.Lock()
	_, ended := m.closedDeployments[deploymentID]
	var ch chan string
	if !ended {
		ch = make(chan string, 100)
		m.subscribers[deploymentID] = append(m.subscribers[deploymentID], ch)
	}
	m.mu.Unlock()

	if ended {
		closed := make(chan string)
		close(closed)
		return closed, func() {}
	}

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
// streamLines has no Scanner-style token limit and always drains the reader,
// so an overlong build-output line can neither abort the pipe nor leave the
// producer blocked on write().
func (m *LogManager) PipeReader(deploymentID string, r io.Reader) {
	if err := streamLines(r, func(text string) {
		m.LogLine(deploymentID, text)
	}); err != nil && err != io.EOF {
		m.LogLine(deploymentID, "[log error] "+err.Error())
	}
	_, _ = io.Copy(io.Discard, r)
}
