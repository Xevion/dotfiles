# Finding Schema & Topic Template Reference

## Finding Format

Subagents return findings in this YAML structure:

```yaml
findings:
  - topic: error-handling          # corpus topic slug
    project: blink                 # directory name from ~/projects/
    finding_type: new              # stale | improved | new | exemplar_update
    path: src/error.rs             # file path within project (optional for some types)
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

```yaml
exemplars:
  - repo: Xevion/project-name     # GitHub org/repo format
    path: src/relevant/file.rs     # specific file or directory
    note: Brief description of what makes this a good example
```

Local-only projects (not on GitHub) use directory name:

```yaml
exemplars:
  - repo: local/project-name      # prefix with local/
    path: src/relevant/file.rs
    note: Brief description
```
