package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamReader reads SSE events from the API
type StreamReader struct {
	reader   *bufio.Reader
	body     io.ReadCloser
	closed   bool
	response *MessagesResponse
}

// NewStreamReader creates a new stream reader
func NewStreamReader(body io.ReadCloser) *StreamReader {
	return &StreamReader{
		reader: bufio.NewReader(body),
		body:   body,
		response: &MessagesResponse{
			Content: make([]Content, 0),
		},
	}
}

// StreamChunk represents a chunk of streamed data
type StreamChunk struct {
	Type         string   // "text", "tool_use_start", "tool_use_delta", "content_block_stop", "message_stop", "error"
	Text         string   // For text chunks
	ContentBlock *Content // For tool use starts
	Index        int      // Content block index
	PartialJSON  string   // For tool use input deltas
	StopReason   string   // For message stop
	Error        error    // For errors
}

// Next reads the next event from the stream
func (s *StreamReader) Next() (*StreamChunk, error) {
	if s.closed {
		return nil, io.EOF
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.Close()
			}
			return nil, err
		}

		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse SSE data
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				s.Close()
				return nil, io.EOF
			}

			chunk, err := s.parseEvent(data)
			if err != nil {
				return &StreamChunk{Type: "error", Error: err}, nil
			}
			if chunk != nil {
				return chunk, nil
			}
		}

		// Skip event type lines
		if strings.HasPrefix(line, "event: ") {
			continue
		}
	}
}

func (s *StreamReader) parseEvent(data string) (*StreamChunk, error) {
	var event StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}

	switch event.Type {
	case "message_start":
		// Initialize response from message
		if event.Message != nil {
			var msg MessagesResponse
			if err := json.Unmarshal(event.Message, &msg); err == nil {
				s.response.ID = msg.ID
				s.response.Model = msg.Model
				s.response.Role = msg.Role
			}
		}
		return nil, nil

	case "content_block_start":
		if event.ContentBlock != nil {
			// Ensure we have enough capacity
			for len(s.response.Content) <= event.Index {
				s.response.Content = append(s.response.Content, Content{})
			}
			s.response.Content[event.Index] = *event.ContentBlock

			if event.ContentBlock.Type == ContentTypeToolUse {
				return &StreamChunk{
					Type:         "tool_use_start",
					ContentBlock: event.ContentBlock,
					Index:        event.Index,
				}, nil
			}
		}
		return nil, nil

	case "content_block_delta":
		if event.Delta != nil {
			// Handle text delta
			if event.Delta.Text != "" {
				if event.Index < len(s.response.Content) {
					s.response.Content[event.Index].Text += event.Delta.Text
				}
				return &StreamChunk{
					Type:  "text",
					Text:  event.Delta.Text,
					Index: event.Index,
				}, nil
			}

			// Handle tool input delta
			if event.Delta.PartialJSON != "" {
				return &StreamChunk{
					Type:        "tool_use_delta",
					PartialJSON: event.Delta.PartialJSON,
					Index:       event.Index,
				}, nil
			}
		}
		return nil, nil

	case "content_block_stop":
		return &StreamChunk{
			Type:  "content_block_stop",
			Index: event.Index,
		}, nil

	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason != "" {
			s.response.StopReason = event.Delta.StopReason
		}
		if event.Usage != nil {
			s.response.Usage.OutputTokens = event.Usage.OutputTokens
		}
		return nil, nil

	case "message_stop":
		return &StreamChunk{
			Type:       "message_stop",
			StopReason: s.response.StopReason,
		}, nil

	case "ping":
		return nil, nil

	case "error":
		return &StreamChunk{
			Type:  "error",
			Error: fmt.Errorf("stream error: %s", data),
		}, nil
	}

	return nil, nil
}

// GetResponse returns the accumulated response
func (s *StreamReader) GetResponse() *MessagesResponse {
	return s.response
}

// Close closes the stream
func (s *StreamReader) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.body.Close()
}
