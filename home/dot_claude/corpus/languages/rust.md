---
name: rust
category: languages
last_audited: 2026-11-12
exemplars:
  - repo: local/kk-engine
    path: kk-core problem enums, kk_engine exit module
    note: "snafu everywhere including the binary: one enum per module, size-bounded, main prints the chain via snafu::Report, workspace lint table"
  - repo: local/idakit
    path: Error / CallError / InitError, Result alias
    note: "snafu struct-style variants with context, one error type per boundary, From between boundary errors so ? flattens"
  - repo: local/steamlib
    path: error module (HttpError, RpcError)
    note: "snafu with implicit Location and transparent wrappers, credential-redacting newtype kept off the source chain"
  - repo: local/ferrite-uefi
    path: ferrite-xhci DmaError / Error, workspace lints, clippy.toml
    note: "snafu in a no_std workspace, typed context (device, register, step), single-home API bans via clippy.toml"
  - repo: local/Kessa
    path: workspace Cargo.toml lints and profiles, kessa-core launch errors
    note: "snafu with size bound, workspace lint table, named per-package dev opt-levels, cargo-hakari with heavy-stack excludes"
  - repo: local/mc-seed-preview
    path: workspace lint tables, profiling and release-dist profiles
    note: "#[expect(lint, reason)] throughout, strum::VariantArray over hand-kept arrays, profiling profile that keeps debug info"
  - repo: Xevion/Pac-Man
    path: pacman/src/error.rs
    note: "GameError aggregating sub-enums, error type as a Bevy ECS Event; moved from thiserror to snafu"
  - repo: Xevion/banner
    path: src/web/error.rs
    note: Extension traits on Option/Result for domain-specific error mapping
  - repo: Xevion/instant-upscale
    path: crates/common/error module
    note: Manual From<sqlx::Error> with domain-logic mapping, or_not_found(entity, id) extension trait
  - repo: local/game-hacking
    path: game-core memory module, pod-and-glam doc
    note: "glam types directly inside bytemuck Pod structs, offsets declared on the class-level layout struct"
  - repo: local/gimme
    path: ffmpeg build scripts (zig cc wrappers)
    note: "C dependencies built with zig cc at a glibc 2.17 floor, static codecs, dynamic glibc"
  - repo: local/star-chasers-rs
    path: scripts release recipe
    note: "cargo zigbuild with a .2.17 target suffix for the shipped Linux binary"
  - repo: local/toriix
    path: src/fs.rs + src/lib.rs
    note: "cfg-gated struct fields for WASM/native bifurcation, rstest+proptest+assert2 nested tests"
---

# Rust

## Philosophy

Ownership-centric design, zero-cost abstractions, compile-time guarantees over runtime checks. Prefer borrowing over cloning, iterators over loops, and typed errors over string messages.

## Conventions

### Errors: snafu

`snafu` is the primary error library, in libraries and binaries alike. Recent projects use no `thiserror`, `anyhow` or `Box<dyn Error>` at all; older code in the fleet still carries them.

- **One enum per module or small cluster, never one per crate.** Each variant is a struct variant that carries what the operation was doing (device, path, step) plus a `source` where there is a cause. Prefer `{ doing, #[snafu(source)] }` over a bare conversion for network and storage errors, because the reader of a log has neither the request nor the response
- **Derive shape**: `#[snafu(module, context(suffix(false)), visibility(pub(crate)))]` on the enum so context selectors live in a module and read as `Variant { .. }.fail()`. Add `#[snafu(implicit)] location: Location` on variants whose call site matters
- **`context(false)` for transparent conversions** so `?` converts a wrapped error without a selector. Use it for wrapper variants that only forward
- **Keep `Result` small**: put a compile-time bound beside each enum, `const _: () = assert!(size_of::<E>() <= 64);`, and set `large-error-threshold = 64` in `clippy.toml`. Box a large source instead of letting the whole error grow
- **Boundary errors convert**: when a public boundary has its own error type (`Error`, `CallError`, `InitError`), implement `From` between them so `?` flattens across the boundary. A `Result<T, E = Error>` alias keeps signatures short
- **Binaries print the chain once**: `main` hands the error to a single `finish` function that prints `snafu::Report` and returns an exit status. See [cli-tool-design](../dx/cli-tool-design.md) for mapping variants to sysexits codes. Pair with `miette::Diagnostic` when users need codes, help text and spans
- **Redact credentials in errors**: when an upstream error's `Display` or `Debug` embeds a URL carrying a token, wrap it in a newtype that redacts both and expose the raw value only through an explicit `expose()`. Name the field something other than `source` so snafu keeps it off the source chain, and test that the secret appears in no link of the chain
- **Typed outcomes for partial failure**: when a tool can finish with some problems, return them as values (each layer has its own enum implementing a shared `Problem` trait with a severity) instead of logging and moving on. A strictness setting lets an interactive viewer stay lenient while a CLI can fail with `--strict`

