---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git show:*), Bash(git commit:*)
description: Amend the most recent commit (with staged changes and/or message reword)
---

## Context

**Current staged changes:**
!`git diff --cached --stat`

**Files in most recent commit:**
!`git show --stat --pretty=format: HEAD | grep -v '^$'`

**Recent commit history (for style reference):**
!`git log --oneline -5`

## Your task

Amend the most recent commit using `git commit --amend -m "your new message"`.

**CRITICAL: You MUST write a new commit message. DO NOT use --no-edit.**

**Process:**

1. Analyze what files are changing:
   - If staged changes exist: combined old commit files + new staged files
   - If no staged changes: just the files from the original commit
   
2. Write an appropriate commit message that describes ALL the changes (both original and newly staged)
   - Follow the commit style from recent history
   - Scale complexity to the changes (simple renames = short message, complex features = detailed message)
   
3. Execute: `git commit --amend -m "your new message"`

**Important:**

- NEVER use `--no-edit` - always write a fresh commit message
- DO NOT fetch the old commit message - it's irrelevant
- The message should describe what the commit does NOW (after amendment), not what it did before
- If in plan mode, proceed anyway - command execution is implied
- Use a single bash command: `git commit --amend -m "message"`
- Do not stage additional files beyond what is already staged
