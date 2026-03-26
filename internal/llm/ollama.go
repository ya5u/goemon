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
		client:   &http.Client{Timeout: 120 * time.Second},
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

func (o *OllamaBackend) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error) {
	// Build OpenAI-compatible request
	type openAIMessage struct {
		Role       string `json:"role"`
		Content    string `json:"content,omitempty"`
		Name       string `json:"name,omitempty"`
		ToolCallID string `json:"tool_call_id,omitempty"`
	}
	type openAITool struct {
		Type     string `json:"type"`
		Function struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Parameters  map[string]any `json:"parameters"`
		} `json:"function"`
	}

	var oaiMessages []openAIMessage
	for _, m := range messages {
		msg := openAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			Name:       m.Name,
			ToolCallID: m.ToolID,
		}
		// Ollama doesn't accept empty content or "tool" role well.
		// Convert tool results to user messages for compatibility.
		if m.Role == "tool" {
			msg.Role = "user"
			msg.Content = fmt.Sprintf("[Tool result from %s]: %s", m.Name, m.Content)
			msg.ToolCallID = ""
			msg.Name = ""
		}
		if msg.Content == "" {
			msg.Content = "(no content)"
		}
		oaiMessages = append(oaiMessages, msg)
	}

	var oaiTools []openAITool
	for _, t := range tools {
		ot := openAITool{Type: "function"}
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.Parameters
		oaiTools = append(oaiTools, ot)
	}

	body := map[string]any{
		"model":    o.model,
		"messages": oaiMessages,
	}
	if len(oaiTools) > 0 {
		body["tools"] = oaiTools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/v1/chat/completions", bytes.NewReader(jsonBody))
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

	var oaiResp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := oaiResp.Choices[0].Message
	result := &Response{Content: choice.Content}
	for _, tc := range choice.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
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
