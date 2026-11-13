---
name: asset-pipeline-atlas
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/Pac-Man
    path: pacman/build.rs + pacman/src/texture/
    note: "Atlas JSON baked by build.rs into a phf map, typed GameSprite enum building the string keys, a test checking the enum against the atlas, a second font atlas rendered at startup"
  - repo: local/currents-render
    path: shader sources, renderer noise bake
    note: "No sprite atlas: embedded WGSL with a directory-read alternative, and a procedural texture baked once into a mip chain"
---

# Asset Pipeline & Atlas

An atlas is one packed image plus a table of rectangles (name to position and size), so many small sprites are drawn from a single texture.

## Philosophy

Pack assets at build time and bake the lookup table into the binary. Give runtime code typed handles. Keep authoring (many source images) separate from consumption (one atlas and its table). The packing step must be recorded and repeatable, or the pipeline cannot be rebuilt.

## Conventions

- **One packed PNG plus JSON metadata.** The JSON maps slash-separated names (`ghost/inky/up_a.png`) to `{x, y, width, height}`. The names are the lookup keys. The source images stay tracked next to the atlas so it can be regenerated
- **Bake the table with `build.rs`.** Read the JSON with `include_str!`, deserialize with serde, write a `phf::Map` to `$OUT_DIR`, and pull it in with `include!`. Emit `cargo:rerun-if-changed` for the JSON so editing it rebuilds. Embedding the JSON removes runtime parsing and gives O(1) lookups. If the generated code does not reference `phf`, list it in the machete ignore list

```rust
pub static ATLAS_FRAMES: phf::Map<&'static str, MapperFrame> = phf_map! {
    "ghost/inky/up_a.png" => MapperFrame { pos: glam::U16Vec2::new(68, 0), size: glam::U16Vec2::new(16, 16) },
};
```

- **A typed enum tree builds the keys.** `GameSprite::{Pacman, Ghost, Maze, ...}` with a `to_path()` that composes the string (the ghost branch uses `GhostType::as_str()`), so the key format is tied to an enum. Animation is `TileSequence` (wrapping frame index) and `DirectionalTiles` (one sequence per direction)
- **The typed layer is typed only up to a string key.** The link between the enum and the atlas is checked by a test (`test_all_sprite_paths_exist`), not by the compiler. Add such a test for every enum that builds keys
- **Which files ship is a separate question** from the atlas. An `Asset` enum covers it (atlas image, font, sound), embedded on desktop with `rust_embed` and loaded from the filesystem on the browser target
- **A second atlas for text.** Bitmap glyphs live in the sprite atlas under `text/`, and a TTF font pre-rendered into a texture at startup is a separate, optional atlas. Compute per-font metrics (max character height) once, not per frame
- **A procedural texture is its own case.** A noise tile baked once at startup into a mip chain with rendering passes (each level averaging the finer one) is generated, not packed. Embed shader source with `include_str!` and keep a `from_dir` loader so edits apply without a rebuild

## Anti-Patterns

- **An unrecorded packing step.** Pac-Man's atlas is made by an external packer (a crate called `clutterd`, noted only in a project story document), and no recipe, script or doc in the repo runs it. The sources are tracked, but regenerating the atlas is manual and undocumented. Record the exact command in the Justfile or the project notes
- Loading individual files at runtime when an atlas exists
- String-based asset lookups with no test linking them to the table
- Hardcoded sprite coordinates
- Writing frame names into generated Rust string literals without escaping (a name containing `"` or `\` would generate invalid code)

## Open Questions

- Hot-reload of atlases during development (a directory-read mode like the shader `from_dir` loader)
- Multi-resolution atlases (the current pipeline has a single atlas and no variant lookup)
- Animation timelines encoded as data instead of ordered frame-name lists in code
