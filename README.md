# GoEmon

GoEmon is a personal AI agent written in Go. It runs on a Raspberry Pi or any Linux/macOS machine, uses [Ollama](https://ollama.com/) as the primary LLM backend, and can delegate complex coding tasks to [Claude Code](https://claude.com/claude-code) via a built-in skill.

GoEmon can create its own **skills** (modular extensions) and propose improvements to its own codebase via pull requests.

**Name origin:** Go (the language) + 右衛門 (emon, Japanese name suffix) = GoEmon. Also a reference to Goemon Ishikawa, the legendary thief who operates autonomously in the shadows.

## Architecture

GoEmon has three layers:

- **Adapters** — External interfaces (CLI, Telegram, Discord, Web UI) that connect users to the agent
- **Core Agent** — ReAct loop, LLM routing, memory
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
goemon version                   # Show version

goemon skill list                # List installed skills
goemon skill run <name> [input]  # Run a skill
goemon skill install <url>       # Install skill from GitHub
goemon skill remove <name>       # Remove a skill

goemon workflow list             # List workflows
goemon workflow run <name>       # Run a workflow manually
goemon serve                     # Start adapters + workflow scheduler
```

### Chat Slash Commands

| Command   | Description              |
|-----------|--------------------------|
| `/quit`   | Exit the chat session    |
| `/tools`  | List available tools     |
| `/skills` | List installed skills    |
| `/memory` | Memory operations        |
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

Shipped with GoEmon and extracted on `goemon init`. Users can view and modify them. Source is in [`internal/skill/stdskills/skills/`](internal/skill/stdskills/skills/).

| Skill          | Description                                      |
|----------------|--------------------------------------------------|
| `claude-code`  | Delegate complex coding tasks to Claude Code CLI |
| `github-pr`    | Create pull requests on GitHub repositories      |
| `hello-world`  | Minimal example skill for testing                |

### Skill Config

Each skill can have its own `config.json` in its directory:

```
~/.goemon/skills/my-skill/
├── SKILL.md
├── main.sh
└── config.json
```

Configuration is entirely the skill's responsibility. For example, a skill might use a `config.json` in its directory:

```json
{
    "api_key_env": "MY_API_KEY",
    "some_setting": "value"
}
```

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

## Configuration

Config lives at `~/.goemon/config.json`.

```jsonc
{
  "llm": {
    "backends": {
      "ollama": { "endpoint": "http://localhost:11434", "model": "qwen2.5-coder:14b" }
    },
    "routing": { "default": "ollama" }
  },
  "agent": { "max_iterations": 10 }
}
```

## Cross-Compilation

GoEmon is pure Go (no CGO) and cross-compiles to any target:

```bash
# Linux ARM64
make build-linux-arm64

# Deploy to a remote host
make deploy TARGET=user@host:/path/to/goemon
```

## Development

```bash
make build       # Build binary
make test        # Run all tests
make run         # Run chat directly via go run
make clean       # Remove build artifacts
```

## License

[MIT](LICENSE)
