package agent

import (
	"sync"

	"github.com/anthropics/claude-code-go/internal/api"
)

// Conversation manages the message history for a conversation
type Conversation struct {
	messages   []api.Message
	systemMsg  string
	mu         sync.RWMutex
}

// NewConversation creates a new conversation
func NewConversation(systemMsg string) *Conversation {
	return &Conversation{
		messages:  make([]api.Message, 0),
		systemMsg: systemMsg,
	}
}

// AddMessage adds a message to the conversation
func (c *Conversation) AddMessage(msg api.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

// AddUserMessage adds a user text message
func (c *Conversation) AddUserMessage(text string) {
	c.AddMessage(api.NewTextMessage(api.RoleUser, text))
}

// AddAssistantMessage adds an assistant message with content blocks
func (c *Conversation) AddAssistantMessage(content []api.Content) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, api.Message{
		Role:    api.RoleAssistant,
		Content: content,
	})
}

// AddToolResult adds a tool result message
func (c *Conversation) AddToolResult(toolUseID string, result string, isError bool) {
	c.AddMessage(api.NewToolResultMessage(toolUseID, result, isError))
}

// AddToolResults adds multiple tool results as a single message
func (c *Conversation) AddToolResults(results []api.Content) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, api.Message{
		Role:    api.RoleUser,
		Content: results,
	})
}

// GetMessages returns a copy of all messages
func (c *Conversation) GetMessages() []api.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	messages := make([]api.Message, len(c.messages))
	copy(messages, c.messages)
	return messages
}

// GetSystemMessage returns the system message
func (c *Conversation) GetSystemMessage() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.systemMsg
}

// SetSystemMessage sets the system message
func (c *Conversation) SetSystemMessage(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.systemMsg = msg
}

// Clear removes all messages from the conversation
func (c *Conversation) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = make([]api.Message, 0)
}

// MessageCount returns the number of messages
func (c *Conversation) MessageCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}

// LastMessage returns the last message, if any
func (c *Conversation) LastMessage() *api.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.messages) == 0 {
		return nil
	}
	msg := c.messages[len(c.messages)-1]
	return &msg
}
