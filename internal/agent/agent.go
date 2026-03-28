package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ya5u/goemon/internal/llm"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/tool"
)

const systemPrompt = `You are GoEmon, a personal AI agent for a solo developer.

You have access to tools that let you execute shell commands, read/write files, fetch web pages, and manage persistent memory. You also have a skill system for reusable automation.

IMPORTANT RULES:
- Only use tools when the user's request actually requires an action (running commands, reading files, fetching data, etc.)
- For greetings, casual conversation, questions you can answer from knowledge, or simple replies, just respond directly with text. Do NOT call any tools.
- Always respond in the same language the user is using.
- Be concise and direct.
- If you encounter an error, try to diagnose and fix it before giving up.

Available tools:
- shell_exec: Execute shell commands
- file_read/file_write: Read/write files
- web_fetch: Fetch web pages
- memory_store/memory_recall: Persistent key-value memory
- skill_list/skill_run/skill_create: Manage and run reusable skills in ~/.goemon/skills/

When creating skills, prefer shell scripts for simple tasks, Python for complex logic.`

type Agent struct {
	router        *Router
	registry      *tool.Registry
	store         *memory.Store
	maxIterations int
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
	// Load conversation history
	history, err := a.store.LoadHistory(50)
	if err != nil {
		slog.Warn("failed to load history", "error", err)
	}

	var messages []llm.Message
	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})
	for _, h := range history {
		messages = append(messages, llm.Message{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, llm.Message{Role: "user", Content: userInput})

	// Save user message
	if err := a.store.SaveMessage("user", userInput); err != nil {
		slog.Warn("failed to save user message", "error", err)
	}

	toolDefs := a.registry.Definitions()

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
			a.onResponse(resp.Content)
			if err := a.store.SaveMessage("assistant", resp.Content); err != nil {
				slog.Warn("failed to save assistant message", "error", err)
			}
			return resp.Content, nil
		}

		// Has thinking content
		if resp.Content != "" {
			a.onThinking(resp.Content)
		}

		// Build assistant message describing what tools were called
		assistantContent := resp.Content
		if assistantContent == "" {
			// Summarize tool calls so the model knows what it did
			var parts []string
			for _, tc := range resp.ToolCalls {
				parts = append(parts, fmt.Sprintf("I'm calling tool %s with args: %s", tc.Name, string(tc.Arguments)))
			}
			assistantContent = strings.Join(parts, "\n")
		}
		messages = append(messages, llm.Message{Role: "assistant", Content: assistantContent})

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
