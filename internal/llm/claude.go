package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ClaudeBackend struct {
	model     string
	apiKeyEnv string
	client    *http.Client
}

func NewClaude(model, apiKeyEnv string) *ClaudeBackend {
	return &ClaudeBackend{
		model:     model,
		apiKeyEnv: apiKeyEnv,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ClaudeBackend) Name() string {
	return "claude/" + c.model
}

func (c *ClaudeBackend) apiKey() string {
	return os.Getenv(c.apiKeyEnv)
}

func (c *ClaudeBackend) IsAvailable(_ context.Context) bool {
	return c.apiKey() != ""
}

func (c *ClaudeBackend) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error) {
	apiKey := c.apiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set in %s", c.apiKeyEnv)
	}

	// Convert messages to Anthropic format
	var system string
	var anthropicMsgs []map[string]any

	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}

		msg := map[string]any{"role": m.Role}

		if m.ToolID != "" {
			// Tool result
			msg["content"] = []map[string]any{{
				"type":        "tool_result",
				"tool_use_id": m.ToolID,
				"content":     m.Content,
			}}
		} else {
			msg["content"] = m.Content
		}
		anthropicMsgs = append(anthropicMsgs, msg)
	}

	body := map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"messages":   anthropicMsgs,
	}
	if system != "" {
		body["system"] = system
	}

	if len(tools) > 0 {
		var anthropicTools []map[string]any
		for _, t := range tools {
			anthropicTools = append(anthropicTools, map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.Parameters,
			})
		}
		body["tools"] = anthropicTools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude returned %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text,omitempty"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	result := &Response{}
	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	return result, nil
}
