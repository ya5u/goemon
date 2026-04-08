# Skill System

## Overview

Skills are reusable automation modules implemented as external scripts (bash, python, etc.). They are automatically registered as LLM tools and dynamically discovered — no restart is needed when adding or removing skills.

## Directory Structure

```
~/.goemon/skills/
├── web-search/
│   ├── SKILL.md      # Metadata definition (required)
│   └── main.py       # Entry point script
├── claude-code/
│   ├── SKILL.md
│   └── main.sh
└── hello-world/
    ├── SKILL.md
    └── main.py
```

## SKILL.md Format

```markdown
# Skill Name

## Description
One-line description. Used as the tool description for the LLM.

## Trigger
- manual: "trigger phrase"

## Entry Point
main.sh

## Language
bash

## Input
- field_name: Field description
- optional_field: (optional) Optional field description

## Output
- result_field: Output field description

## Dependencies
- required external commands
```

### Sections

| Section      | Required | Description |
|--------------|----------|-------------|
| Description  | Yes      | Tool description shown to the LLM |
| Trigger      | No       | Manual/automatic trigger definitions |
| Entry Point  | Yes      | Script filename to execute |
| Language     | Yes      | Determines execution method (`bash`, `python`, etc.) |
| Input        | No       | Input parameter definitions, converted to JSON Schema for the LLM |
| Output       | No       | Output format documentation |
| Dependencies | No       | Required external commands documentation |

## Input Parameters

Each line in the `## Input` section is parsed into a JSON Schema parameter for the LLM tool definition.

### Syntax

```markdown
## Input
- query: Search query string
- max_results: (optional) Maximum number of results. Default 5.
```

### Parsing Rules

- `- field_name: description` → required parameter
- `- field_name: (optional) description` → optional parameter
- Field name is before the colon, description is after
- Descriptions starting with `(optional)` mark the field as optional

### Generated JSON Schema

The example above produces the following schema, which is sent to the LLM:

```json
{
  "type": "object",
  "properties": {
    "query": {
      "type": "string",
      "description": "Search query string"
    },
    "max_results": {
      "type": "string",
      "description": "Maximum number of results. Default 5."
    }
  },
  "required": ["query"]
}
```

## Execution Model

### I/O

- **Input**: LLM parameters are passed as JSON via stdin
- **Output**: Results written to stdout (JSON recommended)
- **Errors**: stderr is logged
- **Timeout**: 60 seconds

### Execution Method

Determined by the Language field:

| Language           | Command                  |
|--------------------|--------------------------|
| `bash`, `sh`       | `bash <entry_point>`     |
| `python`, `python3` | `python3 <entry_point>` |
| Other              | `./<entry_point>` (direct execution) |

## LLM Tool Registration

Skills are automatically registered as LLM tools at runtime.

- Tool name: `skill_<skill-name>` (e.g., `skill_web-search`)
- Description: from the `## Description` section
- Parameters: auto-generated from the `## Input` section

### Dynamic Discovery

Skills are discovered via the `ToolProvider` interface. The skills directory is scanned on each LLM call, so adding or removing a skill directory takes effect immediately without restarting GoEmon.

## Standard Skills

Extracted to `~/.goemon/skills/` on `goemon init`. Source is embedded in the binary from `templates/skills/`.

| Skill        | Description |
|--------------|-------------|
| `web-search` | Search the web via DuckDuckGo. No API key required |
| `claude-code`| Delegate complex coding tasks to Claude Code CLI |
| `github-pr`  | Create pull requests on GitHub repositories |
| `hello-world`| Minimal example skill |

## CLI Commands

```bash
goemon skill list                    # List skills
goemon skill run <name> [input-json] # Run a skill
goemon skill install <github-url>    # Install from GitHub
goemon skill remove <name>           # Remove a skill
```

## Execution Logs

Skill runs are logged to the `skill_runs` table in SQLite.

| Column         | Description |
|----------------|-------------|
| `skill_name`   | Skill name |
| `input`        | Input JSON |
| `output`       | Output |
| `success`      | Success/failure |
| `error_message`| Error message |
| `duration_ms`  | Execution time in milliseconds |

## Creating Skills

### Manual

1. Create a directory under `~/.goemon/skills/<name>/`
2. Add a `SKILL.md` following the format above
3. Add an entry point script
4. The skill is available immediately (no restart needed)

### From GitHub

```bash
goemon skill install https://github.com/user/repo
```

The repository is cloned to `~/.goemon/skills/<repo-name>/`. It must contain a `SKILL.md`.
