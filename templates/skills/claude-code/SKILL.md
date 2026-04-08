# Claude Code

## Description
Delegate complex coding tasks to Claude Code CLI. Use this skill when a task requires advanced code generation, refactoring, debugging, or multi-file changes.

## Trigger
- manual: "use claude code"
- manual: "delegate to claude"

## Entry Point
main.sh

## Language
bash

## Config
- No config required. Claude Code CLI must be installed and authenticated on the host.

## Input
- prompt: The coding task to delegate to Claude Code
- workdir: (optional) Working directory for Claude Code. Defaults to current directory.
- continue: (optional) Set to true to continue the last Claude Code conversation.
- session_id: (optional) Resume a specific Claude Code session by ID.

## Output
- success: boolean
- session_id: Session ID for resuming the conversation
- output: Claude Code's response
- error: error message if failed

## Dependencies
- claude (Claude Code CLI, authenticated)
- jq
