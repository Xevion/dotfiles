---
name: binary-reverse-engineering
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/bose-re
    path: docs/ + crates/bose-protocol/
    note: APK decompilation with jadx, BMAP protocol implementation validated against live hardware
  - repo: local/topaz-video-ai-re
    path: docs/ + inference/
    note: PE analysis with rabin2/Ghidra, ONNX model extraction, inference pipeline validated on real models
  - repo: local/game-hacking
    path: docs/workflow (re-workflow, re-naming) + CLAUDE.md
    note: "One canonical findings doc per subject with a status line, confidence encoded in symbol-name form, live disassembler database as the source of truth, live-observation discipline"
  - repo: local/assrip
    path: docs/ + reference directory
    note: "Prior-art clones in a git-excluded reference directory, decoders validated against known plaintext"
---

# Binary Reverse Engineering

## Philosophy

Documentation-driven RE: capture confirmed facts immediately, tag speculation explicitly, and build implementations that validate findings against real targets. The implementation is a test harness for the analysis. If the code works against the real system, the analysis is correct. Keep the number of artifacts small: every extra synchronized file is another place for a finding to go stale.

## Conventions

- **Static analysis first**: map imports, exports, strings and structures before attempting dynamic analysis. Tools: `rabin2 -E/-i/-l/-I` for PE binaries, `jadx` for Android APKs, `strings` with categorization
- **Confirmed facts vs speculation**: unconfirmed details are marked as such everywhere they appear. Quick-reference sections in CLAUDE.md contain only confirmed, tested facts
- **Implementation-as-validation**: build a parallel implementation (protocol library, inference CLI) that exercises the reverse-engineered interfaces against real targets. Passing tests against hardware or real models are the strongest validation
- **Validate decoders against known plaintext, not against each other.** Two backends once agreed on every surface while one decoded 12 of 72 corpus streams wrongly. Agreement between two implementations proves only that they share an error
- **Search for prior art, then treat it as a lead.** Keep clones of prior work in a git-excluded reference directory, fetched by a manifest and a recipe. A found source is a lead, not a fact; confirm it against the target before relying on it
- **One canonical findings doc per subject.** A single `Status:` line at the top (`CONFIRMED`, with the evidence in one clause), then the findings. No dates, commit hashes or counts in durable docs, because they rot. Each claim carries its provenance (the command or observation that produced it); a falsified claim is struck with the reason, not deleted. When findings scatter, consolidate by verifying then fusing into one doc, never by keeping both
- **The live disassembler database is the source of truth.** Query it directly (an IDA MCP, plus an arithmetic tool for every offset computation) instead of trusting stored offsets. Treat the database as a disposable render of a registry
- **Symbol-name form encodes confidence.** A verbatim qualified name means confirmed, `snake_case` means a guess, and `field_`, `pad_` or `sub_` plus an offset means structural. A registry keyed by stable anchors drives an idempotent formatter. Machine-written comments carry a `[gen]` ownership marker so a human's comments are never overwritten
- **Do not trust a decompiler's offsets as bases.** Disassemble a window of 60 to 120 instructions and do a live vptr sanity read before building a hook. A register can hold a loop-invariant render context where the decompiler shows the per-iteration entity. A static address that is a pointer slot is typed `Global<Ptr<T>>`, distinct from `Global<Class>`, so a missing dereference cannot compile
- **Observing is not validating.** For write surfaces, drive each candidate in each state context with a symmetric plus and minus before publishing, and split a constant when two contexts behave differently (`IDX_FOO_FOOT` and `IDX_FOO_VEHICLE`)
- **Check what a user would see, in the same step as the numbers.** State flags can say "in level" while the screen shows the front end. Take a screenshot alongside any live reading
- **Task-to-doc routing**: a mapping of finding types to documentation files (protocol details, app structure, API endpoints) so a new session consults the routing table instead of guessing
- **Annotated file tree in CLAUDE.md**: per-file purpose descriptions, marking generated files "DO NOT EDIT" with regeneration commands
- **Anti-hallucination rules for numeric domains**: for RE work where LLMs participate, all hex arithmetic goes through a tool call, every numeric claim cites its source, memory is read before it is written, and tool results are authoritative. See [ai-assisted-reverse-engineering-loop](./ai-assisted-reverse-engineering-loop.md)

## Language-Specific

### Android (APK/XAPK)

- Extract via `innoextract` (Inno Setup) or direct unzip (XAPK is ZIP). Decompile with `jadx`; look for `@Keep` annotations, which survive R8 obfuscation
- Obfuscated classes get short identifiers (`kotlin.a4g`); preserved public API classes are the best starting points

### Windows (PE32+)

- Use `rabin2 -E` for exports (handles MSVC mangled names), `rabin2 -I` for binary info including PDB paths that reveal build system details
- QML UI is often AOT-compiled into the executable, so there are no plaintext `.qml` files for custom components

## Anti-Patterns

- Speculation documented as fact
- Analysis without reproducibility: every finding should cite how it was produced
- Keeping several synchronized artifacts (an annotated header, a pointer-chain table and offset constants) for the same facts. This was tried and did not scale past a few hours of work; it is redundant with a findings doc plus the live database
- Dates, commit hashes and counts inside durable findings docs
- Publishing a write offset after observing it in one state only

## Related Topics

- [ai-assisted-reverse-engineering-loop](./ai-assisted-reverse-engineering-loop.md): the working loop when an LLM is in the analysis path
- [linux-process-memory-injection](../architecture/linux-process-memory-injection.md): runtime process manipulation (counterpart to static RE)
- [reloadable-rust-payloads](../architecture/reloadable-rust-payloads.md): iterating on injected code without restarting the target

## Open Questions

- Dynamic analysis workflow standardization (Frida probes, BT sniffing, mitmproxy)
- Automated protocol fuzzing informed by static analysis findings
- Classifying a memory slot's type by probabilistic evidence (log-likelihood per kind, composed into a distribution with a margin) instead of a precedence chain; designed in game-hacking, not implemented