### Errors: patterns that are independent of the library

- **Extension traits for domain-specific conversions**: attach methods like `.or_not_found()` to `Option<T>` and `.conflict_on_unique()` to `Result<T, sqlx::Error>` via extension traits, centralizing error mapping instead of scattering match arms. The pattern generalizes to any third-party error that lacks `std::error::Error`
- **Named constructors on error types**: `ApiError::not_found(msg)`, `ApiError::bad_request(msg)` instead of direct struct construction
- **`help()` method on error sub-enums**: each variant carries a `help() -> Option<&'static str>` with actionable guidance, and a top-level `user_message()` fuses `Display` and `help()`. Useful for MCP and CLI tools where callers need next steps
- **Manual `From` for domain-logic conversions**: write a manual `From<sqlx::Error>` when the conversion carries logic (mapping DB error codes to `NotFound` or `Conflict`)
- **Dual-channel error separation for thread pools**: separate result and error into distinct `crossbeam-channel` channels rather than `Result<T, E>`-wrapping results. The error channel carries fatal worker state; the coordinator uses `select!` to race both with a timeout arm
- **Typed errors at Tauri IPC boundary**: implement `serde::Serialize` on command error enums so Tauri commands return `Result<T, CommandError>` instead of `Result<T, String>`

### Types

- **Ids and refusals are types, not strings.** A newtype with a private field per pair of ids that could be confused; a small bitset (`KindSet(u128)`) for a closed set; an enum with `Display` for a refusal. Log `id.get()` so the number stays a number
- **Enum variant lists come from `strum`** (`VariantArray`, `EnumIter`, `Display`), never a hand-kept `const ALL` or a hand-written `match`. An enum with a data-carrying variant keeps its own mapping, because the derives would fill the payload with `Default`
- **Conversions between foreign types** are `easy-ext` extension traits in the module that owns the boundary, not free `to_x(v)` functions
- **Time is `Duration` and `Instant`**, never `timeout_ms: u64`
- **Iterator combinators**: `.map()`, `.filter()`, `.find_map()`, `.collect()` over explicit for loops
- **Lifetime-bound filter structs** borrow from the request context rather than cloning into owned Strings. Consider a `Params<'a>` suffix for insert and upsert inputs that borrow
- **SQLx QueryBuilder**: use `QueryBuilder<Postgres>` for dynamic multi-condition queries instead of string interpolation
- **ts-rs for TypeScript contracts** where a TypeScript client exists. See [cross-language-type-generation](../dx/cross-language-type-generation.md)
- **Pod-boundary math types**: when bridging raw memory layouts (FFI, mmap, cross-process reads), glam vector and matrix types can sit directly inside `bytemuck::Pod` structs through glam's `bytemuck` feature. Declare offsets as attributes on the fields of one class-level layout struct, not as a separate table of constants. A raw `[f32; 3]` in a Pod struct that is meant to be a `Vec3` is the mistake to avoid. This replaced an earlier two-tier design that converted at each read helper
- **`#[cfg]` on individual struct fields** when a struct compiles everywhere but some fields exist only on certain platforms, combined with matching `#[cfg]` on the impl methods that touch them. Preferred over two struct definitions

### Lint policy

Lint policy lives in files, not convention.

