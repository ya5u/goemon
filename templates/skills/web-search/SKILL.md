# Web Search

## Description
Search the web using DuckDuckGo and return results. No API key required.

## Trigger
- manual: "search the web"
- manual: "look up"

## Entry Point
main.py

## Language
python

## Input
- query: Search query string
- max_results: (optional) Maximum number of results to return. Default 5.

## Output
- success: boolean
- results: array of {title, url, snippet}
- error: error message if failed

## Dependencies
- python3
- curl (used internally)
