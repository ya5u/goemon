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

A message of the form `/<skill-name> [input]` (or `/skills` to list) runs that skill directly through the agent, injecting its instructions up front instead of making the model discover the skill. This works both in `goemon chat` and through message adapters like Telegram.

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
| `conversation_history` | Read recent conversation history from SQLite (read-only; used for memory reflection) |
| `memory`     | Save/read/list/delete long-term memories (file-based; see below) |

### Skills

Anthropic-style instruction packages. Each skill is a directory containing a single `SKILL.md` that guides the LLM through a task — there is no entry point script and nothing is executed as a subprocess.

- `SKILL.md` has YAML frontmatter (`name`, `description`) followed by markdown instructions
- Skills are **not** registered as LLM tools; they are loaded on demand (`Manager.GetSkill`) and injected into the agent prompt
- Their main consumer is workflows: a workflow step references a skill by name, and the skill's instructions become the basis of that step's prompt
- Discovered by scanning `~/.goemon/skills/`, so adding or removing a skill directory takes effect without restart
- `goemon skill list` shows installed skills; `goemon skill run <name> [input]` runs one once through the agent (ad-hoc / for testing) — the CLI equivalent of a single workflow step

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

GoEmon supports multiple LLM backends, selected per call by the router:

- **`openrouter`** (default) — DeepSeek V4 Flash (`deepseek/deepseek-v4-flash`) via [OpenRouter](https://openrouter.ai/)'s OpenAI-compatible API. Requires an API key from the env var named by `api_key_env` (default `OPENROUTER_API_KEY`). The same backend works with any OpenAI-compatible endpoint.
- **`ollama`** (fallback) — A local or LAN Ollama instance, using its native API.

Backends are constructed in `setupAgent` from the `llm.backends` config map and registered with the router by name.

### Router

The LLM router selects which backend to use:

1. Default backend available → use it (typically OpenRouter)
2. Default unavailable → fallback (if configured, typically Ollama)
3. Nothing available → error

A background goroutine runs periodic health checks (configurable interval).

## Memory

GoEmon has two distinct memory systems, kept separate on purpose:

- **Conversation history** — verbatim chat log in SQLite (`conversations`), windowed to the last 50 messages and replayed into context each turn. High-volume, ephemeral.
- **Long-term memory** — distilled, durable facts about the user, stored as human-readable Markdown files under `~/.goemon/memory/` (one fact per file). Low-volume, persistent, hand-editable.

Long-term memory is implemented in `internal/usermemory` (separate from `internal/memory`, the SQLite store). Each memory file has frontmatter (`name`, `description`, `type`) and a Markdown body; types are `user`, `feedback`, `task`, and `reference`. A regenerated index, `MEMORY.md`, lists one line per memory.

Flow:

1. **Inject** — `buildSystemPrompt()` appends the memory index (and a usage protocol) to every system prompt, so the agent always knows what it has learned.
2. **Read** — when an index entry is relevant, the agent calls the `memory` tool (`read`) to load that memory's full body.
3. **Write** — the agent calls the `memory` tool (`save`) when it learns something durable (a preference, feedback, a recurring task, a request pattern), and `delete` to drop obsolete facts. Writes regenerate `MEMORY.md`.

This is tool-driven: the model decides what is worth remembering, guided by the protocol in the system prompt. In addition, a scheduled **reflection workflow** (the `distilling-memories` skill, read via the `conversation_history` tool) can periodically review recent conversation history and update memory in batch — the CLI/adapter tool path and the workflow path both write to the same `~/.goemon/memory/` files.

## Data

All user data lives in `~/.goemon/`:

```
~/.goemon/
├── config.json      # User configuration
├── AGENTS.md        # System prompt customization
├── memory.db        # SQLite: conversation history + run logs
├── memory/          # Long-term memory (Markdown, one fact per file)
│   ├── MEMORY.md    # auto-generated index
│   └── *.md
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
| `kv_memory`        | Legacy key-value store; superseded by file-based long-term memory (`~/.goemon/memory/`) and no longer used by the `memory` tool |
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
| `goemon skill run`     | Run a single skill once (ad-hoc / testing)           |
| `goemon memory list`   | List long-term memories                              |
| `goemon memory show`   | Show one memory's full content                       |
| `goemon version`       | Show version                                         |
