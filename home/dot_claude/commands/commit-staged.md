---
allowed-tools: Bash(git commit:*), Bash(commit-helper:*)
argument-hint: [optional custom instructions]
description: Commit currently staged changes with an appropriate message
---

## Context

!`commit-helper --staged`

## Your task

Based on the above staged changes, create a single git commit.

## Commit message style requirements

**Match the project first.** Check the "Recent Commit Style" section in the context above before
anything else below. If those commits use a convention (e.g. conventional-commit prefixes like
`feat:`/`fix:`/`chore:`/`refactor:`), your message must use it too — that convention overrides
the generic examples in this section wherever they'd otherwise disagree.

**Don't repeat a recent subject line.** Compare your drafted subject against the "Recent Commit
Style" section above. If it's identical or near-identical to one of those (same wording, only the
scope or a word swapped), the diff you're looking at almost certainly differs from that prior
commit in some real way — find that difference and say it. Two truly identical changes (e.g. a
revert immediately followed by redoing the same commit) are the only case where reusing a subject
is correct; don't force a difference that isn't there.

**Default to Conventional Commits** (`type(scope): subject`) unless the project's recent history
clearly uses something else. Standard types: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`,
`perf`, `build`, `ci`.

**Use a scope whenever the change is localized to one module, subsystem, or feature area** — omit
it only when the change is genuinely repo-wide (e.g. a formatting pass, a broad rename). The scope
names *where* the change lives, not what kind of change it is or which file got touched:

- Good: `feat(guard): add dynamic rm safety policy` — `guard` is the subsystem
- Good: `fix(seo): emit correct canonical URL for project pages` — `seo` is the feature area
- Good: `perf(telemetry): disable session recording and autocapture`
- Bad: `feat(feat): add rm safety policy` — scope restates the type, says nothing new
- Bad: `fix(fix): correct canonical URL` — same problem
- Bad: `chore(master): release 0.2.0` — `master` is a branch, not a module; scope should describe
  the subsystem being released, or be omitted
- Bad: `feat(app): add rm safety policy` / `feat(misc): ...` / `feat(stuff): ...` — too generic to
  narrow anything down; if every commit in the project could use this scope, it isn't a scope
- Bad: `feat(guard.rs): add rm safety policy` — a filename isn't a scope; use the module/feature it
  belongs to (`guard`), not the file that happens to hold it

**Copy the project's type/scope convention, not its message length or prose style.** If recent
commits have long subjects or bullet-heavy bodies, do not imitate them.

**Subject line:** one change, imperative verb first, under 72 chars (aim for 50). Use the names a
grep would find (the type, function, flag or command), not an abstract paraphrase. A comma or "and"
listing changes means the extras belong in the body or nowhere.

- Bad: `fix(ui): name a missing NIC plainly, log it over the link, one queued quit, no empty shapes`
- Good: `fix(ui): clean up the no-NIC status display`
- Bad: `feat(cache): ask whether a chunk is held without a runtime`
- Good: `feat(cache): add ChunkStore::contains_blocking`

**Body: scale it to the change.** Messages should vary widely in size across commits.

- Small change, or one the subject fully explains (most commits, including big mechanical renames
  or moves): subject alone.
- Reason or cause not obvious from the diff: one to three plain sentences on the why. No bullets.
- Large change with genuinely separate parts: a short paragraph, then two to four short bullets.
  Rare, not the default for a `feat`.

Never in the body: a diff walkthrough, "X now does Y" restatements, constants, thresholds or
timings, one bullet per file or module. One headline number is fine when it is the point. Plain
ASCII, wrapped at 72 columns.

- **NEVER mention:**
  - Test results, coverage percentages, or "all tests pass"
  - Lockfile hash changes or dependency graph updates
  - Number of files changed (we can see that in git)
  - Build success or warnings

## Custom instructions

$ARGUMENTS

## Important notes

- You should only use 'git commit' and create a single commit
- If in plan mode, proceed with the commit anyway - command execution is implied
- Do not stage any additional files
- Execute the commit using the Bash tool with a single git commit command
- Do not use any other tools besides Bash
- Do not send any other text or messages - only invoke the Bash tool to execute the commit
