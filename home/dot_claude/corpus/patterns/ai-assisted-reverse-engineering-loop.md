---
name: ai-assisted-reverse-engineering-loop
category: patterns
last_audited: 2026-04-10
exemplars:
  - repo: local/game-hacking
    path: docs/agent-architecture.md + targets/assaultcube/analysis/
    note: "Six-phase RE loop, four-tier confidence KB, multi-agent roles, anti-hallucination rules"
---

# AI-Assisted Reverse Engineering Loop

## Philosophy

LLMs are exceptional pattern-matchers on text and code but **silently catastrophic at numeric reasoning** — hex arithmetic, address computation, and struct offset math produce confident wrong answers with no internal signal that anything is off. Reverse engineering is almost entirely numeric reasoning over untrusted, evidence-constrained data. The answer is not to ban LLMs from RE but to **structure the loop so the LLM never does the thing it's bad at**: let it read source, reason about intent, and generate hypotheses; let tools do the arithmetic and verify the hypotheses. The RE workflow becomes a closed feedback loop with hard boundaries between agent reasoning and verified facts.

This topic extends [binary-reverse-engineering](./binary-reverse-engineering.md) with the specific workflow patterns that make AI-assisted RE tractable. If you're doing static RE without an AI in the loop, start there.

## The Six-Phase Loop

```
OBSERVE → HYPOTHESIZE → INSTRUMENT → EXPERIMENT → ANALYZE → UPDATE KB
   ↑                                                              │
   └──────────────────────────────────────────────────────────────┘
```

- **OBSERVE**: read the current knowledge base state, the target's current behavior via the observation channel (logs, memory snapshots, trace output), and any prior session notes
- **HYPOTHESIZE**: generate candidate explanations for the observed behavior. This is the *only* phase where freeform LLM reasoning is valuable — pattern-matching against similar code, drawing analogies, proposing struct layouts
- **INSTRUMENT**: design an experiment that would distinguish competing hypotheses. Concretely: what memory regions to read, what values to write, what invariants to check post-mutation
- **EXPERIMENT**: execute the experiment via state-change tools (write memory, inject library, trigger game event). The LLM *calls tools*; it does not *compute* values
- **ANALYZE**: read the result back via the observation channel. Compare against predicted values. The verification oracle is authoritative — if the LLM's prediction disagrees with the observed value, the prediction is wrong
- **UPDATE KB**: promote, demote, or invalidate entries in the knowledge base based on experiment outcomes

The loop requires three infrastructure pieces, each non-optional:

1. **Persistent observation channel** — streamed logs, memory dumps, or trace output from the target that the agent can read without re-running experiments
2. **State-change tools** — typed MCP tools (or equivalent) for every mutation the agent is allowed to perform. Mutations must not happen via shell commands or ad-hoc scripts
3. **Verification oracle** — a source of ground truth (test suite, assertion library, game behavior observation) that independently confirms predicted outcomes

## Knowledge Base Structure

Maintain three artifacts per target, each machine-readable AND human-readable:

- **Confidence-annotated C header (`structs.h`)** — struct layouts with an inline confidence tier on every field:
  ```c
  struct Player {
      vec3 pos;           // +0x04 [CONFIRMED]
      float health;       // +0x10 [LIKELY] — correlates with HUD but overflow unverified
      uint32_t team;      // +0x14 [HYPOTHESIS]
      char _pad[0x20];    // [STALE] — was flags field in v1.2
  };
  ```
- **`pointer_chains.json`** — structured per-chain entries with `confidence`, `validated_count`, `notes`, `base_module`, `offsets[]`. Increment `validated_count` on every session that confirms the chain still resolves to the expected value
- **Offset constants in source** — typed constants (Rust `const`, C `#define`) with doc-comment confidence labels and evidence citations. Colocated with the code that uses them so the offset and its consumer drift together

Confidence tiers, in declining order of trust:

| Tier | Meaning | Treatment |
|---|---|---|
| `CONFIRMED` | Verified by multiple experiments across multiple sessions | Trust; use in production tooling |
| `LIKELY` | Single experiment confirms; no contradictions | Use with read-before-write guards |
| `HYPOTHESIS` | Reasoned guess; not yet tested | Test first; do not write based on it |
| `STALE` | Was CONFIRMED in a previous binary version; may still be valid | Re-verify before trusting |

## Anti-Hallucination Rules

