#!/usr/bin/env python3
"""Web search skill using DuckDuckGo HTML lite."""

import json
import sys
import urllib.request
import urllib.parse
import re


def search(query, max_results=5):
    """Search DuckDuckGo and return parsed results."""
    url = "https://html.duckduckgo.com/html/?" + urllib.parse.urlencode({"q": query})

    req = urllib.request.Request(url, headers={
        "User-Agent": "GoEmon/1.0",
    })

    with urllib.request.urlopen(req, timeout=15) as resp:
        html = resp.read().decode("utf-8", errors="replace")

    results = []
    # Parse result blocks from DuckDuckGo HTML lite
    for match in re.finditer(
        r'<a rel="nofollow" class="result__a" href="([^"]*)">(.*?)</a>.*?'
        r'<a class="result__snippet"[^>]*>(.*?)</a>',
        html,
        re.DOTALL,
    ):
        if len(results) >= max_results:
            break

        raw_url = match.group(1)
        title = re.sub(r"<[^>]+>", "", match.group(2)).strip()
        snippet = re.sub(r"<[^>]+>", "", match.group(3)).strip()

        # Resolve DuckDuckGo redirect URL
        parsed = urllib.parse.urlparse(raw_url)
        params = urllib.parse.parse_qs(parsed.query)
        actual_url = params.get("uddg", [raw_url])[0]

        results.append({
            "title": title,
            "url": actual_url,
            "snippet": snippet,
        })

    return results


def main():
    try:
        data = json.load(sys.stdin)
    except Exception:
        data = {}

    query = data.get("query", "")
    if not query:
        json.dump({"success": False, "error": "query is required"}, sys.stdout)
        return

    max_results = data.get("max_results", 5)

    try:
        results = search(query, max_results)
        json.dump({"success": True, "results": results}, sys.stdout)
    except Exception as e:
        json.dump({"success": False, "error": str(e)}, sys.stdout)


if __name__ == "__main__":
    main()
