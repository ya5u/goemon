package usermemory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveListGetDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "memory")
	m := NewManager(dir)

	if err := m.Save(Entry{
		Name:        "Prefers Japanese Responses",
		Type:        "user",
		Description: "Wants all replies in Japanese",
		Content:     "Always respond in Japanese.",
	}); err != nil {
		t.Fatal(err)
	}

	// File is written under the normalized slug.
	if _, err := os.Stat(filepath.Join(dir, "prefers-japanese-responses.md")); err != nil {
		t.Fatalf("expected slug file: %v", err)
	}

	entries, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "prefers-japanese-responses" {
		t.Fatalf("unexpected list: %+v", entries)
	}
	if entries[0].Type != "user" || entries[0].Description != "Wants all replies in Japanese" {
		t.Fatalf("metadata not parsed: %+v", entries[0])
	}

	got, err := m.Get("prefers-japanese-responses")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "Always respond in Japanese.") {
		t.Fatalf("body missing: %q", got.Content)
	}

	// Index reflects the memory.
	idx, err := m.IndexText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(idx, "prefers-japanese-responses (user): Wants all replies in Japanese") {
		t.Fatalf("unexpected index: %q", idx)
	}

	// MEMORY.md is regenerated, not counted as a memory.
	if _, err := os.Stat(filepath.Join(dir, indexFile)); err != nil {
		t.Fatalf("expected index file: %v", err)
	}
	if entries, _ := m.List(); len(entries) != 1 {
		t.Fatalf("index file should not be listed as a memory")
	}

	if err := m.Delete("prefers-japanese-responses"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := m.List(); len(entries) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(entries))
	}
}

func TestSaveUpdatesInPlace(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "memory"))
	_ = m.Save(Entry{Name: "x", Type: "user", Description: "old", Content: "a"})
	_ = m.Save(Entry{Name: "x", Type: "user", Description: "new", Content: "b"})

	entries, _ := m.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after update, got %d", len(entries))
	}
	if entries[0].Description != "new" {
		t.Fatalf("expected updated description, got %q", entries[0].Description)
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Prefers Japanese": "prefers-japanese",
		"  Hello World!  ":  "hello-world",
		"already-kebab":     "already-kebab",
		"UPPER_Case":        "uppercase",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
