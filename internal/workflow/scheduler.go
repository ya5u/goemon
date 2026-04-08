package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ya5u/goemon/internal/memory"
)

// AgentRunner executes a prompt through the agent and returns the result.
type AgentRunner func(ctx context.Context, prompt string) (string, error)

// ScriptRunner executes a script file with input and returns the result.
type ScriptRunner func(ctx context.Context, dir, entryPoint, input string) (string, error)

// Notifier sends a workflow result to an adapter.
type Notifier func(ctx context.Context, workflowName, message string)

type Scheduler struct {
	manager   *Manager
	runAgent  AgentRunner
	runScript ScriptRunner
	store     *memory.Store
	notify    Notifier
	parser    cron.Parser
	running   map[string]bool
	mu        sync.Mutex
}

func NewScheduler(manager *Manager, runAgent AgentRunner, runScript ScriptRunner, store *memory.Store, notify Notifier) *Scheduler {
	return &Scheduler{
		manager:   manager,
		runAgent:  runAgent,
		runScript: runScript,
		store:     store,
		notify:    notify,
		parser:    cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		running:   make(map[string]bool),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	workflows, err := s.manager.ListWorkflows()
	if err != nil {
		slog.Warn("failed to list workflows", "error", err)
		return
	}

	now := time.Now()
	for _, wf := range workflows {
		if s.shouldRun(wf, now) {
			s.execute(ctx, wf)
		}
	}
}

func (s *Scheduler) shouldRun(wf WorkflowInfo, now time.Time) bool {
	sched, err := s.parser.Parse(wf.Schedule)
	if err != nil {
		slog.Warn("invalid cron expression", "workflow", wf.Name, "schedule", wf.Schedule, "error", err)
		return false
	}

	truncated := now.Truncate(time.Minute)
	prev := truncated.Add(-1 * time.Minute)
	next := sched.Next(prev)

	return next.Equal(truncated)
}

func (s *Scheduler) execute(ctx context.Context, wf WorkflowInfo) {
	s.mu.Lock()
	if s.running[wf.Name] {
		s.mu.Unlock()
		slog.Debug("workflow already running, skipping", "workflow", wf.Name)
		return
	}
	s.running[wf.Name] = true
	s.mu.Unlock()

	slog.Info("starting workflow", "workflow", wf.Name)

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, wf.Name)
			s.mu.Unlock()
		}()

		result, err := RunWorkflowSteps(ctx, wf, s.runAgent, s.runScript, s.store)
		if err != nil {
			slog.Error("workflow failed", "workflow", wf.Name, "error", err)
			return
		}

		slog.Info("workflow completed", "workflow", wf.Name)

		if s.notify != nil && wf.Notify != "" {
			s.notify(ctx, wf.Name, result)
		}
	}()
}

// RunWorkflowSteps executes all steps of a workflow sequentially.
// A shared workspace directory is created for each run. Each step's output
// is saved as a file (step_N_<name>.txt) in the workspace so that subsequent
// steps can read all prior results reliably.
//
// Scripts receive the workspace path via the GOEMON_WORKSPACE environment variable
// and the previous step's output file via GOEMON_PREV_RESULT.
// Prompt steps receive all prior results concatenated in context.
func RunWorkflowSteps(ctx context.Context, wf WorkflowInfo, runAgent AgentRunner, runScript ScriptRunner, store *memory.Store) (string, error) {
	// Create workspace directory
	workspace, err := os.MkdirTemp("", "goemon-workflow-*")
	if err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	slog.Info("workflow workspace created", "workspace", workspace)

	var lastResult string
	var stepFiles []string

	for i, step := range wf.Steps {
		slog.Info("executing workflow step", "workflow", wf.Name, "step", i+1, "name", step.Name, "type", step.Type)

		start := time.Now()
		var result string
		var stepErr error

		// Write previous result to a file for script steps to read
		prevResultFile := ""
		if lastResult != "" && i > 0 {
			prevResultFile = stepFiles[len(stepFiles)-1]
		}

		switch step.Type {
		case StepTypePrompt:
			prompt := step.Prompt
			if lastResult != "" {
				prompt = "Previous step result:\n" + lastResult + "\n\n" + prompt
			}
			result, stepErr = runAgent(ctx, prompt)

		case StepTypeScript:
			// Set environment variables for script
			os.Setenv("GOEMON_WORKSPACE", workspace)
			if prevResultFile != "" {
				os.Setenv("GOEMON_PREV_RESULT", prevResultFile)
			}
			result, stepErr = runScript(ctx, wf.Dir, step.EntryPoint, lastResult)
			os.Unsetenv("GOEMON_WORKSPACE")
			os.Unsetenv("GOEMON_PREV_RESULT")

		default:
			return "", fmt.Errorf("step %q: unknown type %q", step.Name, step.Type)
		}

		durationMs := time.Since(start).Milliseconds()

		// Save step output to workspace file
		stepFile := filepath.Join(workspace, fmt.Sprintf("step_%d_%s.txt", i+1, step.Name))
		output := result
		if stepErr != nil {
			output = fmt.Sprintf("ERROR: %v", stepErr)
		}
		if writeErr := os.WriteFile(stepFile, []byte(output), 0644); writeErr != nil {
			slog.Warn("failed to write step output", "error", writeErr)
		}
		stepFiles = append(stepFiles, stepFile)

		// Log step to database
		if store != nil {
			success := stepErr == nil
			errMsg := ""
			if stepErr != nil {
				errMsg = stepErr.Error()
			}
			input := lastResult
			if step.Type == StepTypePrompt {
				input = step.Prompt
			}
			if logErr := store.LogWorkflowStep(wf.Name, step.Name, string(step.Type), input, output, success, errMsg, durationMs); logErr != nil {
				slog.Warn("failed to log workflow step", "error", logErr)
			}
		}

		if stepErr != nil {
			return "", fmt.Errorf("step %q failed: %w", step.Name, stepErr)
		}

		slog.Info("workflow step completed", "workflow", wf.Name, "step", step.Name, "duration_ms", durationMs, "output_file", stepFile)
		lastResult = result
	}

	return lastResult, nil
}
