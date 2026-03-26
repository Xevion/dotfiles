---
name: design-corpus-maintain
description: Maintain the design corpus — structured manual updates, subagent-driven audits, and first-time population from project scans. Use when asked to update corpus topics, add exemplars, audit the corpus against projects, or review corpus freshness.
---

# Design Corpus Maintenance

Update and audit the design corpus stored at `home/dot_claude/corpus/`.

## Modes

```dot
digraph modes {
    "User request?" [shape=diamond];
    "Update" [shape=box];
    "Audit" [shape=box];
    "Populate" [shape=box];

    "User request?" -> "Update" [label="add/edit/remove\ntopic content"];
    "User request?" -> "Audit" [label="audit drift on\npopulated topics"];
    "User request?" -> "Populate" [label="first-time content\nfrom project scan"];
}
```

**Mode selection heuristic:**
- User says "update/add/edit/remove" a specific convention → **Update**
- User says "audit/scan/review" against projects AND topics have real content → **Audit**
- User says "audit/scan" against projects AND topics are stubs (HTML comment placeholders) → **Populate**
- Auto-detect: read the target topics. If most sections are HTML comment placeholders, suggest Populate instead of Audit

---

## Update Mode

Structured manual editing of corpus topic files.

### When to Use

- Add a new convention or pattern
- Update an existing entry with new information
- Add/remove/update exemplar references
- Resolve an Open Question
- Add a new topic file to the corpus

### Process

1. **Identify the target topic** from `home/dot_claude/corpus/INDEX.md`
2. **Read the current topic file** from `home/dot_claude/corpus/<category>/<topic>.md`
3. **Present current state** to the user — show what's there now
4. **Propose changes** with before/after for the relevant section
5. **Apply changes** after user approval
6. **Update `last_audited` date** in frontmatter to today's date
7. **Update INDEX.md** if the one-line summary changed or a new topic was added

### Adding a New Topic

1. Determine category and slug
2. Create stub using the template in `./finding-schema.md`
3. Add entry to `home/dot_claude/corpus/INDEX.md` in the correct category section
4. Fill initial content based on user input

---

## Populate Mode

First-time content generation for stub topics from real project scans. Use when topics exist but contain only placeholder comments.

### When to Use

- Topics are stubs with HTML comment placeholders instead of real content
- A new project was completed and the corpus hasn't been seeded from it yet
- Multiple new topics were created and need initial content

### How It Differs from Audit

| Aspect | Audit | Populate |
|--------|-------|----------|
| Topic state | Has real content | Stubs with placeholders |
| Subagent goal | Find drift/improvements | Discover patterns to write |
| Finding types | stale, improved, exemplar_update | Almost exclusively `new` |
| Review focus | Per-finding approve/reject | Structural decisions + batch approve |
| Interview questions | Rare (content exists) | Frequent (establishing conventions) |

### Process

1. **Gather project context** — read key files (Cargo.toml, package.json, CLAUDE.md, Justfile, etc.) to understand the tech stack before dispatching subagents
2. **Dispatch subagents** using the same templates as Audit mode (see `./auditor-prompt.md`), but group topics by domain to reduce prompt size:
   - Languages (rust, typescript, sql, css, svelte, etc.)
   - Architecture & Patterns (api-design, data-modeling, error-handling, concurrency, logging)
   - DX & Project Structure (build-systems, ci-cd, git-workflow, testing, ai-dev, repo-layout)
3. **Aggregate and deduplicate** — merge findings that overlap across auditors (common with per-project batching)
4. **Interview-driven review** — mix structural brainstorming with finding approval:
   - **Structural questions first**: new topics needed? Cross-cutting patterns? Detail level? These shape how findings are applied
   - **Batch approval via multi-select**: present findings grouped by theme, use multi-select "which to SKIP?" (approve-by-default). Most findings from stubs will be approved
   - **Mix brainstorming throughout**: when presenting a batch, ask about related structural decisions (e.g. "Should Svelte be its own topic or fold into TypeScript?")
5. **Write corpus content** — apply all approved findings, generalized into conventions:
   - Pattern descriptions + short illustrative snippets (3-5 lines, anonymized or using public project name)
   - Reference projects by name + module/type/function, not file paths (paths drift)
   - Add project as exemplar where it demonstrates a pattern well
6. **Update INDEX.md** with new entries and refreshed descriptions

### Interview Questions to Consider

During populate review, proactively raise questions like:
- "This finding doesn't fit existing topics. Should we create a new one?"
- "These findings span 3 topics. Should we duplicate from each perspective or cross-reference?"
- "This pattern is very project-specific. Should we generalize it or skip?"
- "The corpus has no [category] coverage yet. Should we add a topic for it?"

---

## Audit Mode

Subagent-driven discovery of drift, improvements, and new patterns across real projects. Use when topics already have real content.

### Discovery Sources

**Primary — local projects (preferred):**
```bash
ls ~/projects/
```
Read the directory listing directly. Each subdirectory is a project.

