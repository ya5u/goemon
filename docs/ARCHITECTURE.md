# GoEmon Architecture

## Layers

```
┌──────────────────────────────────────────┐
│              Adapters                    │
│       CLI (chat/run) │ Telegram          │
├──────────────────────────────────────────┤
│              Core Agent                  │
│  ReAct / Plan-and-Execute │ LLM Router  │
├──────────────────────────────────────────┤
│    Tools          Skills       Workflows │
│  (built-in)    (scripts)    (YAML+scripts)│
└──────────────────────────────────────────┘
```

### Adapters

External interfaces that connect users to the GoEmon agent. Started by `goemon serve`.

- **CLI** — `goemon chat` (interactive REPL) and `goemon run` (one-shot)
- **Telegram Bot** — Long-running bot that receives messages and sends responses/notifications

Multiple adapters run simultaneously. All adapters connect to the same Agent instance.

```json
{
    "adapters": {
        "telegram": {
            "enabled": true,
            "bot_token_env": "TELEGRAM_BOT_TOKEN",
            "allowed_users": [123456789]
        }
    }
}
```

### Core Agent

The agent uses two execution modes:

- **ReAct Loop** — For simple tasks. Calls LLM with tool definitions, executes tool calls, repeats until the LLM produces a final response.
- **Plan-and-Execute** — For complex multi-step tasks. First generates a structured plan (JSON), then executes each step independently via the ReAct loop. Automatically selected based on input complexity, or forced via `RunWithPlan()`.

The `AGENTS.md` file in `~/.goemon/` customizes the system prompt (personality, behavior rules, etc.). Changes take effect immediately without restart.

### Tools

Built-in capabilities compiled into the GoEmon binary.

| Tool         | Description                              |
|--------------|------------------------------------------|
| `shell_exec` | Execute a shell command (30s timeout)    |
| `file_read`  | Read file contents (max 100KB)           |
| `file_write` | Write content to file                    |
| `web_fetch`  | HTTP GET with script/style/tag stripping |
| `memory`     | Store/recall key-value pairs in SQLite   |

### Skills

Reusable automation modules implemented as external scripts (bash, python, etc.). Skills run as subprocesses with JSON in (stdin) and JSON out (stdout).

- Each skill is a directory with a `SKILL.md` and an entry point script
- Skills are **dynamically discovered** via `ToolProvider` — adding/removing a skill directory takes effect on the next LLM call without restart
- Each skill's `## Input` section in `SKILL.md` is parsed into a JSON Schema and exposed as tool parameters to the LLM
- Tool name format: `skill_<name>` (e.g., `skill_web-search`)
- Users can install skills from GitHub via `goemon skill install <url>`

See [SKILL.md](SKILL.md) for the full specification.

#### Standard Skills

Embedded in the binary via `go:embed` (`templates/skills/`). Extracted to `~/.goemon/skills/` on `goemon init`.

| Skill          | Description                                      |
|----------------|--------------------------------------------------|
| `web-search`   | Search the web via DuckDuckGo (no API key needed)|
| `claude-code`  | Delegate complex coding tasks to Claude Code CLI |
| `github-pr`    | Create pull requests on GitHub repositories      |
| `hello-world`  | Minimal example skill for testing                |

### Workflows

Multi-step automation tasks defined in `workflow.yaml`. Each step is either a **prompt** (LLM execution) or a **script** (shell/python execution).

- A shared workspace directory (`$GOEMON_WORKSPACE`) is created per run for state passing between steps
- Cron-scheduled via `goemon serve`, or run manually via `goemon workflow run <name>`
- Dynamically discovered — adding a workflow directory takes effect without restart
- Execution logs are stored in SQLite (`workflow_runs` table)

See [WORKFLOW.md](WORKFLOW.md) for the full specification.

## LLM Backend

GoEmon uses a local Ollama instance as its primary LLM backend. The `claude-code` skill can handle complex coding tasks without a direct cloud API key.

### Router

The LLM router selects which backend to use:

1. Default backend available → use it (typically Ollama)
2. Default unavailable → fallback (if configured)
3. Nothing available → error

A background goroutine runs periodic health checks (configurable interval).

## Data

All user data lives in `~/.goemon/`:

```
~/.goemon/
├── config.json      # User configuration
├── AGENTS.md        # System prompt customization
├── memory.db        # SQLite: conversations, KV memory, skill/workflow logs
├── skills/          # Installed skills (standard + user)
│   ├── web-search/
│   ├── claude-code/
│   ├── github-pr/
│   └── hello-world/
└── workflows/       # Workflow definitions
    └── ai-news-digest/
        ├── workflow.yaml
        └── *.sh
```

### SQLite Tables

| Table              | Purpose                                      |
|--------------------|----------------------------------------------|
| `conversations`    | Chat history (role, content, timestamp)       |
| `kv_memory`        | Persistent key-value store for the agent      |
| `skill_runs`       | Skill execution logs                          |
| `workflow_runs`    | Workflow step execution logs                  |

## Commands

| Command                | Description                                          |
|------------------------|------------------------------------------------------|
| `goemon init`          | Initialize `~/.goemon/` with config, AGENTS.md, and standard skills |
| `goemon chat`          | Interactive REPL                                     |
| `goemon run "<msg>"`   | One-shot command                                     |
| `goemon serve`         | Start adapters + workflow scheduler                  |
| `goemon workflow list` | List installed workflows                             |
| `goemon workflow run`  | Run a workflow manually                              |
| `goemon skill list`    | List installed skills                                |
| `goemon skill run`     | Run a skill                                          |
| `goemon skill install` | Install skill from GitHub                            |
| `goemon skill remove`  | Remove a skill                                       |
| `goemon version`       | Show version                                         |
