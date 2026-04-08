package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ya5u/goemon/internal/config"
	"github.com/ya5u/goemon/internal/llm"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/tool"
)

const baseSystemPrompt = `You have access to tools. Only call tools when the user's request requires an action. For conversation or questions you can answer from knowledge, respond directly without tools.`

type Agent struct {
	router        *Router
	registry      *tool.Registry
	store         *memory.Store
	maxIterations int
	skipHistory   bool // when true, don't save to conversation history
	onThinking    func(text string)
	onToolCall    func(name string, args json.RawMessage)
	onToolResult  func(name string, result string)
	onResponse    func(text string)
}

type AgentOption func(*Agent)

func WithMaxIterations(n int) AgentOption {
	return func(a *Agent) { a.maxIterations = n }
}

func WithCallbacks(
	onThinking func(string),
	onToolCall func(string, json.RawMessage),
	onToolResult func(string, string),
	onResponse func(string),
) AgentOption {
	return func(a *Agent) {
		a.onThinking = onThinking
		a.onToolCall = onToolCall
		a.onToolResult = onToolResult
		a.onResponse = onResponse
	}
}

func NewAgent(router *Router, registry *tool.Registry, store *memory.Store, opts ...AgentOption) *Agent {
	a := &Agent{
		router:        router,
		registry:      registry,
		store:         store,
		maxIterations: 10,
		onThinking:    func(string) {},
		onToolCall:    func(string, json.RawMessage) {},
		onToolResult:  func(string, string) {},
		onResponse:    func(string) {},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	if needsPlanning(userInput) {
		return a.runPlanAndExecute(ctx, userInput)
	}
	return a.runSimple(ctx, userInput)
}

// RunWithPlan forces plan-and-execute mode regardless of complexity detection.
func (a *Agent) RunWithPlan(ctx context.Context, userInput string) (string, error) {
	return a.runPlanAndExecute(ctx, userInput)
}

// RunWithoutHistory runs the agent without saving to conversation history.
// Used by workflow steps to avoid polluting chat history.
// Always uses simple ReAct (no plan-and-execute) since workflow steps
// are already decomposed into focused tasks.
func (a *Agent) RunWithoutHistory(ctx context.Context, userInput string) (string, error) {
	a.skipHistory = true
	defer func() { a.skipHistory = false }()

	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: a.buildSystemPrompt()})
	messages = append(messages, llm.Message{Role: "user", Content: userInput})

	result, err := a.executeReAct(ctx, messages, a.registry.Definitions())
	if err != nil {
		return "", err
	}

	a.onResponse(result)
	return result, nil
}

// runSimple is the original Run() logic with conversation history and memory.
func (a *Agent) runSimple(ctx context.Context, userInput string) (string, error) {
	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: a.buildSystemPrompt()})

	if !a.skipHistory {
		history, err := a.store.LoadHistory(50)
		if err != nil {
			slog.Warn("failed to load history", "error", err)
		}
		for _, h := range history {
			messages = append(messages, llm.Message{Role: h.Role, Content: h.Content})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: userInput})

	if !a.skipHistory {
		if err := a.store.SaveMessage("user", userInput); err != nil {
			slog.Warn("failed to save user message", "error", err)
		}
	}

	result, err := a.executeReAct(ctx, messages, a.registry.Definitions())
	if err != nil {
		return "", err
	}

	a.onResponse(result)
	if !a.skipHistory {
		if err := a.store.SaveMessage("assistant", result); err != nil {
			slog.Warn("failed to save assistant message", "error", err)
		}
	}
	return result, nil
}

// executeReAct runs the core ReAct loop: LLM call → tool execution → repeat.
// Returns the final text response from the LLM.
func (a *Agent) executeReAct(ctx context.Context, messages []llm.Message, toolDefs []llm.ToolDefinition) (string, error) {
	for i := range a.maxIterations {
		slog.Debug("agent iteration", "iteration", i+1)

		backend, err := a.router.Select("")
		if err != nil {
			return "", fmt.Errorf("select backend: %w", err)
		}

		resp, err := a.chatWithRetry(ctx, backend, messages, toolDefs)
		if err != nil {
			return "", fmt.Errorf("chat (iteration %d): %w", i+1, err)
		}

		// No tool calls → final response
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		// Has thinking content
		if resp.Content != "" {
			a.onThinking(resp.Content)
		}

		// Add assistant message with tool_calls preserved
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call
		for _, tc := range resp.ToolCalls {
			a.onToolCall(tc.Name, tc.Arguments)

			t, err := a.registry.Get(tc.Name)
			var result string
			if err != nil {
				result = fmt.Sprintf("Error: tool %q not found", tc.Name)
			} else {
				result, err = t.Execute(ctx, tc.Arguments)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
				}
			}

			a.onToolResult(tc.Name, result)

			messages = append(messages, llm.Message{
				Role:    "tool",
				Content: result,
				ToolID:  tc.ID,
				Name:    tc.Name,
			})
		}
	}

	return "", fmt.Errorf("max iterations (%d) reached", a.maxIterations)
}

func (a *Agent) buildSystemPrompt() string {
	prompt := baseSystemPrompt

	// Load user customizations from AGENTS.md
	if dataDir, err := config.DataDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(dataDir, "AGENTS.md")); err == nil {
			prompt += "\n\n" + string(data)
		}
	}

	return prompt
}

// callLLM makes a single LLM call without tools (for planning, summarization, etc.)
func (a *Agent) callLLM(ctx context.Context, messages []llm.Message) (string, error) {
	backend, err := a.router.Select("")
	if err != nil {
		return "", fmt.Errorf("select backend: %w", err)
	}
	resp, err := a.chatWithRetry(ctx, backend, messages, nil)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (a *Agent) chatWithRetry(ctx context.Context, backend llm.Backend, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	var lastErr error
	for attempt := range 3 {
		resp, err := backend.Chat(ctx, messages, tools)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		slog.Warn("chat attempt failed", "attempt", attempt+1, "error", err)
		if attempt < 2 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}
