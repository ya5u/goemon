// Package usermemory implements GoEmon's long-term, file-based memory: a
// directory of human-readable Markdown facts about the user, plus an index
// (MEMORY.md) that is injected into the agent's system prompt on every call.
//
// This is intentionally separate from internal/memory (the SQLite store, which
// holds verbatim conversation history). Memory here is distilled, low-volume,
// durable, and editable by hand.
package usermemory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Entry is a single memory: one fact, stored as one Markdown file.
type Entry struct {
	Name        string // kebab-case slug, also the filename stem
	Type        string // user | feedback | task | reference
	Description string // one-line summary, shown in the index
	Content     string // Markdown body (loaded only by Get)
}

const indexFile = "MEMORY.md"

type Manager struct {
	dir string
}

func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

func (m *Manager) Dir() string { return m.dir }

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// Slug normalizes an arbitrary name into a filesystem-safe kebab-case slug.
func Slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	return s
}

// List returns metadata (no body) for every memory, sorted by name.
func (m *Manager) List() ([]Entry, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Entry
	for _, e := range entries {
		if e.IsDir() || e.Name() == indexFile || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, e.Name()))
		if err != nil {
			continue
		}
		meta, _ := parse(string(data))
		if meta.Name == "" {
			meta.Name = strings.TrimSuffix(e.Name(), ".md")
		}
		out = append(out, Entry{Name: meta.Name, Type: meta.Type, Description: meta.Description})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get loads the full memory (including body) by name.
func (m *Manager) Get(name string) (*Entry, error) {
	path := filepath.Join(m.dir, Slug(name)+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read memory %q: %w", name, err)
	}
	meta, body := parse(string(data))
	if meta.Name == "" {
		meta.Name = Slug(name)
	}
	return &Entry{Name: meta.Name, Type: meta.Type, Description: meta.Description, Content: body}, nil
}

// Save writes (or overwrites) a memory file, then rebuilds the index.
func (m *Manager) Save(e Entry) error {
	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return err
	}
	slug := Slug(e.Name)
	if slug == "" {
		return fmt.Errorf("invalid memory name: %q", e.Name)
	}
	typ := e.Type
	if typ == "" {
		typ = "reference"
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", slug)
	fmt.Fprintf(&b, "description: %s\n", strings.ReplaceAll(e.Description, "\n", " "))
	fmt.Fprintf(&b, "type: %s\n", typ)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(e.Content))
	b.WriteString("\n")

	if err := os.WriteFile(filepath.Join(m.dir, slug+".md"), []byte(b.String()), 0644); err != nil {
		return err
	}
	return m.rebuildIndex()
}

// Delete removes a memory file and rebuilds the index.
func (m *Manager) Delete(name string) error {
	path := filepath.Join(m.dir, Slug(name)+".md")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete memory %q: %w", name, err)
	}
	return m.rebuildIndex()
}

// IndexText returns the human-readable index that is injected into the system
// prompt. It is regenerated from the current files so it never drifts.
func (m *Manager) IndexText() (string, error) {
	entries, err := m.List()
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, e := range entries {
		typ := e.Type
		if typ == "" {
			typ = "reference"
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", e.Name, typ, e.Description)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// rebuildIndex regenerates MEMORY.md from the current memory files.
func (m *Manager) rebuildIndex() error {
	idx, err := m.IndexText()
	if err != nil {
		return err
	}
	content := "# Memory Index\n\nFacts GoEmon has learned about the user. One file per fact.\n\n"
	if idx == "" {
		content += "_(empty)_\n"
	} else {
		content += idx + "\n"
	}
	return os.WriteFile(filepath.Join(m.dir, indexFile), []byte(content), 0644)
}

type meta struct {
	Name        string
	Type        string
	Description string
}

// parse splits YAML frontmatter (name/type/description) from the Markdown body.
func parse(content string) (meta, string) {
	var mt meta
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return mt, content
	}
	rest := strings.TrimPrefix(trimmed, "---")
	frontmatter, after, found := strings.Cut(rest, "\n---")
	if !found {
		return mt, content
	}
	body := strings.TrimSpace(after)
	for line := range strings.SplitSeq(frontmatter, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch k {
		case "name":
			mt.Name = v
		case "type":
			mt.Type = v
		case "description":
			mt.Description = v
		}
	}
	return mt, body
}
