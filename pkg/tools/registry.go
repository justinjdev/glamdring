package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Registry holds all registered tools and dispatches calls by name.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	name := t.Name()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// Execute dispatches a tool call by name.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	t := r.tools[name]
	if t == nil {
		return Result{Output: fmt.Sprintf("unknown tool: %s", name), IsError: true}, nil
	}
	return t.Execute(ctx, input)
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if t, ok := r.tools[name]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Schemas returns the tool definitions for the API request in registration order.
func (r *Registry) Schemas() []json.RawMessage {
	out := make([]json.RawMessage, 0, len(r.order))
	for _, name := range r.order {
		t, ok := r.tools[name]
		if !ok {
			continue
		}
		schema := map[string]any{
			"name":         t.Name(),
			"description":  t.Description(),
			"input_schema": json.RawMessage(t.Schema()),
		}
		b, _ := json.Marshal(schema)
		out = append(out, b)
	}
	return out
}
