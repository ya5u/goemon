# GoEmon Architecture

## Layers

GoEmon is composed of three distinct layers:

```
┌──────────────────────────────────────────┐
│              Adapters                    │
│  CLI (chat/run) │ Telegram │ Discord │ Web UI │
├──────────────────────────────────────────┤
│              Core Agent                  │
│  ReAct Loop │ LLM Router │ Memory       │
├──────────────────────────────────────────┤
│         Tools              Skills        │
│  (built-in, Go)    (external scripts)    │
└──────────────────────────────────────────┘
```

### Adapters

Adapters are external interfaces that connect users to the GoEmon agent. They are part of the core binary and started by `goemon serve`.

- **CLI** — `goemon chat` (interactive REPL) and `goemon run` (one-shot)
- **Telegram Bot** — Long-running bot in a goroutine (Phase 3)
- **Discord Bot** — Long-running bot in a goroutine (Phase 3)
- **Web UI** — HTTP/WebSocket server (Phase 3)

Multiple adapters can run simultaneously. All adapters connect to the same Agent instance.

```json
{
    "adapters": {
        "telegram": {
            "enabled": true,
            "bot_token_env": "TELEGRAM_BOT_TOKEN"
        },
        "discord": {
            "enabled": false,
            "bot_token_env": "DISCORD_BOT_TOKEN"
        },
        "web": {
            "enabled": true,
            "listen": "0.0.0.0:8080"
        }
    }
}
```

### Tools

Built-in capabilities compiled into the GoEmon binary. The Agent calls these directly during the ReAct loop.

| Tool           | Description                     |
|----------------|---------------------------------|
| `shell_exec`   | Execute a shell command         |
| `file_read`    | Read file contents              |
| `file_write`   | Write content to file           |
| `web_fetch`    | HTTP GET with HTML stripping    |
| `memory_store` | Store key-value in SQLite       |
| `memory_recall`| Recall by key (partial match)   |

### Skills

Reusable automation modules implemented as external scripts (bash, python, etc.). Skills are **not** compiled into the binary — they run as subprocesses.

Skills live in `~/.goemon/skills/` and are managed dynamically:

- The Agent can create, list, and run skills via `skill_create`, `skill_list`, `skill_run` tools
- Users can install skills from GitHub via `goemon skill install <url>`
- Each skill is a directory with a `SKILL.md` and an entry point script
- JSON in (stdin) → JSON out (stdout)

#### Standard Skills

GoEmon ships with standard skills embedded in the binary via `go:embed`. On `goemon init`, these are extracted to `~/.goemon/skills/`. Users can view, modify, or delete them.

| Skill          | Description                                          |
|----------------|------------------------------------------------------|
| `claude-code`  | Delegate complex coding tasks to Claude Code CLI     |
| `github-pr`    | Create pull requests on GitHub repositories          |
| `hello-world`  | Minimal example skill for testing                    |

The `claude-code` skill allows GoEmon to use a local Ollama model as its primary brain while delegating heavy coding tasks to Claude Code, eliminating the need for direct cloud API fallback for coding tasks.

#### Skill Config

Skills that require configuration (API keys, settings) use the `skills` section in `config.json`:

```json
{
    "skills": {
        "reddit-monitor": {
            "subreddits": ["golang", "localllm"],
            "api_key_env": "REDDIT_API_KEY"
        }
    }
}
```

Secrets follow the `_env` suffix convention — only the environment variable name is stored in the config file. The executor resolves the actual value at runtime before passing it to the skill.

## LLM Backend

GoEmon uses a local Ollama instance as its primary LLM backend. Cloud APIs are available as fallback but are not required — the `claude-code` standard skill can handle complex coding tasks without a direct API key.

### Router

The LLM router selects which backend to use:

1. Default backend available → use it (typically Ollama)
2. Default unavailable → fallback (if configured)
3. Nothing available → error

A background goroutine runs periodic health checks.

## Data

All user data lives in `~/.goemon/`:

```
~/.goemon/
├── config.json      # User configuration
├── memory.db        # SQLite: conversations, KV memory, skill run logs
└── skills/          # Installed skills (standard + user)
    ├── claude-code/
    ├── github-pr/
    ├── hello-world/
    └── ...
```

## Commands

| Command              | Description                          |
|----------------------|--------------------------------------|
| `goemon init`        | Initialize `~/.goemon/` with config and standard skills |
| `goemon chat`        | Interactive REPL (CLI adapter)       |
| `goemon run "<msg>"` | One-shot command                     |
| `goemon serve`       | Start all enabled adapters (Phase 3) |
| `goemon skill list`  | List installed skills                |
| `goemon skill run`   | Run a skill                          |
| `goemon skill install` | Install skill from GitHub          |
| `goemon skill remove`  | Remove a skill                     |
| `goemon version`     | Show version                         |
