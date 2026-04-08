package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type OllamaBackend struct {
	endpoint string
	model    string
	client   *http.Client
}

func NewOllama(endpoint, model string) *OllamaBackend {
	return &OllamaBackend{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: 15 * time.Minute},
	}
}

func (o *OllamaBackend) Name() string {
	return "ollama/" + o.model
}

func (o *OllamaBackend) IsAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Ollama native API types

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunction `json:"function"`
}

type ollamaFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type ollamaResponse struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

func (o *OllamaBackend) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error) {
	// Build Ollama native API request
	var ollamaMessages []ollamaMessage
	for _, m := range messages {
		msg := ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}

		// Include tool_calls in assistant messages
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				var args map[string]any
				json.Unmarshal(tc.Arguments, &args)
				msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
					Function: ollamaFunction{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
		}

		ollamaMessages = append(ollamaMessages, msg)
	}

	var ollamaTools []ollamaTool
	for _, t := range tools {
		ot := ollamaTool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.Parameters
		ollamaTools = append(ollamaTools, ot)
	}

	body := map[string]any{
		"model":    o.model,
		"messages": ollamaMessages,
		"stream":   false,
	}
	if len(ollamaTools) > 0 {
		body["tools"] = ollamaTools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("ollama request", "tools_count", len(ollamaTools), "messages_count", len(ollamaMessages), "body", string(jsonBody))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("ollama raw response", "body", string(respBody))

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	result := &Response{Content: ollamaResp.Message.Content}

	// Parse native tool calls
	for i, tc := range ollamaResp.Message.ToolCalls {
		argsJSON, err := json.Marshal(tc.Function.Arguments)
		if err != nil {
			slog.Warn("failed to marshal tool call arguments", "error", err)
			continue
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        fmt.Sprintf("call_%d", i),
			Name:      tc.Function.Name,
			Arguments: argsJSON,
		})
	}

	// Fallback: if model returned tool call as plain text, try to parse it
	if len(result.ToolCalls) == 0 && result.Content != "" {
		if tc, ok := parseToolCallFromText(result.Content); ok {
			slog.Info("parsed tool call from text content", "tool", tc.Name)
			result.ToolCalls = append(result.ToolCalls, tc)
			result.Content = ""
		}
	}

	return result, nil
}

// parseToolCallFromText attempts to extract a tool call from plain text.
// Some models return tool calls as JSON in the content instead of using the tool_calls field.
func parseToolCallFromText(text string) (ToolCall, bool) {
	text = strings.TrimSpace(text)

	// Strip markdown code blocks
	codeBlockRe := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
	if matches := codeBlockRe.FindStringSubmatch(text); len(matches) > 1 {
		text = strings.TrimSpace(matches[1])
	}

	// Strip <tool_call> tags (Qwen style)
	toolCallRe := regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*</tool_call>`)
	if matches := toolCallRe.FindStringSubmatch(text); len(matches) > 1 {
		text = strings.TrimSpace(matches[1])
	}

	// Try to parse as JSON with name + arguments
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(text), &call); err == nil && call.Name != "" {
		return ToolCall{
			ID:        "text_call_1",
			Name:      call.Name,
			Arguments: call.Arguments,
		}, true
	}

	return ToolCall{}, false
}
