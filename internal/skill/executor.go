package skill

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ya5u/goemon/internal/memory"
)

type Executor struct {
	store *memory.Store
}

func NewExecutor(store *memory.Store) *Executor {
	return &Executor{store: store}
}

func (e *Executor) Run(ctx context.Context, info *SkillInfo, input string) (string, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	entryPoint := filepath.Join(info.Dir, info.EntryPoint)

	var cmd *exec.Cmd
	switch info.Language {
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, "bash", entryPoint)
	case "python", "python3":
		cmd = exec.CommandContext(ctx, "python3", entryPoint)
	default:
		cmd = exec.CommandContext(ctx, entryPoint)
	}

	cmd.Dir = info.Dir
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start).Milliseconds()
	output := stdout.String()

	if stderr.Len() > 0 {
		slog.Debug("skill stderr", "skill", info.Name, "stderr", stderr.String())
	}

	success := err == nil
	errMsg := ""
	if err != nil {
		errMsg = fmt.Sprintf("%v: %s", err, stderr.String())
		output = errMsg
	}

	if e.store != nil {
		if logErr := e.store.LogSkillRun(info.Name, input, output, success, errMsg, duration); logErr != nil {
			slog.Warn("failed to log skill run", "error", logErr)
		}
	}

	return output, err
}
