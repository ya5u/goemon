package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SkillListTool — exposed to LLM as skill_list

type SkillListTool struct {
	manager *Manager
}

func NewSkillListTool(manager *Manager) *SkillListTool {
	return &SkillListTool{manager: manager}
}

func (t *SkillListTool) Name() string        { return "skill_list" }
func (t *SkillListTool) Description() string { return "List all installed skills with descriptions and triggers." }

func (t *SkillListTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *SkillListTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	skills, err := t.manager.ListSkills()
	if err != nil {
		return "", err
	}
	if len(skills) == 0 {
		return "No skills installed.", nil
	}

	var sb strings.Builder
	for _, s := range skills {
		fmt.Fprintf(&sb, "- %s: %s (trigger: %s)\n", s.Name, s.Description, s.Trigger)
	}
	return sb.String(), nil
}

// SkillRunTool — exposed to LLM as skill_run

type SkillRunTool struct {
	manager  *Manager
	executor *Executor
}

func NewSkillRunTool(manager *Manager, executor *Executor) *SkillRunTool {
	return &SkillRunTool{manager: manager, executor: executor}
}

func (t *SkillRunTool) Name() string        { return "skill_run" }
func (t *SkillRunTool) Description() string { return "Run an installed skill by name." }

func (t *SkillRunTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Name of the skill to run",
			},
			"input": map[string]any{
				"type":        "string",
				"description": "JSON input for the skill (optional)",
			},
		},
		"required": []string{"name"},
	}
}

func (t *SkillRunTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name  string `json:"name"`
		Input string `json:"input"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	info, err := t.manager.GetSkill(params.Name)
	if err != nil {
		return "", fmt.Errorf("get skill: %w", err)
	}

	input := params.Input
	if input == "" {
		input = "{}"
	}

	return t.executor.Run(ctx, info, input)
}

// SkillCreateTool — exposed to LLM as skill_create

type SkillCreateTool struct {
	creator *Creator
}

func NewSkillCreateTool(creator *Creator) *SkillCreateTool {
	return &SkillCreateTool{creator: creator}
}

func (t *SkillCreateTool) Name() string { return "skill_create" }
func (t *SkillCreateTool) Description() string {
	return "Create a new reusable skill. Use when no existing skill fits the task."
}

func (t *SkillCreateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Kebab-case name for the skill",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Description of what the skill should do",
			},
		},
		"required": []string{"name", "description"},
	}
}

func (t *SkillCreateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	info, err := t.creator.Create(ctx, params.Name, params.Description)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Created skill %q at %s\nEntry point: %s\nLanguage: %s", info.Name, info.Dir, info.EntryPoint, info.Language), nil
}
