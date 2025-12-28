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

**Default style - keep it minimal:**

- **Single line** for most commits (under 72 chars)
- **Two lines max** for changes that need brief elaboration
- **Bullet points** (2-4 max) ONLY for large, complex feature additions
- Focus on WHAT changed and WHY, not implementation details
- **NEVER mention:**
  - Test results, coverage percentages, or "all tests pass"
  - Lockfile hash changes or dependency graph updates
  - Number of files changed (we can see that in git)
  - Build success or warnings

**Mechanical changes deserve minimal messages:**
- Package updates: "update [package] to vX.Y.Z" (don't describe lockfile changes)
- Renames/moves: "rename X to Y" or "move X to Y"
- Formatting: "format [files/area]" or "apply prettier"
- Simple fixes: "fix [issue]" or "correct [thing]"

**Complex changes can have more detail:**
- New features: brief description + why it's needed
- Refactors: what changed architecturally + motivation
- Bug fixes: what was broken + how it's fixed (if non-obvious)

## Custom instructions

$ARGUMENTS

## Important notes

- You should only use 'git commit' and create a single commit
- If in plan mode, proceed with the commit anyway - command execution is implied
- Do not stage any additional files
- Create the commit using a single bash command
- Do not use any other tools or do anything else
- Do not send any other text or messages besides the git commit command
