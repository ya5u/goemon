package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ya5u/goemon/internal/memory"
)

// MemoryStore

type MemoryStore struct {
	store *memory.Store
}

func NewMemoryStore(store *memory.Store) *MemoryStore {
	return &MemoryStore{store: store}
}

func (m *MemoryStore) Name() string        { return "memory_store" }
func (m *MemoryStore) Description() string { return "Store a key-value pair in persistent memory." }

func (m *MemoryStore) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type":        "string",
				"description": "Key to store the value under",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "Value to store",
			},
		},
		"required": []string{"key", "value"},
	}
}

func (m *MemoryStore) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("memory_store", "key", params.Key)

	if err := m.store.Store(params.Key, params.Value); err != nil {
		return "", err
	}
	return fmt.Sprintf("Stored key %q", params.Key), nil
}

// MemoryRecall

type MemoryRecall struct {
	store *memory.Store
}

func NewMemoryRecall(store *memory.Store) *MemoryRecall {
	return &MemoryRecall{store: store}
}

func (m *MemoryRecall) Name() string { return "memory_recall" }
func (m *MemoryRecall) Description() string {
	return "Recall stored values by key. Supports partial matching."
}

func (m *MemoryRecall) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type":        "string",
				"description": "Key or partial key to search for",
			},
		},
		"required": []string{"key"},
	}
}

func (m *MemoryRecall) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("memory_recall", "key", params.Key)

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
}
