---
name: linear-issue
description: Create Linear issues with smart project detection, contextual detail, and label suggestions. Use when creating issues, filing bugs, tracking features, or decomposing work into sub-issues.
---

# Linear Issue Creation

## Overview

Create well-structured Linear issues with automatic project detection, contextual detail scaling, label suggestions, and priority reasoning. Supports single issues, sub-issues, and relation linking.

**Always preview before creating. Never fire-and-forget.**

**Workspace note:** Issue prefixes are **team-level** in Linear's data model. `XEV-` is the key for the "Xevion's Personal" team. Each workspace typically has one team, so the prefix is effectively per-workspace. `MOT-` is a different workspace with its own team. The prefix comes from whichever Linear MCP is configured, not from the project.

## CRITICAL: Use the Question Tool for ALL Decisions

**Every decision point in this skill MUST use the Question tool.** Do not present options as plain text, bullet lists, or inline prose. If you're asking the user to choose, confirm, or weigh in — use the Question tool. Always.

**This includes:**
- Project selection (when not auto-detected)
- Parent/relation linking
- Whether to batch-create sub-issues
- Final confirmation before creating

**Auto-assign without asking:** Type, priority, and labels. Infer these from context and show them in the preview. The user corrects in the preview step if wrong. Only ask when genuinely ambiguous (e.g., issue could be Bug or Improvement and it matters for triage).

**Default to `multiSelect: true`** unless options are mutually exclusive. Relations and scope questions almost always allow multiple selections.

## Step 1: Detect Project Context

Check for Linear configuration in order:

1. **CLAUDE.md in current project** — look for a `## Linear Issue Tracking` section containing project name, team, domain labels
2. **If not found** — use the Question tool to present existing Linear projects:
   - Query `list_projects` first
   - Present project names with summaries as options
   - Include a "No project (standalone)" option
   - Include a "Create new project" option (suggest the `linear-project` skill)
   - If the git repo name or directory name closely matches a project, put that option first with "(Recommended)" in the label

**Cache the detected project for the session** — don't re-detect on every issue.

## Step 2: Gather Issue Details

**Use the Question tool to interview the user.** Batch related questions into a single Question tool call (up to 4 questions). Do NOT ask one question, wait, then ask another — batch them.

### What to ask (batch into 1-2 Question tool calls)

**Auto-assigned fields (no questions — show in preview):**

- **Type** — infer from context: broken behavior → Bug, new capability → Feature, improving existing behavior → Improvement, code restructuring → Refactoring, open question → Spike/Investigation
- **Priority** — use the priority guidelines at the bottom of this skill
- **Labels** — query existing labels with `list_issue_labels` using `limit: 250` BEFORE drafting. Paginate if needed — you must have the **complete** label list. Pick the right type label + any obvious domain labels from the project's CLAUDE.md label list. 1-3 labels total.

**Only ask when needed:**

Question 1 (if applicable): **Relations and hierarchy** (`multiSelect: true`)
- Only include this question if the user mentioned related issues, or the issue sounds like it could be a sub-task
- Options: "Sub-issue of [XEV-###]", "Blocks [XEV-###]", "Blocked by [XEV-###]", "Standalone (no relations)"

Question 2 (if vague): **Scope clarification**
- Only if the user's description is too vague to draft a useful issue
- Reproduction details (for bugs), scope boundaries (for features), decomposition (for large features)

### Batch mode (decomposing a large feature)

When the user describes a large feature that naturally breaks into pieces, use the Question tool to confirm:
1. Present the proposed decomposition as options (`multiSelect: true`): "Create parent + these sub-issues: [list]"
2. Let the user select which sub-issues to include, add new ones, or reject the decomposition
3. Draft the full tree, then show preview before creating anything
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

**Show the full issue, then use the Question tool to confirm:**

Present the preview as text:
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

Then **immediately use the Question tool** (`multiSelect: false`) — do NOT present this as plain text options:
- "Create this issue" (Recommended)
- "Edit before creating" — let the user specify what to change

No "Cancel" option — if the user wants to bail, they'll say so naturally.

**Assignment logic:**
- Status Todo or In Progress → assign to "me"
- Status Backlog → leave unassigned (it's an idea, not a commitment)

**Default status: Todo, not Backlog.** Most issues created during active work sessions represent concrete work the user intends to do. Use Backlog only when the issue is explicitly speculative, low-priority exploratory, or the user says "someday" / "maybe" / "backlog". When in doubt, default to Todo — it's easier to deprioritize than to forget about something stuck in Backlog.

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
- **Match labels by name, not description.** A label named "API" is the right label for API work even if its description says something narrow. The name is the primary signal; the description is context, not a filter.

## Priority Guidelines

Never default to High. The scale should feel meaningful:

| Priority | When to suggest |
|----------|----------------|
| **Urgent** | Production is broken, data loss risk, security vulnerability |
| **High** | Blocks other work, core functionality broken, user-facing regression |
| **Medium** | Important improvement, non-blocking bug, planned feature work |
| **Low** | Nice-to-have, cleanup, investigation spikes, future considerations |

Assign automatically based on these guidelines. Show your choice in the preview — the user will correct if wrong.