These are non-negotiable in the agent's system prompt:

- **No hex arithmetic in your head** — any address math (base + offset, pointer dereferences, size calculations) must go through a tool call. Compute `0x804B000 + 0x2C` via a `compute_address(base, offset)` tool, never inline in agent reasoning. LLMs make consistent hex arithmetic errors with zero internal signal
- **Evidence-first** — every numeric claim in agent output must cite its source: "offset `0x2C` is the position y-component, confirmed by experiment 2026-04-08 (commit a7f3b2) where writing to `base+0x2C` moved the player on the y-axis"
- **Read-before-write** — before writing to any memory address, read it first and confirm the current value matches the expected type range. If `health` is supposed to be `0.0..100.0` and the read returns `1.5e38`, the offset is wrong; abort the write
- **Tool results are authoritative** — if the LLM's prediction disagrees with a tool's return value, the tool wins unconditionally. Do not "reinterpret" tool output to match the hypothesis
- **No freeform shell for mutations** — mutations go through typed tools, not `bash("echo ... > /proc/$pid/mem")`. The typed tools enforce invariants the shell can't

## Multi-Agent Role Decomposition

For targets beyond a single agent's effective context (large binaries, multi-system codebases), decompose into specialized agents dispatched as subagents:

| Role | Model class | Task |
|---|---|---|
| **Scout** | Small/fast (Haiku) | Enumerate functions, index symbols, summarize binary sections, report "here's what exists" without deep analysis |
| **Analyst** | Large/reasoning (Opus) | Deep analysis of specific functions, propose hypotheses, design experiments |
| **Experimenter** | Mid-tier (Sonnet) | Execute experiments via typed tools, collect results, report pass/fail |
| **Librarian** | Small/fast (Haiku) | Maintain the KB — promote/demote confidence tiers, archive stale entries, generate diff reports across sessions |

Scout and Librarian are cheap and run often; Analyst is expensive and runs on specific hard problems; Experimenter is the default workhorse. The Analyst never executes experiments directly — it produces an experiment spec that the Experimenter runs, enforcing the reasoning/execution split.

## Context Window Strategy

Large binaries exceed any single-session context. Strategies:

- **Curate by relevance, not completeness** — load only the KB entries, source files, and trace lines relevant to the current hypothesis. A 10k-line `structs.h` dump wastes 80% of context on fields unrelated to the current work
- **Prompt caching for stable artifacts** — the binary's section metadata, historical experiment logs, and confidence-CONFIRMED entries rarely change within a session. Mark them as cacheable via the API
- **Session resume** — use the Agent SDK `resume=` parameter (or equivalent) to continue a prior session without re-seeding context, rather than summarizing into a fresh session every time
- **Subagents for parallel section analysis** — when analyzing multiple independent subsystems, dispatch one Analyst per subsystem with scoped context, then aggregate findings in the parent

## Human vs. AI Division of Labor

Empirically, after running the loop through several targets:

| AI excels at | Human excels at |
|---|---|
| Pattern-matching across known code idioms | Recognizing "this doesn't smell right" gestalt |
| Generating plausible struct field names from context | Deciding which of 10 plausible hypotheses to test first |
| Boilerplate experiment scaffolding | Choosing the target of investigation |
| Reading large volumes of source quickly | Catching consistent-wrong-answer failure modes |
| Keeping the KB updated and consistent | Bending the rules when a session hits a dead end |

The loop should surface decisions to the human at transition points (new hypothesis selected, experiment result ambiguous, confidence tier promotion) rather than running unattended.

## Anti-Patterns

- **Freeform agent reasoning about addresses** — the LLM will produce confident wrong offsets. Always route through tools
- **Unverified KB updates** — promoting HYPOTHESIS to CONFIRMED without running a verifying experiment corrupts the entire KB
- **Single long-running agent session** — context fills with noise; periodically restart with the KB as the primary context seed
- **Skipping read-before-write** — silent memory corruption that manifests as game crashes an hour later
- **Treating the agent's output as a patch instead of a proposal** — the agent proposes; the human (or a verification gate) disposes

## Open Questions

- Automated regression detection when a binary updates — how much of the confidence-tier decay can be automated vs. requiring re-run of the full test suite?
- Integration with Ghidra/IDA scripting APIs as a tool surface — current practice uses file-based export, but direct MCP integration would reduce round-trip latency
