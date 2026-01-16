package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner provides a loading animation
type Spinner struct {
	frames   []string
	interval time.Duration
	message  string
	running  bool
	stopCh   chan struct{}
	mu       sync.Mutex
}

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	return &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		interval: 80 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the spinner with a message
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.message = message
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	// Clear the spinner line
	fmt.Print("\r\033[K")
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

func (s *Spinner) run() {
	frameIdx := 0
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			message := s.message
			s.mu.Unlock()

			frame := s.frames[frameIdx]
			fmt.Printf("\r\033[K%s %s", frame, message)
			frameIdx = (frameIdx + 1) % len(s.frames)
		}
	}
}

// IsRunning returns true if the spinner is running
func (s *Spinner) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
