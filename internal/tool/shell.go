package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

type ShellExec struct{}

func (s *ShellExec) Name() string        { return "shell_exec" }
func (s *ShellExec) Description() string { return "Execute a shell command and return its output." }

func (s *ShellExec) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (s *ShellExec) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	slog.Info("shell_exec", "command", params.Command)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR:\n" + stderr.String()
	}
	if err != nil {
		output += "\nERROR: " + err.Error()
	}

	const maxOutput = 10000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (truncated)"
	}

	return output, nil
}
