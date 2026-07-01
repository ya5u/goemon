package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/ya5u/goemon/internal/adapter"
	"github.com/ya5u/goemon/internal/agent"
	"github.com/ya5u/goemon/internal/config"
	"github.com/ya5u/goemon/internal/llm"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/skill"
	"github.com/ya5u/goemon/internal/tool"
	"github.com/ya5u/goemon/internal/usermemory"
	"github.com/ya5u/goemon/internal/workflow"
	"github.com/ya5u/goemon/templates"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle --verbose flag
	verbose := false
	for _, arg := range os.Args[2:] {
		if arg == "--verbose" || arg == "-v" {
			verbose = true
		}
	}
	if verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	switch os.Args[1] {
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("GoEmon %s\n", version)
	case "chat":
		if err := runChat(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "run":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: goemon run \"<command>\"\n")
			os.Exit(1)
		}
		if err := runOneShot(strings.Join(os.Args[2:], " ")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "skill":
		if err := runSkill(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "workflow":
		if err := runWorkflow(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "memory":
		if err := runMemory(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `GoEmon — Personal AI Agent

Usage:
  goemon <command>

Commands:
  init       Initialize ~/.goemon/ directory
  chat       Start interactive chat session
  run        Run a one-shot command
  serve      Start enabled adapters (Telegram, etc.)
  skill      Manage skills (list, run)
  workflow   Manage workflows (list, run)
  memory     Manage long-term memory (list, show)
  version    Show version
`)
}

func runInit() error {
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	for _, dir := range []string{"skills", "workflows", "memory"} {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0755); err != nil {
			return err
		}
	}

	// Write config
	cfgPath := filepath.Join(dataDir, "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		cfg := config.Default()
		data, err := json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(cfgPath, data, 0644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", cfgPath)
	} else {
		fmt.Printf("Config already exists: %s\n", cfgPath)
	}

	// Write AGENTS.md
	agentsPath := filepath.Join(dataDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if err := os.WriteFile(agentsPath, templates.AgentsMD, 0644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", agentsPath)
	} else {
		fmt.Printf("AGENTS.md already exists: %s\n", agentsPath)
	}

	// Extract standard skills
	if err := extractStandardSkills(filepath.Join(dataDir, "skills")); err != nil {
		return fmt.Errorf("extract standard skills: %w", err)
	}

	fmt.Printf("GoEmon initialized at %s\n", dataDir)
	return nil
}

func extractStandardSkills(skillsDir string) error {
	return fs.WalkDir(templates.StandardSkills, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel("skills", path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		destPath := filepath.Join(skillsDir, relPath)
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		// Don't overwrite existing files
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}
		data, err := templates.StandardSkills.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		fmt.Printf("Extracted skill: %s\n", relPath)
		return nil
	})
}

func setupAgent(cfg *config.Config, store *memory.Store) (*agent.Agent, *agent.Router) {
	backends := make(map[string]llm.Backend)

	for name, bc := range cfg.LLM.Backends {
		switch name {
		case "ollama":
			backends[name] = llm.NewOllama(bc.Endpoint, bc.Model)
		case "openrouter":
			backends[name] = llm.NewOpenRouter(bc.Endpoint, bc.Model, os.Getenv(bc.APIKeyEnv))
		default:
			slog.Warn("unknown LLM backend, skipping", "name", name)
		}
	}

	router := agent.NewRouter(agent.RouterConfig{
		Default:              cfg.LLM.Routing.Default,
		Fallback:             cfg.LLM.Routing.Fallback,
		ForceCloudFor:        cfg.LLM.Routing.ForceCloudFor,
		HealthCheckIntervalS: cfg.LLM.Routing.HealthCheckIntervalS,
	}, backends)

	registry := tool.NewRegistry()
	registry.Register(&tool.ShellExec{})
	registry.Register(&tool.FileRead{})
	registry.Register(&tool.FileEdit{})
	registry.Register(&tool.FileWrite{})
	registry.Register(tool.NewWebFetch())
	registry.Register(tool.NewHistory(store))
	if dataDir, err := config.DataDir(); err == nil {
		registry.Register(tool.NewMemory(usermemory.NewManager(filepath.Join(dataDir, "memory"))))
	}

	callbacks := agent.WithCallbacks(
		func(text string) {
			fmt.Printf("\033[33mGoEmon [thinking]:\033[0m %s\n", text)
		},
		func(name string, args json.RawMessage) {
			fmt.Printf("\033[36mGoEmon [tool:%s]:\033[0m %s\n", name, string(args))
		},
		func(name string, result string) {
			display := result
			if len(display) > 500 {
				display = display[:500] + "..."
			}
			fmt.Printf("\033[90mGoEmon [observe]:\033[0m %s\n", display)
		},
		func(text string) {
			fmt.Printf("\033[32mGoEmon:\033[0m %s\n", text)
		},
	)

	ag := agent.NewAgent(
		router, registry, store,
		agent.WithMaxIterations(cfg.Agent.MaxIterations),
		callbacks,
	)

	return ag, router
}

func runChat() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	store, err := memory.New(filepath.Join(dataDir, "memory.db"))
	if err != nil {
		return fmt.Errorf("open memory: %w", err)
	}
	defer store.Close()

	ag, router := setupAgent(cfg, store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	router.Start(ctx)
	defer router.Stop()

	skillMgr := skill.NewManager(filepath.Join(dataDir, "skills"))

	fmt.Printf("GoEmon %s | LLM: %s\n", version, router.CurrentBackendName())
	fmt.Printf("Type /skills to list skills, /<skill-name> [input] to run one, /quit to exit.\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleSlashCommand(ctx, input, ag, skillMgr) {
				continue
			}
			break
		}

		if _, err := ag.Run(ctx, input); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println()
	}
	return nil
}

