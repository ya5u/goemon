# Workflow System

## Overview

Workflows are multi-step automation tasks that can be scheduled via cron or run manually from the CLI. Each step is either a **prompt** (executed by the LLM with tools) or a **script** (executed directly as a shell/python script).

A shared workspace directory is created for each run, enabling reliable state passing between steps via files.

## Directory Structure

```
~/.goemon/workflows/
└── ai-news-digest/
    ├── workflow.yaml    # Workflow definition (required)
    ├── search.sh        # Script steps
    ├── fetch.sh
    ├── generate.sh
    └── publish.sh
```

## workflow.yaml

Workflows are defined in YAML format.

```yaml
name: AI News Digest
schedule: "0 8 * * *"    # cron (minute hour day month weekday)
notify: telegram          # notification target (optional)

steps:
  - name: search
    type: script
    entry_point: search.sh

  - name: generate
    type: prompt
    prompt: |
      Generate an article based on the following data...

  - name: publish
    type: script
    entry_point: publish.sh
```

### Top-level Fields

| Field      | Required | Description |
|------------|----------|-------------|
| `name`     | Yes      | Workflow name |
| `schedule` | Yes      | Cron expression (5 fields: minute hour day month weekday) |
| `notify`   | No       | Adapter name to notify on completion (e.g. `telegram`) |
| `steps`    | Yes      | Array of steps (at least one) |

### Step Fields

| Field         | Required        | Description |
|---------------|-----------------|-------------|
| `name`        | Yes             | Step name (used in logs and output filenames) |
| `type`        | Yes             | `prompt` or `script` |
| `prompt`      | If type=prompt  | Prompt text sent to the LLM |
| `entry_point` | If type=script  | Script filename to execute |

## Step Types

### prompt

Executes the step via the LLM (ReAct loop) with access to all registered tools.

- The previous step's stdout is prepended as `Previous step result:` in the prompt
- Does **not** save to conversation history (isolated from chat)

### script

Executes a shell script or Python script directly.

- Execution method is determined by file extension (`.sh` → bash, `.py` → python3)
- Timeout: 5 minutes
- The previous step's stdout is passed via stdin
- stdout becomes input for the next step

## Workspace

A temporary workspace directory is created for each workflow run, enabling reliable file-based state passing between steps.

### Environment Variables

The following environment variables are set for script steps:

| Variable             | Description |
|----------------------|-------------|
| `GOEMON_WORKSPACE`   | Absolute path to the workspace directory |
| `GOEMON_PREV_RESULT` | Path to the previous step's output file |

### Passing State Between Steps

There are two ways to pass data between steps:

1. **stdin/stdout** (simple) — Previous step's stdout is passed as the next step's stdin
2. **Workspace files** (recommended) — Write files to `$GOEMON_WORKSPACE` and read them in subsequent steps

Workspace files are more reliable for large data or structured data.

```bash
# search.sh — save to workspace
echo "${RESULTS}" > "${GOEMON_WORKSPACE}/search_results.json"

# fetch.sh — read from workspace
cat "${GOEMON_WORKSPACE}/search_results.json" | python3 process.py
```

### Lifecycle

- Created at workflow run start
- Each step's output is automatically saved as `step_N_<name>.txt`
- Automatically deleted after workflow completion

## Running Workflows

### Manual Execution

```bash
goemon workflow run <name>
```

### Scheduled Execution

When `goemon serve` is running, the scheduler checks workflows every minute and runs those matching their cron schedule.

- Duplicate runs of the same workflow are prevented
- Adding or modifying workflows in `~/.goemon/workflows/` takes effect without restart

### Listing Workflows

```bash
goemon workflow list
```

## Execution Logs

Each step's result is logged to the `workflow_runs` table in SQLite.

| Column          | Description |
|-----------------|-------------|
| `workflow_name` | Workflow name |
| `step_name`     | Step name |
| `step_type`     | `prompt` or `script` |
| `input`         | Step input |
| `output`        | Step output |
| `success`       | Success/failure |
| `error_message` | Error message (on failure) |
| `duration_ms`   | Execution time in milliseconds |
| `created_at`    | Execution timestamp |

## Notifications

When `notify` is set to an adapter name, the final step's result is sent via that adapter on completion. Currently supported:

- `telegram` — Sends to all configured `allowed_users`

## Design Guidelines

- **Use script steps for deterministic operations** — git operations, file I/O, API calls, cleanup
- **Use prompt steps for tasks requiring LLM judgment** — text generation, analysis, decision-making
- **Pass state via workspace files** — don't rely solely on stdin/stdout
- **Keep each step focused** — one step = one responsibility
