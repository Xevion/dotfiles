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
           cross_topics:              # optional: other topics this finding relates to
             - [other-topic-slug]
           project: [directory-name]
           finding_type: stale | improved | new | exemplar_update
           path: [file path within project, for evidence only]
           description: [what you found, 1-3 sentences — generalize the pattern, don't just describe the project's code]
           evidence: |
             [3-8 lines of relevant code showing the pattern shape]
           suggested_change: [what should change in the corpus topic file — phrased as a general convention, not project-specific]
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
    - **Generalize**: extract the underlying preference or pattern, not a description of what one project does. "Use UNLOGGED TABLE for ephemeral state" not "Banner uses UNLOGGED TABLE for scheduler timestamps."
    - **Cross-topic awareness**: if a finding is relevant to topics beyond the one being audited, list them in `cross_topics`. This helps the caller deduplicate across auditors.
    - If a project has nothing relevant to this topic, say so briefly and move on.
    - Prioritize quality over quantity — 3 strong findings beat 10 weak ones.
    - If a topic has only placeholder content (HTML comments), focus on discovering NEW patterns.
    - If unsure whether something qualifies, include it with a note.
```

## Per-Project Variant

One subagent per project, checking multiple corpus topics grouped by domain.

**Grouping guidance for the caller:** when dispatching per-project, group topics by domain to keep prompt size manageable and reduce cross-auditor duplication:
- **Languages**: rust, typescript, sql, css-styling, svelte, go, python, etc.
- **Architecture & Patterns**: api-design, data-modeling, concurrency-async, error-handling, logging-observability, state-management
- **DX & Project Structure**: build-systems, project-automation, ci-cd-deployment, git-workflow, testing-quality, ai-assisted-dev, repo-layout

```
Agent tool:
  description: "Audit project: [project-name] ([domain-group])"
  model: "sonnet"
  prompt: |
    You are auditing a project against multiple design corpus topics.

    ## Project to Audit

    Directory: ~/projects/[project-name]

    [Optional: include a brief project summary if available from CLAUDE.md or README, to save the subagent time discovering the tech stack]

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
           cross_topics:              # optional: other topics this is relevant to
             - [other-topic-slug]
           project: [directory-name]
           finding_type: stale | improved | new | exemplar_update
           path: [file path within project, for evidence only]
           description: [1-3 sentences — generalize the pattern]
           evidence: |
             [3-8 lines of code showing the pattern shape]
           suggested_change: [what should change in corpus — phrased as a general convention]
       ```

    Same rules: findings only, be specific, quality over quantity, generalize patterns.
    If a topic has only placeholder content, focus on NEW pattern discovery.
    Use `cross_topics` when a finding spans topic boundaries — this helps dedup across auditors.
```
