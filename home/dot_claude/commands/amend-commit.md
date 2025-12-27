---
allowed-tools: Bash(git commit:*)
argument-hint: [optional custom instructions]
description: Amend the most recent commit (with staged changes and/or message reword)
---

## Context

!`commit-helper --amend`

## Your task

Amend the most recent commit using `git commit --amend -m "your new message"`.

**CRITICAL: You MUST write a new commit message. DO NOT use --no-edit.**

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

## Process

1. Analyze what files are changing:
   - If staged changes exist: combined old commit files + new staged files
   - If no staged changes: just the files from the original commit
   
2. Write an appropriate commit message that describes ALL the changes (both original and newly staged)
   - Follow the commit style from recent history
   - Follow the style requirements above
   
3. Execute: `git commit --amend -m "your new message"`

## Important notes

- NEVER use `--no-edit` - always write a fresh commit message
- DO NOT fetch the old commit message - it's irrelevant
- The message should describe what the commit does NOW (after amendment), not what it did before
- If in plan mode, proceed anyway - command execution is implied
- Use a single bash command: `git commit --amend -m "message"`
- Do not stage additional files beyond what is already staged
