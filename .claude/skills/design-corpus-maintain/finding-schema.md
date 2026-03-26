# Finding Schema & Topic Template Reference

## Finding Format

Subagents return findings in this YAML structure:

```yaml
findings:
  - topic: error-handling          # corpus topic slug (primary home)
    cross_topics:                  # other topics this finding is relevant to
      - rust
      - api-design
    project: blink                 # directory name from ~/projects/
    finding_type: new              # stale | improved | new | exemplar_update
    path: src/error.rs             # file path within project (for evidence only — not stored in corpus)
    description: >
      Uses thiserror for typed error enums with automatic Display impl.
      Each module defines its own error type that converts into the crate-level error.
    evidence: |
      #[derive(Debug, thiserror::Error)]
      pub enum BlinkError {
          #[error("config: {0}")]
          Config(#[from] ConfigError),
      }
    suggested_change: >
      Add to error-handling exemplars. Update Rust conventions section
      to document the per-module error type pattern.
```

## Finding Types

| Type | Meaning | When to use |
|------|---------|-------------|
| `stale` | Corpus is outdated | Project moved past a documented convention |
| `improved` | Better version exists | Project shows a superior approach to a documented pattern |
| `new` | Undocumented pattern | Project demonstrates something the corpus doesn't cover |
| `exemplar_update` | Exemplar needs refresh | Listed exemplar is outdated or this project is a better one |

## Fields

### `cross_topics` (optional)

List of other corpus topic slugs this finding is relevant to. Used during the dedup/review phase to:
- Identify findings that should be written into multiple topics (from each topic's perspective)
- Flag potential cross-auditor duplicates when using per-project batching

If a finding only belongs in one topic, omit this field.

### `path` (evidence only)

The file path in `path` is for locating evidence during review. It is **not** stored in the corpus — corpus exemplars reference projects by module/type/function, not file paths (paths drift).

### `evidence` (guidelines)

Keep evidence concise — 3-8 lines of the most relevant code. The purpose is to prove the pattern exists, not to reproduce the full implementation. Strip project-specific names if the pattern should be anonymized; keep real names for public projects.

## Topic File Template

New corpus topic files use this structure:

```yaml
---
name: <topic-slug>
category: <languages|architecture|patterns|project-structure|dx>
last_audited: <YYYY-MM-DD>
exemplars: []
---
```

### Language Topics

```markdown
# <Topic Display Name>

## Philosophy

## Conventions

## Anti-Patterns

## Open Questions
```

### Cross-Language Topics (architecture, patterns, dx)

```markdown
# <Topic Display Name>

## Philosophy

## Conventions

## Language-Specific

### Rust

### TypeScript

### Go

## Anti-Patterns

## Open Questions
```

Add or remove language subsections based on relevance. Not every topic needs every language.

## Exemplar Entry Format

Reference projects by module/type/function, not file paths. Paths drift when code is refactored; module and type names are more stable.

**Public repos (on GitHub):**

```yaml
exemplars:
  - repo: Xevion/project-name     # GitHub org/repo format
    path: src/services/            # module or directory (broad, not a specific file)
    note: Brief description of what makes this a good example
```

**Local-only projects (not on GitHub):**

```yaml
exemplars:
  - repo: local/project-name      # prefix with local/
    path: error handling module    # descriptive reference, not a file path
    note: Brief description
```

**What makes a good exemplar reference:**
- `path: src/services/` — module directory (stable)
- `path: ServiceManager type` — type name (very stable)
- `path: error handling in scraper worker` — descriptive (resilient to refactoring)

**Avoid:**
- `path: src/services/manager.rs:42` — line number (extremely fragile)
- `path: src/services/manager.rs` — specific file (fragile across renames)
