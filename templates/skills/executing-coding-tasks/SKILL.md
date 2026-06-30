---
name: executing-coding-tasks
description: Implements code changes directly by reading source files, editing them, and verifying the result with build or test commands. Use when a task requires modifying existing code, creating new files, or refactoring across a repository.
---

## Coding workflow

```
- [ ] Step 1: Locate relevant files
- [ ] Step 2: Plan changes
- [ ] Step 3: Implement changes
- [ ] Step 4: Verify
- [ ] Step 5: Fix and re-verify (repeat until clean or 3 attempts)
```

### Step 1: Locate relevant files

Extract the repository path from the step instructions. If not specified, ask before proceeding.

Survey the codebase with `shell_exec`, then read candidate files with `file_read`:

```bash
# Identify structure
find /path/to/repo -type f -name "*.go" | head -50

# Find relevant symbols
grep -rn "TargetSymbol" /path/to/repo --include="*.go"
```

### Step 2: Plan changes

Before writing any file, list:
- Each file to modify and what exactly changes
- New files to create
- Files to delete

If the scope is larger than expected, report this and ask for confirmation.

### Step 3: Implement changes

**Prefer `file_edit` for modifying existing files:**
1. `file_read` — read the file to identify the exact string to replace
2. `file_edit` — replace only the changed portion (`old_string` must be unique in the file; add surrounding lines for context if needed)

**Use `file_write` only for:**
- Creating a new file from scratch
- Rewriting a file almost entirely (e.g. >80% changed)

`file_edit` is preferred because it is less error-prone — the LLM does not need to reproduce the entire file.

### Step 4: Verify

Run verification with `shell_exec`. Adapt commands to the language and toolchain found in the repository:

```bash
go build ./...               # build check
go vet ./...                 # static analysis
go test ./internal/pkg/...   # test only affected packages
```

Prefer targeted package tests over the full test suite to keep feedback fast.

### Step 5: Fix and re-verify

If verification fails:
1. Read the error output carefully
2. Return to Step 3 for the specific files causing errors
3. Re-run verification

Repeat until verification passes. After 3 failed attempts, stop and report the remaining errors clearly rather than continuing to guess.

## Reporting

When complete, report:
- **Changed files**: list each with a one-line summary of what changed
- **Verification result**: passed, or failed with the error details
- **Assumptions or gaps**: anything not covered or decided without explicit instruction
