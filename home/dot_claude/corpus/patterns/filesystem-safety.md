---
name: filesystem-safety
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/cargo-sweep-ng
    path: sweep-platform remove module, git spawning, lint-io xtask; sweep-engine removal pipeline
    note: "Guards no config layer can disable, fd-anchored deletes, untrusted-repo git spawning, textual boundary lint"
  - repo: local/Kessa
    path: kessa-core files module (write_atomic, read_optional, list_dir)
    note: "One module owns all disk access: temp-file-and-rename writes, NotFound-as-None reads"
  - repo: local/mc-seed-preview
    path: msp-core atomic module, msp-config ConfigFile
    note: "Exclusive-create temp, fsync, rename; unparseable old file moved aside; comment-preserving config write-back"
---

# Filesystem Safety

## Philosophy

A tool that deletes or rewrites files makes precision structural, and trades recall for it on purpose. Disk access goes through one module, writes are atomic, and anything read from an untrusted tree is treated as hostile.

## Conventions

### Deleting

- **Guards that no configuration layer can disable or override.** Typed refusals for the cases that must never happen, and a fail-closed default for any unknown answer
- **Resolve every name relative to a directory the walk already holds open** (`unlinkat` style), so replacing a directory with a symlink mid-walk cannot redirect an unlink
- **A single platform crate owns the dangerous I/O**, and a cheap textual lint in an xtask enforces that other crates do not touch it (name the exempt crate, exempt tests, and state what the scan cannot see). See [rust](../languages/rust.md)

### Inspecting untrusted repositories

- **Config must not run code.** Spawn `git` with hooks and fsmonitor off, no system or global config, a cleared environment and literal pathspecs. A scanned repository's `core.fsmonitor` ran during a listing; the reproduction is why this exists
- **Resolve privileged binaries from fixed paths**, never from `PATH`, so a program planted on the path cannot be run with elevated rights

### Writing

- **Write atomically.** A temp file beside the target (exclusive create), the old file's permissions copied, `fsync`, then rename over the target. A reader never sees a half-written file
- **Move an unparseable previous file aside** under a numbered name instead of overwriting it
- **Do not ask first.** Map `NotFound` to `None` on the read itself instead of `exists()` before a read: the answer can change in between. Log and skip unreadable directory entries
- **Write back only changed keys** through a comment-preserving editor (TOML), and touch nothing on disk in read-only mode

## Anti-Patterns

- `exists()` before a read or an open
- Overwriting a file in place
- Following a path that may have been replaced mid-walk
- Spawning `git` against an untrusted checkout with the user's config and hooks enabled
- Looking up `sudo` (or any privileged helper) on `PATH`

## Related Topics

- [claude-code-hook-binaries](../dx/claude-code-hook-binaries.md)
