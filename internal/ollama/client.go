// Package ollama is a minimal client for the local Ollama HTTP API.
// It speaks the /api/chat endpoint with streaming (NDJSON) and native tool calls,
// using only the Go standard library.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is a single chat message. Role is one of: system, user, assistant, tool.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// Thinking holds qwen3 reasoning text when think is enabled (Ollama field "thinking").
	Thinking string `json:"thinking,omitempty"`
	// ToolName is set on role=="tool" replies so the model knows which call this answers.
	ToolName string `json:"tool_name,omitempty"`
}

// ToolCall is a function invocation requested by the model.
type ToolCall struct {
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the called function name and its decoded arguments.
type ToolCallFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Tool advertises a callable function to the model (JSON-schema parameters).
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes one tool: its name, purpose and parameter schema.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Client talks to a single Ollama instance.
type Client struct {
	Host  string
	Model string
	http  *http.Client
}

// New builds a client from env: OLLAMA_HOST and HELPASSIST_MODEL, with sane defaults.
func New() *Client {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	if !strings.HasPrefix(host, "http") {
		host = "http://" + host
	}
	model := os.Getenv("HELPASSIST_MODEL")
	if model == "" {
		model = "qwen3:8b"
	}
	return &Client{
		Host:  strings.TrimRight(host, "/"),
		Model: model,
		// No overall timeout: local generation can be slow; rely on context cancellation.
		http: &http.Client{},
	}
}

// chatRequest is the body of POST /api/chat.
type chatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Tools    []Tool         `json:"tools,omitempty"`
	Stream   bool           `json:"stream"`
	Think    bool           `json:"think"`
	Options  map[string]any `json:"options,omitempty"`
}

// chatChunk is one streamed object from /api/chat.
type chatChunk struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
	Error   string  `json:"error"`
}

// Chat streams a completion. Generated text is forwarded to onToken as it arrives.
// It returns the full assistant message (content + any tool calls).
//
// think enables qwen3's reasoning; when true the reasoning text is streamed to onThink.
func (c *Client) Chat(ctx context.Context, messages []Message, tools []Tool, think bool, onToken func(string), onThink func(string)) (Message, error) {
	reqBody := chatRequest{
		Model:    c.Model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
		Think:    think,
		Options:  map[string]any{"temperature": 0.4},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return Message{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Host+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return Message{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Message{}, fmt.Errorf("не удалось связаться с Ollama (%s): %w", c.Host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := bufio.NewReader(resp.Body).ReadString('\n')
		return Message{}, fmt.Errorf("Ollama вернул %s: %s", resp.Status, strings.TrimSpace(body))
	}

	out := Message{Role: "assistant"}
	var content strings.Builder
	dec := json.NewDecoder(resp.Body)
	for {
		var chunk chatChunk
		if err := dec.Decode(&chunk); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			out.Content = content.String()
			return out, fmt.Errorf("чтение потока Ollama: %w", err)
		}
		if chunk.Error != "" {
			out.Content = content.String()
			return out, fmt.Errorf("Ollama: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onToken != nil {
				onToken(chunk.Message.Content)
			}
		}
		if think && onThink != nil && chunk.Message.Thinking != "" {
			onThink(chunk.Message.Thinking)
		}
		// Tool calls arrive accumulated in chunks; collect them all.
		out.ToolCalls = append(out.ToolCalls, chunk.Message.ToolCalls...)
		if chunk.Done {
			break
		}
	}
	out.Content = content.String()
	return out, nil
}

// Ping checks that the Ollama server is reachable and the model is present.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Host+"/api/tags", nil)
	if err != nil {
		return err
	}
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama недоступен на %s: %w", c.Host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama /api/tags вернул %s", resp.Status)
	}
	return nil
}
