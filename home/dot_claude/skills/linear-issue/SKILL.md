---
name: linear-issue
description: Create Linear issues with smart project detection, contextual detail, and label suggestions. Use when creating issues, filing bugs, tracking features, or decomposing work into sub-issues.
---

# Linear Issue Creation

## Overview

Create well-structured Linear issues with automatic project detection, contextual detail scaling, label suggestions, and priority reasoning. Supports single issues, sub-issues, and relation linking.

**Always preview before creating. Never fire-and-forget.**

## Step 1: Detect Project Context

Check for Linear configuration in order:

1. **CLAUDE.md in current project** — look for a `## Linear Issue Tracking` section containing project name, team, domain labels
2. **If not found** — list existing Linear projects using `list_projects` and present them:
   - Show project names with summaries
   - Include a "No project (standalone)" option
   - Include a "Create new project" option (suggest the `linear-project` skill)
   - If the git repo name or directory name closely matches a project, highlight that suggestion

**Cache the detected project for the session** — don't re-detect on every issue.

## Step 2: Gather Issue Details

Use the Question tool to interview the user. Scale the interview depth to the task:

### Quick mode (user provides a clear, specific request)

If the user says something like "create a bug: X is broken" or "track this feature: Y", skip to drafting. Extract:
- Title from their description
- Type (Bug/Feature/Improvement) from context
- Description from conversation context

### Standard mode (needs refinement)

Ask 2-3 questions using the Question tool. Focus on:

**What's the issue?**
- Get a specific, scannable title (~10 words max)
- Understand the problem or desired behavior

**What type and priority?**
- Suggest a type label (Bug/Feature/Improvement/Refactoring) based on description
- Suggest priority with natural language reasoning:
  - "This feels Medium — it's an improvement that isn't blocking anything, but it touches a core system."
  - "I'd suggest High — this is a data integrity bug that could silently corrupt state."
  - Present an alternative: "Could argue Low if you want to batch this with the other cleanup items."
- Let the user confirm or adjust

**Labels and relations?**
- Query existing labels for the target project with `list_issue_labels`
- Auto-suggest 1-3 labels: one type label + relevant domain labels
- If the user mentions related issues, offer to set `blockedBy`, `blocks`, or `relatedTo`
- If this is a sub-task of a larger effort, offer to set `parentId`

### Batch mode (decomposing a large feature)

When the user describes a large feature that naturally breaks into pieces:
1. Draft a parent issue (overview + scope)
2. Draft 2-5 sub-issues with individual titles, descriptions, and labels
3. Show the full tree for review before creating anything
4. Create parent first, then sub-issues with `parentId` set

## Step 3: Draft the Issue

### Description Detail Scaling

Scale the description structure to the issue type:

**Bug:**
```markdown
## Problem

[What's broken — observed behavior]

## Expected

[What should happen instead]

## Context

[How it was discovered, reproduction hints if known]
```

**Feature:**
```markdown
## Overview

[What this adds and why]

## Scope

[What's included, key decisions, constraints]
```

**Improvement:**
A focused paragraph or two. No section headers needed unless the scope is large.

**Spike / Investigation:**
```markdown
## Question

[What we're trying to answer]

## Investigation Scope

[Where to look, what to evaluate, time-box if applicable]
```

### Description Rules

- **No boilerplate sections.** If a section would be empty or "N/A", omit it entirely.
- **No time estimates** — no story points, hours, t-shirt sizes.
- **No acceptance criteria checklists** — unless the issue is genuinely ambiguous and benefits from explicit success criteria.
- **User story format** — only for exploratory features where "As a [user], I want [thing] so that [reason]" genuinely clarifies intent. Never for bugs, internal tooling, or specific implementation tasks.
- **No file:line references** — only include file paths when the file is central to the issue and unlikely to be renamed/moved. Never include line numbers or column numbers.
- **Don't re-summarize the conversation.** Extract the actionable parts. If the conversation already established context, reference it concisely.
- **Match the user's language and specificity.** If they described the problem technically, keep it technical. Don't dilute with generic language.

### Title Rules

- Specific and scannable, ~10 words max
- No ticket-speak prefixes (`[BUG]`, `[FEAT]`, `TASK:`) — labels handle categorization
- No vague titles ("Improve performance", "Fix auth bug") — name the specific problem
- Mix of imperative ("Fix X") and descriptive ("X fails when Y") is fine — match what feels natural

## Step 4: Preview and Confirm

**Always show the full issue before creating it:**

```
Title: [title]
Project: [project name]
Type: [Bug/Feature/Improvement]
Priority: [priority with brief reasoning]
Labels: [label1, label2]
Assignee: [me / unassigned]
Parent: [XEV-### if applicable]
Relations: [blocks/blockedBy/relatedTo if any]

---
[Full description markdown]
```

**Assignment logic:**
- Status Todo or In Progress → assign to "me"
- Status Backlog → leave unassigned (it's an idea, not a commitment)
- Default status: Backlog for new issues unless the user indicates they're starting work

**Wait for explicit confirmation before calling `save_issue`.**

## Step 5: Create

Call `save_issue` with all fields. After creation:
- Report the issue ID and URL
- If sub-issues were planned, create them sequentially with `parentId` set to the parent
- If relations were specified, include `blockedBy`/`blocks`/`relatedTo` arrays

## Label Guidelines

- **1-3 labels** is the sweet spot. More is noise.
- **Always include a type label** (Bug, Feature, Improvement, Refactoring) — it's the primary categorization
- **Domain labels are contextual** — only add if the issue clearly belongs to a subsystem. "Frontend" on a purely backend issue is wrong even if the project has the label.
- **Don't create new labels** without asking. Suggest creating if nothing fits, but default to using existing labels.

## Priority Guidelines

Never default to High. The scale should feel meaningful:

| Priority | When to suggest |
|----------|----------------|
| **Urgent** | Production is broken, data loss risk, security vulnerability |
| **High** | Blocks other work, core functionality broken, user-facing regression |
| **Medium** | Important improvement, non-blocking bug, planned feature work |
| **Low** | Nice-to-have, cleanup, investigation spikes, future considerations |

**Always explain your reasoning** in 1-2 sentences. Present the suggested priority AND one alternative with its reasoning. Let the user pick what resonates.
