package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ya5u/goemon/internal/usermemory"
)

// Memory is the agent's long-term memory tool. It stores durable facts about
// the user as human-readable Markdown files (see internal/usermemory).
type Memory struct {
	mgr *usermemory.Manager
}

func NewMemory(mgr *usermemory.Manager) *Memory {
	return &Memory{mgr: mgr}
}

func (m *Memory) Name() string { return "memory" }

func (m *Memory) Description() string {
	return "Long-term memory of durable facts about the user. " +
		"Use 'save' to remember something worth keeping across sessions (a preference, a recurring task and how it was done, feedback the user gave, a pattern in their requests). " +
		"Use 'read' to load the full text of a memory listed in the memory index. " +
		"Use 'list' to see all memories, 'delete' to remove one that is wrong or obsolete. " +
		"Each memory is one fact. Before saving, check the index for an existing memory on the same topic and update it instead of duplicating."
}

func (m *Memory) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"save", "read", "list", "delete"},
				"description": "save | read | list | delete",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "Short kebab-case slug identifying the memory (required for save/read/delete). E.g. 'prefers-japanese-responses'.",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        []string{"user", "feedback", "task", "reference"},
				"description": "Memory category (for save): user=identity/preferences, feedback=guidance/corrections, task=recurring task and how to do it, reference=external pointer.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "One-line summary shown in the memory index (for save).",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The full fact in Markdown (for save). For feedback/task, include why it matters and how to apply it.",
			},
		},
		"required": []string{"action"},
	}
}

func (m *Memory) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Action      string `json:"action"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Content     string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	switch p.Action {
	case "save":
		if p.Name == "" {
			return "", fmt.Errorf("'name' is required for save")
		}
		slog.Info("memory save", "name", p.Name, "type", p.Type)
		if err := m.mgr.Save(usermemory.Entry{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Content:     p.Content,
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("Saved memory %q.", usermemory.Slug(p.Name)), nil

	case "read":
		if p.Name == "" {
			return "", fmt.Errorf("'name' is required for read")
		}
		slog.Info("memory read", "name", p.Name)
		e, err := m.mgr.Get(p.Name)
		if err != nil {
			return "No such memory.", nil
		}
		return fmt.Sprintf("# %s (%s)\n%s\n\n%s", e.Name, e.Type, e.Description, e.Content), nil

	case "list":
		entries, err := m.mgr.List()
		if err != nil {
			return "", err
		}
		if len(entries) == 0 {
			return "No memories yet.", nil
		}
		var sb strings.Builder
		for _, e := range entries {
			fmt.Fprintf(&sb, "- %s (%s): %s\n", e.Name, e.Type, e.Description)
		}
		return sb.String(), nil

	case "delete":
		if p.Name == "" {
			return "", fmt.Errorf("'name' is required for delete")
		}
		slog.Info("memory delete", "name", p.Name)
		if err := m.mgr.Delete(p.Name); err != nil {
			return "", err
		}
		return fmt.Sprintf("Deleted memory %q.", usermemory.Slug(p.Name)), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use save|read|list|delete)", p.Action)
	}
}
