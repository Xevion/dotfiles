---
name: project-automation
category: dx
last_audited: 2026-11-12
exemplars:
  - repo: Xevion/tempo
    path: engine (scheduler, cache, exec, lock, supervise modules)
    note: "Task-graph engine: needs/after edges, content-fingerprinted inputs, stat-checked outputs, no-shell spawning with exec, hard-link lock files, watch cycles that interrupt"
  - repo: Xevion/xevion.dev
    path: tempo.config.ts
    note: "Rust dev-build/dev split with watch interrupt, codegen tasks with inputs/outputs, regenerate-into-tmpdir bindings with a prune step"
  - repo: Xevion/banner
    path: scripts/
    note: Bun-based build orchestration with command registry (earlier shape, now partly tempo)
  - repo: local/inkwell
    path: scripts/check.ts
    note: "Warn-only exit code pattern, TTY-aware live progress ticker"
---

# Project Automation

## Philosophy

Build scripts are code: write them in a typed language, not shell. Centralize command definitions so every script agrees on how to invoke tools. Detect staleness automatically rather than requiring manual regeneration steps. Express ordering as dependencies, not as the order of a list.

## Conventions

### Task graphs

- **Tasks are a flat list with edges.** Each task has a name (a `web:` or `rust:` prefix replaces a subsystem), a body, tags, optional `inputs`/`outputs` globs, and `requires` (tools it needs). Commands are tag selectors over the graph
- **Two edge strengths.** `needs` requires success and pulls the dependency into the run. `after` only orders, and is inert unless the target is already selected. Use `after` for two fix-writers that touch the same files (`biome:lint-fix` after `biome:format-fix`) so they never overlap, without forcing both to run
- **Persistent tasks release dependents on readiness, not on exit.** A dev server's dependents start when it reports ready
- **Auto-fix is an ordinary task.** A fix-writer is tagged for formatting and ordered with `after`; there is no separate fix strategy enum. Always re-verify after fixing. When parallel checks run, auto-format only if formatting is the sole failure, so formatting never masks other errors
- **A missing required tool skips the task locally and fails it in CI**, so a machine without a tool loses a check visibly and never passes silently. A local `requires` skip is not a CI pass
- **A target that matches nothing is an error that lists the valid scopes**, never a quietly smaller run. Whole-tree tasks opt out of narrowing with `always`
- **Progress comes from an event stream** that renderers consume, not from racing a map of promises. Skip the live ticker in non-TTY (CI) mode

### Staleness

- **Declare it as `inputs` and `outputs` globs on a task**, not as hand-written mtime code. Codegen is a tagged task that dependents pull in with `needs`
- **Fingerprint inputs by content, check outputs by stat.** The file list comes from git, digests are memoized by size, mtime and ctime, and content avoids spurious misses after a restored checkout. Outputs are checked by size and mtime, which catches hand edits without re-reading large artifacts
- **Regenerate into a temp directory, then copy only changed files and prune deleted ones.** A failed generator leaves the tree alone. Check with a separate verify task that diffs against git
- **`ensureFresh`-style mtime comparison** (newest source mtime versus newest artifact mtime, regenerate when stale, print a visible skip when fresh) remains a fine hand-written form for projects without an engine

### Process handling

- **A string command spawns without a shell unless it has shell syntax.** When a shell is needed, prefix the final command with `exec` so no wrapper process survives to swallow SIGTERM, and signal the child's process group
- **Dev processes that rebuild their own binary use `watch` with `interrupt`.** When cargo rewrites the running binary, the old process is interrupted first so it releases its port before the new one binds. Model the cycle as named wake and cycle states, not ad-hoc flags
- **Exclusion across processes without `flock`**: write the holder (pid, host, task) to a staging file and hard-link it to the lock name, which fails atomically if the lock is held. A waiter treats a holder whose pid is gone on the same host as abandoned, and an unparseable holder as abandoned too, so a corrupt file never hangs a waiter
- **Yield the machine to a foreground process** by wrapping background children in `taskset`, `chrt` and `ionice`. Each wrapper `exec`s its argument, so pids and signal groups are unchanged. Detection scans `/proc` comm names (truncated to 15 characters) at most once a second, and the concurrency cap can only lower

### Command registries and scripts

- **Command registry**: map `(subsystem, action)` tuples to command definitions in one module so scripts do not duplicate command strings

```typescript
const REGISTRY: Record<Subsystem, Record<Action, CommandDef>> = {
  frontend: { lint: { cmd: ['bun', 'run', '--cwd', 'web', 'lint'] } },
  backend: { lint: { cmd: ['cargo', 'clippy', '--all-features', '--', '-D', 'warnings'] } },
};
```

- **Warn-only exit code**: a `warnIfExitCode` field per check distinguishes advisory failures from blocking ones. Print a colored warning with a hint and continue
- **TTY-aware live progress ticker** in TTY mode: a live line of remaining check names with elapsed time, updated at 100 ms with `\r\x1b[K`
- **Pre-commit partial-staging safety**: detect partially-staged files and refuse to re-stage when staged and unstaged changes share a file

## Anti-Patterns

- Duplicating command strings across scripts
- Requiring manual regeneration of generated artifacts
- Auto-formatting during check runs that also have lint or type errors
- Pre-commit hooks that silently include unstaged changes
- A lock that only works if the holder exits cleanly (use a recoverable holder record)
- Treating a skipped required tool as a pass in CI

## Open Questions

- Turborepo/Nx-style task caching for monorepo builds
- Watcher-based vs poll-based staleness detection trade-offs
- Whether tempo continues or something replaces it; see [build-systems](../project-structure/build-systems.md)
