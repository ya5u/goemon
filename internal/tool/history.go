package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ya5u/goemon/internal/memory"
)

// History is a read-only tool that exposes recent conversation history from the
// SQLite store. It lets the agent reflect on past interactions — e.g. to distill
// durable facts into long-term memory.
type History struct {
	store *memory.Store
}

func NewHistory(store *memory.Store) *History { return &History{store: store} }

func (h *History) Name() string { return "conversation_history" }

func (h *History) Description() string {
	return "Read recent conversation history (past user and assistant messages) from persistent storage. " +
		"Use it to reflect on what the user has asked and how you responded — for example to distill durable facts into long-term memory with the memory tool."
}

func (h *History) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of most-recent messages to return (default 100).",
			},
		},
	}
}

func (h *History) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Limit int `json:"limit"`
	}
	_ = json.Unmarshal(args, &p)
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}

	slog.Info("conversation_history read", "limit", limit)
	msgs, err := h.store.LoadHistory(limit)
	if err != nil {
		return "", err
	}
	if len(msgs) == 0 {
		return "No conversation history yet.", nil
	}

	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", m.CreatedAt.Format("2006-01-02 15:04"), m.Role, m.Content)
	}
	return sb.String(), nil
}
