package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InputField struct {
	Name        string
	Description string
	Optional    bool
}

type SkillInfo struct {
	Name        string
	Description string
	Trigger     string
	EntryPoint  string
	Language    string
	Dir         string
	Inputs      []InputField
}

type Manager struct {
	skillsDir string
}

func NewManager(skillsDir string) *Manager {
	return &Manager{skillsDir: skillsDir}
}

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
		info, err := m.GetSkill(e.Name())
		if err != nil {
			continue
		}
		skills = append(skills, *info)
	}
	return skills, nil
}

func (m *Manager) GetSkill(name string) (*SkillInfo, error) {
	dir := filepath.Join(m.skillsDir, name)
	skillMD := filepath.Join(dir, "SKILL.md")

	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	info := &SkillInfo{
		Name: name,
		Dir:  dir,
	}
	parseSkillMD(string(data), info)
	return info, nil
}

func (m *Manager) SkillsDir() string {
	return m.skillsDir
}

func parseSkillMD(content string, info *SkillInfo) {
	lines := strings.Split(content, "\n")
	var currentSection string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") && info.Name == "" {
			info.Name = strings.TrimPrefix(trimmed, "# ")
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "- ") {
			trimmed = strings.TrimPrefix(trimmed, "- ")
		}
		if trimmed == "" {
			continue
		}
		switch currentSection {
		case "description":
			if info.Description == "" {
				info.Description = trimmed
			}
		case "trigger":
			if info.Trigger == "" {
				info.Trigger = trimmed
			}
		case "entry point":
			if info.EntryPoint == "" {
				info.EntryPoint = trimmed
			}
		case "language":
			if info.Language == "" {
				info.Language = strings.ToLower(trimmed)
			}
		case "input":
			// Parse "field_name: description" or "field_name: (optional) description"
			name, desc, _ := strings.Cut(trimmed, ":")
			name = strings.TrimSpace(name)
			desc = strings.TrimSpace(desc)
			if name == "" || strings.HasPrefix(name, "(") {
				continue
			}
			field := InputField{Name: name}
			if strings.HasPrefix(desc, "(optional)") {
				field.Optional = true
				desc = strings.TrimSpace(strings.TrimPrefix(desc, "(optional)"))
			}
			field.Description = desc
			info.Inputs = append(info.Inputs, field)
		}
	}
}