**Secondary — GitHub API (rate-limit aware):**
```bash
# Check rate limit first
gh api rate_limit --jq '.resources.core.remaining'
# Then list repos
gh repo list Xevion --limit 100 --json name,primaryLanguage,updatedAt
```
Use for supplemental metadata (language, recent activity). Do not rely on this as the primary source.

**Manual specification:**
The user can directly name repos or topics to audit.

### Batching Strategy

Ask the user which approach to use each time:

```dot
digraph batching {
    "What triggered the audit?" [shape=diamond];
    "Per-topic\n(1 subagent per topic,\nall relevant projects)" [shape=box];
    "Per-project\n(1 subagent per project,\nall relevant topics)" [shape=box];

    "What triggered the audit?" -> "Per-topic\n(1 subagent per topic,\nall relevant projects)" [label="routine maintenance\nor specific topic"];
    "What triggered the audit?" -> "Per-project\n(1 subagent per project,\nall relevant topics)" [label="just finished work\non a specific project"];
}
```

**Per-topic batching:**
- One Sonnet subagent per corpus topic
- Each subagent receives the full topic file + list of project directories to check
- Best for: routine maintenance, auditing specific topics across the portfolio

**Per-project batching:**
- Group topics by domain (Languages, Architecture/Patterns, DX/Structure) — one subagent per group, not per individual topic
- Each subagent receives all topic files in its group + the project directory
- Best for: post-project audits, comprehensive review of a single project
- Expect cross-auditor duplicates at domain boundaries (handled in dedup step)

### Dispatching Subagents

Use the prompt template in `./auditor-prompt.md`. Key points:

- **Model:** Dispatch with `model: "sonnet"` for cost efficiency
- **Input:** Paste full corpus topic content into the prompt (don't make subagents read corpus files)
- **Projects:** Pass directory paths, not repo names — subagents read `~/projects/` directly
- **File discovery:** Subagents get the canonical file tree first, then scan including gitignored files
- **Output:** Structured YAML findings per `./finding-schema.md`

### Subagent File Scanning Rules

Subagents MUST:
1. Get the canonical git-tracked tree: `git ls-files` (or `fd -H --type f` if not a git repo)
2. Be **aware** of what's gitignored (know which files are outside git tracking)
3. **NOT skip gitignored files** — scan everything, including:
   - AI rulesets: `.claude/`, `.cursor/`, `.github/copilot/`, CLAUDE.md, .cursorrules
   - Local configs: .env.example, docker-compose.yml, Justfile, Makefile
   - Build configs: Cargo.toml, package.json, build.gradle.kts, pyproject.toml
4. Batch reads in parallel — dispatch multiple Read calls simultaneously

### Review Phase

After all subagents return:

1. **Aggregate findings** from all subagents into a single list
2. **Deduplicate** findings that overlap — especially cross-auditor duplicates from per-project batching. Merge findings with the same pattern into one, noting all source projects
3. **Present summary** — finding counts per topic, high-level overview table
4. **Batch review** using the Question tool with multi-select:
   - Group findings by theme (not strictly by topic — related findings from different topics go together)
   - Present as "which to SKIP?" with multi-select (approve-by-default, since most findings are valid)
   - Mix in structural/brainstorming questions when relevant (new topics, cross-cutting concerns)
   - 4-8 findings per multi-select question for efficiency
5. **Apply approved changes** to the corpus topic files in `home/dot_claude/corpus/`
6. **Update `last_audited`** dates on all touched topics
7. **Update INDEX.md** descriptions if content changed significantly

### Scope Controls

| Scope | What gets checked |
|-------|-------------------|
| Full audit | All topics, all projects (expensive) |
| Topic-focused | Specific topics, all projects |
| Project-focused | Specific projects, all topics |
| Targeted | Specific topics, specific projects |

Default to targeted or topic-focused. Full audits should be rare.

---

## Content Conventions

These apply when writing or updating corpus content in any mode.

### Detail Level

- **Pattern descriptions**: prose explaining the convention and why it matters
- **Short illustrative snippets**: 3-5 line code blocks showing the pattern shape. Anonymize if the project is private; use the real project name if public
- **No file path references in corpus**: reference by project name + module/type/function (e.g. "Banner's `ServiceManager`" not "src/services/manager.rs"). Paths drift; names are stable
- **Exemplars in frontmatter**: use `repo: Xevion/project-name` for public repos, `repo: local/project-name` for private. Path field references the module/directory, not a specific file

### Cross-Topic Patterns

When a pattern spans multiple topics (e.g. ts-rs touches rust, typescript, api-design, repo-layout):
- **Duplicate from each topic's perspective** — write the pattern as it relates to that topic specifically
- Each topic is self-contained; readers shouldn't need to cross-reference to understand a convention
- The `cross_topics` field in findings helps identify these during review

### Generalization

- Extract the underlying preference, strip project-specific details
- "Banner uses UNLOGGED TABLE for scheduler timestamps" → "Use UNLOGGED TABLE for ephemeral app state where crash-loss is acceptable"
- If a pattern is only observed in one project and isn't clearly generalizable, note it as a convention with a caveat, or add it to Open Questions instead
