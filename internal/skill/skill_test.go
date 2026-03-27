package skill

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ya5u/goemon/internal/memory"
)

func TestManagerListSkills(t *testing.T) {
	dir := t.TempDir()

	// Create a test skill
	skillDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`# Test Skill

## Description
A test skill.

## Trigger
- manual: "test"

## Entry Point
main.sh

## Language
bash
`), 0644)

	mgr := NewManager(dir)
	skills, err := mgr.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", skills[0].Name)
	}
	if skills[0].Description != "A test skill." {
		t.Errorf("expected description 'A test skill.', got %q", skills[0].Description)
	}
	if skills[0].Language != "bash" {
		t.Errorf("expected language 'bash', got %q", skills[0].Language)
	}
}

func TestManagerEmptyDir(t *testing.T) {
	mgr := NewManager(filepath.Join(t.TempDir(), "nonexistent"))
	skills, err := mgr.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestExecutor(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "main.sh")
	os.WriteFile(scriptPath, []byte(`#!/usr/bin/env bash
cat  # echo stdin to stdout
`), 0755)

	store, err := memory.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	executor := NewExecutor(store)
	info := &SkillInfo{
		Name:       "test",
		EntryPoint: "main.sh",
		Language:   "bash",
		Dir:        dir,
	}

	output, err := executor.Run(context.Background(), info, `{"hello":"world"}`)
	if err != nil {
		t.Fatal(err)
	}
	// No config.json in skill dir, so input passes through as-is
	if output != `{"hello":"world"}` {
		t.Errorf("unexpected output: %q", output)
	}

	// Check logged
	runs, err := store.GetSkillRuns("test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if !runs[0].Success {
		t.Error("expected success")
	}
}

func TestParseCreationResponse(t *testing.T) {
	content := `Here are the files:

---SKILL.md---
# My Skill

## Description
Does something.

## Entry Point
main.sh

## Language
bash
---main.sh---
#!/usr/bin/env bash
echo "hello"
`

	files, err := parseCreationResponse(content)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["SKILL.md"]; !ok {
		t.Error("missing SKILL.md")
	}
	if _, ok := files["main.sh"]; !ok {
		t.Error("missing main.sh")
	}
}
