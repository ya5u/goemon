---
name: drafting-web-articles
description: Drafts a structured web article from research results or collected information. Use after a web search or research step to synthesize findings into a publishable article.
---

## Article drafting workflow

```
- [ ] Step 1: Extract source material from previous step results
- [ ] Step 2: Plan the article structure
- [ ] Step 3: Draft the article
- [ ] Step 4: Review against step requirements
```

### Step 1: Extract source material

The previous step's output contains the research material. Read it carefully and extract all URLs that appear in it.

**HARD STOP: Count the real URLs present in the source material.**
- Fewer than 3 real URLs → output `ERROR: 素材不足のため記事生成を中断しました（実URL: {n}件）` and stop immediately. Do NOT draft anything.
- 3 or more real URLs → proceed to Step 2.

**Never fabricate, guess, or complete URLs.** Only use URLs copied verbatim from the source material.

### Step 2: Plan the article structure

Use this default structure, adapting as the step instructions require:

```
- Headline
- Introduction  (1–2 paragraphs: what this article covers and why it matters)
- Body sections (2–4 sections, each with a subheading)
- Conclusion    (1 paragraph: key takeaway or next steps)
```

Decide the section titles based on the source material before writing.

### Step 3: Draft the article

Write in Markdown. **Write the article in the language specified by the step instructions. If no language is specified, match the language of the source material.**

Follow any tone, length, or format requirements specified in the step instructions. When none are given:
- Tone: informative and accessible, not overly technical
- Length: 400–800 words
- Each section: 1–3 paragraphs

Attribute sources inline or as a reference list at the end.

### Step 4: Review

Check the draft against the step instructions:
- Does it cover the required topics?
- Does it meet the specified tone and length?
- Are all factual claims supported by the source material?

Revise if any requirement is unmet, then output the final article.
