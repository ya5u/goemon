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
│  (built-in)  (SKILL.md)   (WORKFLOW.md)   │
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
| `file_edit`  | Replace a string in a file               |
| `file_write` | Write content to file                    |
| `web_fetch`  | HTTP GET with script/style/tag stripping |
| `memory`     | Store/recall key-value pairs in SQLite   |

### Skills

Anthropic-style instruction packages. Each skill is a directory containing a single `SKILL.md` that guides the LLM through a task — there is no entry point script and nothing is executed as a subprocess.

- `SKILL.md` has YAML frontmatter (`name`, `description`) followed by markdown instructions
- Skills are **not** registered as LLM tools; they are loaded on demand (`Manager.GetSkill`) and injected into the agent prompt
- Their main consumer is workflows: a workflow step references a skill by name, and the skill's instructions become the basis of that step's prompt
- Discovered by scanning `~/.goemon/skills/`, so adding or removing a skill directory takes effect without restart
- `goemon skill list` shows installed skills

See [SKILL.md](SKILL.md) for the full specification.

#### Standard Skills

Embedded in the binary via `go:embed` (`templates/skills/`). Extracted to `~/.goemon/skills/` on `goemon init`.

| Skill                   | Description                                                |
|-------------------------|------------------------------------------------------------|
| `searching-the-web`     | Search the web via DuckDuckGo and collect real article URLs |
| `validating-sources`    | Verify collected sources are real and accessible            |
| `drafting-web-articles` | Draft a structured article from research results            |
| `executing-coding-tasks`| Implement code changes and verify with build/test commands  |
| `creating-github-prs`   | Open a pull request on GitHub via the `gh` CLI              |
| `hello-world`           | Minimal example skill for testing                           |

### Workflows

Multi-step automation tasks defined in a single `WORKFLOW.md`. YAML frontmatter sets `name`, the cron `schedule`, and an optional `notify` adapter; each `### skill-name` step references a skill and adds step-specific instructions plus completion criteria.

- Each step loads its skill's instructions, runs the agent (ReAct loop with tools), then asks the LLM to verify the completion criteria — retrying up to 3 times on failure
- A temporary workspace directory is created per run for file-based state passing between steps, and removed when the run finishes
- Cron-scheduled via `goemon serve`, or run manually via `goemon workflow run <name>`
- Dynamically discovered — adding a workflow directory takes effect without restart
- Execution logs are stored in SQLite (`workflow_runs` table)

See [WORKFLOW.md](WORKFLOW.md) for the full specification.

## LLM Backend

GoEmon uses a local Ollama instance as its LLM backend. Complex coding tasks are handled by the `executing-coding-tasks` skill, which drives the agent's built-in tools rather than calling out to a cloud API.

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
│   ├── searching-the-web/
│   │   └── SKILL.md
│   ├── executing-coding-tasks/
│   │   └── SKILL.md
│   └── hello-world/
│       └── SKILL.md
└── workflows/       # Workflow definitions
    └── ai-news-digest/
        └── WORKFLOW.md
```

### SQLite Tables

| Table              | Purpose                                      |
|--------------------|----------------------------------------------|
| `conversations`    | Chat history (role, content, timestamp)       |
| `kv_memory`        | Persistent key-value store for the agent      |
| `skill_runs`       | Legacy skill-execution log table (no longer written to; skills are not executed as subprocesses) |
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
| `goemon version`       | Show version                                         |
