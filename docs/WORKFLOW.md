# Workflow System

## Overview

Workflows are multi-step automation tasks that can be scheduled via cron or run manually from the CLI. A workflow chains together [skills](SKILL.md): each step references a skill by name and adds step-specific instructions plus completion criteria.

Every step runs through the agent (ReAct loop with the built-in tools). After a step finishes, the LLM verifies the step's completion criteria; if they are not met, the step is retried. A temporary workspace directory is created per run so steps can pass state to each other through files.

## Directory Structure

```
~/.goemon/workflows/
└── ai-news-digest/
    └── WORKFLOW.md    # Workflow definition (required)
```

A workflow directory contains a single required file: `WORKFLOW.md`.

## WORKFLOW.md

```markdown
---
name: ai-news-digest
schedule: "0 8 * * *"    # cron (minute hour day month weekday)
notify: telegram          # notification target (optional)
---

# AI News Digest

Collect AI news every morning and publish a Japanese article.

## Steps

### searching-the-web
Search for today's AI news and save the results to the workspace.

完了条件: workspace に sources.md が存在し、http/https の URL が5件以上含まれていること。

### drafting-web-articles
Draft a Japanese article from the collected sources and save it to article.md.

完了条件: article.md が存在し、800字以上の本文と出典 URL が含まれていること。
```

### Frontmatter

| Field      | Required | Description |
|------------|----------|-------------|
| `name`     | No       | Workflow name. Falls back to the directory name if omitted. |
| `schedule` | Yes      | Cron expression (5 fields: minute hour day month weekday). |
| `notify`   | No       | Adapter name to notify with the final result (e.g. `telegram`). |

A workflow with no `schedule` or no steps is rejected when loaded.

### Steps

The body's `### <skill-name>` headers define the steps, in order. Each header names a skill that must exist in `~/.goemon/skills/`. A leading number is allowed and ignored (`### 1. searching-the-web` is treated as `searching-the-web`).

The markdown under a header is that step's instructions and completion criteria. It is combined with the referenced skill's own instructions to form the step prompt.

## Step Execution

For each step the scheduler:

1. Loads the referenced skill's `SKILL.md` instructions (via `Manager.GetSkill`).
2. Builds a prompt combining the skill instructions, the step-specific instructions, the workspace path, the previous step's result, and any retry feedback.
3. Runs the agent (ReAct loop with all built-in tools), isolated from chat history.
4. Validates the completion criteria (see below).
5. Retries from step 2 up to **3 times** if the agent errors or validation fails. If all attempts are exhausted, the workflow stops with an error.

### Completion Criteria & Validation

If a step's instructions mention a completion criterion (the text contains `完了条件`, `success`, or `検証`), the LLM is asked to verify whether the criteria are met against the step result and workspace — replying `COMPLETE` or `INCOMPLETE: <reason>`. The reason is fed back into the next retry attempt.

If a step states no explicit criteria, it is treated as successful as soon as the agent returns without error.

## Workspace

A temporary workspace directory is created for each workflow run (e.g. `goemon-workflow-*`) and removed when the run finishes.

- The absolute workspace path is included in each step's prompt, so steps can write and read shared files (using `file_write` / `file_read` / `shell_exec`).
- Each step's output is also saved to the workspace as `step_<N>_<skill-name>.txt`.
- The previous step's textual result is passed into the next step's prompt under "前のステップの結果".

Prefer writing intermediate data to workspace files (e.g. `sources.md`, `article.md`) rather than relying on the passed-through text, especially for large or structured data.

## Running Workflows

### Manual Execution

```bash
goemon workflow run <name>
```

### Scheduled Execution

When `goemon serve` is running, the scheduler checks workflows every minute and runs those whose cron schedule matches the current minute.

- Duplicate concurrent runs of the same workflow are prevented.
- Adding or modifying workflows in `~/.goemon/workflows/` takes effect without restart.

### Listing Workflows

```bash
goemon workflow list
```

## Execution Logs

Each step's result is logged to the `workflow_runs` table in SQLite.

| Column          | Description |
|-----------------|-------------|
| `workflow_name` | Workflow name |
| `step_name`     | Step name (the referenced skill) |
| `step_type`     | Always `skill` |
| `input`         | Step instructions |
| `output`        | Step output (or error text) |
| `success`       | Success/failure |
| `error_message` | Error message (on failure) |
| `duration_ms`   | Execution time in milliseconds |
| `created_at`    | Execution timestamp |

## Notifications

When `notify` is set to an adapter name, the final step's result is sent via that adapter on completion. Currently supported:

- `telegram` — Sends to all configured `allowed_users`

## Design Guidelines

- **One step = one skill = one responsibility** — keep each step focused.
- **State the completion criteria explicitly** — include a `完了条件` line so the validator can catch incomplete or hallucinated results.
- **Pass state via workspace files** — write outputs to `$workspace/<file>` and read them in later steps instead of relying solely on the passed-through text.
- **Make instructions concrete** — name the exact files, tools, and checks a step should perform.
