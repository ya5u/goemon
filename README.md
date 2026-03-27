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
| `memory_store` | Store key-value pair in SQLite     |
| `memory_recall`| Recall by key (partial matching)   |

## Skills

Skills are reusable automation modules stored in `~/.goemon/skills/`. Each skill is a directory containing:

- `SKILL.md` — metadata (description, trigger, entry point, language, I/O spec, config)
- An entry point script (bash or python) that reads JSON on stdin and writes JSON on stdout

### Standard Skills

Shipped with GoEmon and extracted on `goemon init`. Users can view and modify them.

| Skill          | Description                                      |
|----------------|--------------------------------------------------|
| `claude-code`  | Delegate complex coding tasks to Claude Code CLI |
| `github-pr`    | Create pull requests on GitHub repositories      |
| `hello-world`  | Minimal example skill for testing                |

### Skill Config

Skills that need configuration use the `skills` section in `config.json`. Secrets use the `_env` suffix convention (only the environment variable name is stored):

```json
{
    "skills": {
        "my-skill": {
            "api_key_env": "MY_API_KEY",
            "some_setting": "value"
        }
    }
}
```

GoEmon can also create new skills on the fly via the `skill_create` tool during chat.

## Configuration

Config lives at `~/.goemon/config.json`. See [`templates/config.example.json`](templates/config.example.json) for the full schema.

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
