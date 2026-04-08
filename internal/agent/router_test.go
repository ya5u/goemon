package agent

import (
	"context"
	"testing"

	"github.com/ya5u/goemon/internal/llm"
)

type mockBackend struct {
	name      string
	available bool
}

func (m *mockBackend) Name() string                       { return m.name }
func (m *mockBackend) IsAvailable(_ context.Context) bool { return m.available }
func (m *mockBackend) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition) (*llm.Response, error) {
	return &llm.Response{Content: "mock response"}, nil
}

func TestRouterSelectDefault(t *testing.T) {
	backends := map[string]llm.Backend{
		"ollama": &mockBackend{name: "ollama", available: true},
		"claude": &mockBackend{name: "claude", available: true},
	}
	r := NewRouter(RouterConfig{Default: "ollama", Fallback: "claude"}, backends)
	r.checkAll(context.Background())

	b, err := r.Select("")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "ollama" {
		t.Errorf("expected ollama, got %s", b.Name())
	}
}

func TestRouterFallback(t *testing.T) {
	backends := map[string]llm.Backend{
		"ollama": &mockBackend{name: "ollama", available: false},
		"claude": &mockBackend{name: "claude", available: true},
	}
	r := NewRouter(RouterConfig{Default: "ollama", Fallback: "claude"}, backends)
	r.checkAll(context.Background())

	b, err := r.Select("")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "claude" {
		t.Errorf("expected claude, got %s", b.Name())
	}
}

func TestRouterForceCloud(t *testing.T) {
	backends := map[string]llm.Backend{
		"ollama": &mockBackend{name: "ollama", available: true},
		"claude": &mockBackend{name: "claude", available: true},
	}
	r := NewRouter(RouterConfig{
		Default:       "ollama",
		Fallback:      "claude",
		ForceCloudFor: []string{"skill_creation"},
	}, backends)
	r.checkAll(context.Background())

	b, err := r.Select("skill_creation")
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "claude" {
		t.Errorf("expected claude for forced cloud, got %s", b.Name())
	}
}

func TestRouterNoneAvailable(t *testing.T) {
	backends := map[string]llm.Backend{
		"ollama": &mockBackend{name: "ollama", available: false},
		"claude": &mockBackend{name: "claude", available: false},
	}
	r := NewRouter(RouterConfig{Default: "ollama", Fallback: "claude"}, backends)
	r.checkAll(context.Background())

	_, err := r.Select("")
	if err == nil {
		t.Error("expected error when no backends available")
	}
}