func runOneShot(input string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	store, err := memory.New(filepath.Join(dataDir, "memory.db"))
	if err != nil {
		return fmt.Errorf("open memory: %w", err)
	}
	defer store.Close()

	ag, router := setupAgent(cfg, store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	router.Start(ctx)
	defer router.Stop()

	_, err = ag.Run(ctx, input)
	return err
}

func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	store, err := memory.New(filepath.Join(dataDir, "memory.db"))
	if err != nil {
		return fmt.Errorf("open memory: %w", err)
	}
	defer store.Close()

	ag, router := setupAgent(cfg, store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	router.Start(ctx)
	defer router.Stop()

	var adapters []adapter.Adapter

	if cfg.Adapters.Telegram.Enabled {
		tg, err := adapter.NewTelegram(cfg.Adapters.Telegram.BotTokenEnv, cfg.Adapters.Telegram.AllowedUsers)
		if err != nil {
			return fmt.Errorf("telegram adapter: %w", err)
		}
		adapters = append(adapters, tg)
	}

	if len(adapters) == 0 {
		return fmt.Errorf("no adapters enabled in config. Enable at least one adapter in ~/.goemon/config.json")
	}

	fmt.Printf("GoEmon %s | LLM: %s\n", version, router.CurrentBackendName())

	skillMgr := skill.NewManager(filepath.Join(dataDir, "skills"))

	handler := func(ctx context.Context, userMessage string) (string, error) {
		// A "/<skill-name> [input]" message runs that skill directly, skipping
		// the discovery round-trips a plain request would need.
		if name, input, ok := parseSlashCommand(userMessage); ok {
			return runSkillMessage(ctx, ag, skillMgr, name, input)
		}
		return ag.Run(ctx, userMessage)
	}

	// Start workflow scheduler with the same skill manager
	wfMgr := workflow.NewManager(filepath.Join(dataDir, "workflows"))
	scheduler := workflow.NewScheduler(wfMgr, skillMgr, ag.RunWithoutHistory, store,
		func(ctx context.Context, workflowName, message string) {
			for _, a := range adapters {
				if err := a.Send(ctx, fmt.Sprintf("[%s]\n%s", workflowName, message)); err != nil {
					slog.Error("workflow notification failed", "workflow", workflowName, "adapter", a.Name(), "error", err)
				}
			}
		})
	go scheduler.Start(ctx)

	errCh := make(chan error, len(adapters))
	for _, a := range adapters {
		fmt.Printf("Starting adapter: %s\n", a.Name())
		go func(a adapter.Adapter) {
			errCh <- a.Start(ctx, handler)
		}(a)
	}

	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down...")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("adapter error: %w", err)
		}
	}

	for _, a := range adapters {
		a.Stop()
	}

	return nil
}

