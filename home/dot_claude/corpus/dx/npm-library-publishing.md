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

<!-- Ship ESM only. Conditional exports for Bun/Node/types. Cross-runtime compatibility testing. Automated releases gated on CI. -->

## Conventions

- **Bun workspace source exports for internal consumers**: within a Bun workspace, point `exports` at raw `.ts` source files (`"import": "./src/index.ts"`). Bun resolves TypeScript natively, eliminating the build step during development. The build script (tsc + tsc-alias) produces `dist/` only for external publishing
- **Dual export modes**: workspace-internal consumption uses source exports. Before `npm publish`, exports must point at compiled `dist/` files with conditional `types`/`import` fields. The `tsc-alias` tool rewrites path aliases in the compiled output
- Bun build pipeline, conditional exports (bun/types/default), package.json fields (files, bin, engines), release-please, are-the-types-wrong validation

## Anti-Patterns

<!-- CJS-only packages, missing types field, no engines constraint, manual version bumps, publishing without CI gate -->

## Open Questions

<!-- Dual CJS/ESM publishing when consumers require it, JSR as npm alternative, provenance attestations -->
