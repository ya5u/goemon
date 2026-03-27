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

	"github.com/ya5u/goemon/internal/agent"
	"github.com/ya5u/goemon/internal/config"
	"github.com/ya5u/goemon/internal/llm"
	"github.com/ya5u/goemon/internal/memory"
	"github.com/ya5u/goemon/internal/skill"
	"github.com/ya5u/goemon/internal/skill/stdskills"
	"github.com/ya5u/goemon/internal/tool"
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
	case "skill":
		if err := runSkill(os.Args[2:]); err != nil {
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
  skill      Manage skills (list, run, create, install, remove)
  version    Show version
`)
}

func runInit() error {
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	skillsDir := filepath.Join(dataDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
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

	// Extract standard skills
	if err := extractStandardSkills(skillsDir); err != nil {
		return fmt.Errorf("extract standard skills: %w", err)
	}

	fmt.Printf("GoEmon initialized at %s\n", dataDir)
	return nil
}

func extractStandardSkills(skillsDir string) error {
	return fs.WalkDir(stdskills.StandardSkills, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "skills/" prefix to get the relative path
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

		// Don't overwrite existing files (user may have modified them)
		if _, err := os.Stat(destPath); err == nil {
			return nil
		}

		data, err := stdskills.StandardSkills.ReadFile(path)
		if err != nil {
			return err
		}

		perm := os.FileMode(0644)
		if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py") {
			perm = 0755
		}

		if err := os.WriteFile(destPath, data, perm); err != nil {
			return err
		}
		fmt.Printf("Extracted skill: %s\n", relPath)
		return nil
	})
}

func setupAgent(cfg *config.Config, store *memory.Store) (*agent.Agent, *agent.Router) {
	// Create backends
	backends := make(map[string]llm.Backend)

	if bc, ok := cfg.LLM.Backends["ollama"]; ok {
		backends["ollama"] = llm.NewOllama(bc.Endpoint, bc.Model)
	}

	router := agent.NewRouter(agent.RouterConfig{
		Default:              cfg.LLM.Routing.Default,
		Fallback:             cfg.LLM.Routing.Fallback,
		ForceCloudFor:        cfg.LLM.Routing.ForceCloudFor,
		HealthCheckIntervalS: cfg.LLM.Routing.HealthCheckIntervalS,
	}, backends)

	// Create tool registry
	registry := tool.NewRegistry()
	registry.Register(&tool.ShellExec{})
	registry.Register(&tool.FileRead{})
	registry.Register(&tool.FileWrite{})
	registry.Register(tool.NewWebFetch())
	registry.Register(tool.NewMemoryStore(store))
	registry.Register(tool.NewMemoryRecall(store))

	// Skill tools
	dataDir, _ := config.DataDir()
	skillsDir := filepath.Join(dataDir, "skills")
	mgr := skill.NewManager(skillsDir)
	executor := skill.NewExecutor(store)
	registry.Register(skill.NewSkillListTool(mgr))
	registry.Register(skill.NewSkillRunTool(mgr, executor))
	// skill_create requires a backend; we'll register it after router is available

	callbacks := agent.WithCallbacks(
		func(text string) {
			fmt.Printf("\033[33mGoEmon [thinking]:\033[0m %s\n", text)
		},
		func(name string, args json.RawMessage) {
			fmt.Printf("\033[36mGoEmon [tool:%s]:\033[0m %s\n", name, string(args))
		},
		func(name string, result string) {
			// Truncate long results for display
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

	fmt.Printf("GoEmon %s | LLM: %s\n\n", version, router.CurrentBackendName())

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

		// Slash commands
		if strings.HasPrefix(input, "/") {
			if handleSlashCommand(input, ag) {
				continue
			}
			break // /quit
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

func runSkill(args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `Usage:
  goemon skill list
  goemon skill run <name> [input-json]
  goemon skill install <github-url>
  goemon skill remove <name>
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
			return fmt.Errorf("usage: goemon skill run <name> [input-json]")
		}
		store, err := memory.New(filepath.Join(dataDir, "memory.db"))
		if err != nil {
			return err
		}
		defer store.Close()

		info, err := mgr.GetSkill(args[1])
		if err != nil {
			return err
		}
		input := "{}"
		if len(args) >= 3 {
			input = args[2]
		}
		executor := skill.NewExecutor(store)
		output, err := executor.Run(context.Background(), info, input)
		if err != nil {
			return err
		}
		fmt.Println(output)

	case "install":
		if len(args) < 2 {
			return fmt.Errorf("usage: goemon skill install <github-url>")
		}
		installer := skill.NewInstaller(skillsDir)
		info, err := installer.Install(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Installed skill %q: %s\n", info.Name, info.Description)

	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: goemon skill remove <name>")
		}
		installer := skill.NewInstaller(skillsDir)
		if err := installer.Remove(args[1]); err != nil {
			return err
		}
		fmt.Printf("Removed skill %q\n", args[1])

	default:
		return fmt.Errorf("unknown skill command: %s", args[0])
	}
	return nil
}

func handleSlashCommand(input string, _ *agent.Agent) bool {
	cmd := strings.Fields(input)[0]
	switch cmd {
	case "/quit", "/exit":
		fmt.Println("Goodbye!")
		return false
	case "/tools":
		fmt.Println("Available tools: shell_exec, file_read, file_write, web_fetch, memory_store, memory_recall")
	case "/skills":
		fmt.Println("skill commands: not yet implemented")
	case "/memory":
		fmt.Println("memory commands: not yet implemented")
	case "/config":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
		}
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Available: /quit, /tools, /skills, /memory, /config")
	}
	return true
}