- **Workspace lint table in the root manifest**: `clippy::pedantic` and `clippy::nursery` at `warn` with `priority = -1`, `unreachable_pub` so items default to `pub(crate)`, `allow_attributes_without_reason`, deny-level rustdoc link lints, and `unwrap_used`/`expect_used` warn outside tests (`allow-unwrap-in-tests` in `clippy.toml`). Every wide `allow` carries a one-line reason. List only rustc lints the tree is already clean against
- **`#[expect(lint, reason = "...")]` instead of `#[allow]`**, so a suppression that stops firing becomes a warning. Reserve `#[allow]` for cases where an expectation cannot be fulfilled (a lint that fires only under `cfg(test)`). Cargo's `[lints]` table cannot take `expect`, so workspace-level entries stay `allow`
- **`clippy.toml` bans with reasons** make a rule enforceable. `disallowed-types` for std `HashMap`/`HashSet` ("randomly seeded, iteration order varies per run; use rustc_hash") and std `Mutex`/`RwLock` (parking_lot); `disallowed-methods` for transcendental float methods where results must match across platforms (use `libm`); `disallowed-macros` for `assert_eq!` (use assert2). Each ban names the sanctioned replacement
- **Single-home APIs**: an operation that must have exactly one implementation (raw volatile access, a clock read, a stall, a reset) is banned workspace-wide with a reason that names its home. Only the home carries `#[expect(clippy::disallowed_*, reason = ...)]`. A second home is not allowed. See [firmware-no-std-uefi](../architecture/firmware-no-std-uefi.md)
- **Crate-boundary rules** can be enforced by a cheap textual lint in an xtask. Name the one exempt crate, exempt test files, and state in the lint's own docs what a textual scan cannot see

## Project Defaults

- **Edition 2024**: always use the latest stable edition
- **Toolchain pinned in `rust-toolchain.toml`**, with CI installing onto the pin (not an ignored env var) and `msrv` set in `clippy.toml`. A floating stable moves under a project without a commit
- **Linker**: mold is configured machine-wide in `~/.cargo/config.toml`. Never commit a linker choice into a project's `.cargo/config.toml`, where it becomes a hard error for anyone without it. A project config may say "the linker is left to each machine" and hold only project settings
- **Explicit rustls-tls**: disable reqwest default features and enable `rustls-tls` specifically to avoid OpenSSL. For fine control use `rustls-no-provider` plus manual `ring`/`aws-lc-rs` and TLS version selection
- **`TS_RS_EXPORT_DIR` in `.cargo/config.toml`**: set `TS_RS_EXPORT_DIR = { value = "web/src/lib/bindings/", relative = true }` to centralize ts-rs output
- **Hand-edit `Cargo.toml`, then `cargo check`**, in workspaces. `cargo remove` can delete root `[workspace.dependencies]` entries and `[profile.*.package.*]` blocks that no member uses

### Workspace shape

- A virtual root manifest with `members = ["crates/*"]` (a glob, so an absent crate cannot break a clone), one `[workspace.dependencies]` table with no versions in member manifests, inherited package and lint tables (`[lints] workspace = true`), `resolver = "3"`
- **cargo-hakari** keeps feature unification in a generated workspace-hack crate, gated by a `hakari-check` recipe in the gate. Exclude heavy stacks (GPU, network) in `traversal-excludes` so a small crate never builds wgpu to check a struct. Pair with `cargo machete`
- **No hakari in cross-target or `no_std` workspaces.** Cargo unifies features across every package in one invocation, so a host crate turning on `std` would reach the firmware build. ferrite-uefi rejects hakari for this reason; it is a deliberate choice there, not an omission
- Vendored forks stay outside the workspace and the audit (`exclude = ["vendor"]`, `[patch.crates-io]`)

### Cargo profiles

- **`dev` tuned per package.** Own code at `opt-level = 1` (0 gives huge stack frames, which matters on small stacks); a named list of hot dependencies at 2, each with the reason. Level 3 is rarely worth more than 2, and most dependencies are fine at 0 or 1. Reserve 3 for the few intensive or critical packages. Measure it: one project cut a load from 4.8 s to 0.4 s with a short list. A blanket `[profile.dev.package."*"]` level is acceptable when the list would be long, but named entries with reasons are preferred
- **`dev-release`**: `inherits = "dev"`, `debug-assertions = false`. Dev compilation speed with release-mode assertion behavior
- **`profiling`**: `inherits = "release"`, `debug = 1` (line tables) or `true`, `lto = false`, `strip = "none"`. Never strip here: stripping drops the debuginfo the profile exists to carry. Keep a separate `release-dist` profile with `strip = "symbols"` for distribution
- **`wasm-release`**: `opt-level = "s"`, `lto = true`, `strip = "debuginfo"` (keep the name section for crash stacks), `panic = "abort"`. Optimize hot crates per package. Combine with `wasm-opt`
- **`mutant`**: `inherits = "dev"`, `opt-level = 1`, `debug = 0`, `codegen-units = 256`, `incremental = false`. Maximum parallelism for cargo-mutants

