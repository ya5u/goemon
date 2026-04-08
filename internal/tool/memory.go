package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ya5u/goemon/internal/memory"
)

// Memory is a unified tool that combines store and recall operations.

type Memory struct {
	store *memory.Store
}

func NewMemory(store *memory.Store) *Memory {
	return &Memory{store: store}
}

func (m *Memory) Name() string { return "memory" }
func (m *Memory) Description() string {
	return "Store or recall key-value pairs in persistent memory. Use action 'store' to save, 'recall' to retrieve."
}

func (m *Memory) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"store", "recall"},
				"description": "Action to perform: 'store' or 'recall'",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "Key to store under or search for",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "Value to store (required for 'store' action)",
			},
		},
		"required": []string{"action", "key"},
	}
}

func (m *Memory) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action string `json:"action"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	switch params.Action {
	case "store":
		slog.Info("memory store", "key", params.Key)
		if err := m.store.Store(params.Key, params.Value); err != nil {
			return "", err
		}
		return fmt.Sprintf("Stored key %q", params.Key), nil

	case "recall":
		slog.Info("memory recall", "key", params.Key)
		results, err := m.store.Recall(params.Key)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "No matching memories found.", nil
		}
		var sb strings.Builder
		for _, r := range results {
			fmt.Fprintf(&sb, "%s: %s\n", r.Key, r.Value)
		}
		return sb.String(), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use 'store' or 'recall')", params.Action)
	}
}
