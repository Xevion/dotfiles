---
name: ai-assisted-dev
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: CLAUDE.md
    note: Comprehensive AI agent context with decision thresholds, safety restrictions, and pre-work checklists
  - repo: Xevion/doujin-ocr-summary
    path: CLAUDE.md
    note: Task-to-doc routing table, domain vocabulary anti-patterns
  - repo: Xevion/tempo
    path: CLAUDE.md + DESIGN.md
    note: Two-tier doc hierarchy, skills section with maintenance directives
  - repo: local/maestro
    path: CLAUDE.md
    note: 30-minute decision threshold, multi-stage questioning, domain-specific skills
  - repo: local/inkwell
    path: CLAUDE.md + docs/
    note: "Domain vocabulary enforcement, runtime-version guidance (Go 1.26 new(expr)), two-tier docs with STYLE.md"
  - repo: local/bose-re
    path: CLAUDE.md
    note: "Quick Reference protocol fact sheet, per-finding-type doc routing, documentation maintenance rules"
  - repo: Xevion/railway-collector
    path: CLAUDE.md
    note: "Annotated file tree with per-file purpose descriptions, generated file annotations"
---

# AI-Assisted Development

## Philosophy

CLAUDE.md as project context — the single document that orients an AI agent in a codebase. Skills for reusable workflows. Interview before implement. Scale questioning to complexity.

## Conventions

- **Decision threshold**: before implementing a choice that would cost 30+ minutes to redo if wrong, ask first. This is the core guardrail for AI agent autonomy — cheap-to-redo decisions can be made autonomously, expensive ones require confirmation. Scale question rounds to feature complexity:
  - Simple (1 round): scope and defaults
  - Medium (2-3 rounds): architecture, integration points, error handling
  - Complex (3-5 rounds): scope, discovered issues, API changes, edge cases

- **Explicit safety restrictions**: CLAUDE.md must document banned commands with file:line citations for the destructive code, live-server assumptions that affect migration immutability, and irreversible operations. Safety docs cite exact code locations, not just describe behavior

```markdown
<!-- Pattern: safety restriction with code citation -->
> **Never run `just db reset`**
> The destructive code lives in `scripts/db.ts:42-79`.
> It executes `DROP DATABASE` unconditionally.
```

- **Style docs as dual-purpose reading**: design style guides (STYLE.md, RUST.md, SVELTE.md) to serve both human developers and AI agents. CLAUDE.md includes a pre-work checklist gating subsystem work on reading the relevant guide
- **Task-to-doc routing table in CLAUDE.md**: directs AI agents to read the relevant style/architecture doc before acting on a given task type. Pair with a domain vocabulary anti-pattern table (wrong term → correct term) in STYLE.md
- **Skills section in CLAUDE.md**: document project-specific skills with trigger path, activation condition, and scope. Include explicit maintenance directives telling agents when to update both CLAUDE.md and skills
- **Two-tier doc hierarchy**: CLAUDE.md = quick reference + agent constraints. DESIGN.md = authoritative spec, full schema, decisions. Point agents to the spec doc for architectural questions
- **Quick Reference for RE/exploratory projects**: a dense machine-readable section in CLAUDE.md containing confirmed protocol facts, packet formats, UUIDs, enum values, and key class names. Paired with per-finding-type doc routing rules and maintenance instructions. Reading this section should orient a new session without reading all docs
- **Annotated architecture file tree**: per-file purpose descriptions in the architecture section of CLAUDE.md, marking generated files with "DO NOT EDIT" and regeneration commands. Valuable when a directory contains both hand-written and codegen outputs
- **Runtime/language quirk documentation in CLAUDE.md**: when using a language feature that is new or surprising (e.g., Go 1.26 `new(expr)`, Svelte 5 runes migration), document it in CLAUDE.md so AI agents don't generate outdated patterns. This is especially important for features that change idiomatic usage patterns
- **Project CLAUDE.md with conventions and restrictions**: document technology stack, quick-reference commands, workflow patterns, and explicit enforcement framing ("pattern violations will be rejected")
- **TDD with AI assistance**: when test infrastructure exists, load the TDD skill before implementation

## Anti-Patterns

- Trusting AI output without review or testing
- Vague prompts that produce vague results
- Skipping the interview/brainstorm phase for non-trivial features
- AI-generated comments that restate what the code already says
- Safety docs that describe behavior without citing source locations

## Open Questions

- Multi-agent coordination patterns for large features
- Skill composition — combining multiple skills in a single workflow
- Corpus-driven prompt engineering and feedback loops
