package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ya5u/goemon/internal/llm"
)

// Plan represents a structured multi-step plan.
type Plan struct {
	Goal  string `json:"goal"`
	Steps []Step `json:"steps"`
}

// Step represents a single step in a plan.
type Step struct {
	ID             int      `json:"id"`
	Description    string   `json:"description"`
	ToolsHint      []string `json:"tools_hint,omitempty"`
	ExpectedOutput string   `json:"expected_output"`
}

// StepResult holds the outcome of executing a step.
type StepResult struct {
	StepID      int
	Description string
	Output      string
	Summary     string
	Success     bool
	Error       string
}

const planningSystemPrompt = `You are a task planner. Given a user request, break it down into sequential steps.

IMPORTANT: Respond with ONLY a JSON object. No explanation, no markdown, no text before or after the JSON.

The JSON must follow this exact structure:
{"goal":"overall goal","steps":[{"id":1,"description":"what to do","tools_hint":["tool_name"],"expected_output":"what success looks like"}]}

Rules:
- Each step should be a single, focused action
- Steps execute sequentially; each step can use results from previous steps
- Keep steps atomic: one tool call or one logical action per step
- 3-8 steps maximum
- tools_hint should list which tools the step will likely need`

// generatePlan calls the LLM to produce a structured Plan from the user input.
func (a *Agent) generatePlan(ctx context.Context, userInput string) (*Plan, error) {
	// Build tool list for the planning prompt
	toolDefs := a.registry.Definitions()
	var toolList strings.Builder
	toolList.WriteString("\nAvailable tools:\n")
	for _, t := range toolDefs {
		fmt.Fprintf(&toolList, "- %s: %s\n", t.Name, t.Description)
	}

	messages := []llm.Message{
		{Role: "system", Content: planningSystemPrompt + toolList.String()},
		{Role: "user", Content: userInput},
	}

	content, err := a.callLLM(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("planning LLM call: %w", err)
	}

	plan, err := parsePlan(content)
	if err != nil {
		slog.Warn("plan parse failed, retrying", "error", err)
		// Retry with explicit instruction
		messages = append(messages,
			llm.Message{Role: "assistant", Content: content},
			llm.Message{Role: "user", Content: "That was not valid JSON. Please respond with ONLY a JSON object, no other text."},
		)
		content, err = a.callLLM(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("planning retry LLM call: %w", err)
		}
		plan, err = parsePlan(content)
		if err != nil {
			return nil, fmt.Errorf("plan parse failed after retry: %w", err)
		}
	}

	if len(plan.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	slog.Info("plan generated", "goal", plan.Goal, "steps", len(plan.Steps))
	return plan, nil
}

// parsePlan extracts a Plan from LLM output, handling markdown fences and extra text.
func parsePlan(content string) (*Plan, error) {
	content = strings.TrimSpace(content)

	// Strip markdown code fences
	codeBlockRe := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
	if matches := codeBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		content = strings.TrimSpace(matches[1])
	}

	// Try to find JSON object in content
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var plan Plan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w (content: %.200s)", err, content)
	}
	return &plan, nil
}

// needsPlanning determines if a request is complex enough to warrant plan-and-execute.
func needsPlanning(input string) bool {
	signals := 0

	// Check for numbered steps (1. 2. 3. or 1) 2) 3))
	numberedRe := regexp.MustCompile(`(?m)^\s*\d+[.)]\s`)
	if matches := numberedRe.FindAllString(input, -1); len(matches) >= 3 {
		signals++
	}

	// Check for sequential keywords
	seqKeywords := []string{
		"first", "then", "next", "finally", "after that", "last",
		"まず", "次に", "その後", "最後に", "続いて",
		"step ", "ステップ",
	}
	seqCount := 0
	lower := strings.ToLower(input)
	for _, kw := range seqKeywords {
		if strings.Contains(lower, kw) {
			seqCount++
		}
	}
	if seqCount >= 2 {
		signals++
	}

	// Check for multiple action verbs
	actionVerbs := []string{
		"search", "fetch", "write", "create", "read", "execute", "run",
		"send", "install", "delete", "update", "build", "deploy",
		"検索", "取得", "作成", "書", "実行", "送信", "生成",
	}
	verbCount := 0
	for _, v := range actionVerbs {
		if strings.Contains(lower, v) {
			verbCount++
		}
	}
	if verbCount >= 3 {
		signals++
	}

	// Long input with multiple sentences
	if len(input) > 500 {
		sentences := strings.Count(input, "。") + strings.Count(input, ". ") + strings.Count(input, ".\n")
		if sentences >= 3 {
			signals++
		}
	}

	return signals >= 2
}
