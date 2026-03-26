#!/usr/bin/env bash
set -euo pipefail

INPUT=$(cat)

REPO_DIR=$(echo "$INPUT" | jq -r '.repo_dir')
BRANCH_NAME=$(echo "$INPUT" | jq -r '.branch_name')
COMMIT_MSG=$(echo "$INPUT" | jq -r '.commit_message')
PR_TITLE=$(echo "$INPUT" | jq -r '.pr_title')
PR_BODY=$(echo "$INPUT" | jq -r '.pr_body')

cd "$REPO_DIR"

git checkout main 2>/dev/null || git checkout master 2>/dev/null
git pull --ff-only 2>/dev/null || true

git checkout -b "$BRANCH_NAME"

echo "$INPUT" | jq -r '.files | to_entries[] | "\(.key)\t\(.value)"' | while IFS=$'\t' read -r filepath content; do
    mkdir -p "$(dirname "$filepath")"
    printf '%s' "$content" > "$filepath"
    echo "Wrote: $filepath" >&2
done

git add -A
git commit -m "$COMMIT_MSG"

if git push origin "$BRANCH_NAME" 2>&1; then
    PR_URL=$(gh pr create \
        --title "$PR_TITLE" \
        --body "$PR_BODY" \
        --base main \
        --head "$BRANCH_NAME" 2>&1)
    echo "{\"success\": true, \"pr_url\": \"$PR_URL\"}"
else
    echo "{\"success\": false, \"error\": \"Failed to push branch\"}"
fi