func runSkill(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `Usage:
  goemon skill list
  goemon skill run <name> [input]
`)
		return nil
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}
	skillsDir := filepath.Join(dataDir, "skills")
	mgr := skill.NewManager(skillsDir)

	switch args[0] {
	case "list":
		skills, err := mgr.ListSkills()
		if err != nil {
			return err
		}
		if len(skills) == 0 {
			fmt.Println("No skills installed.")
			return nil
		}
		for _, s := range skills {
			fmt.Printf("  %s — %s\n", s.Name, s.Description)
		}

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: goemon skill run <name> [input]")
		}
		return runSkillOnce(mgr, args[1], strings.Join(args[2:], " "))

	default:
		return fmt.Errorf("unknown skill command: %s", args[0])
	}
	return nil
}

// runSkillOnce loads a skill's instructions and runs them once through the
// agent (no conversation history). This is the CLI equivalent of a single
// workflow step, for ad-hoc use and testing skills during development.
func runSkillOnce(mgr *skill.Manager, name, input string) error {
	skillInfo, err := mgr.GetSkill(name)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	store, err := memory.New(filepath.Join(dataDir, "memory.db"))
	if err != nil {
		return fmt.Errorf("open memory: %w", err)
	}
	defer store.Close()

	ag, router := setupAgent(cfg, store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	router.Start(ctx)
	defer router.Stop()

	fmt.Printf("Running skill %q...\n", skillInfo.Name)
	_, err = ag.RunWithoutHistory(ctx, skillPrompt(skillInfo.Content, input))
	return err
}

// skillPrompt builds the one-shot prompt for running a skill: its instructions
// plus any caller-provided input. Shared by CLI, chat, and adapter entry points.
func skillPrompt(content, input string) string {
	prompt := "# Task\n\n" + content
	if input != "" {
		prompt += "\n\n# Input\n\n" + input
	}
	return prompt
}

// skillRunner is satisfied by *agent.Agent; it runs a prompt without touching
// conversation history.
type skillRunner interface {
	RunWithoutHistory(ctx context.Context, input string) (string, error)
}

// parseSlashCommand splits a "/<name> [input]" message into its command name
// (without the leading slash) and the trailing input. ok is false if msg does
// not start with "/".
func parseSlashCommand(msg string) (name, input string, ok bool) {
	msg = strings.TrimSpace(msg)
	if !strings.HasPrefix(msg, "/") {
		return "", "", false
	}
	cmd := strings.Fields(msg)[0]
	name = strings.TrimPrefix(cmd, "/")
	input = strings.TrimSpace(strings.TrimPrefix(msg, cmd))
	return name, input, true
}

// skillListText renders the installed skills as a "/name — description" list.
func skillListText(mgr *skill.Manager) string {
	skills, err := mgr.ListSkills()
	if err != nil {
		return fmt.Sprintf("Error listing skills: %v", err)
	}
	if len(skills) == 0 {
		return "No skills installed."
	}
	var sb strings.Builder
	sb.WriteString("Run a skill with /<name> [input]:\n")
	for _, s := range skills {
		fmt.Fprintf(&sb, "/%s — %s\n", s.Name, s.Description)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// runSkillMessage handles a slash message ("/skills" or "/<skill> [input]") by
// running the referenced skill and returning its result. Used by adapters,
// which need the result as a string rather than streamed callbacks.
func runSkillMessage(ctx context.Context, ag skillRunner, mgr *skill.Manager, name, input string) (string, error) {
	if name == "skills" || name == "help" {
		return skillListText(mgr), nil
	}
	skillInfo, err := mgr.GetSkill(name)
	if err != nil {
		return "Unknown skill: " + name + "\n\n" + skillListText(mgr), nil
	}
	return ag.RunWithoutHistory(ctx, skillPrompt(skillInfo.Content, input))
}

func runWorkflow(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `Usage:
  goemon workflow list
  goemon workflow run <name>
`)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	wfMgr := workflow.NewManager(filepath.Join(dataDir, "workflows"))

	switch args[0] {
	case "list":
		workflows, err := wfMgr.ListWorkflows()
		if err != nil {
			return err
		}
		if len(workflows) == 0 {
			fmt.Println("No workflows installed.")
			return nil
		}
		for _, wf := range workflows {
			fmt.Printf("  %s — schedule: %s\n", wf.Name, wf.Schedule)
		}

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: goemon workflow run <name>")
		}
		wf, err := wfMgr.GetWorkflow(args[1])
		if err != nil {
			return err
		}

		store, err := memory.New(filepath.Join(dataDir, "memory.db"))
		if err != nil {
			return fmt.Errorf("open memory: %w", err)
		}
		defer store.Close()

		ag, router := setupAgent(cfg, store)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		router.Start(ctx)
		defer router.Stop()

		skillMgr := skill.NewManager(filepath.Join(dataDir, "skills"))

		fmt.Printf("Running workflow %q...\n", wf.Name)
		result, err := workflow.RunWorkflowSteps(ctx, *wf, skillMgr, ag.RunWithoutHistory, store)
		if err != nil {
			return fmt.Errorf("workflow failed: %w", err)
		}
		fmt.Println(result)

	default:
		return fmt.Errorf("unknown workflow command: %s", args[0])
	}
	return nil
}

func runMemory(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `Usage:
  goemon memory list
  goemon memory show <name>
`)
		return nil
	}

	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}
	mgr := usermemory.NewManager(filepath.Join(dataDir, "memory"))

	switch args[0] {
	case "list":
		entries, err := mgr.List()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No memories yet.")
			return nil
		}
		for _, e := range entries {
			fmt.Printf("  %s (%s) — %s\n", e.Name, e.Type, e.Description)
		}

	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: goemon memory show <name>")
		}
		e, err := mgr.Get(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("# %s (%s)\n%s\n\n%s\n", e.Name, e.Type, e.Description, e.Content)

	default:
		return fmt.Errorf("unknown memory command: %s", args[0])
	}
	return nil
}

