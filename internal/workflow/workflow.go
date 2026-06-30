package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WorkflowStep is one step in a workflow, referencing a Skill by name.
// Content holds the markdown under the step header (instructions + completion criteria).
type WorkflowStep struct {
	SkillName string
	Content   string
}

// WorkflowInfo is parsed from WORKFLOW.md.
type WorkflowInfo struct {
	Name     string
	Schedule string
	Notify   string
	Steps    []WorkflowStep
	Dir      string
}

type Manager struct {
	workflowsDir string
}

func NewManager(workflowsDir string) *Manager {
	return &Manager{workflowsDir: workflowsDir}
}

func (m *Manager) ListWorkflows() ([]WorkflowInfo, error) {
	entries, err := os.ReadDir(m.workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var workflows []WorkflowInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wf, err := m.GetWorkflow(e.Name())
		if err != nil {
			continue
		}
		workflows = append(workflows, *wf)
	}
	return workflows, nil
}

func (m *Manager) GetWorkflow(name string) (*WorkflowInfo, error) {
	dir := filepath.Join(m.workflowsDir, name)

	data, err := os.ReadFile(filepath.Join(dir, "WORKFLOW.md"))
	if err != nil {
		return nil, fmt.Errorf("read WORKFLOW.md: %w", err)
	}

	wf, err := parseWorkflowMD(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse WORKFLOW.md: %w", err)
	}

	wf.Dir = dir
	if wf.Name == "" {
		wf.Name = name
	}

	if wf.Schedule == "" {
		return nil, fmt.Errorf("workflow %q has no schedule", name)
	}
	if len(wf.Steps) == 0 {
		return nil, fmt.Errorf("workflow %q has no steps", name)
	}

	return wf, nil
}

// stepHeaderRe matches "### skill-name" or "### 1. skill-name"
var stepHeaderRe = regexp.MustCompile(`^###\s+(?:\d+\.\s+)?(.+)$`)

// parseWorkflowMD parses a WORKFLOW.md file.
//
// Format:
//
//	---
//	name: my-workflow
//	schedule: "0 8 * * *"
//	notify: telegram
//	---
//
//	# Title
//
//	Description...
//
//	## Steps
//
//	### skill-name
//	Step instructions and completion criteria.
//
//	### another-skill
//	More instructions.
func parseWorkflowMD(content string) (*WorkflowInfo, error) {
	wf := &WorkflowInfo{}

	// Parse YAML frontmatter
	body := content
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		rest := strings.TrimPrefix(strings.TrimSpace(content), "---")
		end := strings.Index(rest, "\n---")
		if end != -1 {
			frontmatter := rest[:end]
			body = strings.TrimSpace(rest[end+4:])
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if k, v, ok := strings.Cut(line, ":"); ok {
					k = strings.TrimSpace(k)
					v = strings.Trim(strings.TrimSpace(v), `"'`)
					switch k {
					case "name":
						wf.Name = v
					case "schedule":
						wf.Schedule = v
					case "notify":
						wf.Notify = v
					}
				}
			}
		}
	}

	// Parse steps from ### headers
	lines := strings.Split(body, "\n")
	var currentStep *WorkflowStep
	var currentLines []string

	flushStep := func() {
		if currentStep != nil {
			currentStep.Content = strings.TrimSpace(strings.Join(currentLines, "\n"))
			wf.Steps = append(wf.Steps, *currentStep)
			currentStep = nil
			currentLines = nil
		}
	}

	for _, line := range lines {
		if m := stepHeaderRe.FindStringSubmatch(line); m != nil {
			flushStep()
			skillName := strings.TrimSpace(m[1])
			currentStep = &WorkflowStep{SkillName: skillName}
			currentLines = nil
			continue
		}
		if currentStep != nil {
			currentLines = append(currentLines, line)
		}
	}
	flushStep()

	return wf, nil
}
