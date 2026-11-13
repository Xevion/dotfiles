---
name: documentation-naming
category: project-structure
last_audited: 2026-11-12
exemplars:
  - repo: local/guard
    path: DESIGN.md + CLAUDE.md
    note: "A numbered design spec that ends in a known-gaps section, with CLAUDE.md as the short working summary pointing to it"
  - repo: local/idakit
    path: public API naming, README and crate docs gate
    note: "Domain words in the public API with the SDK spelling kept as doc aliases, enforced by tests; README required to match the crate docs"
  - repo: local/Kessa
    path: ROADMAP.md
    note: "Roadmap entries are deleted when built, never marked done"
---

# Documentation & Naming

## Philosophy

README is the entry point, code is the primary documentation, and naming conventions reduce cognitive load. Every extra document is another place for a fact to go stale, so write fewer of them and keep each one durable.

## Conventions

- **File naming**: `UPPER_CASE.md` for root docs (README, CLAUDE, TODO, ROADMAP), kebab-case for directories, snake_case for Rust source files, camelCase for TypeScript
- **A design doc for a binary is a numbered spec.** Sections explain why, measured claims sit in tables, a normative scenario table states expected behavior, and the document ends with a **Known gaps** section listing every place where the document describes an intent the code does not yet meet. CLAUDE.md stays the short working summary and points to the spec. When code and spec disagree on a value, the code is the source of truth and the spec is stale (a retention period stated as 7 days in one section and 3 in another)
- **Durable docs carry no dates, commit hashes or counts.** They rot. Findings go in one canonical doc per subject with a one-line status header. See [binary-reverse-engineering](../patterns/binary-reverse-engineering.md)
- **A roadmap lists what is not built.** Delete an entry when it is built; never mark it done. A finished list is a changelog, which git already keeps
- **Keep the README in step with the crate docs.** Generate or check it from the crate's `//!` text (a `cargo-rdme` gate), so the two cannot drift
- **Public names follow who sees them.** Users get domain words with bare-noun accessors, and the foreign SDK's spelling is kept as `#[doc(alias)]`. Two static tests enforce it: every alias must name a real header symbol, and every method that forwards to the SDK must carry one. An allowlist entry for "the name already equals the SDK symbol" is banned, because it rewards the leak the test exists to catch
- **Sentinels become niches.** A `NonZeroU64` holding the bitwise-not of the raw value keeps `Option<Address>` the size of a `u64`
- **Doc summaries have one shape per kind of item**, each opening with a single self-contained line

## Anti-Patterns

- Stale documentation kept around
- Comments and docs that restate the code
- Inconsistent casing within a project
- A status document that must be edited in several places for one change
- Reciting how a fact was confirmed, or comparing the code to another codebase, inside tracked comments

## Open Questions

- ADR format and where decision records live (a dated decisions table versus per-decision files)
- README templates for libraries versus applications
