---
name: ai-assisted-reverse-engineering-loop
category: patterns
last_audited: 2026-11-12
exemplars:
  - repo: local/game-hacking
    path: docs/workflow (re-workflow, agent-architecture) + CLAUDE.md
    note: "Task-based agents, live-database tools, anti-hallucination rules, hard tool-use cap, observe-versus-validate discipline"
---

# AI-Assisted Reverse Engineering Loop

## Philosophy

LLMs are exceptional pattern-matchers on text and code but **silently catastrophic at numeric reasoning**: hex arithmetic, address computation and struct offset math produce confident wrong answers with no internal signal that anything is off. Reverse engineering is almost entirely numeric reasoning over untrusted, evidence-constrained data. The answer is not to ban LLMs from RE but to **structure the loop so the LLM never does the thing it is bad at**: let it read source, reason about intent and generate hypotheses; let tools do the arithmetic and verify the hypotheses.

This topic extends [binary-reverse-engineering](./binary-reverse-engineering.md) with the workflow patterns that make AI-assisted RE tractable.

## The Loop

```
OBSERVE -> HYPOTHESIZE -> INSTRUMENT -> EXPERIMENT -> ANALYZE -> UPDATE FINDINGS
   ^                                                                    |
   +--------------------------------------------------------------------+
```

- **OBSERVE**: read the current findings, the target's current behavior through the observation channel (logs, memory snapshots, trace output, a screenshot), and any prior notes
- **HYPOTHESIZE**: generate candidate explanations. This is the one phase where freeform reasoning is the point
- **INSTRUMENT**: design an experiment that distinguishes the hypotheses: what to read, what to write, what to check after
- **EXPERIMENT**: run it through typed tools. The model calls tools; it does not compute values
- **ANALYZE**: read the result back and compare with the prediction. The oracle is authoritative: if they disagree, the prediction is wrong
- **UPDATE FINDINGS**: edit the one findings doc for the subject (status line, claim, provenance). See [binary-reverse-engineering](./binary-reverse-engineering.md)

The loop needs three pieces of infrastructure: a **persistent observation channel**, **typed state-change tools** for every mutation (never a shell), and a **verification oracle** (a test suite, the live disassembler database, or the observed game behavior).

## Agents

- **One agent carries an RE task from start to finish across every source it needs** (the disassembler, the leaked source, the code), instead of three single-source agents reporting back. Splitting by source costs a round trip per hand-off and loses cross-source reasoning
- **Extra agents only for genuinely independent chunks**, such as separate subsystems of one large binary. When a second agent needs what the first learned, the coordinator relays the finding into its prompt instead of letting it re-derive it
- **A subagent's report is a lead, not evidence.** A rule built on a subagent's claim about one object name once turned out to describe an empty marker in all 654 prefabs. Verify before building on it
- **Autonomy**: the agent runs its own probes. The human is brought in for interactive probes, ambiguous results and confidence promotions, not at every transition

## Working Rules

- **A hard tool-use cap** (for example 25) on any investigation; if the cap approaches, stop and report. **Stop when a planned lead fails its precondition** and report, rather than hunting around it
- **No hex arithmetic in the model**: any address math goes through an arithmetic tool (base plus offset, dereference chains, hashes such as CRC32 and FNV-1a, packed-vector decodes, bitflag fields)
- **Evidence-first**: every numeric claim cites its source experiment or address
- **Read before write**: read an address and confirm the value fits its expected range before writing. If `health` should be `0.0..100.0` and the read returns `1.5e38`, the offset is wrong; abort
- **Tool results are authoritative** over the model's predictions
- **State describes memory, not what the player sees.** Take a screenshot in the same step as the numbers before trusting a flag
- **Edit a probe in place rather than cloning it into variants**, and delete code that did not earn its place on every pass
- **Observe is not validate**: drive each candidate in each state context, symmetric plus and minus, before publishing

## Context Strategy

- **Curate by relevance, not completeness**: load only the findings, source and trace lines relevant to the current hypothesis
- **Prompt caching for stable artifacts**: section metadata, historical experiment logs and confirmed findings rarely change inside a session
- **Session resume** (the Agent SDK `resume=` parameter) to continue a prior session without re-seeding context
- **Restart periodically** with the findings docs as the context seed, because a single long session fills with noise

## Human vs. AI Division of Labor

| AI excels at | Human excels at |
|---|---|
| Pattern-matching across known code idioms | Recognizing "this doesn't smell right" |
| Generating plausible names from context | Choosing which of ten hypotheses to test first |
| Boilerplate experiment scaffolding | Choosing the target of investigation |
| Reading large volumes of source quickly | Catching consistent-wrong-answer failure modes |
| Keeping findings updated and consistent | Bending the rules when a session hits a dead end |

## Anti-Patterns

- Freeform agent reasoning about addresses
- Unverified findings updates (promoting a claim without a verifying experiment)
- A single unbounded agent session, or an investigation with no tool-use cap
- Skipping read-before-write
- Treating the agent's output as a patch instead of a proposal
- Splitting one RE task across several single-source agents

## Open Questions

- Automated regression detection when a binary updates: how much of the re-verification can be automated
- Direct MCP integration with the disassembler (done for IDA; Ghidra untested) versus file-based export
