---
name: searching-the-web
description: Searches the web and collects real article URLs with summaries. Use when the task requires current information from the web, the user asks to look something up, or the answer cannot be derived from existing knowledge.
---

Search the web using `web_fetch` and collect results with real, accessible URLs.

## Search workflow

### Step 1: Fetch search results

**Primary — HTML search results page** (returns real article URLs):

```
url: https://html.duckduckgo.com/html/?q={query}
```

**Alternative for news queries — Google News RSS**:

```
url: https://news.google.com/rss/search?q={query}&hl=en&gl=US
```

Replace `{query}` with URL-encoded search terms. Run each query specified in the step instructions.

### Step 2: Extract real URLs

From each response, extract URLs that appear in the results.

**CRITICAL: Only use URLs that appear verbatim in the fetched content. Never generate, guess, or complete URLs.**

For each result record:
- Title
- URL (copied exactly from the response)
- Snippet

### Step 3: Fetch article content (if instructed)

If the step instructions ask for full article content, call `web_fetch` on each URL and append the body text to the result.

### Step 4: Save or present results

- If the step instructions specify a workspace file path, write the collected results there using `file_write`.
- Otherwise, present the results directly.

If fewer than 3 real URLs were found across all queries, report this explicitly before finishing.
