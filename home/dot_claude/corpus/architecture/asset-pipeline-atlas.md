---
name: asset-pipeline-atlas
category: architecture
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/build.rs + pacman/src/texture/
    note: "Sprite atlas JSON → build.rs PHF map → typed sprite enums for O(1) lookups"
---

# Asset Pipeline & Atlas

## Philosophy

<!-- Pack assets at build time. Typed handles for safe runtime access. Separate asset authoring from runtime consumption. -->

## Conventions

<!-- Atlas packing (packed PNG + JSON metadata), build.rs for baking compile-time maps, typed sprite/asset enums -->

## Language-Specific

### Rust

<!-- build.rs codegen from JSON atlas, PHF static maps for O(1) lookups, typed enum wrappers over atlas indices -->

## Anti-Patterns

<!-- Loading individual files at runtime when an atlas exists, string-based asset lookups, hardcoded sprite coordinates -->

## Open Questions

<!-- Hot-reload of atlases during development, multi-resolution atlas support, animation timeline encoding -->
