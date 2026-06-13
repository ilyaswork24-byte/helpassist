// Package tools defines the agent's callable tools and a registry that maps
// model tool calls to their Go implementations.
package tools

import (
	"context"
	"fmt"

	"helpassist/internal/ollama"
)

// Tool is one capability the model can invoke.
type Tool interface {
	// Name is the function name advertised to the model.
	Name() string
	// Schema returns the JSON-schema description sent to Ollama.
	Schema() ollama.Tool
	// Run executes the call with decoded arguments and returns text for the model.
	// ctx carries the current turn's cancellation (e.g. Ctrl-C).
	Run(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the available tools and shared dependencies.
type Registry struct {
	tools   map[string]Tool
	order   []string
	confirm *Confirmer
}

// NewRegistry builds the default tool set rooted at the current working dir.
func NewRegistry(confirm *Confirmer) *Registry {
	r := &Registry{tools: map[string]Tool{}, confirm: confirm}
	r.add(&readFile{})
	r.add(&listDir{})
	r.add(&globTool{})
	r.add(&grepTool{})
	r.add(&writeFile{confirm: confirm})
	r.add(&editFile{confirm: confirm})
	r.add(&runBash{confirm: confirm})
	return r
}

func (r *Registry) add(t Tool) {
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

// Schemas returns all tool schemas in registration order.
func (r *Registry) Schemas() []ollama.Tool {
	out := make([]ollama.Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name].Schema())
	}
	return out
}

// Dispatch runs a tool call by name and returns its textual result.
// Unknown tools return an error string (not a Go error) so the model can recover.
func (r *Registry) Dispatch(ctx context.Context, call ollama.ToolCall) (string, error) {
	t, ok := r.tools[call.Function.Name]
	if !ok {
		return "", fmt.Errorf("неизвестный инструмент %q", call.Function.Name)
	}
	return t.Run(ctx, call.Function.Arguments)
}

// argString extracts a string argument, tolerating numbers/bools coerced by JSON.
func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", s)
	}
}

// argBool extracts a boolean argument with a default.
func argBool(args map[string]any, key string, def bool) bool {
	v, ok := args[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// schema is a tiny helper to build JSON-schema parameter objects.
func schema(props map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}
