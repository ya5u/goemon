package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ya5u/goemon/internal/llm"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/tool"
)

// mockLLMBackend simulates LLM responses for testing
type mockLLMBackend struct {
	responses []*llm.Response
	callIdx   int
}

func (m *mockLLMBackend) Name() string                       { return "mock" }
func (m *mockLLMBackend) IsAvailable(_ context.Context) bool { return true }

func (m *mockLLMBackend) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDefinition) (*llm.Response, error) {
	if m.callIdx >= len(m.responses) {
		return &llm.Response{Content: "done"}, nil
	}
	resp := m.responses[m.callIdx]
	m.callIdx++
	return resp, nil
}

func TestAgentSimpleResponse(t *testing.T) {
	store, err := memory.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mock := &mockLLMBackend{
		responses: []*llm.Response{
			{Content: "Hello! I'm GoEmon."},
		},
	}

	backends := map[string]llm.Backend{"mock": mock}
	router := NewRouter(RouterConfig{Default: "mock"}, backends)
	router.checkAll(context.Background())

	registry := tool.NewRegistry()
	ag := NewAgent(router, registry, store)

	result, err := ag.Run(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello! I'm GoEmon." {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestAgentToolCall(t *testing.T) {
	store, err := memory.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	mock := &mockLLMBackend{
		responses: []*llm.Response{
			{
				Content: "Let me check that.",
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call_1",
						Name:      "shell_exec",
						Arguments: json.RawMessage(`{"command":"echo hello from tool"}`),
					},
				},
			},
			{Content: "The command output 'hello from tool'."},
		},
	}

	backends := map[string]llm.Backend{"mock": mock}
	router := NewRouter(RouterConfig{Default: "mock"}, backends)
	router.checkAll(context.Background())

	registry := tool.NewRegistry()
	registry.Register(&tool.ShellExec{})

	ag := NewAgent(router, registry, store)
	result, err := ag.Run(context.Background(), "run echo")
	if err != nil {
		t.Fatal(err)
	}
	if result != "The command output 'hello from tool'." {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestAgentMaxIterations(t *testing.T) {
	store, err := memory.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Always returns tool calls, never a final response
	mock := &mockLLMBackend{
		responses: func() []*llm.Response {
			var resps []*llm.Response
			for range 20 {
				resps = append(resps, &llm.Response{
					ToolCalls: []llm.ToolCall{
						{ID: "call", Name: "shell_exec", Arguments: json.RawMessage(`{"command":"echo loop"}`)},
					},
				})
			}
			return resps
		}(),
	}

	backends := map[string]llm.Backend{"mock": mock}
	router := NewRouter(RouterConfig{Default: "mock"}, backends)
	router.checkAll(context.Background())

	registry := tool.NewRegistry()
	registry.Register(&tool.ShellExec{})

	ag := NewAgent(router, registry, store, WithMaxIterations(3))
	_, err = ag.Run(context.Background(), "loop forever")
	if err == nil {
		t.Error("expected max iterations error")
	}
}
