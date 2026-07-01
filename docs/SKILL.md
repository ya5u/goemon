# Skill System

## Overview

Skills are reusable, Anthropic-style **instruction packages**. Each skill is a directory containing a single `SKILL.md` that guides the LLM through a specific task.

Skills are **not** executed as scripts and are **not** registered as LLM tools. Instead, a skill's instructions are loaded on demand and injected into the agent's prompt — most commonly as a step inside a [workflow](WORKFLOW.md). The agent then carries out the task using its built-in tools (`shell_exec`, `file_read`, `file_edit`, `file_write`, `web_fetch`, `memory`).

## Directory Structure

```
~/.goemon/skills/
├── searching-the-web/
│   └── SKILL.md
├── executing-coding-tasks/
│   └── SKILL.md
└── hello-world/
    └── SKILL.md
```

A skill directory contains exactly one required file: `SKILL.md`. Supporting files may be added, but there is no entry point script — nothing in the directory is executed directly.

## SKILL.md Format

```markdown
---
name: searching-the-web
description: Searches the web and collects real article URLs with summaries. Use when the task requires current information from the web.
---

Search the web using `web_fetch` and collect results with real, accessible URLs.

## Search workflow

### Step 1: Fetch search results
...
```

### Frontmatter

| Field         | Required | Description |
|---------------|----------|-------------|
| `name`        | No       | Skill name. Falls back to the directory name if omitted. |
| `description` | Yes      | One-line summary. Shown by `goemon skill list` and used to decide when the skill is relevant. |

### Body

Everything after the closing `---` is free-form markdown instructions written for the LLM. Describe the goal, the step-by-step procedure, which built-in tools to use, and any output expectations. The body is loaded verbatim into the prompt when the skill runs.

## How Skills Are Used

Skills are managed by `internal/skill.Manager`:

- `ListSkills()` — reads only the frontmatter (`name`, `description`) of every skill, for listing.
- `GetSkill(name)` — loads the full `SKILL.md` body for one skill.

When a workflow step references a skill by name, the scheduler calls `GetSkill` and combines the skill's instructions with the step-specific instructions and workspace state to build the step prompt. See [WORKFLOW.md](WORKFLOW.md).

## Standard Skills

Embedded in the binary via `go:embed` (`templates/skills/`) and extracted to `~/.goemon/skills/` on `goemon init`. Existing files are never overwritten.

| Skill                    | Description |
|--------------------------|-------------|
| `searching-the-web`      | Search the web via DuckDuckGo and collect real article URLs with summaries |
| `validating-sources`     | Verify collected sources are real and accessible before they are used |
| `drafting-web-articles`  | Draft a structured article from research results |
| `executing-coding-tasks` | Implement code changes by reading/editing files and verifying with build/test commands |
| `creating-github-prs`    | Commit changes to a new branch and open a pull request via the `gh` CLI |
| `hello-world`            | Minimal example skill for testing and as a starting template |

## CLI Commands

```bash
goemon skill list                # List installed skills (name — description)
goemon skill run <name> [input]  # Run a single skill once through the agent
```

`skill run` loads the skill's `SKILL.md` instructions and runs them once through the agent (ReAct loop with the built-in tools), without touching conversation history — the CLI equivalent of a single workflow step. The optional `input` is appended to the prompt under an `# Input` heading. Use it for ad-hoc invocation and for testing a skill while developing it.

In an interactive `goemon chat` session, the same thing is available as a slash command: `/<skill-name> [input]` (e.g. `/hello-world Yasu`) runs that skill, and `/skills` lists the available names.

## Creating Skills

1. Create a directory under `~/.goemon/skills/<name>/`.
2. Add a `SKILL.md` with `name`/`description` frontmatter and markdown instructions following the format above.
3. Test it with `goemon skill run <name> [input]`, and/or reference it by `<name>` from a workflow step (see [WORKFLOW.md](WORKFLOW.md)).

The skill is picked up immediately — no restart needed. To ship a skill with GoEmon, add its directory under `templates/skills/`; it will be embedded in the binary and extracted on `goemon init`.
