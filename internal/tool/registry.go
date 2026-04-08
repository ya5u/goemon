package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ya5u/goemon/internal/llm"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolProvider dynamically supplies tools (e.g. from skills on disk).
type ToolProvider interface {
	Tools() []Tool
}

type Registry struct {
	tools     map[string]Tool
	providers []ToolProvider
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) RegisterProvider(p ToolProvider) {
	r.providers = append(r.providers, p)
}

func (r *Registry) Get(name string) (Tool, error) {
	if t, ok := r.tools[name]; ok {
		return t, nil
	}
	for _, p := range r.providers {
		for _, t := range p.Tools() {
			if t.Name() == name {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	for _, p := range r.providers {
		for _, t := range p.Tools() {
			defs = append(defs, llm.ToolDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			})
		}
	}
	return defs
}

func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	for _, p := range r.providers {
		tools = append(tools, p.Tools()...)
	}
	return tools
}
