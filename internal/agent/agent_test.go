package agent

import (
	"strings"
	"testing"

	"helpassist/internal/ollama"
)

func TestTrimHistoryKeepsSmallConversation(t *testing.T) {
	a := &Agent{history: []ollama.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}}
	a.trimHistory()
	if len(a.history) != 3 {
		t.Fatalf("small conversation was trimmed: %d messages left", len(a.history))
	}
}

func TestTrimHistoryDropsOldTurns(t *testing.T) {
	big := strings.Repeat("x", 20*1024) // 20 KB per assistant reply
	a := &Agent{history: []ollama.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "turn1"},
		{Role: "assistant", Content: big},
		{Role: "user", Content: "turn2"},
		{Role: "assistant", Content: big},
		{Role: "user", Content: "turn3"},
		{Role: "assistant", Content: big},
	}}
	a.trimHistory()

	// System prompt must survive.
	if a.history[0].Role != "system" {
		t.Fatalf("system prompt not kept, first role = %q", a.history[0].Role)
	}
	// Retained tail must begin at a user boundary (no split tool/assistant runs).
	if a.history[1].Role != "user" {
		t.Fatalf("history does not resume at a user message, got %q", a.history[1].Role)
	}
	// The most recent turn must always be present.
	last := a.history[len(a.history)-1]
	if last.Content != big {
		t.Error("most recent turn was dropped")
	}
	// Oldest turn should have been dropped to fit the budget.
	for _, m := range a.history {
		if m.Content == "turn1" {
			t.Error("oldest turn was not trimmed")
		}
	}
}

func TestTrimHistoryNeverSplitsToolSequence(t *testing.T) {
	big := strings.Repeat("y", 30*1024)
	a := &Agent{history: []ollama.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "old"},
		{Role: "assistant", Content: big},
		{Role: "user", Content: "recent"},
		{Role: "assistant", ToolCalls: []ollama.ToolCall{{Function: ollama.ToolCallFunction{Name: "read_file"}}}},
		{Role: "tool", ToolName: "read_file", Content: "data"},
		{Role: "assistant", Content: "done"},
	}}
	a.trimHistory()

	// A tool message must never be the first kept message after the system prompt.
	if len(a.history) > 1 && a.history[1].Role == "tool" {
		t.Fatal("trim left a dangling tool message at the head")
	}
}
