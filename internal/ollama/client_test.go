package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(handler http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return &Client{Host: srv.URL, Model: "test", http: srv.Client()}, srv
}

func TestChatStreamsContentAndToolCalls(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		// Three NDJSON chunks: two text fragments then a tool call + done.
		w.Write([]byte(`{"message":{"role":"assistant","content":"Hel"}}` + "\n"))
		w.Write([]byte(`{"message":{"role":"assistant","content":"lo"}}` + "\n"))
		w.Write([]byte(`{"message":{"role":"assistant","tool_calls":[{"function":{"name":"read_file","arguments":{"path":"x.go"}}}]},"done":true}` + "\n"))
	})
	defer srv.Close()

	var streamed strings.Builder
	msg, err := c.Chat(context.Background(), nil, nil, false,
		func(s string) { streamed.WriteString(s) }, nil)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	if msg.Content != "Hello" {
		t.Errorf("content = %q, want %q", msg.Content, "Hello")
	}
	if streamed.String() != "Hello" {
		t.Errorf("streamed = %q, want %q", streamed.String(), "Hello")
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "read_file" {
		t.Fatalf("tool calls = %+v, want one read_file call", msg.ToolCalls)
	}
	if got := msg.ToolCalls[0].Function.Arguments["path"]; got != "x.go" {
		t.Errorf("arg path = %v, want x.go", got)
	}
}

func TestChatReturnsStreamError(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"message":{"content":"partial"}}` + "\n"))
		w.Write([]byte(`{"error":"model exploded"}` + "\n"))
	})
	defer srv.Close()

	msg, err := c.Chat(context.Background(), nil, nil, false, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "model exploded") {
		t.Fatalf("expected stream error, got err=%v", err)
	}
	// Partial content accumulated before the error must be preserved.
	if msg.Content != "partial" {
		t.Errorf("partial content lost: %q", msg.Content)
	}
}

func TestChatHTTPError(t *testing.T) {
	c, srv := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer srv.Close()

	if _, err := c.Chat(context.Background(), nil, nil, false, nil, nil); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}
