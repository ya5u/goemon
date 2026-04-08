package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// FileRead

type FileRead struct{}

func (f *FileRead) Name() string        { return "file_read" }
func (f *FileRead) Description() string { return "Read the contents of a file." }

func (f *FileRead) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file",
			},
		},
		"required": []string{"path"},
	}
}

func (f *FileRead) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("file_read", "path", params.Path)

	data, err := os.ReadFile(params.Path)
	if err != nil {
		return "", err
	}

	const maxSize = 100 * 1024
	if len(data) > maxSize {
		return string(data[:maxSize]) + "\n... (truncated at 100KB)", nil
	}
	return string(data), nil
}

// FileWrite

type FileWrite struct{}

func (f *FileWrite) Name() string { return "file_write" }
func (f *FileWrite) Description() string {
	return "Write content to a file. Creates parent directories if needed."
}

func (f *FileWrite) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Absolute or relative path to the file",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (f *FileWrite) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("file_write", "path", params.Path)

	if err := os.MkdirAll(filepath.Dir(params.Path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Written %d bytes to %s", len(params.Content), params.Path), nil
}
