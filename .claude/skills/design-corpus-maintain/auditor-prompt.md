# Auditor Subagent Prompt Template

Use these templates when dispatching Sonnet subagents for corpus audits. Fill in the bracketed placeholders.

## Per-Topic Variant

One subagent per corpus topic, checking multiple projects.

```
Agent tool:
  description: "Audit corpus topic: [topic-name]"
  model: "sonnet"
  prompt: |
    You are auditing a design corpus topic against real project code.

    ## Topic Being Audited

    [PASTE FULL CONTENT of the corpus topic file here — do not make the subagent read it]

    ## Projects to Check

    [List project directories to scan:]
    - ~/projects/[project-a]
    - ~/projects/[project-b]
    - ~/projects/[project-c]

    ## Your Job

    For each project:

    1. **Get the file tree:**
       - Run: `git -C [project-dir] ls-files` for git-tracked files
       - Also run: `fd -H --type f --no-ignore . [project-dir] | head -200` to find ALL files including gitignored ones
       - Note which files are gitignored — they often contain AI rulesets, local configs, and conventions worth examining

    2. **Scan broadly.** Do NOT skip gitignored files. Specifically look for:
       - AI rulesets: `.claude/`, `.cursor/`, `.github/copilot/`, CLAUDE.md, .cursorrules
       - Local configs: .env.example, docker-compose.yml, Justfile, Makefile
       - Build configs: Cargo.toml, package.json, build.gradle.kts, pyproject.toml, go.mod
       - Source code relevant to the topic being audited

    3. **Batch reads in parallel** — dispatch multiple Read tool calls at once for efficiency

    4. **Compare** what you find against the corpus topic's:
       - Philosophy — does the project align or diverge?
       - Conventions — does the project follow, extend, or contradict?
       - Anti-Patterns — does the project exhibit any listed anti-patterns?
       - Exemplars — are listed exemplars still accurate? Is this project a better example?

    5. **Return findings** in this exact YAML format:

       ```yaml
       findings:
         - topic: [topic-slug]
           project: [directory-name]
           finding_type: stale | improved | new | exemplar_update
           path: [file path within project, if applicable]
           description: [what you found, 1-3 sentences]
           evidence: |
             [relevant code snippet or config excerpt, keep concise]
           suggested_change: [what should change in the corpus topic file]
       ```

    ## Finding Types

    - **stale** — corpus says X, but this project (and possibly others) have moved past X
    - **improved** — corpus convention is good, but this project shows a better version
    - **new** — project demonstrates a pattern/convention not yet in the corpus
    - **exemplar_update** — an existing exemplar is outdated, or this project is a better exemplar

    ## Rules

    - Return findings ONLY. Do not modify any files.
    - Be specific — include file paths and code snippets as evidence.
    - One finding per observation. Don't bundle unrelated things.
    - If a project has nothing relevant to this topic, say so briefly and move on.
    - Prioritize quality over quantity — 3 strong findings beat 10 weak ones.
    - If a topic has only placeholder content (HTML comments), focus on discovering NEW patterns.
    - If unsure whether something qualifies, include it with a note.
```

## Per-Project Variant

One subagent per project, checking multiple corpus topics.

```
Agent tool:
  description: "Audit project: [project-name] against corpus"
  model: "sonnet"
  prompt: |
    You are auditing a project against multiple design corpus topics.

    ## Project to Audit

    Directory: ~/projects/[project-name]

    ## Corpus Topics to Check

    [For each relevant topic, paste the FULL CONTENT:]

    ### Topic: [topic-name-1]
    [full topic file content]

    ### Topic: [topic-name-2]
    [full topic file content]

    [... repeat for each topic ...]

    ## Your Job

    1. **Get the file tree:**
       - Run: `git -C ~/projects/[project-name] ls-files` for git-tracked files
       - Also run: `fd -H --type f --no-ignore . ~/projects/[project-name] | head -200` for ALL files
       - Note which files are gitignored

    2. **Scan broadly.** Do NOT skip gitignored files. Check:
       - AI rulesets: `.claude/`, `.cursor/`, `.github/copilot/`, CLAUDE.md, .cursorrules
       - Local configs, build configs, source code
       - All files relevant to the topics being checked

    3. **Batch reads in parallel**

    4. **For each topic**, compare the project against Philosophy, Conventions,
       Anti-Patterns, and Exemplars sections

    5. **Return findings** in the same YAML format:

       ```yaml
       findings:
         - topic: [topic-slug]
           project: [directory-name]
           finding_type: stale | improved | new | exemplar_update
           path: [file path within project]
           description: [1-3 sentences]
           evidence: |
             [code snippet or config excerpt]
           suggested_change: [what should change in corpus]
       ```

    Same rules: findings only, be specific, quality over quantity.
    If a topic has only placeholder content, focus on NEW pattern discovery.
```
