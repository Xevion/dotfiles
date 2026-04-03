---
name: npm-library-publishing
category: dx
last_audited: 2026-03-27
exemplars:
  - repo: Xevion/tempo
    path: package.json + src/
    note: Bun-built ESM package with conditional exports, cross-runtime compat testing, release-please automation
---

# npm Library Publishing

## Philosophy

Ship ESM only. Conditional exports for Bun/Node/types. Cross-runtime compatibility testing. Automated releases gated on CI.

## Conventions

- **Workspace source exports and dual modes**: see [monorepo-workspace-library](../project-structure/monorepo-workspace-library.md) for Bun workspace source exports and the dual export mode pattern
- Bun build pipeline, conditional exports (bun/types/default), package.json fields (files, bin, engines), release-please, are-the-types-wrong validation

## Anti-Patterns

- CJS-only packages without ESM entry point
- Missing `types` field in package.json exports
- No `engines` constraint (consumers discover compat issues at runtime)
- Manual version bumps (use release-please or similar)
- Publishing without CI gate

## Open Questions

- Dual CJS/ESM publishing when consumers require it
- JSR as npm alternative
- Provenance attestations
