# GoEmon

GoEmon is a personal AI agent written in Go. It runs on a Raspberry Pi 4B, uses a remote [Ollama](https://ollama.com/) instance as the primary LLM backend, and falls back to cloud APIs (Claude, Gemini) for heavy tasks.

GoEmon can create its own **skills** (modular extensions) and propose improvements to its own codebase via pull requests.

**Name origin:** Go (the language) + 右衛門 (emon, Japanese name suffix) = GoEmon. Also a reference to Goemon Ishikawa, the legendary thief who operates autonomously in the shadows.

## Requirements

- Go 1.26+
- An Ollama instance on LAN, **or** an Anthropic API key (`ANTHROPIC_API_KEY`)
- SQLite is embedded via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — no CGO required

## Quick Start

```bash
# Build
make build

# Initialize config and data directory
./bin/goemon init

# Edit ~/.goemon/config.json to set your Ollama endpoint / API keys

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

- `SKILL.md` — metadata (description, trigger, entry point, language, I/O spec)
- An entry point script (bash or python) that reads JSON on stdin and writes JSON on stdout

GoEmon can create new skills on the fly via the `skill_create` tool during chat.

See [`examples/skills/`](examples/skills/) for examples.

## LLM Routing

GoEmon routes LLM requests through a configurable backend system:

1. Tasks in `force_cloud_for` list → cloud backend (e.g., skill creation)
2. Default backend available → use it (typically Ollama)
3. Default unavailable → fallback (typically Claude)
4. Nothing available → error

A background health check runs periodically to track backend availability.

## Configuration

Config lives at `~/.goemon/config.json`. See [`templates/config.example.json`](templates/config.example.json) for the full schema.

Key settings:

```jsonc
{
  "llm": {
    "backends": {
      "ollama": { "endpoint": "http://localhost:11434", "model": "qwen3-coder:14b" },
      "claude": { "model": "claude-sonnet-4-6-20250514", "api_key_env": "ANTHROPIC_API_KEY" }
    },
    "routing": { "default": "ollama", "fallback": "claude" }
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
