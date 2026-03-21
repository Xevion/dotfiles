---
name: linear-project
description: Scaffold new Linear projects with label taxonomies, milestones, and CLAUDE.md integration. Use when starting a new project, setting up Linear tracking for an existing repo, or auditing/reorganizing project structure.
---

# Linear Project Scaffolding

## Overview

Create fully configured Linear projects with domain-specific labels, optional milestones, and a CLAUDE.md integration section. Audits existing workspace state to prevent duplication.

## CRITICAL: Use the Question Tool for ALL Decisions

**Every decision point in this skill MUST use the Question tool.** Do not present options as plain text, bullet lists, or inline prose. If you're asking the user to choose, confirm, or weigh in — use the Question tool. Always.

**This includes:**
- Whether to create a new project or update an existing one
- Project name, priority, and scope
- Which existing labels to reuse vs which new labels to create
- Label colors and descriptions
- Whether to add milestones and what they should be
- Final confirmation before executing

**Default to `multiSelect: true`** unless options are mutually exclusive. Label selection, milestone inclusion, and scope questions almost always allow multiple selections.

**Batch related questions** into a single Question tool call (up to 4 questions). Do NOT ask one question, wait, then ask another — batch them.

## Workspace Context

- Issue prefixes are **team-level** in Linear's data model. `XEV-` is the key for the "Xevion's Personal" team. Each workspace typically has one team, so the prefix is effectively per-workspace — all projects within the team share it.
- `MOT-` is a different workspace with its own team (configured as a project-level MCP server), not a project under XEV-.
- The team name (e.g., "Xevion's Personal") may match the workspace name. Don't confuse them — `list_teams` returns teams, not workspaces.
- When the Linear MCP is configured at the project level, it connects to a specific workspace. Don't assume all projects use the same workspace.

## Step 1: Reuse Audit

**Before creating anything, understand what already exists.**

1. Query `list_projects` — get existing projects with their status and priority
2. Query `list_issue_labels` — catalog all workspace and team labels
3. Query `list_issue_statuses` for the team — confirm the status workflow

Present a brief summary of what exists, then use the Question tool (`multiSelect: false`):

- "Create new project" (Recommended) — with the proposed name in the description
- "Update existing project: [name]" — if an existing project closely matches what the user described
- "This belongs in [existing project] — add labels/milestones instead" — if the request sounds like extending, not creating
- "Cancel"

## Step 2: Project Interview

**Batch questions into 1-2 Question tool calls.** Query Linear data first, then ask informed questions.

### Round 1 — Identity and Priority (batch into one Question tool call)

Question 1: **Project name** (`multiSelect: false`)
- Suggest based on repo name or user's description — put it first with "(Recommended)"
- Include 1-2 alternatives if the name is ambiguous
- Include "Other" implicitly (the Question tool always provides this)

Question 2: **Priority** (`multiSelect: false`)
- Options: Urgent, High, Medium (Recommended), Low
- Put reasoning in descriptions: "Medium — new personal project, no external deadlines or dependencies"

Question 3: **Project scope / architecture** (`multiSelect: true`)
- Present component types the project might have based on the user's description:
  - "Backend API" — with tech stack guess in description
  - "Web Frontend" — with framework guess
  - "CLI tool"
  - "Library/SDK"
  - "Discord/Slack bot"
  - "Minecraft mod"
  - etc. — tailor to what the user described
- This informs the description template AND the domain labels

Question 4: **Start date** (`multiSelect: false`)
- "Today" (Recommended)
- "Custom date"
- "No start date"

### Round 2 — Labels and Milestones (batch into one Question tool call)

**Before asking, cross-reference the reuse audit results.**

Question 1: **Reuse existing labels** (`multiSelect: true`)
- Present existing workspace labels that are relevant to this project type
- Group by category in descriptions: "Type label — workspace-wide", "Domain label — already exists"
- Type labels (Bug, Feature, Improvement, Refactoring) are workspace-wide — always suggest reusing, never recreating