### Compile targets and the glibc floor

- **Pick the CPU baseline per project.** `target-cpu=native` for personal tools; `x86-64-v3` where the userbase is known to be modern (borde.rs pins it for Linux and Windows, after checking that Rust never fuses multiply-adds on its own, so floats are unchanged); `x86-64-v2` or the default for general downloads. Where results must be identical across machines, see [performance](../patterns/performance.md) for compile-time versus runtime SIMD
- **Prebuilt Linux binaries should target an old glibc.** Most binaries a user downloads now link glibc 2.34 or newer, which is a recurring install failure on older distributions. Build the shipped artifact at a floor such as 2.17 with zig: `cargo zigbuild --target x86_64-unknown-linux-gnu.2.17` for pure Rust (star-chasers-rs)
- **The suffixed target floors only the Rust link.** C dependencies linked statically keep whatever glibc they were built against. gimme builds its whole codec stack with `zig cc -target x86_64-linux-gnu.2.17` through single-word `CC`/`CXX` wrapper scripts (configure scripts mishandle a multi-word compiler), pins the zig version exactly because the bundled libc++ ABI follows it, and keeps archives static while leaving libc, libm and the loader dynamic
- **Stay dynamic against glibc when the program loads drivers.** Static glibc and musl break `dlopen`, and hardware acceleration works by dlopening vendor drivers
- **Verify the floor with a link, not a header read.** Link every archive with `--whole-archive` against the floor's stubs; a newer symbol fails to link. Object files carry no GLIBC version tags, which is why a plain check shows nothing. The floor of the final binary is a separate check that none of these projects does yet
- **Use zig and cargo-xwin for cross lint.** borde.rs lints macOS from Linux with `cargo-zigbuild clippy` (plus a stub `xcrun` for a shader build script) and Windows with `cargo xwin clippy`, so platform-gated code stays lint-clean on a Linux host
- Where a system library exists only as a link stub on the build host (libasound), `-Wl,--allow-shlib-undefined` lets the link succeed but also lets that library's newer glibc references through

## Anti-Patterns

- Excessive `.clone()` to satisfy the borrow checker; restructure ownership instead
- Stringly-typed errors, or an opaque error type (`anyhow`, `Box<dyn Error>`) at any boundary a caller might match on
- `Rc<RefCell<T>>` as a first resort
- A bare `#[allow]` with no reason, or a clippy `allow` for a whole group to silence one warning
- Stripping symbols from the profile meant for a profiler
- Committing a linker or CPU-specific `rustflags` into a project that others build

- **cfg on individual enum variants**: use `#[cfg(not(target_arch = "wasm32"))]` on specific variants (e.g., `NetworkMode::Remote`) rather than gating the module. Match arms referencing cfg-gated variants must also be cfg-gated

## Related Topics

- [error-handling](../patterns/error-handling.md): cross-language error philosophy and extension trait pattern
- [concurrency-async](../architecture/concurrency-async.md): service lifecycle, CancellationToken, singleflight
- [cross-language-type-generation](../dx/cross-language-type-generation.md): ts-rs conventions
- [ffi-facade-and-unsafe-containment](../architecture/ffi-facade-and-unsafe-containment.md): wrapping a C or C++ SDK
- [firmware-no-std-uefi](../architecture/firmware-no-std-uefi.md): no_std workspaces
- [dependency-management](../dx/dependency-management.md): cargo-deny and advisory gating

## Open Questions

- When to reach for `async` versus OS threads: a short-lived process with two or three pipe copiers used threads and `std::io::pipe` with no runtime (see [claude-code-hook-binaries](../dx/claude-code-hook-binaries.md))
- Macro hygiene standards and when to prefer proc macros versus declarative macros
- Verifying the glibc floor of a final binary, not only of its static archives
