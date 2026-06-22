// Package ai provides a thin, reusable client for communicating with Ollama.
// Identical across all bootcamp projects
package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	DefaultModel  = "llama3.2:3b"
	ollamaBaseURL = "http://localhost:11434"
	chatEndpoint  = ollamaBaseURL + "/api/chat"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"` // added in Project 06, unused here
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type streamChunk struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// Chat sends messages to Ollama and returns the complete response as a string.
func Chat(model string, messages []Message) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	resp, err := http.Post(chatEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: ollama unreachable — is `ollama serve` running? %w", err)
	}
	defer resp.Body.Close()

	var result streamChunk
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ai: decode response: %w", err)
	}
	return result.Message.Content, nil
}

// ChatStream calls onChunk for every token as it arrives.
func ChatStream(model string, messages []Message, onChunk func(string) error) error {
	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("ai: marshal request: %w", err)
	}

	resp, err := http.Post(chatEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ai: ollama unreachable — is `ollama serve` running? %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk streamChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			if err := onChunk(chunk.Message.Content); err != nil {
				return nil
			}
		}
		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}
