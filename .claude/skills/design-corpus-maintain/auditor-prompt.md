# Auditor Subagent Prompt Template

Use these templates when dispatching Sonnet subagents for corpus audits. Fill in the bracketed placeholders.

## Per-Topic Variant

One subagent per corpus topic, checking multiple projects. Best for routine maintenance.

```
Agent tool:
  description: "Audit corpus topic: [topic-name]"
  model: "sonnet"
  prompt: |
    You are auditing a design corpus topic against real project code.

    ## Topic Being Audited

    Read the corpus topic file: [path-to-topic-file]
    Key conventions to check for: [1-2 line summary of what the topic covers]

    ## Projects to Check

    [List project directories with brief summaries:]
    - ~/projects/[project-a] — [tech stack, LOC, key patterns]
    - ~/projects/[project-b] — [tech stack, LOC, key patterns]

    ## Instructions

    1. Read the corpus topic file first
    2. For each project: `git ls-files` + `fd -H --type f --max-depth 2 --no-ignore`
    3. Read key files (CLAUDE.md, configs, source relevant to the topic)
    4. Compare against the corpus topic's Philosophy, Conventions, Anti-Patterns, Exemplars
    5. Return findings as YAML (see format below)

    ## Output Format

    ```yaml
    findings:
      - topic: [topic-slug]
        cross_topics: []  # optional
        project: [directory-name]
        finding_type: stale | improved | new | exemplar_update
        path: [file path within project]
        description: [1-3 sentences, generalized]
        evidence: |
          [3-8 lines of code]
        suggested_change: [general convention for corpus]
    ```

    Rules: findings only, no file modifications, one per observation,
    generalize patterns, quality over quantity. For stub topics (HTML
    comment placeholders), focus on NEW pattern discovery.
```

## Per-Project Variant

One subagent per project, checking all relevant corpus topics. **Default choice** — avoids cross-subagent duplicates.

Do NOT split a single project into domain groups (Languages vs Arch+DX). This causes duplicate findings at domain boundaries and doubles token cost for overlapping patterns.

```
Agent tool:
  description: "Audit project: [project-name]"
  model: "sonnet"
  prompt: |
    You are auditing a project against design corpus topics. Return YAML findings only.

    ## Project

    Directory: ~/projects/[project-name]
    [Brief summary: tech stack, LOC, architecture, key patterns]

    ## Topics to Check

    Read each corpus topic file before auditing against it. For STUBs
    (HTML comment placeholders), discover NEW patterns.

    [For each relevant topic, list name + file path + 1-line summary:]
    **[topic-name]** ([path]) — [key conventions or "STUB"]
    ...

    ## Instructions

    1. Get file tree: `git ls-files` + `fd -H --type f --max-depth 2 --no-ignore`
    2. Read key files: CLAUDE.md, configs, source code relevant to topics
    3. Read each corpus topic file listed above
    4. Compare project against each topic's conventions
    5. Return findings as YAML (same format as per-topic variant)

    Rules: findings only, no file modifications, one per observation,
    generalize patterns, quality over quantity. For STUBs focus on NEW
    discoveries. Use cross_topics when findings span topic boundaries.
```

## Gap Analysis Variant

Always dispatch alongside audit subagents. Cheap (~65s, ~68K tokens) and identifies missing topics.

```
Agent tool:
  description: "Analyze corpus topic gaps"
  model: "sonnet"
  prompt: |
    Analyze projects for design corpus GAPS — topics that should exist but don't.

    ## Current Topics
    [List all topic names by category from INDEX.md]

    ## Projects
    [List each project with directory path and brief summary]

    ## Instructions
    For each project: get file tree, read CLAUDE.md + key configs.
    Identify technologies, patterns, and domains NOT covered by existing topics.

    ## Output
    ```yaml
    gaps:
      - suggested_topic: [slug]
        category: [category]
        projects_demonstrating: [list]
        description: [2-3 sentences]
        existing_overlap: [which existing topics partially cover this]
        priority: high | medium | low
    observations:
      - [structural observations about the corpus]
    ```
```
