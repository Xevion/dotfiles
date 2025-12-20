---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git commit:*)
description: Commit currently staged changes with an appropriate message
---

## Context

- Current git status:
!`git status`
- Current git diff line count: !`git diff --cached | wc -l`
- Current git diff (staged changes only):
!`if [ $(git diff --cached | wc -l) -lt 200 ]; then git diff --cached; else git diff --cached --stat; fi`
- Recent commits:
!`git log --oneline -10`

## Your task

Based on the above staged changes, create a single git commit.

**Important notes:**

- You should only use 'git commit' and create a single commit.
- If in plan mode, proceed with the commit anyway - command execution and file modification is implied
- Scale commit message complexity appropriately:
  - Mechanical/wide commits (renames, formatting, etc.) deserve only a single sentence, even if they touch many files
  - Complex feature additions or refactors deserve more detailed messages explaining the reasoning
- Do not stage any additional files
- Create the commit using a single message with parallel tool calls
- Do not use any other tools or do anything else
- Do not send any other text or messages besides these tool calls
- Include git status output in your response if not already available in the context
