# GoEmon

GoEmon is a personal AI agent written in Go. It runs on a Raspberry Pi or any Linux/macOS machine, uses [Ollama](https://ollama.com/) as the primary LLM backend, and can delegate complex tasks to [Claude Code](https://claude.com/claude-code) via a built-in skill.

**Name origin:** Go (the language) + 右衛門 (emon, Japanese name suffix) = GoEmon. Also a reference to Goemon Ishikawa, the legendary thief who operates autonomously in the shadows. And yes, inspired by a certain blue robotic cat from the future.

## Architecture

GoEmon has three layers:

- **Adapters** — External interfaces (CLI, Telegram) that connect users to the agent
- **Core Agent** — ReAct loop with Plan-and-Execute for complex tasks, LLM routing, memory
- **Tools & Skills** — Built-in tools (Go) and external skills (bash/python scripts)

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
goemon skill run <name> [input]  # Run a skill
goemon skill install <url>       # Install skill from GitHub
goemon skill remove <name>       # Remove a skill

goemon workflow list             # List workflows
goemon workflow run <name>       # Run a workflow manually
```

### Chat Slash Commands

| Command   | Description              |
|-----------|--------------------------|
| `/quit`   | Exit the chat session    |
| `/tools`  | List available tools     |
| `/config` | Show current config      |

## Built-in Tools

| Tool           | Description                        |
|----------------|------------------------------------|
| `shell_exec`   | Execute a shell command (30s timeout) |
| `file_read`    | Read file contents (max 100KB)     |
| `file_write`   | Write content to file              |
| `web_fetch`    | HTTP GET with HTML tag stripping   |
| `memory`       | Store/recall key-value pairs in SQLite |

## Skills

Skills are reusable automation modules stored in `~/.goemon/skills/`. Each skill is a directory containing a `SKILL.md` and an entry point script. Skills are automatically registered as LLM tools and dynamically discovered (no restart needed).

See [docs/SKILL.md](docs/SKILL.md) for the full specification.

### Standard Skills

Shipped with GoEmon and extracted on `goemon init`. Source is in [`templates/skills/`](templates/skills/).

| Skill          | Description                                      |
|----------------|--------------------------------------------------|
| `web-search`   | Search the web via DuckDuckGo (no API key needed)|
| `claude-code`  | Delegate complex coding tasks to Claude Code CLI |
| `github-pr`    | Create pull requests on GitHub repositories      |
| `hello-world`  | Minimal example skill for testing                |

## Workflows

Workflows are multi-step automation tasks defined in YAML. Steps can be **prompt** (LLM execution) or **script** (shell/python). A shared workspace directory handles state between steps.

```
~/.goemon/workflows/
└── ai-news-digest/
    ├── workflow.yaml
    ├── search.sh
    ├── fetch.sh
    └── generate.sh
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
      "ollama": { "endpoint": "http://192.168.x.x:11434", "model": "gemma4:26b" }
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
