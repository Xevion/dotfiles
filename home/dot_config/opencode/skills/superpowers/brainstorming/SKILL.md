---
name: brainstorming
description: "You MUST use this before any creative work - creating features, building components, adding functionality, or modifying behavior. Explores user intent, requirements and design before implementation."
---

# Brainstorming Ideas Into Designs

## Overview

Help turn ideas into fully formed designs and specs through natural collaborative dialogue.

Start by understanding the current project context, then ask questions one at a time to refine the idea. Once you understand what you're building, present the design in small sections (200-300 words), checking after each section whether it looks right so far.

## The Process

**Understanding the idea:**
- Check out the current project state first (files, docs, recent commits)
- Ask questions one at a time to refine the idea
- Prefer multiple choice questions when possible, but open-ended is fine too
- Only one question per message - if a topic needs more exploration, break it into multiple questions
- Focus on understanding: purpose, constraints, success criteria

**Exploring approaches:**
- Propose 2-3 different approaches with trade-offs
- Present options conversationally with your recommendation and reasoning
- Lead with your recommended option and explain why

**Presenting the design:**
- Once you believe you understand what you're building, present the design
- Break it into sections of 200-300 words
- Ask after each section whether it looks right so far
- Cover: architecture, components, data flow, error handling, testing
- Be ready to go back and clarify if something doesn't make sense

## After the Design

Once the design is validated, **ask the user what they want to do next**. Do NOT assume they want documentation files, implementation plans, or git operations.

**Present options like:**
- Continue straight to implementation
- Write a combined design + implementation plan document
- Write just a design summary document
- Nothing — the conversation itself is the artifact
- Something else (let them say)

**If the user wants a document:**
- Write ONE combined document unless they specifically ask for separate files
- Ask where they want it saved (suggest `docs/plans/YYYY-MM-DD-<topic>.md` as a default)
- Use elements-of-style:writing-clearly-and-concisely skill if available
- Do NOT commit to git unless the user explicitly asks

**If the user wants to continue to implementation:**
- superpowers:writing-plans and superpowers:using-git-worktrees are available if the user wants a formal plan or isolated workspace
- But don't assume they're needed — ask first

## Key Principles

- **One question at a time** - Don't overwhelm with multiple questions
- **Multiple choice preferred** - Easier to answer than open-ended when possible
- **YAGNI ruthlessly** - Remove unnecessary features from all designs
- **Explore alternatives** - Always propose 2-3 approaches before settling
- **Incremental validation** - Present design in sections, validate each
- **Be flexible** - Go back and clarify when something doesn't make sense
