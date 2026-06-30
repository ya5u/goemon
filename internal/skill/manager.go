package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillInfo represents an Anthropic-style Skill: a SKILL.md instruction package
// that guides the LLM to accomplish a specific task.
type SkillInfo struct {
	Name        string
	Description string
	Content     string // SKILL.md body (loaded on demand via GetSkill)
	Dir         string
}

type Manager struct {
	skillsDir string
}

func NewManager(skillsDir string) *Manager {
	return &Manager{skillsDir: skillsDir}
}

// ListSkills returns metadata (name + description) for all installed skills.
// Content is not loaded — use GetSkill to load the full instructions.
func (m *Manager) ListSkills() ([]SkillInfo, error) {
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := m.loadMetadata(e.Name())
		if err != nil {
			continue
		}
		skills = append(skills, *info)
	}
	return skills, nil
}

// GetSkill loads the full SKILL.md content for the named skill.
func (m *Manager) GetSkill(name string) (*SkillInfo, error) {
	dir := filepath.Join(m.skillsDir, name)
	skillMD := filepath.Join(dir, "SKILL.md")

	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	metaName, description, body := parseFrontmatter(string(data))
	if metaName != "" {
		name = metaName
	}

	return &SkillInfo{
		Name:        name,
		Description: description,
		Content:     body,
		Dir:         dir,
	}, nil
}

func (m *Manager) SkillsDir() string {
	return m.skillsDir
}

// loadMetadata reads only the YAML frontmatter of SKILL.md (name + description).
func (m *Manager) loadMetadata(dirName string) (*SkillInfo, error) {
	skillMD := filepath.Join(m.skillsDir, dirName, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, err
	}

	name, description, _ := parseFrontmatter(string(data))
	if name == "" {
		name = dirName
	}
	return &SkillInfo{
		Name:        name,
		Description: description,
		Dir:         filepath.Join(m.skillsDir, dirName),
	}, nil
}

// parseFrontmatter parses YAML frontmatter and returns (name, description, body).
// The body is the markdown content after the closing "---".
func parseFrontmatter(content string) (name, description, body string) {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return "", "", content
	}

	// Find the closing ---
	rest := strings.TrimPrefix(strings.TrimSpace(content), "---")
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", "", content
	}

	frontmatter := rest[:end]
	body = strings.TrimSpace(rest[end+4:]) // skip "\n---"

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, ":"); ok {
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			switch k {
			case "name":
				name = v
			case "description":
				description = v
			}
		}
	}
	return name, description, body
}
