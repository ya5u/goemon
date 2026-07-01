package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/skill"
)

// AgentRunner executes a prompt through the agent and returns the result.
type AgentRunner func(ctx context.Context, prompt string) (string, error)

// Notifier sends a workflow result to an adapter.
type Notifier func(ctx context.Context, workflowName, message string)

const maxStepRetries = 3

type Scheduler struct {
	manager   *Manager
	skillMgr  *skill.Manager
	runAgent  AgentRunner
	store     *memory.Store
	notify    Notifier
	parser    cron.Parser
	running   map[string]bool
	mu        sync.Mutex
}

func NewScheduler(manager *Manager, skillMgr *skill.Manager, runAgent AgentRunner, store *memory.Store, notify Notifier) *Scheduler {
	return &Scheduler{
		manager:  manager,
		skillMgr: skillMgr,
		runAgent: runAgent,
		store:    store,
		notify:   notify,
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		running:  make(map[string]bool),
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

		result, err := RunWorkflowSteps(ctx, wf, s.skillMgr, s.runAgent, s.store)
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
//
// For each step:
//  1. Load the Skill's SKILL.md instructions
//  2. Build a prompt combining skill instructions + step-specific context + workspace state
//  3. Run the agent (ReAct loop with tools)
//  4. Ask the LLM to verify the completion criteria
//  5. Retry up to maxStepRetries times on failure
func RunWorkflowSteps(ctx context.Context, wf WorkflowInfo, skillMgr *skill.Manager, runAgent AgentRunner, store *memory.Store) (string, error) {
	workspace, err := os.MkdirTemp("", "goemon-workflow-*")
	if err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	slog.Info("workflow workspace created", "workspace", workspace)

	var lastResult string

	for i, step := range wf.Steps {
		slog.Info("executing workflow step", "workflow", wf.Name, "step", i+1, "skill", step.SkillName)

		start := time.Now()
		var result string
		var stepErr error

		// Load skill instructions
		skillInfo, err := skillMgr.GetSkill(step.SkillName)
		if err != nil {
			return "", fmt.Errorf("step %q: load skill %q: %w", step.SkillName, step.SkillName, err)
		}

		var attempt int
		var feedback string
		var validationOk bool
		for attempt = 0; attempt < maxStepRetries; attempt++ {
			prompt := buildStepPrompt(skillInfo.Content, step.Content, workspace, lastResult, feedback)
			result, stepErr = runAgent(ctx, prompt)
			if stepErr != nil {
				feedback = fmt.Sprintf("The previous attempt errored: %v", stepErr)
				continue
			}

			// Validate completion criteria
			var reason string
			validationOk, reason = validateStep(ctx, step.Content, workspace, result, runAgent)
			if validationOk {
				break
			}

			slog.Info("step validation failed, retrying",
				"workflow", wf.Name, "skill", step.SkillName,
				"attempt", attempt+1, "reason", reason)
			feedback = fmt.Sprintf("Completion criteria not met: %s", reason)
		}

		// If all retries exhausted without validation passing, propagate as error
		if stepErr == nil && !validationOk {
			stepErr = fmt.Errorf("completion criteria not met after %d attempts", maxStepRetries)
		}

		durationMs := time.Since(start).Milliseconds()

		// Save step output to workspace
		stepFile := filepath.Join(workspace, fmt.Sprintf("step_%d_%s.txt", i+1, step.SkillName))
		output := result
		if stepErr != nil {
			output = fmt.Sprintf("ERROR: %v", stepErr)
		}
		if writeErr := os.WriteFile(stepFile, []byte(output), 0644); writeErr != nil {
			slog.Warn("failed to write step output", "error", writeErr)
		}

		// Log to DB
		if store != nil {
			success := stepErr == nil
			errMsg := ""
			if stepErr != nil {
				errMsg = stepErr.Error()
			}
			if logErr := store.LogWorkflowStep(wf.Name, step.SkillName, "skill", step.Content, output, success, errMsg, durationMs); logErr != nil {
				slog.Warn("failed to log workflow step", "error", logErr)
			}
		}

		if stepErr != nil {
			return "", fmt.Errorf("step %q failed after %d attempts: %w", step.SkillName, attempt, stepErr)
		}

		slog.Info("workflow step completed",
			"workflow", wf.Name, "skill", step.SkillName,
			"attempts", attempt+1, "duration_ms", durationMs)
		lastResult = result
	}

	return lastResult, nil
}

// buildStepPrompt constructs the prompt for a workflow step.
func buildStepPrompt(skillContent, stepContent, workspace, prevResult, feedback string) string {
	var b strings.Builder

	b.WriteString("# Task\n\n")
	b.WriteString(skillContent)
	b.WriteString("\n\n")

	b.WriteString("# Step instructions\n\n")
	b.WriteString(stepContent)
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "# Workspace\n\nWorking directory: %s\n\n", workspace)

	if prevResult != "" {
		b.WriteString("# Previous step result\n\n")
		b.WriteString(prevResult)
		b.WriteString("\n\n")
	}

	if feedback != "" {
		b.WriteString("# Feedback (from the previous attempt)\n\n")
		b.WriteString(feedback)
		b.WriteString("\n\n")
	}

	b.WriteString("When the task is complete, briefly report what you did.")

	return b.String()
}

// validateStep asks the LLM to verify whether the completion criteria in stepContent are met.
// Returns (true, "") on success, or (false, reason) on failure.
func validateStep(ctx context.Context, stepContent, workspace, result string, runAgent AgentRunner) (bool, string) {
	// stepContent comes from user-authored WORKFLOW.md and may be written in
	// English or Japanese; trigger validation when it mentions completion
	// criteria in either language.
	lc := strings.ToLower(stepContent)
	if !strings.Contains(lc, "completion criteria") &&
		!strings.Contains(lc, "success") &&
		!strings.Contains(lc, "verify") &&
		!strings.Contains(stepContent, "完了条件") &&
		!strings.Contains(stepContent, "検証") {
		// No explicit criteria — treat as successful
		return true, ""
	}

	validationPrompt := fmt.Sprintf(`Check whether the completion criteria for the following step are met.

# Step instructions (including completion criteria)

%s

# Result

%s

# Workspace

%s

Verify that all completion criteria are satisfied.
- If they are: reply with exactly "COMPLETE"
- If not: reply "INCOMPLETE: <what is missing>"

Use tools to check when necessary (e.g. confirm a file exists).`, stepContent, result, workspace)

	response, err := runAgent(ctx, validationPrompt)
	if err != nil {
		// Validation error — assume success to avoid infinite retries
		slog.Warn("validation agent call failed", "error", err)
		return true, ""
	}

	if strings.HasPrefix(strings.TrimSpace(response), "COMPLETE") {
		return true, ""
	}

	reason := strings.TrimPrefix(strings.TrimSpace(response), "INCOMPLETE:")
	return false, strings.TrimSpace(reason)
}