// handleSlashCommand handles chat slash commands. It returns false to end the
// session. Besides the built-in commands, any /<skill-name> [input] runs that
// skill once through the current agent.
func handleSlashCommand(ctx context.Context, input string, ag *agent.Agent, skillMgr *skill.Manager) bool {
	cmd := strings.Fields(input)[0]
	switch cmd {
	case "/quit", "/exit":
		fmt.Println("Goodbye!")
		return false
	case "/tools":
		fmt.Println("Available tools: shell_exec, file_read, file_edit, file_write, web_fetch, conversation_history, memory")
	case "/skills":
		skills, err := skillMgr.ListSkills()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else if len(skills) == 0 {
			fmt.Println("No skills installed.")
		} else {
			fmt.Println("Run a skill with /<name> [input]:")
			for _, s := range skills {
				fmt.Printf("  /%s — %s\n", s.Name, s.Description)
			}
		}
	case "/memory":
		fmt.Println("Use: goemon memory list | goemon memory show <name>")
	case "/config":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
		}
	default:
		// Treat /<name> [input] as "run this skill".
		name := strings.TrimPrefix(cmd, "/")
		skillInfo, err := skillMgr.GetSkill(name)
		if err != nil {
			fmt.Printf("Unknown command or skill: %s\n", cmd)
			fmt.Println("Available: /quit, /tools, /skills, /memory, /config, or /<skill-name> [input]")
			return true
		}
		skillInput := strings.TrimSpace(strings.TrimPrefix(input, cmd))
		fmt.Printf("Running skill %q...\n", skillInfo.Name)
		if _, err := ag.RunWithoutHistory(ctx, skillPrompt(skillInfo.Content, skillInput)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	return true
}
