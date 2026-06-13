// Package agent runs the conversation loop between the model and the tools.
package agent

import (
	"context"
	"fmt"
	"strings"

	"helpassist/internal/ollama"
	"helpassist/internal/tools"
	"helpassist/internal/ui"
)

// maxToolIterations caps tool round-trips per user turn to avoid runaway loops.
const maxToolIterations = 25

// maxHistoryBytes bounds the conversation size (sum of message content) kept in
// context. When exceeded, the oldest whole turns are dropped (system prompt kept).
const maxHistoryBytes = 48 * 1024

// Agent owns the chat history and drives model/tool interaction.
type Agent struct {
	client   *ollama.Client
	registry *tools.Registry
	history  []ollama.Message
	think    bool
}

// New creates an agent with a fresh conversation seeded by the system prompt.
func New(client *ollama.Client, registry *tools.Registry, think bool) *Agent {
	return &Agent{
		client:   client,
		registry: registry,
		think:    think,
		history:  []ollama.Message{{Role: "system", Content: systemPrompt()}},
	}
}

// SetThink toggles qwen3 reasoning for subsequent turns.
func (a *Agent) SetThink(on bool) { a.think = on }

// Think reports the current reasoning setting.
func (a *Agent) Think() bool { return a.think }

// Reset clears the conversation, keeping a fresh system prompt.
func (a *Agent) Reset() {
	a.history = []ollama.Message{{Role: "system", Content: systemPrompt()}}
}

// Turn processes one user message: it streams assistant text and runs any tool
// calls until the model produces a final answer with no further calls.
func (a *Agent) Turn(ctx context.Context, userInput string) error {
	a.history = append(a.history, ollama.Message{Role: "user", Content: userInput})
	a.trimHistory()

	for i := 0; i < maxToolIterations; i++ {
		printedText := false
		onToken := func(s string) {
			if !printedText {
				fmt.Print(ui.Cyan("helpassist") + " ")
				printedText = true
			}
			fmt.Print(s)
		}
		thinkingOpen := false
		onThink := func(s string) {
			if !thinkingOpen {
				fmt.Print(ui.Gray("💭 "))
				thinkingOpen = true
			}
			fmt.Print(ui.Gray(s))
		}

		msg, err := a.client.Chat(ctx, a.history, a.registry.Schemas(), a.think, onToken, onThink)
		if thinkingOpen {
			fmt.Println()
		}
		if printedText {
			fmt.Println()
		}
		if err != nil {
			return err
		}

		a.history = append(a.history, msg)

		// No tool calls => this is the final assistant answer for the turn.
		if len(msg.ToolCalls) == 0 {
			// If the model emitted neither text nor tool calls, nudge once.
			if strings.TrimSpace(msg.Content) == "" && !printedText {
				ui.Info("(пустой ответ модели)")
			}
			return nil
		}

		// Execute each requested tool and feed results back into the history.
		for _, call := range msg.ToolCalls {
			result, err := a.registry.Dispatch(ctx, call)
			if err != nil {
				result = "Ошибка инструмента: " + err.Error()
				ui.Error("%s: %v", call.Function.Name, err)
			}
			a.history = append(a.history, ollama.Message{
				Role:     "tool",
				ToolName: call.Function.Name,
				Content:  result,
			})
		}
	}

	ui.Error("достигнут лимит итераций инструментов (%d)", maxToolIterations)
	return nil
}

// trimHistory drops the oldest whole turns when the conversation grows past
// maxHistoryBytes. A turn spans from one user message up to (but excluding) the
// next user message, so tool-call/result pairs are never split. The system
// prompt (index 0) and the most recent turn are always kept.
func (a *Agent) trimHistory() {
	total := 0
	for _, m := range a.history {
		total += len(m.Content)
	}
	if total <= maxHistoryBytes {
		return
	}

	// Indices of turn boundaries (user messages) after the system prompt.
	var starts []int
	for i := 1; i < len(a.history); i++ {
		if a.history[i].Role == "user" {
			starts = append(starts, i)
		}
	}
	if len(starts) <= 1 {
		return // only the current turn present; nothing safe to drop
	}

	// Keep as many trailing turns as fit; always keep the last one.
	turnSize := func(k int) int {
		end := len(a.history)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		sz := 0
		for j := starts[k]; j < end; j++ {
			sz += len(a.history[j].Content)
		}
		return sz
	}

	kept := 0
	keepFrom := starts[len(starts)-1]
	for k := len(starts) - 1; k >= 0; k-- {
		sz := turnSize(k)
		if k != len(starts)-1 && kept+sz > maxHistoryBytes {
			break
		}
		kept += sz
		keepFrom = starts[k]
	}

	if keepFrom == starts[0] {
		return // everything already fits within the budget
	}
	trimmed := make([]ollama.Message, 0, 1+len(a.history)-keepFrom)
	trimmed = append(trimmed, a.history[0])
	trimmed = append(trimmed, a.history[keepFrom:]...)
	a.history = trimmed
}
