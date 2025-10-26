---
allowed-tools: Bash(git status:*), Bash(git diff:*), Bash(git log:*), Bash(git commit:*)
description: Create a git commit (staged files only)
---

## Context

- Current git status: !`git status`
- Current git diff (staged changes only): !`git diff --cached`
- Current branch: !`git branch --show-current`
- Recent commits: !`git log --oneline -10`

## Your task

Based on the above staged changes, create a single git commit.

You have the capability to call multiple tools in a single response. Create the commit using a single message. Do not stage any additional files. Do not use any other tools or do anything else. Do not send any other text or messages besides these tool calls.
