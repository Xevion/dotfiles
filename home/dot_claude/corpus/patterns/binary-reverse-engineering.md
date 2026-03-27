---
name: binary-reverse-engineering
category: patterns
last_audited: 2026-03-26
exemplars:
  - repo: local/bose-re
    path: docs/ + crates/bose-protocol/
    note: APK decompilation with jadx, BMAP protocol implementation validated against live hardware
  - repo: local/topaz-video-ai-re
    path: docs/ + inference/
    note: PE analysis with rabin2/Ghidra, ONNX model extraction, inference pipeline validated on real models
---

# Binary Reverse Engineering

## Philosophy

Documentation-driven RE: capture confirmed facts immediately, tag speculation explicitly, and build implementations that validate findings against real targets. The implementation is a test harness for the analysis — if the code works against the real system, the analysis is correct.

## Conventions

- **Static analysis first**: map imports, exports, strings, and structures before attempting dynamic analysis. Tools: `rabin2 -E/-i/-l/-I` for PE binaries, `jadx` for Android APKs, `strings` with categorization
- **Confirmed facts vs speculation**: prefix unconfirmed details with "likely" or "suspected" in all documentation. The Quick Reference section in CLAUDE.md contains only confirmed, tested facts
- **Implementation-as-validation**: build a parallel implementation (protocol library, inference CLI) that exercises the reverse-engineered interfaces against real targets. Passing tests against hardware/real models are the strongest validation
- **Documentation maintenance discipline**: after every analysis session update (1) analysis-status.md — check off completed items, (2) TODO.md — move completed items, add new tasks, (3) CLAUDE.md Quick Reference — add newly confirmed facts. Include tool commands and file paths that produced each finding for reproducibility
- **Task-to-doc routing**: maintain a mapping of finding types to documentation files (protocol details → bluetooth-protocol.md, app structure → architecture.md, API endpoints → api-endpoints.md). New sessions consult the routing table instead of guessing where to look
- **Quick Reference as agent entry point**: a dense machine-readable section in CLAUDE.md containing packet formats, UUIDs, enum values, function block mappings, key class names. Reading this section alone should orient a new session without requiring all docs
- **Annotated file tree in CLAUDE.md**: per-file purpose descriptions in the architecture section, marking generated files with "DO NOT EDIT" and regeneration commands. Especially valuable when a directory contains both hand-written and generated outputs

## Language-Specific

### Android (APK/XAPK)

- Extract via `innoextract` (Inno Setup) or direct unzip (XAPK is ZIP). Decompile with `jadx` — look for `@Keep` annotations which survive R8 obfuscation
- Obfuscated classes get short identifiers (`kotlin.a4g`); preserved public API classes are the best starting points

### Windows (PE32+)

- Use `rabin2 -E` for exports (handles MSVC mangled names), `rabin2 -I` for binary info including PDB paths that reveal build system details
- QML UI is often AOT-compiled into the executable — no plaintext `.qml` files for custom components

## Anti-Patterns

- Speculation documented as fact — always distinguish confirmed from suspected
- Analysis without reproducibility — every finding should cite the tool command and file path that produced it
- Ignoring community RE work — search for existing implementations before starting from scratch

## Open Questions

- Dynamic analysis workflow standardization (Frida hooks, BT sniffing, mitmproxy)
- Automated protocol fuzzing informed by static analysis findings
