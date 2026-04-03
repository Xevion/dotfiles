---
name: design-corpus-bootstrap
description: Bootstrap a new or existing project with corpus-driven scaffolding. Detects tech stack, generates CLAUDE.md, Justfile, tempo.config.ts, and .env.example based on design corpus conventions. Run from any project directory.
---

# Design Corpus Bootstrap

Generate project scaffolding files based on your personal design corpus conventions.

## When to Activate

- User says "bootstrap this project", "set up conventions", "generate CLAUDE.md", "corpus bootstrap"
- Starting a new project and want standard scaffolding
- Adding AI assistant support to an existing project

## Procedure

### 1. Detect Tech Stack

Same detection as the audit skill. Scan for `Cargo.toml`, `package.json`, `go.mod`, `pyproject.toml`, `build.gradle.kts`, `svelte.config.*`, `Dockerfile`, `Justfile`, `.github/`, `migrations/`, etc.

Build a stack profile with:
- Primary language(s)
- Framework(s) (SvelteKit, Axum, Chi, FastAPI, etc.)
- Database (Postgres, SQLite, DuckDB, etc.)
- Package manager (cargo, bun, pnpm, uv, etc.)
- Build system (Justfile, Makefile, scripts/)
- Test runner (nextest, vitest, go test, pytest)
- Formatter (cargo fmt, prettier, biome, ruff)

### 2. Read Relevant Corpus Topics

Read `~/.claude/corpus/INDEX.md`, then read topic files relevant to the detected stack. Extract conventions that should be reflected in the generated files.

### 3. Check Existing Files

Before generating, check what already exists:
- `CLAUDE.md` — append/merge or create fresh?
- `Justfile` — extend or create?
- `tempo.config.*` — exists already?
- `.env.example` / `.env.template` — exists?

**Ask the user** (via Question tool) before overwriting any existing file. Present options: skip, merge, overwrite.

### 4. Generate Files

#### CLAUDE.md

Structure:
```markdown
# [Project Name]

## Stack
[Detected stack summary]

## Quick Reference
[Key commands: build, test, check, format]

## Architecture
[Brief layout description from repo scan]

## Conventions
[Corpus-derived conventions relevant to this stack, with @-references to corpus topics]

## Restrictions
[Standard safety restrictions: no destructive commands, etc.]
```

**Conventions section** should reference corpus topics by path for the AI agent to read on demand:
```markdown
### Error Handling
Follow conventions in @~/.claude/corpus/patterns/error-handling.md
- [1-2 project-specific notes if applicable]

### Testing
Follow conventions in @~/.claude/corpus/patterns/testing-quality.md
- Test runner: [detected runner]
- Integration tests: [detected pattern or recommendation]
```

Keep the CLAUDE.md concise. Don't copy corpus content — reference it. The AI agent reads the corpus topic when needed.

#### Justfile

Generate standard recipes based on detected toolchain:

```just
# Justfile for [project name]

# Check types, lint, and format
check:
    [detected check commands]

# Run tests
test *args:
    [detected test command] {{args}}

# Format code
fmt:
    [detected formatter]

# Build
build:
    [detected build command]
```

Adapt recipes to the project's actual tools:
- Rust: `cargo clippy`, `cargo nextest run`, `cargo fmt`
- TypeScript/Bun: `bun run typecheck`, `bun test`, `biome check --write`
- Go: `go vet ./...`, `go test ./...`, `gofmt -w .`
- Python: `uv run basedpyright`, `uv run ruff check`, `uv run pytest`

#### tempo.config.ts

Only generate if the project uses Bun/TypeScript and doesn't have one. Structure based on detected project layout:

```typescript
import { defineConfig } from "@xevion/tempo";

export default defineConfig({
  subsystems: {
    // Based on detected directories
  },
  presets: {
    check: {
      // Type check + lint + format
    },
    test: {
      // Test runner
    },
  },
});
```

#### .env.example

Generate based on detected dependencies:
- Database URLs if migrations/ exists
- S3/R2 if object storage patterns detected
- API keys placeholder if external services detected

```env
# Database
DATABASE_URL=postgres://user:pass@localhost:5432/dbname

# Object Storage (if detected)
S3_ENDPOINT=
S3_BUCKET=
S3_ACCESS_KEY=
S3_SECRET_KEY=
```

### 5. Present Results

Show what was generated. Explain which corpus topics influenced each file. Ask if adjustments are needed before writing.

## Rules

- **Always ask before overwriting** existing files. Use the Question tool.
- **Reference, don't copy.** CLAUDE.md should point to corpus topics, not duplicate their content. The corpus stays the source of truth.
- **Detect, don't assume.** If you can't detect the test runner, ask. Don't guess `vitest` for every TypeScript project.
- **Minimal viable scaffolding.** Generate what the project needs now. Don't add recipes for tools that aren't installed.
- **Corpus path:** `~/.claude/corpus/` (absolute path, works from any directory).
- **tempo is optional.** Only generate tempo.config.ts if the project is TypeScript/Bun-based and would benefit from it. Ask first for non-obvious cases.
