# guard — transparent pipeline capture for Claude Code's Bash tool

Rust rewrite and architectural inversion of `home/dot_claude/hooks/bash-guard.ts`.
One binary, two subcommands. `guard hook` is the logic (PreToolUse: discipline
rules, approval, command rewriting). `guard run` is the machine (executes one
pipeline itself: spawns stages, wires pipes, taps the unfiltered source stream,
reports exit/duration/counts in a footer).

> **Status**: design. Sections marked *PROVISIONAL* await research results
> (parser crate selection, hooks-API confirmation, pipeline-executor traps).

---

## 1. Problem and evidence

The current TS hook only intervenes on `| head` / `| tail`, always writes a
capture file (even when the preview already shows the full content), cannot see
the source command's exit code or duration, and shows the *first* 50 lines even
when the agent asked for the tail.

Mined from 23,339 Bash tool calls across 1,138 session transcripts (61 projects):

| Pattern | Share | Implication |
|---|---|---|
| `\| head` | 22.9% | — |
| `\| tail` | 13.3% | 36% of all commands route through head/tail |
| `build-cmd 2>&1 \| tail` | 9.0% | agent wants the END (compiler errors); old preview showed the head |
| `\| grep` / `\| rg` filtering | 11.7% | full stream discarded; zero recovery when over-filtered |
| head/tail pipe that also merges `2>&1` | 7–12% | `$?` becomes the filter's exit; real status lost |
| manual `echo "exit: $?"` workarounds | 1.8% | agents fighting the harness to see status |
| `head -n ≤10` | 3.0% | capture file guaranteed redundant with inline output |
| re-reads of the guard's saved `.log` | 1.2% | ~99% of files written today are never opened |

Design goals, distilled from the above:

1. **Execute exactly what the agent typed.** Filters run natively (`tail -30`
   really shows the last 30). guard never re-engineers head/tail/grep/rg — it
   only captures the stream *before* the filters so over-filtering is
   recoverable by reading a file, not by re-running.
2. **No file unless something was actually lost.** Inline output that equals
   the full stream ⇒ zero disk, zero footer noise.
3. **Surface exit code and duration** of the source command, prominently, so
   status-code avoidance (`|| true`, `2>&1 | tail`) loses its motivation.
4. **Invisible execution.** Same PATH resolution, same env, same cwd
   semantics, same signal behavior as the unwrapped command. The only visible
   difference: a one-line footer after the output.

Non-goals: sandboxing, output rewriting, replacing the permission system,
handling interactive/TTY programs, fish/PowerShell support.

## 2. Ground truth: what actually executes a "Bash" tool command

Empirically verified on this machine (2026-07-02):

```
/usr/bin/zsh -c source ~/.claude/shell-snapshots/snapshot-zsh-<ts>.sh 2>/dev/null || true \
  && setopt NO_EXTENDED_GLOB NO_BARE_GLOB_QUAL 2>/dev/null || true \
  && eval '<agent command>' < /dev/null \
  && pwd -P >| /tmp/claude-<id>-cwd
```

