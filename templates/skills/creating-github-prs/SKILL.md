---
name: creating-github-prs
description: Creates a pull request on a GitHub repository using gh CLI. Use when proposing code changes that are already written to disk — commits them to a new branch and opens a PR.
---

## PR creation workflow

```
- [ ] Step 1: Collect required parameters from step instructions
- [ ] Step 2: Commit and push the branch
- [ ] Step 3: Create the pull request
- [ ] Step 4: Report result
```

### Step 1: Collect parameters

All parameters below are required. Stop and ask if any are missing.

| Parameter | Description |
|-----------|-------------|
| `repo_dir` | Absolute local path to the git repository |
| `branch_name` | Branch name for the PR |
| `commit_message` | Commit message |
| `pr_title` | Pull request title |
| `pr_body` | Pull request description (Markdown supported) |

### Step 2: Commit and push

```bash
cd {repo_dir}
git checkout -b {branch_name}
git add -A
git commit -m "{commit_message}"
git push -u origin {branch_name}
```

If the branch already exists locally, use `git checkout {branch_name}` instead.

### Step 3: Create the PR

```bash
cd {repo_dir}
gh pr create --title "{pr_title}" --body "{pr_body}" --head {branch_name}
```

If `gh` is not installed or not authenticated, report this clearly and stop.

### Step 4: Report result

- **Success**: report the PR URL printed by `gh pr create`
- **Failure**: report the exact error message and suggest a specific fix
