# GitHub PR

## Description
Create a pull request on a GitHub repository. Used by GoEmon to propose changes to its own codebase. Requires gh CLI.

## Trigger
- manual: "create a pull request"
- manual: "propose a code change"

## Entry Point
main.sh

## Language
bash

## Input
- repo_dir: local path to the git repository
- branch_name: branch name for the PR
- commit_message: commit message
- pr_title: pull request title
- pr_body: pull request description
- files: object mapping file paths to their new contents

## Output
- success: boolean
- pr_url: URL of the created PR
- error: error message if failed

## Dependencies
- gh (GitHub CLI, authenticated)
- git
- jq
