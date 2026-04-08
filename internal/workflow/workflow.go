package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type StepType string

const (
	StepTypePrompt StepType = "prompt"
	StepTypeScript StepType = "script"
)

type WorkflowStep struct {
	Name       string   `yaml:"name"`
	Type       StepType `yaml:"type"`
	Prompt     string   `yaml:"prompt,omitempty"`      // for type: prompt
	EntryPoint string   `yaml:"entry_point,omitempty"` // for type: script
}

type WorkflowInfo struct {
	Name     string         `yaml:"name"`
	Schedule string         `yaml:"schedule"`
	Notify   string         `yaml:"notify,omitempty"`
	Steps    []WorkflowStep `yaml:"steps"`
	Dir      string         `yaml:"-"`
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

	// Try workflow.yaml first, then workflow.yml
	var data []byte
	var err error
	for _, filename := range []string{"workflow.yaml", "workflow.yml"} {
		data, err = os.ReadFile(filepath.Join(dir, filename))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read workflow.yaml: %w", err)
	}

	var wf WorkflowInfo
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow.yaml: %w", err)
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

	return &wf, nil
}
