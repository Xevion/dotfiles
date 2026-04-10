---
name: binary-reverse-engineering
category: patterns
last_audited: 2026-04-10
exemplars:
  - repo: local/bose-re
    path: docs/ + crates/bose-protocol/
    note: APK decompilation with jadx, BMAP protocol implementation validated against live hardware
  - repo: local/topaz-video-ai-re
    path: docs/ + inference/
    note: PE analysis with rabin2/Ghidra, ONNX model extraction, inference pipeline validated on real models
  - repo: local/game-hacking
    path: targets/assaultcube/analysis/ + CLAUDE.md + docs/agent-architecture.md
    note: "Three-artifact confidence KB (structs.h + pointer_chains.json + offset constants), anti-hallucination rules, AI-assisted RE loop"
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
- **Three-artifact confidence-tiered knowledge base**: maintain three synchronized artifacts per target for RE findings. (1) A confidence-annotated C header (`structs.h`) with inline tier comments on every field — `// +0x10 [CONFIRMED]`, `// [HYPOTHESIS]`, `// [STALE]`. (2) A `pointer_chains.json` with structured entries per chain: `confidence`, `validated_count`, `notes`, `base_module`, `offsets[]`. Increment `validated_count` each session the chain resolves correctly. (3) Typed offset constants in source (Rust `const`, C `#define`) with doc-comment confidence labels and evidence citations, colocated with the code that uses them. The tiers — `CONFIRMED` → `LIKELY` → `HYPOTHESIS` → `STALE` — form a decay pipeline: `STALE` entries from previous binary versions need re-verification before trust
- **Anti-hallucination rules for numeric domains**: for any RE work where LLMs participate in the analysis loop, add an explicit Anti-Hallucination section to CLAUDE.md. Core rules: (1) all hex arithmetic goes through a tool call — never in-agent reasoning; (2) every numeric claim cites the source experiment or address; (3) read memory before writing to confirm the value matches the expected type range; (4) tool results are authoritative over agent predictions; (5) mutations happen only through typed tools, never via shell. See [ai-assisted-reverse-engineering-loop](./ai-assisted-reverse-engineering-loop.md) for the full loop

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

## Related Topics

- [ai-assisted-reverse-engineering-loop](./ai-assisted-reverse-engineering-loop.md) — Six-phase RE loop, multi-agent role decomposition, context window strategy when an LLM is in the analysis loop
- [linux-process-memory-injection](../architecture/linux-process-memory-injection.md) — Runtime process manipulation (counterpart to static RE)

## Open Questions

- Dynamic analysis workflow standardization (Frida hooks, BT sniffing, mitmproxy)
- Automated protocol fuzzing informed by static analysis findings
