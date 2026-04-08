package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ya5u/goemon/internal/llm"
)

// runPlanAndExecute orchestrates the full plan-and-execute flow.
func (a *Agent) runPlanAndExecute(ctx context.Context, userInput string) (string, error) {
	// Phase 1: Generate plan
	a.onThinking("Generating plan...")
	plan, err := a.generatePlan(ctx, userInput)
	if err != nil {
		// Fall back to simple ReAct if planning fails
		slog.Warn("planning failed, falling back to simple execution", "error", err)
		return a.runSimple(ctx, userInput)
	}

	// Report plan
	var planSummary strings.Builder
	fmt.Fprintf(&planSummary, "Plan: %s (%d steps)", plan.Goal, len(plan.Steps))
	for _, s := range plan.Steps {
		fmt.Fprintf(&planSummary, "\n  %d. %s", s.ID, s.Description)
	}
	a.onThinking(planSummary.String())

	// Phase 2: Execute each step
	var results []StepResult
	replanCount := 0

	for i := 0; i < len(plan.Steps); i++ {
		step := plan.Steps[i]
		a.onThinking(fmt.Sprintf("Step %d/%d: %s", i+1, len(plan.Steps), step.Description))

		result := a.executeStep(ctx, step, results, plan.Goal)

		if !result.Success && replanCount < 2 {
			slog.Warn("step failed, attempting replan", "step", step.ID, "error", result.Error)
			newPlan, err := a.replan(ctx, plan.Goal, results, step, result.Error)
			if err != nil {
				slog.Warn("replan failed", "error", err)
				results = append(results, *result)
				continue
			}
			plan.Steps = newPlan.Steps
			i = -1 // restart from first new step
			replanCount++
			a.onThinking(fmt.Sprintf("Replanned with %d new steps", len(plan.Steps)))
			continue
		}

		result.Summary = result.Output

		results = append(results, *result)
	}

	// Phase 3: Synthesize final response
	a.onThinking("Synthesizing final response...")
	finalResponse, err := a.synthesizeResults(ctx, plan.Goal, results)
	if err != nil {
		// Fallback: concatenate results
		slog.Warn("synthesis failed, concatenating results", "error", err)
		var sb strings.Builder
		for _, r := range results {
			if r.Success {
				fmt.Fprintf(&sb, "## Step %d: %s\n%s\n\n", r.StepID, r.Description, r.Summary)
			}
		}
		finalResponse = sb.String()
	}

	a.onResponse(finalResponse)
	if !a.skipHistory {
		if err := a.store.SaveMessage("assistant", finalResponse); err != nil {
			slog.Warn("failed to save assistant message", "error", err)
		}
	}
	return finalResponse, nil
}

// executeStep runs a single step using the ReAct loop with a focused prompt.
func (a *Agent) executeStep(ctx context.Context, step Step, priorResults []StepResult, goal string) *StepResult {
	// Build step-specific context
	var contextMsg strings.Builder
	fmt.Fprintf(&contextMsg, "Overall goal: %s\n\n", goal)

	if len(priorResults) > 0 {
		contextMsg.WriteString("Completed steps:\n")
		for _, r := range priorResults {
			status := "OK"
			if !r.Success {
				status = "FAILED"
			}
			fmt.Fprintf(&contextMsg, "- Step %d (%s) [%s]: %s\n", r.StepID, r.Description, status, r.Summary)
		}
		contextMsg.WriteString("\n")
	}

	fmt.Fprintf(&contextMsg, "YOUR CURRENT TASK (Step %d):\n%s\n\n", step.ID, step.Description)
	if step.ExpectedOutput != "" {
		fmt.Fprintf(&contextMsg, "Expected output: %s\n\n", step.ExpectedOutput)
	}
	contextMsg.WriteString("Complete ONLY this step, then provide the result. Do NOT attempt work beyond this step.")

	messages := []llm.Message{
		{Role: "system", Content: `You are a task executor. You have no prior context, no conversation history, and no knowledge of the current environment.
Do NOT explore the environment (e.g. ls, pwd). Execute the task directly using the information provided.
When using shell_exec, always use absolute paths and explicit commands. Do not assume any working directory.
When done, provide a clear summary of what was accomplished.`},
		{Role: "user", Content: contextMsg.String()},
	}

	output, err := a.executeReAct(ctx, messages, a.registry.Definitions())
	if err != nil {
		return &StepResult{
			StepID:      step.ID,
			Description: step.Description,
			Success:     false,
			Error:       err.Error(),
		}
	}

	return &StepResult{
		StepID:      step.ID,
		Description: step.Description,
		Output:      output,
		Success:     true,
	}
}

// summarizeResult condenses a long step result for context passing.
func (a *Agent) summarizeResult(ctx context.Context, result string, stepDescription string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "Summarize the following result concisely (max 500 characters). Preserve key facts, data, URLs, and names. Remove boilerplate."},
		{Role: "user", Content: fmt.Sprintf("Context: Result of \"%s\"\n\nResult:\n%s", stepDescription, result)},
	}
	return a.callLLM(ctx, messages)
}

// synthesizeResults combines all step results into a coherent final response.
func (a *Agent) synthesizeResults(ctx context.Context, goal string, results []StepResult) (string, error) {
	var content strings.Builder
	fmt.Fprintf(&content, "You completed a multi-step task. Combine the results into a final response.\n\n")
	fmt.Fprintf(&content, "Goal: %s\n\n", goal)
	content.WriteString("Step results:\n")
	for _, r := range results {
		if r.Success {
			fmt.Fprintf(&content, "- Step %d (%s): %s\n", r.StepID, r.Description, r.Summary)
		} else {
			fmt.Fprintf(&content, "- Step %d (%s): FAILED - %s\n", r.StepID, r.Description, r.Error)
		}
	}
	content.WriteString("\nProvide a clear, complete response that addresses the original goal.")

	messages := []llm.Message{
		{Role: "system", Content: "You are summarizing the results of a completed multi-step task. Provide a coherent response for the user."},
		{Role: "user", Content: content.String()},
	}
	return a.callLLM(ctx, messages)
}

// replan generates a new plan for remaining work after a step failure.
func (a *Agent) replan(ctx context.Context, originalGoal string, completedResults []StepResult, failedStep Step, errMsg string) (*Plan, error) {
	var context strings.Builder
	fmt.Fprintf(&context, "Original goal: %s\n\n", originalGoal)
	context.WriteString("Completed steps:\n")
	for _, r := range completedResults {
		fmt.Fprintf(&context, "- Step %d (%s): %s\n", r.StepID, r.Description, r.Summary)
	}
	fmt.Fprintf(&context, "\nFailed step: %s\nError: %s\n\n", failedStep.Description, errMsg)
	context.WriteString("Create a new plan for the REMAINING work, taking into account what has already been completed and the error encountered.")

	return a.generatePlan(ctx, context.String())
}
