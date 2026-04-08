package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ya5u/goemon/internal/memory"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register(&ShellExec{})
	r.Register(&FileRead{})

	if _, err := r.Get("shell_exec"); err != nil {
		t.Errorf("expected shell_exec, got error: %v", err)
	}
	if _, err := r.Get("nonexistent"); err == nil {
		t.Error("expected error for nonexistent tool")
	}

	defs := r.Definitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}
}

func TestShellExec(t *testing.T) {
	s := &ShellExec{}
	result, err := s.Execute(context.Background(), json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", result)
	}
}

func TestFileReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "test.txt")

	// Write
	fw := &FileWrite{}
	result, err := fw.Execute(context.Background(), json.RawMessage(`{"path":"`+path+`","content":"hello world"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "11 bytes") {
		t.Errorf("unexpected write result: %s", result)
	}

	// Read
	fr := &FileRead{}
	result, err = fr.Execute(context.Background(), json.RawMessage(`{"path":"`+path+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got: %s", result)
	}
}

func TestMemoryTools(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := memory.New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	m := NewMemory(store)

	// Store
	result, err := m.Execute(context.Background(), json.RawMessage(`{"action":"store","key":"test.key","value":"test value"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "test.key") {
		t.Errorf("unexpected store result: %s", result)
	}

	// Recall
	result, err = m.Execute(context.Background(), json.RawMessage(`{"action":"recall","key":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "test value") {
		t.Errorf("expected 'test value' in result, got: %s", result)
	}

	// Recall no match
	result, err = m.Execute(context.Background(), json.RawMessage(`{"action":"recall","key":"nonexistent"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No matching") {
		t.Errorf("expected no match message, got: %s", result)
	}
}

func TestWebFetch(t *testing.T) {
	if os.Getenv("GOEMON_INTEGRATION") == "" {
		t.Skip("skipping web test (set GOEMON_INTEGRATION=1)")
	}
	w := NewWebFetch()
	result, err := w.Execute(context.Background(), json.RawMessage(`{"url":"https://httpbin.org/get"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "200") {
		t.Errorf("expected status 200, got: %s", result)
	}
}