- The tool is *named* Bash; the agent writes bash-flavored syntax; **zsh
  executes it**. Selection is not a fish fallback: Claude Code prefers zsh
  whenever `/usr/bin/zsh` exists, regardless of `$SHELL` (bash only on
  zsh-less systems; fish is never used — claude-code#7490, #11475, #13425).
  Two `setopt`s bash-ify globbing.
- A ~200 KB **snapshot** of a zsh login environment (functions, aliases,
  setopts, a PATH export) is sourced before every command. Commands may
  therefore resolve to **aliases or shell functions**, not just PATH
  executables. Env vars otherwise do NOT persist between calls; PATH survives
  only because the snapshot re-exports it.
- **Claude Code plants its own function wrappers in the snapshot** for
  `rg`, `find`, and `grep`, redirecting them to binaries embedded in the
  claude executable (`ARGV0=rg "$CLAUDE_CODE_EXECPATH"` → embedded ripgrep;
  `find` → embedded bfs; `grep` → embedded ugrep). A pipeline filter named
  `grep` does not run system grep under this harness.
- Each call is a fresh process; **cwd persists via a file**
  (`pwd -P >| /tmp/claude-…-cwd`, re-read next call) — and only on success
  (`&&` chain), only within project/additional dirs (outside ⇒ reset notice).
- stdin is `/dev/null`.

Consequences baked into this design:

| Fact | Design consequence |
|---|---|
| zsh executes, agent assumes bash | `guard hook` parses with a bash grammar (what the agent *meant*); any parse failure ⇒ fail open, no rewrite |
| snapshot defines aliases/functions | capture predicate (§4.3) requires every stage's argv0 to resolve to a real PATH executable; otherwise skip rewrite |
| `rg`/`find`/`grep` are harness wrappers to embedded binaries | `guard run` replicates the wrapper for these three names: spawn `$CLAUDE_CODE_EXECPATH` with `arg0` set to the tool name (§5.1); running the system binaries instead would *diverge* from unwrapped behavior (`grep`→ugrep and `find`→bfs differ materially) |
| cwd persisted by the outer shell | `cd`, `export`, assignments must never move inside guard — the rewrite touches only pipelines (§4.2) |
| `eval '<command>'` in zsh | rewritten text must be zsh-eval-safe; the spliced pipeline is a single-quoted word with `'\''` escaping (§4.4) |
| shell may differ per machine (bash on others) | `guard run` never shells out for simple stages; the compound-stage fallback uses `$SHELL` (§5.4) |

## 3. Architecture

```
             PreToolUse (settings.json)
                     │ JSON on stdin
                     ▼
             ┌───────────────┐   rewrite: pipeline → guard run '…'
             │  guard hook   │──► updatedInput / permissionDecision /
             └───────────────┘    additionalContext / exit 2 (block)
                     │
        harness zsh evals rewritten line
   cd crates && guard run 'cargo test 2>&1 | rg FAIL | head -40'
                     │
                     ▼
             ┌──────────────────────────────────────────────┐
             │  guard run  (one process per pipeline)       │
             │                                              │
             │  spawn cargo test ──► tap thread ──► spawn rg│
             │   (execvp, 2>&1      buffer-w-spill   FAIL   │
             │    wired via fd)     count lines/bytes  │    │
             │                                     spawn head
             │                                         │    │
             │  final stdout ◄─────────────────────────┘    │
             │  … EOF → footer line → exit                  │
             └──────────────────────────────────────────────┘
```

- **One `guard` process per pipeline.** Never two guard touchpoints inside a
  single pipeline; a multi-statement line gets one `guard run` per
  capture-worthy pipeline.
- The **statement skeleton stays in the harness shell**: `cd`, `&&`/`||`/`;`,
  variable assignments, `export`, backgrounding. Only pipeline text moves
  inside `guard run` — pipelines carry no persistent shell state.
- `guard hook` and `guard run` share one parser and one **supported-pipeline
  predicate**, so the rewriter can never produce a pipeline the runner can't
  faithfully execute.

## 4. `guard hook` — PreToolUse

Reads the hook JSON payload. Phases (ported from the TS implementation, same
ordering):

### 4.1 Discipline rules (ported as-is)

| Rule | Verdict |
|---|---|
| `sudo` | **block** (exit 2) |
| pipe-to-shell (`curl … \| bash`, without `-c`) | **block** |
| `git stash` (any spelling) | **block**, full hard-ban message |
| bare `cat <single-file>` | warn → Read tool |
| `find -name/-type` | warn → fd / `rg --files` |
| `rg -rN` flag-bundle (grep muscle memory) | warn, explains `--replace` mangling |
| `2>/dev/null` (except `command -v`/`which`/`type`/`hash`) | warn |

New rules enabled by the mined data (small, additive):

| Rule | Verdict |
|---|---|
| `\|\| true` / `\|\| echo …` swallowing a build/test command's status | warn — footer now reports exit; suppression is unnecessary |
| `echo $?` / `echo "exit: $?"` | warn — exit code is in the footer |

### 4.2 Rewrite

Parse the command line (bash grammar, spans/byte offsets — parser per §8).
Walk **command-position statements only**. For each pipeline that passes the
capture predicate, replace the *pipeline's exact byte span* with:

```
guard run '<pipeline text, single-quote-escaped>'
```

Everything outside pipeline spans is preserved byte-for-byte.

```
cd crates && RUST_BACKTRACE=1 cargo test 2>&1 | rg FAIL | head -40
                      ▼
cd crates && guard run 'RUST_BACKTRACE=1 cargo test 2>&1 | rg FAIL | head -40'
```

Not rewritten (each with the reason):

- **Pipelines without any filter stage** — no narrowing, nothing to capture.
  ~64% of commands remain untouched end-to-end.
- **Command substitutions** `$( … | jq -r .id )` — output feeds a variable,
  not the transcript; capture is pointless.
- **Backgrounded statements** (`… &`) — the harness's own task machinery
  handles these; wrapping changes job semantics.
- **Anything failing the capture predicate** (below).

### 4.3 Capture predicate (shared with `guard run`)

`capturable(pipeline) == true` only when ALL hold:

1. ≥ 2 stages, and the final stage (or any non-first stage) is a known
   **filter**: `head`, `tail`, `grep`, `rg`, `sed`, `awk`, `jq`, `wc`, `sort`,
   `uniq`, `cut`, `tr`. (Initial set; data-driven, extendable.)
2. Every stage is a **simple command** (argv + optional leading `VAR=val`
   assignments + supported redirects), *or* the **first** stage is a compound
   command (§5.4). Compound in any later position ⇒ not capturable.
3. Supported redirects only: `2>&1`, `>file`, `>>file`, `<file`, `2>file`.
   Process substitution `<(…)`, fd-juggling (`3>&2`, `{fd}>`), `|&`, heredocs
   in non-first stages ⇒ not capturable.
4. Every simple stage's argv0 **resolves to an executable on PATH**
   (stat + execute bit, using guard's inherited environment). This is the
   alias/function shield: if argv0 is only meaningful via the zsh snapshot
   (alias, function) or as a builtin, guard must not adopt it. Assignments-only
   stages and reserved words also fail this.
5. No stage argv0 is `guard` itself (idempotency; re-invocation of an already
   rewritten command must be a no-op).
6. `tail -f` / `--follow` anywhere ⇒ not capturable (never terminates).

**Fail open, never approximate**: any predicate failure, parse error, or shfmt
oddity ⇒ command passes through unmodified. The worst case of guard is always
"behaves like today, without capture" — never "broke my command."

### 4.4 Quoting

The pipeline substring is spliced as one single-quoted zsh word; embedded
single quotes become `'\''`. This is the only text transformation in the whole
system and must be property-tested (random pipelines: rewrite → zsh eval →
`guard run` receives byte-identical pipeline text).

### 4.5 Approval (ported, with one simplification)

Same phase-2 logic as today: decompose compound commands, match sub-commands
against merged allow/deny/ask prefixes from user + project settings, expand
`bash -c` payloads, auto-allow when all sub-commands are allowed, deny → exit 2.
The `xevion projects content` carve-out ports unchanged.

Approval evaluates the **original** commands (what the agent asked for);
`guard run '<pipeline>'` wrapping is approval-transparent: the runner is
trusted by construction, and the inner pipeline's stages are what get matched.
(Replaces the TS re-parse of the "effective command" — the runner never adds
approval surface, so approving the original is equivalent and simpler.)

### 4.6 Hook output

Single JSON response combining, as applicable: `updatedInput` (rewritten
command), `permissionDecision: allow`, `additionalContext` (warn lines + a
one-time note that rewritten pipelines print a `[guard]` footer and where full
output lands).

Schema notes (confirmed against current docs, 2026):

- All three fields live under `hookSpecificOutput`; `permissionDecision: allow`
  does not bypass permission rules — normal flow still applies after the hook.
- Hook output strings are capped at 10,000 chars.
- Only ONE hook may emit `updatedInput` for a given tool (parallel hooks,
  last-writer-wins) — guard must remain the sole Bash-input rewriter.
- **Caveat**: claude-code#15897 (closed "not planned") reports
  `updatedInput` being silently ignored when combined with
  `permissionDecision: allow`. This is contradicted empirically: the current
  TS hook emits exactly this combination and its rewrites demonstrably apply
  on this machine's build (2026-07). Treat as version-sensitive, not fatal.
  **Required**: an integration test (`echo-swap` hook) that verifies
  updatedInput application under (a) bare, (b) +additionalContext,
  (c) +permissionDecision:allow — run once per Claude Code upgrade; if (c)
  regresses, fall back to emitting the rewrite without a permissionDecision
  and letting settings prefixes handle approval.

Prior art reviewed: `quiet-bash` (PreToolUse rewrite → mktemp redirect,
`[ok: exit 0 — N lines hidden]` summaries) and `squeez` (PreToolUse command
wrapping + PostToolUse `updatedToolOutput` compression). guard differs by
executing filters as typed and capturing pre-filter — neither tool preserves
the agent's intended output shape.

## 5. `guard run` — the pipeline executor

`guard run '<pipeline>'` re-parses the pipeline with the same parser and the
same predicate (defense in depth: if the predicate now fails — e.g. PATH
changed between hook and run — exec the pipeline via `$SHELL -c` untouched and
add no footer, preserving behavior over features).

### 5.1 Process model

For `S | F1 | F2` (source + filters):

1. `pipe()` per adjacency; spawn each stage with `posix_spawn`/`fork+exec`
   (via `std::process::Command` + fd wiring, no shell):
   - argv exec'd directly (`execvp` semantics — PATH from inherited env,
     identical resolution to the harness shell for real executables).
   - **Harness-wrapper replication**: when argv0 is `rg`, `find`, or `grep`
     and `$CLAUDE_CODE_EXECPATH` names an executable, spawn that binary with
     `CommandExt::arg0(<name>)` instead — byte-identical to the snapshot
     function the harness shell would have invoked. Falls back to the PATH
     binary when the env var is absent. Version-sensitive (the wrapper set
     is Claude Code's, not ours): the per-upgrade test suite (§4.6) compares
     `rg --version` through the harness vs through guard.
   - Leading `VAR=val` words become child env entries.
   - Redirects implemented by fd wiring at spawn: `2>&1` ⇒ child stderr dup'd
     to the same write-end its stdout uses; `>f`/`>>f`/`<f`/`2>f` ⇒
     open + dup2.
2. **Tap**: guard owns the read end of S's output pipe and the write end of
   F1's stdin pipe. A thread copies bytes S→F1 while counting lines/bytes and
   feeding the buffer-with-spill (§5.2).
3. Final stage's stdout is inherited (streams live to the harness — no
   buffering of the visible output; interleaving and pacing match native).
4. `waitpid` all stages; record S's exit status and wall time
   (`Instant::elapsed` around S's lifetime).
5. Print footer (§5.3) to guard's own stdout, then exit (§5.5).

Signal/termination semantics (resolved by research; the five traps below are
the implementation checklist, ranked by likelihood of biting):

1. **Pipe fd hygiene** — create every adjacency with `std::io::pipe()`
   (stabilized Rust 1.87) and `drop()` the parent's copy of each write end
   immediately after the consuming `spawn()`. Never chain `ChildStdout`
   directly into the next stage's `.stdin()` — rust-lang/rust#98209 (open
   since 2022): `Command` keeps the pipe write-end open in the parent until
   `wait()`, so the downstream reader never sees EOF and the pipeline hangs.
   For `2>&1`, `try_clone()` the write end and hand both clones to the child
   (`.stdout(w).stderr(w2)`); audit that no clone outlives its scope.
2. **EPIPE in the tap** — when a downstream filter exits early (`head -40`
   closes), the tap's write returns `ErrorKind::BrokenPipe` (guard itself has
   SIGPIPE ignored, so it's an error, not a signal — easier). **Decision:
   propagate-close, not drain.** The tap stops reading and closes its
   read-end from S, so S's next write raises SIGPIPE and it dies with 141 —
   byte-identical to the shell, and `yes | head -1` terminates. The
   research's alternative ("drain mode": keep reading so S runs to completion
   and yields its true exit code) is **rejected**: it silently changes
   execution semantics (S completes when the shell would have killed it),
   violating the invisibility invariant, and diverges unboundedly on infinite
   producers. The footer annotates instead:
   `exit 141 (SIGPIPE — cut by head)`. Minor accepted skew: S may write a
   few extra buffers into the tap before noticing, so capture can extend
   slightly past what a direct pipe would have allowed.
