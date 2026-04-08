package skill

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ya5u/goemon/internal/tool"
)

// SkillTool wraps a single skill as a tool for the LLM.
type SkillTool struct {
	info     *SkillInfo
	executor *Executor
}

func NewSkillTool(info *SkillInfo, executor *Executor) *SkillTool {
	return &SkillTool{info: info, executor: executor}
}

func (t *SkillTool) Name() string        { return "skill_" + t.info.Name }
func (t *SkillTool) Description() string { return t.info.Description }

func (t *SkillTool) Parameters() map[string]any {
	properties := make(map[string]any)
	var required []string

	for _, input := range t.info.Inputs {
		prop := map[string]any{
			"type": "string",
		}
		if input.Description != "" {
			prop["description"] = input.Description
		}
		properties[input.Name] = prop
		if !input.Optional {
			required = append(required, input.Name)
		}
	}

	params := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		params["required"] = required
	}
	return params
}

func (t *SkillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	// args is already the JSON object matching the skill's input schema.
	// Pass it directly to the executor.
	input := string(args)
	if input == "" || input == "null" {
		input = "{}"
	}

	return t.executor.Run(ctx, t.info, input)
}

// SkillProvider implements tool.ToolProvider by scanning the skills directory
// on each call, so newly added skills are picked up dynamically.
type SkillProvider struct {
	manager  *Manager
	executor *Executor
}

func NewSkillProvider(manager *Manager, executor *Executor) *SkillProvider {
	return &SkillProvider{manager: manager, executor: executor}
}

func (p *SkillProvider) Tools() []tool.Tool {
	skills, err := p.manager.ListSkills()
	if err != nil {
		slog.Warn("failed to list skills", "error", err)
		return nil
	}
	tools := make([]tool.Tool, 0, len(skills))
	for i := range skills {
		tools = append(tools, NewSkillTool(&skills[i], p.executor))
	}
	return tools
}
