package skill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ya5u/goemon/internal/llm"
)

const skillCreationPrompt = `Create a GoEmon skill for the following task:

Task: %s

Generate these files:

1. SKILL.md following this format:
# Name
## Description
## Trigger
## Entry Point
## Language
## Input
## Output
## Dependencies

2. Entry point script (bash for simple, python for complex)
   - Receives JSON on stdin
   - Outputs JSON on stdout
   - Uses stderr for debug
   - Includes error handling

Return files separated by markers:
---SKILL.md---
(content)
---main.sh--- or ---main.py---
(content)`

type Creator struct {
	skillsDir string
	backend   llm.Backend
}

func NewCreator(skillsDir string, backend llm.Backend) *Creator {
	return &Creator{skillsDir: skillsDir, backend: backend}
}

func (c *Creator) Create(ctx context.Context, name, description string) (*SkillInfo, error) {
	prompt := fmt.Sprintf(skillCreationPrompt, description)

	resp, err := c.backend.Chat(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	files, err := parseCreationResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	skillDir := filepath.Join(c.skillsDir, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return nil, fmt.Errorf("create skill dir: %w", err)
	}

	for filename, content := range files {
		path := filepath.Join(skillDir, filename)
		perm := os.FileMode(0644)
		if strings.HasSuffix(filename, ".sh") || strings.HasSuffix(filename, ".py") {
			perm = 0755
		}
		if err := os.WriteFile(path, []byte(content), perm); err != nil {
			return nil, fmt.Errorf("write %s: %w", filename, err)
		}
	}

	info := &SkillInfo{
		Name: name,
		Dir:  skillDir,
	}

	if skillMD, ok := files["SKILL.md"]; ok {
		parseSkillMD(skillMD, info)
	}

	return info, nil
}

func parseCreationResponse(content string) (map[string]string, error) {
	files := make(map[string]string)
	parts := strings.Split(content, "---")

	var currentFile string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		// Check if this is a filename marker
		if isFilename(trimmed) {
			currentFile = trimmed
			continue
		}

		if currentFile != "" {
			files[currentFile] = strings.TrimSpace(part)
			currentFile = ""
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in LLM response")
	}
	return files, nil
}

func isFilename(s string) bool {
	return strings.HasSuffix(s, ".md") ||
		strings.HasSuffix(s, ".sh") ||
		strings.HasSuffix(s, ".py") ||
		strings.HasSuffix(s, ".go") ||
		strings.HasSuffix(s, ".js")
}
