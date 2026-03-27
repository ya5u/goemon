#!/usr/bin/env bash
set -euo pipefail

INPUT=$(cat)

PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty')
WORKDIR=$(echo "$INPUT" | jq -r '.workdir // "."')
CONTINUE=$(echo "$INPUT" | jq -r '.continue // false')
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')

if [ -z "$PROMPT" ]; then
    echo '{"success": false, "error": "prompt is required"}'
    exit 1
fi

if ! command -v claude &> /dev/null; then
    echo '{"success": false, "error": "claude CLI not found. Install Claude Code first."}'
    exit 1
fi

cd "$WORKDIR"

# Build claude command with appropriate flags
CLAUDE_ARGS=(-p "$PROMPT" --output-format json)

if [ -n "$SESSION_ID" ]; then
    CLAUDE_ARGS+=(--resume "$SESSION_ID")
elif [ "$CONTINUE" = "true" ]; then
    CLAUDE_ARGS+=(--continue)
fi

OUTPUT=$(claude "${CLAUDE_ARGS[@]}" 2>&1) || {
    echo "{\"success\": false, \"error\": $(echo "$OUTPUT" | jq -Rs .)}"
    exit 1
}

# Extract session ID from JSON output for future continuation
RESULT_SESSION_ID=$(echo "$OUTPUT" | jq -r '.session_id // empty' 2>/dev/null)

echo "{\"success\": true, \"session_id\": $(echo "$RESULT_SESSION_ID" | jq -Rs .), \"output\": $(echo "$OUTPUT" | jq -Rs .)}"
