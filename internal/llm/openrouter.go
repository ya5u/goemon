package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// OpenRouterBackend talks to OpenRouter's OpenAI-compatible chat completions API.
// It also works with any other OpenAI-compatible endpoint.
type OpenRouterBackend struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

func NewOpenRouter(endpoint, model, apiKey string) *OpenRouterBackend {
	if endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1"
	}
	return &OpenRouterBackend{
		endpoint: endpoint,
		model:    model,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (o *OpenRouterBackend) Name() string {
	return "openrouter/" + o.model
}

func (o *OpenRouterBackend) IsAvailable(ctx context.Context) bool {
	if o.apiKey == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.endpoint+"/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// OpenAI-compatible API types

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON-encoded string, not an object
	} `json:"function"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenRouterBackend) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error) {
	openAIMessages := make([]openAIMessage, 0, len(messages))
	for _, m := range messages {
		msg := openAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolID,
		}

		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				args := string(tc.Arguments)
				if args == "" {
					args = "{}"
				}
				oc := openAIToolCall{ID: tc.ID, Type: "function"}
				oc.Function.Name = tc.Name
				oc.Function.Arguments = args
				msg.ToolCalls = append(msg.ToolCalls, oc)
			}
		}

		openAIMessages = append(openAIMessages, msg)
	}

	var openAITools []openAITool
	for _, t := range tools {
		ot := openAITool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.Parameters
		openAITools = append(openAITools, ot)
	}

	body := map[string]any{
		"model":    o.model,
		"messages": openAIMessages,
		"stream":   false,
	}
	if len(openAITools) > 0 {
		body["tools"] = openAITools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("openrouter request", "tools_count", len(openAITools), "messages_count", len(openAIMessages), "body", string(jsonBody))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/ya5u/goemon")
	req.Header.Set("X-Title", "GoEmon")

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
		return nil, fmt.Errorf("openrouter returned %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("openrouter raw response", "body", string(respBody))

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if openAIResp.Error != nil {
		return nil, fmt.Errorf("openrouter error: %s", openAIResp.Error.Message)
	}
	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("openrouter returned no choices")
	}

	choice := openAIResp.Choices[0].Message
	result := &Response{Content: choice.Content}

	for i, tc := range choice.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", i)
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        id,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(args),
		})
	}

	return result, nil
}
