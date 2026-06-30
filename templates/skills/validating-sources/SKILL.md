---
name: validating-sources
description: Validates collected research sources by counting real URLs and verifying a sample are accessible. Use between a search step and an article generation step to prevent hallucination from thin source material.
---

Validate the source material collected by the previous step.

## Validation workflow

```
- [ ] Step 1: Load source material
- [ ] Step 2: Count real URLs
- [ ] Step 3: Spot-check accessibility
- [ ] Step 4: Report result
```

### Step 1: Load source material

Read the source file specified in the step instructions using `file_read`.
If no file is specified, read the previous step's result directly.

### Step 2: Count real URLs

Extract every URL that appears in the source material.
Count only URLs that start with `http://` or `https://` and appear verbatim in the content.

**Do not count:**
- Placeholder text like "(URL)"
- Partial or incomplete URLs
- URLs you recognise from training data but are not present in the source

### Step 3: Spot-check accessibility

Call `web_fetch` on up to 2 of the extracted URLs.
A URL is accessible if `web_fetch` returns status 200 and non-empty content.

### Step 4: Report result

Output exactly one of the following:

**On pass** (≥ 3 real URLs and at least 1 accessible):
```
VALIDATION PASSED
- Real URLs found: {n}
- Accessible (spot-checked): {m}/2
```

**On fail**:
```
VALIDATION FAILED
- Real URLs found: {n}  ← must be ≥ 3
- Accessible (spot-checked): {m}/2
- Reason: {具体的な理由}
```

Do not proceed further after reporting. The workflow scheduler uses this output to decide whether to continue.
