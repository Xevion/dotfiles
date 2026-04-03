---
name: design-corpus-audit
description: Audit an existing project against design corpus conventions. Detects tech stack, maps to relevant corpus topics, scans for divergences, and produces a structured report. Run from any project directory.
---

# Design Corpus Audit

Produce a divergence report comparing the current project against your personal design corpus conventions.

## When to Activate

- User says "audit this project", "check conventions", "corpus audit", "does this follow my patterns"
- User is reviewing a project they haven't worked on recently and wants to assess alignment
- After major refactors, to verify conventions are still followed

## Procedure

### 1. Detect Tech Stack

Scan the project root for indicators. Build a stack profile:

| Indicator | Stack |
|---|---|
| `Cargo.toml` / `Cargo.lock` | Rust |
| `package.json` / `bun.lockb` / `pnpm-lock.yaml` | TypeScript/JavaScript |
| `go.mod` / `go.sum` | Go |
| `pyproject.toml` / `uv.lock` | Python |
| `build.gradle.kts` / `settings.gradle.kts` | Kotlin/JVM |
| `svelte.config.*` | Svelte |
| `Dockerfile` / `docker-compose.*` | Docker |
| `Justfile` | Justfile conventions |
| `CLAUDE.md` | AI-assisted dev |
| `tempo.config.*` | Tempo |
| `.github/workflows/` | CI/CD |
| `renovate.json` / `.github/dependabot.yml` | Dependency management |
| `sqlc.yml` / `tygo.yaml` / `migrations/` | Data/codegen |

### 2. Map Stack to Corpus Topics

Read `~/.claude/corpus/INDEX.md`. Select topics relevant to the detected stack. Always include:
- The language topic(s) for detected languages
- `error-handling`, `logging-observability`, `testing-quality` (cross-cutting)
- `repo-layout`, `build-systems` (structural)
- `ai-assisted-dev` if CLAUDE.md exists (or is missing)

Add stack-specific topics:
- Rust → `concurrency-async`, `cross-language-type-generation`
- Go → `concurrency-async`, `graceful-shutdown`
- Svelte → `state-management`, `css-styling`
- Docker → `docker-multi-service`
- GraphQL schema files → `graphql-schema-design`
- S3/R2 config → `object-storage-patterns`

### 3. Read Relevant Corpus Topics

Read each selected topic file from `~/.claude/corpus/<category>/<topic>.md`. Extract:
- **Conventions** — the expected patterns
- **Anti-Patterns** — what to flag if found
- **Language-Specific** sections matching the project's languages

Skip stub topics (those with only HTML comment placeholders).

### 4. Scan Project for Divergences

For each convention, search the project code for evidence of conformance or divergence. Be specific — cite file paths and line numbers.

**What to check (by category):**

**Error handling:**
- Rust: thiserror at boundaries? anyhow at app layer? Extension traits?
- Go: sentinel errors? MapDBError boundary? Constructor/extraction helpers?
- TypeScript: typed error classes? Centralized telemetry classification?
- Any language: stringly-typed errors? catch-all handlers? log-and-throw?

**Logging:**
- Structured logging framework in use?
- Level discipline (DEBUG/INFO/WARN/ERROR used appropriately)?
- Timing on heavyweight operations?
- println/console.log in non-CLI code?

**Testing:**
- Test infrastructure exists?
- TDD markers (test files alongside or in tests/)?
- Database isolation pattern (pgtestdb, in-memory)?
- Property tests for invariant-heavy code?

**Project structure:**
- Justfile present with standard recipes?
- CLAUDE.md present and substantive?
- Generated code separated from hand-written?
- docs/ hierarchy if project is complex?

**Dependency management:**
- Renovate/Dependabot configured?
- Lock files committed?

**Code style:**
- Formatter configured and consistent?
- Import style matches conventions?

### 5. Produce Divergence Report

Output a structured report grouped by severity:

```markdown
## Corpus Audit Report — [project name]

**Stack:** Rust, Svelte, PostgreSQL
**Topics checked:** 12 of 62

### Divergences

#### HIGH — Convention violations
- **[topic:convention]** description of divergence
  - `src/api.rs:42` — uses anyhow at public boundary, corpus says thiserror
  - Recommendation: ...

#### MEDIUM — Missing patterns
- **[topic]** pattern not found in project
  - Corpus convention: ...
  - Recommendation: ...

#### LOW — Style/preference differences
- ...

### Conformances (notable)
- [topic:convention] — correctly implemented at `path:line`

### Not Applicable
- Topics checked but not relevant to this project's current scope
```

## Rules

- **Read-only.** Never modify project files. Only produce the report.
- **Be specific.** Every divergence must cite a file path and line number. Vague "you should use X" without evidence is useless.
- **Distinguish intentional from accidental.** If the project's CLAUDE.md documents a deliberate deviation from corpus conventions, note it as intentional, not a divergence.
- **Don't flag stubs.** If a corpus topic is a stub (placeholder comments only), don't audit against it.
- **Scale to project size.** Small projects (< 2K LOC) don't need every convention. Focus on the ones that matter at that scale.
- **Corpus path:** `~/.claude/corpus/` (absolute path, works from any directory).
