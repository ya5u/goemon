package memory

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestConversation(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveMessage("user", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveMessage("assistant", "hi there"); err != nil {
		t.Fatal(err)
	}

	history, err := s.LoadHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0].Role != "user" || history[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", history[0])
	}
	if history[1].Role != "assistant" || history[1].Content != "hi there" {
		t.Errorf("unexpected second message: %+v", history[1])
	}
}

func TestConversationLimit(t *testing.T) {
	s := newTestStore(t)

	for i := range 5 {
		if err := s.SaveMessage("user", string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}

	history, err := s.LoadHistory(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}
	// Should get last 3 in chronological order
	if history[0].Content != "c" {
		t.Errorf("expected 'c', got %q", history[0].Content)
	}
}

func TestKVMemory(t *testing.T) {
	s := newTestStore(t)

	if err := s.Store("project.name", "goemon"); err != nil {
		t.Fatal(err)
	}
	if err := s.Store("project.lang", "go"); err != nil {
		t.Fatal(err)
	}

	results, err := s.Recall("project")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Test upsert
	if err := s.Store("project.name", "GoEmon"); err != nil {
		t.Fatal(err)
	}
	results, err = s.Recall("project.name")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Value != "GoEmon" {
		t.Errorf("expected updated value 'GoEmon', got %+v", results)
	}
}

func TestSkillRuns(t *testing.T) {
	s := newTestStore(t)

	if err := s.LogSkillRun("hello-world", `{"name":"test"}`, `{"message":"hi"}`, true, "", 42); err != nil {
		t.Fatal(err)
	}
	if err := s.LogSkillRun("hello-world", "{}", "", false, "timeout", 5000); err != nil {
		t.Fatal(err)
	}

	runs, err := s.GetSkillRuns("hello-world", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	// Most recent first
	if runs[0].Success {
		t.Error("expected first run (most recent) to be failure")
	}
	if !runs[1].Success {
		t.Error("expected second run to be success")
	}
}
