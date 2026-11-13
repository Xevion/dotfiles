---
name: ffi-facade-and-unsafe-containment
category: architecture
last_audited: 2026-11-12
exemplars:
  - repo: local/idakit
    path: idakit-sys facade, idakit raw module, kernel guard
    note: "Thin FFI facade crate plus an idiomatic crate, abort at the C ABI, an unsafe firewall module, thread affinity re-claimed on every call and proved by a source-scan test"
  - repo: local/ferrite-uefi
    path: ferrite-wifi-sys, ferrite-dev vendor and stubs
    note: "A foreign C codebase behind a Rust-implemented kernel-API shim, trace-and-stop generated stubs"
---

# FFI Facade & Unsafe Containment

## Philosophy

Wrap a C or C++ SDK behind a thin facade with no safe wrappers, plus one idiomatic crate on top. Confine `unsafe` to a few named places, make every one of them carry its contract, and prove the soundness claims with tests that read the source.

## Conventions

- **Two crates.** A facade crate that only declares and forwards (`idakit-sys`), and an idiomatic crate that owns the safe API. The C++ side is generated from a declarative spec where possible; see [build-time-codegen](../dx/build-time-codegen.md)
- **No exception unwinds across the C ABI.** Wrap every allocating C++ body in a catch-all that aborts (`try { ... } catch (...) { std::abort(); }`), baked into the forwarding macro so a body cannot forget it
- **`unsafe` lives in a one-liner firewall module and in handle-owning types.** Each `unsafe fn` carries a `# Safety` section. Out-parameters follow the snprintf convention `(buf, cap)` returning the full length
- **A thread-affine native singleton: `Send + !Sync` made sound by re-claiming affinity at every call.** A `PhantomData<*const ()>` field suppresses the auto traits and a guarded `unsafe impl Send` re-grants `Send`; each method that reaches the FFI first calls `ensure_kernel_thread`. Prove coverage with a kernel-free source-scan test (using `syn`): every method that calls the FFI must also call the guard, or appear on an allowlist
- **Mirror an external enum with `num_enum` and reject unknown values** instead of adding an `Unknown` variant, when the project pins to one SDK minor. Keep drift detection independent of the Rust side: see [differential-oracle-testing](../patterns/differential-oracle-testing.md)
- **Embedding foreign C behind a Rust shim** (a kernel-API compatibility layer): generated stubs that trace and stop on first use, and a way to run the same C as a host process (a "lab") before going to the target hardware. Check the configure result of any vendored build with expect-lists; see [build-time-codegen](../dx/build-time-codegen.md)
- **Public naming for an SDK wrapper** follows [documentation-naming](../project-structure/documentation-naming.md): domain words for users, SDK spelling as doc aliases, tested

## Anti-Patterns

- Safe wrappers in the facade crate
- A C++ exception reaching Rust
- `unsafe` scattered through the idiomatic crate
- Claiming `Send` for a thread-affine object without a guard on every entry

## Related Topics

- [rust](../languages/rust.md)
- [reloadable-rust-payloads](./reloadable-rust-payloads.md)
- [firmware-no-std-uefi](./firmware-no-std-uefi.md)
