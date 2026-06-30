# GoEmon

GoEmon is a personal AI agent written in Go. It runs on a Raspberry Pi or any Linux/macOS machine and uses [Ollama](https://ollama.com/) as its LLM backend. Complex multi-step tasks are composed from reusable **skills** (instruction packages) and run as scheduled **workflows**.

**Name origin:** Go (the language) + 右衛門 (emon, Japanese name suffix) = GoEmon. Also a reference to Goemon Ishikawa, the legendary thief who operates autonomously in the shadows. And yes, inspired by a certain blue robotic cat from the future.

## Architecture

GoEmon has three layers:

- **Adapters** — External interfaces (CLI, Telegram) that connect users to the agent
- **Core Agent** — ReAct loop with Plan-and-Execute for complex tasks, LLM routing, memory
- **Tools, Skills & Workflows** — Built-in tools (Go), skills (`SKILL.md` instruction packages), and workflows (`WORKFLOW.md` that chain skills on a schedule)

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Requirements

- Go 1.26+
- An Ollama instance (local or on LAN)
- SQLite is embedded via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — no CGO required

## Quick Start

```bash
# Build
make build

# Initialize config, data directory, and standard skills
./bin/goemon init

# Edit ~/.goemon/config.json to set your Ollama endpoint

# Start interactive chat
./bin/goemon chat

# Or run a one-shot command
./bin/goemon run "list files in my home directory"
```

## Usage

```
goemon init                      # Initialize ~/.goemon/
goemon chat                      # Interactive REPL
goemon run "do something"        # One-shot command
goemon serve                     # Start adapters + workflow scheduler
goemon version                   # Show version

goemon skill list                # List installed skills

goemon workflow list             # List workflows
goemon workflow run <name>       # Run a workflow manually
```

### Chat Slash Commands

| Command     | Description              |
|-------------|--------------------------|
| `/quit`     | Exit the chat session    |
| `/tools`    | List available tools     |
| `/skills`   | How to list skills       |
| `/memory`   | Memory commands (placeholder) |
| `/config`   | Show current config      |

## Built-in Tools

| Tool           | Description                        |
|----------------|------------------------------------|
| `shell_exec`   | Execute a shell command (30s timeout) |
| `file_read`    | Read file contents (max 100KB)     |
| `file_edit`    | Replace a string in a file         |
| `file_write`   | Write content to file              |
| `web_fetch`    | HTTP GET with HTML tag stripping   |
| `memory`       | Store/recall key-value pairs in SQLite |

## Skills

Skills are reusable instruction packages stored in `~/.goemon/skills/`. Each skill is a directory containing a single `SKILL.md`: YAML frontmatter (`name`, `description`) plus markdown instructions that guide the LLM through a task. Skills are not executed as scripts and are not registered as tools — they are loaded on demand and injected into the agent prompt, primarily as workflow steps.

See [docs/SKILL.md](docs/SKILL.md) for the full specification.

### Standard Skills

Shipped with GoEmon and extracted on `goemon init`. Source is in [`templates/skills/`](templates/skills/).

| Skill                   | Description                                                |
|-------------------------|------------------------------------------------------------|
| `searching-the-web`     | Search the web via DuckDuckGo and collect real article URLs |
| `validating-sources`    | Verify collected sources are real and accessible            |
| `drafting-web-articles` | Draft a structured article from research results            |
| `executing-coding-tasks`| Implement code changes and verify with build/test commands  |
| `creating-github-prs`   | Open a pull request on GitHub via the `gh` CLI              |
| `hello-world`           | Minimal example skill for testing                           |

## Workflows

Workflows are multi-step automation tasks defined in a single `WORKFLOW.md`. YAML frontmatter sets the `name`, cron `schedule`, and optional `notify` adapter; each `### skill-name` step references a skill and adds step-specific instructions plus completion criteria. Each step runs the referenced skill through the agent (ReAct loop with tools), and the LLM verifies the completion criteria, retrying on failure. A shared temporary workspace directory passes file-based state between steps.

```
~/.goemon/workflows/
└── ai-news-digest/
    └── WORKFLOW.md
```

Workflows run on a cron schedule via `goemon serve` or manually via `goemon workflow run <name>`.

See [docs/WORKFLOW.md](docs/WORKFLOW.md) for the full specification.

## Customization

`~/.goemon/AGENTS.md` customizes the agent's system prompt — personality, behavior rules, response style, etc. This file is loaded on every LLM call, so changes take effect immediately without restart.

## Configuration

Config lives at `~/.goemon/config.json`.

```jsonc
{
  "llm": {
    "backends": {
      "ollama": { "endpoint": "http://localhost:11434", "model": "gpt-oss:20b" }
    },
    "routing": { "default": "ollama" }
  },
  "agent": { "max_iterations": 10 },
  "adapters": {
    "telegram": {
      "enabled": true,
      "bot_token_env": "TELEGRAM_BOT_TOKEN",
      "allowed_users": [123456789]
    }
  }
}
```

## Cross-Compilation

GoEmon is pure Go (no CGO) and cross-compiles to any target:

```bash
# Linux ARM64 (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -o bin/goemon-linux-arm64 ./cmd/goemon/
```

## Development

```bash
go build ./...     # Build
go test ./...      # Run all tests
go run ./cmd/goemon/ chat  # Run chat directly
```

## License

[MIT](LICENSE)
