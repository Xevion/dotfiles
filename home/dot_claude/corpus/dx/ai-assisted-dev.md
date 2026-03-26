---
name: ai-assisted-dev
category: dx
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/banner
    path: CLAUDE.md
    note: Comprehensive AI agent context with decision thresholds, safety restrictions, and pre-work checklists
---

# AI-Assisted Development

## Philosophy

CLAUDE.md as project context — the single document that orients an AI agent in a codebase. Skills for reusable workflows. Interview before implement. Scale questioning to complexity.

## Conventions

- **Decision threshold**: if the wrong choice costs 30+ minutes to redo, ask before implementing. Scale question rounds to feature complexity:
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
