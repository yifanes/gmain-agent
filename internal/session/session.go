package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/config"
)

// Session represents a saved conversation session
type Session struct {
	ID          string        `json:"id"`
	Name        string        `json:"name,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	WorkDir     string        `json:"work_dir"`
	Messages    []api.Message `json:"messages"`
	SystemPrompt string       `json:"system_prompt,omitempty"`
}

// SessionManager manages session persistence
type SessionManager struct {
	sessionDir string
}

// NewSessionManager creates a new session manager
func NewSessionManager() (*SessionManager, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}

	sessionDir := filepath.Join(configDir, "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &SessionManager{
		sessionDir: sessionDir,
	}, nil
}

// CreateSession creates a new session
func (m *SessionManager) CreateSession(workDir string) *Session {
	now := time.Now()
	return &Session{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		CreatedAt: now,
		UpdatedAt: now,
		WorkDir:   workDir,
		Messages:  make([]api.Message, 0),
	}
}

// SaveSession saves a session to disk
func (m *SessionManager) SaveSession(session *Session) error {
	session.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	filename := filepath.Join(m.sessionDir, session.ID+".json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk
func (m *SessionManager) LoadSession(id string) (*Session, error) {
	filename := filepath.Join(m.sessionDir, id+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// DeleteSession deletes a session
func (m *SessionManager) DeleteSession(id string) error {
	filename := filepath.Join(m.sessionDir, id+".json")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", id)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// ListSessions lists all saved sessions
func (m *SessionManager) ListSessions() ([]*Session, error) {
	files, err := os.ReadDir(m.sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		id := file.Name()[:len(file.Name())-5] // Remove .json extension
		session, err := m.LoadSession(id)
		if err != nil {
			continue // Skip invalid sessions
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetLatestSession returns the most recently updated session for a work directory
func (m *SessionManager) GetLatestSession(workDir string) (*Session, error) {
	sessions, err := m.ListSessions()
	if err != nil {
		return nil, err
	}

	var latest *Session
	for _, s := range sessions {
		if s.WorkDir != workDir {
			continue
		}
		if latest == nil || s.UpdatedAt.After(latest.UpdatedAt) {
			latest = s
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no session found for directory: %s", workDir)
	}

	return latest, nil
}

// AddMessages adds messages to a session
func (s *Session) AddMessages(messages ...api.Message) {
	s.Messages = append(s.Messages, messages...)
	s.UpdatedAt = time.Now()
}

// ClearMessages clears all messages from a session
func (s *Session) ClearMessages() {
	s.Messages = make([]api.Message, 0)
	s.UpdatedAt = time.Now()
}