3. **SIGPIPE dispositions** — Rust's default is exactly right: guard runs
   with `SIG_IGN` (handles `BrokenPipe` as errors), and libstd resets
   children to `SIG_DFL` before exec, so filters/sources die of SIGPIPE
   normally. Do NOT build with `-Zon-broken-pipe` (the `error` mode makes
   children inherit `SIG_IGN` and breaks pipeline termination outright).
   guard's own final-stdout writes handle `BrokenPipe` by exiting quietly.
4. **Wait ordering** — join the tap thread, then `wait()` stages in
   **reverse pipeline order** (final → … → source). Waiting on S first can
   deadlock (S blocked on a full pipe no one is draining). `Child` does not
   reap on drop — every stage must be waited or it zombifies. Signal deaths
   synthesize the shell convention: `128 + signal` via
   `ExitStatusExt::signal()`.
5. **Byte-level accounting** — pipe reads are arbitrary chunks; count `\n`
   per byte, never per read; a trailing unterminated line counts as one.
   (Hyperfine's pattern for timing: `Instant::now()` bracketing S's
   spawn→wait specifically, so duration is the source's wall time, not the
   pipeline drain time.)

Prior-art notes worth keeping: GNU `tee -p` (`--output-error=warn-nopipe`) is
the drain-mode reference if the EPIPE decision is ever revisited; `pv` exits
on EPIPE without draining (and can stall its upstream — the failure mode trap
2's close-propagation avoids); `duct` was evaluated and rejected — its
`.pipe()` wires stages at the OS level with no seam for an in-process tap,
and its kill/drop semantics don't fit (§8).

### 5.2 Buffer-with-spill (the "don't always write a file" mechanism)

The tap accumulates the source stream in memory up to `MEM_CAP` (default
256 KiB). Three outcomes:

| Outcome | Disk | Footer |
|---|---|---|
| Stream ended ≤ MEM_CAP **and** filters dropped nothing (source bytes == final output bytes) | none | minimal — exit + duration only |
| Stream ended ≤ MEM_CAP, filters dropped data | write buffer once, content-hashed name | full footer with path |
| Stream exceeded MEM_CAP | stream directly to `.pending-<pid>`, rename to hash on close | full footer with path |
| Stream exceeded `SPILL_CAP` (default 64 MiB) | stop writing, keep draining/counting | path + `truncated at 64MiB (stream was N MiB)` |

"Filters dropped nothing" is decided by comparing source byte count to final
visible byte count (guard counts the final stage's output via a second
lightweight tap on the last pipe — pass-through, count only). This is what
kills the `tail -10`-on-9-lines-of-output file: identical counts ⇒ no file,
no path, no noise.

Spill location `/tmp/claude-guard/`, name = first 12 hex of BLAKE3 of content
(dedup across repeated runs — same rationale as the TS version's SHA-256).
A `guard run`-side sweep deletes spills older than 7 days (amortized, at most
once per hour via a marker file).

### 5.3 Footer

One line, appended to final stdout after EOF of the last stage — visible
identically to the user (rendered output) and the agent (tool result). Since
guard owns the last write, no filter can eat it and `2>&1` cannot poison it.

```
[guard] cargo test → exit 101 · 4.2s · source 1204 lines / 48 KB → passed 12 · full: /tmp/claude-guard/a1b2c3d4e5f6.log
```

Minimal form when nothing was dropped and nothing spilled:

```
[guard] cargo test → exit 0 · 3.1s
```

Rules:
- Exit code **always** shown, first. Duration always shown.
- Counts + path only when a spill file exists.
- Truncation, signal death, and cut-by-head annotations inline.
- Multi-stage sources are labeled by argv0 of S (or `{…}` for compound).

### 5.4 Compound first stage

`{ …; } | grep`, `( … ) | head`, `for … done | wc -l`: the source is not a
single argv. Because a non-final pipeline stage **already runs in a subshell**
under every POSIX shell, executing it via `$SHELL -c '<exact source text>'`
is state-neutral by construction — there was never persistent state to lose.
guard uses `$SHELL` (zsh here, bash elsewhere) to match the harness dialect;
the child inherits guard's env, and the tap/footers work identically. Exit
code is the shell child's exit.

Known divergence, accepted: a compound source referencing a snapshot-defined
alias/function will fail under `$SHELL -c` (snapshot not sourced). Predicate
rule 4 cannot inspect inside compound text cheaply; if the spawn produces
exit 127, the footer says so and the agent's natural retry is unwrapped
(fail-open on the retry: predicate memory is stateless, but 127 + footer makes
the cause obvious). *If this ever bites in practice, the fix is sourcing the
snapshot in the child — deliberately NOT done now to keep guard's children
minimal.*

### 5.5 Exit code

Default: guard exits with the **final stage's** status — exactly what the
shell would report for the pipeline, so wrapping never changes observable
control flow (`&&` chains behave identically).

`--exit-source` flag (not enabled by default; the hook could opt specific
commands in later): guard exits with the **source's** status instead,
un-masking `cargo test 2>&1 | tail` failures at the `$?` level. Off by
default because it changes pipeline semantics the agent may rely on.
The footer always shows the source's status regardless, which removes most of
the motivation for the flag.

## 6. Scenario table (normative)

| # | Agent typed | After `guard hook` | Notes |
|---|---|---|---|
| 1 | `cargo build \| tail -30` | `guard run 'cargo build \| tail -30'` | tail runs natively; end-of-stream shown, as intended |
| 2 | `cargo check 2>&1 \| tail -30` | `guard run 'cargo check 2>&1 \| tail -30'` | `2>&1` becomes fd wiring on the cargo child |
| 3 | `RUST_BACKTRACE=1 cargo test 2>&1 \| rg error \| head` | `guard run 'RUST_BACKTRACE=1 cargo test 2>&1 \| rg error \| head'` | env assignment → child env; two filters; full stream captured |
| 4 | `cd crates && cargo nextest run 2>&1 \| rg FAIL` | `cd crates && guard run 'cargo nextest run 2>&1 \| rg FAIL'` | `cd` stays in harness shell → persists via cwd file |
| 5 | `ls src \| head; cargo build 2>&1 \| tail` | `guard run 'ls src \| head'; guard run 'cargo build 2>&1 \| tail'` | one guard per pipeline |
| 6 | `SID=$(curl -s … \| jq -r .id); cargo run --id $SID \| tail` | assignment untouched; second pipeline wrapped | `$(…)` never wrapped; `$SID` expands in harness shell **before** guard sees argv |
| 7 | `cd ~/p && RUST_LOG=debug timeout 60 cargo test 2>&1 \| rg -n 'error\|FAILED' \| grep -v flaky \| tail -50` | `cd ~/p && guard run 'RUST_LOG=debug timeout 60 cargo test 2>&1 \| …'` | `timeout` is just argv0; exit 124 propagates; 3 filters |
| 8 | `{ echo hdr; cat data; } \| grep foo` | `guard run '{ echo hdr; cat data; } \| grep foo'` | compound source via `$SHELL -c` (§5.4) |
| 9 | `cargo build & cargo test \| tail` | background stmt untouched; `cargo test \| tail` wrapped | `&` exclusion |
| 10 | `mycustomalias \| head` | **not rewritten** | argv0 not on PATH (predicate 4) — alias resolves in zsh as before |
| 11 | `cargo test > /tmp/out.log 2>&1` | **not rewritten** | no filter stage; discipline warns may still fire |
| 12 | `guard run 'ls \| head'` | **not rewritten** | idempotent (predicate 5) |

Wait — scenario 6: variable expansion. The pipeline text is spliced inside
**single quotes**, so `$SID` would NOT expand before guard parses it. Two
options: (a) splice with double quotes + escaping so the harness shell expands
variables first (guard sees literal values — matches "shell above, guard
below"), or (b) predicate excludes pipelines containing `$…` expansions.
**Decision: (b) for v1** — exclude pipelines whose capturable stages contain
unexpanded parameter/arithmetic/command expansions; revisit (a) later. This
keeps quoting bulletproof at the cost of skipping ~rare parameterized
pipelines. (The mined data shows `$VAR` in filtered pipelines is uncommon.)

## 7. What replaces today's behaviors (migration)

| Today (`bash-guard.ts`) | guard |
|---|---|
| head/tail segment replaced by `--preview` filter; agent's `-n` ignored | filters run **as typed**; capture is additive |
| always writes `/tmp/claude-bash/<hash>.log` | file only when data was dropped or stream overflowed (§5.2) |
| preview = first 50 + last 5, regardless of head/tail direction | moot — native filters show exactly what was asked |
| no exit code, no duration | footer, always |
| grep/rg pipelines invisible | captured (11.7% of commands gain recovery) |
| discipline rules + approval in TS | ported into `guard hook` |
| Bun runtime (~30–50 ms startup) + `shfmt` subprocess per invocation | single native binary (~1 ms), parser in-process |
| deploys via `home/dot_claude/hooks/` + settings.json | binary at `~/.claude/hooks/guard`; settings.json points PreToolUse at `guard hook` |

Deletion list once guard is live: `home/dot_claude/hooks/bash-guard.ts`, its
build step, and the `/tmp/claude-bash` convention (`.chezmoiremove` not needed
— hook artifacts, not managed targets; settings.json.tmpl updated instead).

## 8. Implementation choices

*PROVISIONAL — pending research agents; recommendations below are the working
assumption and the doc will be amended.*

- **Language**: Rust. Startup latency is the binding constraint (hook runs on
  every Bash call); Bun's cold start is the single biggest cost of the current
  system.
- **Parser**: **`brush-parser`** (from the brush shell project; MIT, actively
  maintained as of 2026, ~1700 bash-compat tests; handles `|&`, process
  substitution, extglob). Entry: `tokenize_str()` → `parse_tokens()`; walk
  `CompleteCommand → AndOrList → Pipeline { seq }`; spans via the
  `SourceLocation` trait (`.location() -> Option<SourceSpan>`).
  Two mandated adaptations:
  - `SourcePosition.index` is a **character** index, not a byte offset —
    guard builds a char→byte table per command line and converts before
    splicing (§4.2). Property-tested against non-ASCII command lines.
  - `location()` returning `None` on any node the rewrite needs ⇒ predicate
    failure ⇒ fail open (no rewrite).
  Eliminated: `conch-parser` (archived 2022), `yash-syntax` (GPL-3.0,
  POSIX-only, no spans on Pipeline/SimpleCommand), `bash-ast` (GPL, no spans,
  not thread-safe). Contingency if brush-parser misparses real-world commands:
  `shfmt --language-dialect bash --to-json` (BSD-3, true byte offsets in
  `Pos.Offset`/`End.Offset` on every node — the strongest span story) at the
  cost of one ~10–30 ms subprocess per hook call.
- **Process plumbing**: `std::process::Command` + `std::io::pipe()`
  (Rust ≥ 1.87; both ends are `Into<Stdio>`) + a tap thread. No `os_pipe`
  needed on current Rust; `nix` only if a `pre_exec` dup2 escape hatch is
  ever required for exotic redirects (not in the v1 supported set). `duct`
  confirmed unusable: no seam for an in-process tap between stages, kill
  doesn't reach grandchildren, drop is poll-based zombie avoidance rather
  than kill-on-drop.
- **Hashing**: BLAKE3 (fast, small dependency) for spill dedup names.
- **No async runtime.** Two threads (tap + final-count) and waitpid; tokio is
  weight without benefit here.

## 9. Repo layout, build, deploy

```
tools/guard/               # repo root — outside .chezmoiroot (home/), so
├── Cargo.toml             # chezmoi apply never touches it
├── DESIGN.md              # this file
├── src/
│   ├── main.rs            # subcommand dispatch: hook | run
│   ├── parse.rs           # parser wrapper: AST + spans + predicate
│   ├── hook.rs            # rules, rewrite, approval (port of TS phases)
│   ├── run.rs             # executor: spawn, fd wiring, waitpid, footer
│   ├── tap.rs             # buffer-with-spill, counting, EPIPE handling
│   └── rules.rs           # discipline rule table
└── tests/
    ├── rewrite.rs         # golden: command line → rewritten line
    ├── exec.rs            # end-to-end pipeline runs (real binaries)
    └── quoting.rs         # property test §4.4
```

Deployment (follows the timeline-revival auto-build precedent):

- `home/run_onchange_after_build-guard.sh.tmpl` — hash-triggered by the crate
  source; runs `cargo build --release` in `tools/guard` and installs to
  `~/.claude/hooks/guard`. Skips with a notice when no Rust toolchain exists
  (cold-bootstrap machines keep the old TS hook until cargo is present).
- `home/dot_claude/settings.json.tmpl` — PreToolUse Bash hook command becomes
  `~/.claude/hooks/guard hook`.
- Cutover is atomic per-machine: settings.json points at exactly one hook.

## 10. Performance budget

| Path | Budget | Notes |
|---|---|---|
| `guard hook`, no rewrite (64% of calls) | < 5 ms | parse + rule walk, no subprocess |
| `guard hook`, rewrite + approval | < 10 ms | settings read is memoized per-invocation |
| `guard run` overhead vs bare pipeline | < 1 ms spawn + one extra copy S→F1 | tap copies through 64 KiB buffers; disk writes only past MEM_CAP |
| Footer latency | 0 | printed after EOF, adds nothing to stream time |

The tap adds one user-space copy on the S→F1 adjacency only. For
`cargo build 2>&1 | tail -30` the bottleneck is cargo, not a memcpy.

## 11. Failure policy (summary)

| Failure | Behavior |
|---|---|
| parse error in hook | no rewrite; discipline rules on raw text where possible; command runs untouched |
| predicate fails | no rewrite |
| `guard run` re-predicate fails | exec pipeline via `$SHELL -c` verbatim, no footer |
| spawn ENOENT mid-run | footer reports exit 127 for that stage, mirrors shell |
| spill dir unwritable | capture disabled for that run, footer says `capture unavailable`; pipeline still runs |
| guard binary missing (fresh machine) | settings.json still references TS hook until first successful build — see §9 |

**Invariant: guard degrades to absence.** Every failure path converges on
"the command runs as if guard didn't exist."

## 12. Open questions (tracked; doc will be amended)

1. ~~Parser crate final pick + span API details.~~ **Resolved**: brush-parser,
   with char→byte span conversion and `shfmt -tojson` as contingency (§8).
2. ~~`updatedInput` + `permissionDecision` + `additionalContext`
   coexistence.~~ **Resolved with caveat**: works empirically on this build;
   version-sensitive per claude-code#15897; guarded by the echo-swap
   integration test (§4.6).
3. ~~SIGPIPE/EPIPE trap list and waitpid ordering.~~ **Resolved**: five-trap
   checklist in §5.1, including the propagate-close-vs-drain decision.
4. ~~PostToolUse utility.~~ **Resolved — deferred to v2**: PostToolUse
   provides `tool_output` (single string, no structured exit code) and
   `updatedToolOutput` (rewrite what the model sees; built-in tools require
   Claude Code ≥ 2.1.119). Nothing guard needs at v1 — the footer already
   travels in-band. v2 candidates: strip ANSI from `tool_output`, or verify
   the harness-reported result contains the `[guard]` footer (rewrite-health
   canary).
5. Variable-expansion pipelines (scenario 6 option (a)) — v2 candidate.
6. `--exit-source` opt-in surface: per-command config? never?
7. Harness-wrapper drift: the `rg`/`find`/`grep` → embedded-binary
   replication (§5.1) mirrors an undocumented Claude Code mechanism; the
   wrapper list or `CLAUDE_CODE_EXECPATH` contract may change between
   releases. Covered by the per-upgrade test; if it churns, consider parsing
   the active snapshot file for `function <name>` markers instead of
   hardcoding the set.