Question 2: **New domain labels to create** (`multiSelect: true`)
- Suggest project-specific labels based on the components selected in Round 1
- Each option includes: name, suggested color (that doesn't clash), and one-line description
- Example: "Capture (#E91E63) — Screenshot capture system and orchestration"
- Only suggest labels that are genuinely project-specific and don't already exist

Question 3: **Milestones** (`multiSelect: true`)
- "No milestones for now" — first option if the project is early-stage
- Suggest 1-2 obvious phase milestones based on the project scope (e.g., "MVP", "Alpha", "v1.0")
- Keep it lean: 1-2 for new projects, 3 max
- Include target date suggestions in descriptions if reasonable
- **Name milestones with short descriptive names only** — e.g., "Ingest MVP", "Alpha", "v1.0". Do NOT prefix with version numbers or use separators like "v0.1 -- Ingest MVP". The milestone name IS the label.

### Description Generation

After both rounds, generate the project description following the established pattern. **Do not ask the user to write the description** — draft it from their answers:

```markdown
# [Project Name]

[1-2 sentence elevator pitch derived from their scope answers]

## Architecture

* **[Component 1]** ([tech stack]) — [role]
* **[Component 2]** ([tech stack]) — [role]

## Technology Stack

* **Backend**: [languages, frameworks]
* **Frontend**: [languages, frameworks]
* **Database**: [storage systems]
* **Deploy**: [infrastructure]
```

Don't force all sections — only include what's relevant. A single-component project doesn't need an Architecture section with one bullet. **Omit any section that would be empty or generic** — no "Dependencies: None" or "Risks: N/A".

## Step 3: Preview and Confirm

Show the full plan as text:

```
PROJECT
  Name: [name]
  Summary: [summary]
  Priority: [priority]
  Lead: Xevion
  Team: Xevion's Personal
  Start date: [today or specified]

LABELS TO CREATE (new)
  - [Label 1] (#color) — [description]
  - [Label 2] (#color) — [description]

LABELS TO REUSE (existing)
  - Bug, Feature, Improvement (workspace-wide)
  - [any other existing labels being reused]

MILESTONES
  - [Milestone 1] — [target date or "no date"]
  - (or "None")

CLAUDE.MD SECTION
  [preview of the generated section]

DESCRIPTION
  [full markdown description]
```

Then **immediately use the Question tool** (`multiSelect: false`) — do NOT present this as plain text options:
- "Create everything as shown" (Recommended)
- "Edit before creating" — let the user specify what to change
- "Cancel"

**If you skip the Question tool here and just ask in prose, you are violating this skill's core requirement.**

## Step 4: Execute

Execute in order:

1. **Create project** via `save_project` — name, summary, description, priority, lead, team, start date
2. **Create new labels** via `create_issue_label` — only the genuinely new ones
3. **Create milestones** via `save_milestone` — if any were defined

Report results with URLs after each step.

## Step 5: Write CLAUDE.md Section

Write a concise Linear integration section directly into the project's CLAUDE.md. **Do NOT present it as text for the user to paste — use the Edit tool to add it to the file immediately.**

### Finding the target file

1. Check if a `CLAUDE.md` exists in the current working directory
2. If yes, append the section to it (find an appropriate location — after existing content, or before a specific section if logical)
3. If no `CLAUDE.md` exists, ask the user where to put it using the Question tool

### Section template

```markdown
## Linear Issue Tracking

[Project name] tracks work in [Linear](https://linear.app/xevion-personal/) under the **[Project name]** project.

- **Team:** `Xevion's Personal`
- **Project:** `[Project name]` — filter by this when querying issues
- **Issue prefix:** `XEV-` (team-level — all projects under this team share it)

### Labels

**Domain:** [list domain labels relevant to this project]
**Type:** Bug, Feature, Improvement[, Refactoring if relevant]

### Working with Issues

Use the `linear-issue` skill for creating issues, or reference issues directly (e.g., "work on XEV-###").

**Always move issues to "In Progress" before writing code. Do NOT mark "Done" until confirmed.**
```

**Key principles for this section:**
- Keep it concise — reference the `linear-issue` skill for detailed workflows instead of duplicating instructions
- Include project-specific details (domain labels, project name) that the skill can't auto-detect
- Don't include full MCP call syntax — the skill handles that

## Label Color Guidelines

When creating new labels, **query existing labels first** with `list_issue_labels` to see what colors are already in use. Then:

- Pick colors that are visually distinct from existing labels
- Avoid reusing the exact hex of an existing label for a different domain
- Stick to saturated, easily distinguishable colors — avoid pastels that blend together
- When in doubt, include color as an option in the Question tool

## When NOT to Create a New Project

Sometimes the user wants to track work that belongs in an existing project:
- New subsystem within an existing project → add labels, not a new project
- Temporary spike or investigation → create issues in the relevant project
- Cross-cutting concern → may belong in multiple projects as individual issues

If the request sounds like it fits an existing project, surface this in the Step 1 Question tool call — present "extend existing project" as an option alongside "create new".
