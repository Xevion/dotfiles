---
name: linear-project
description: Scaffold new Linear projects with label taxonomies, milestones, and CLAUDE.md integration. Use when starting a new project, setting up Linear tracking for an existing repo, or auditing/reorganizing project structure.
---

# Linear Project Scaffolding

## Overview

Create fully configured Linear projects with domain-specific labels, optional milestones, and a CLAUDE.md integration section. Audits existing workspace state to prevent duplication.

## Step 1: Reuse Audit

**Before creating anything, understand what already exists.**

1. Query `list_projects` — show existing projects with their status and priority
2. Query `list_issue_labels` — catalog all workspace and team labels
3. Query `list_issue_statuses` for the team — confirm the status workflow

Present a summary:
```
Existing projects: [list with statuses]
Workspace labels: [grouped by type vs domain]
Status workflow: Backlog → Todo → In Progress → Done (+Canceled, Duplicate)
```

**If an existing project already matches what the user wants**, say so and offer to update it instead of creating a new one.

## Step 2: Project Interview

Use the Question tool to gather project details. Ask 2-3 rounds:

### Round 1: Identity

- **Project name** — suggest based on repo name or user's description
- **Summary** — one sentence, max 255 chars (Linear's limit)
- **Priority** — suggest Medium unless the user indicates urgency

### Round 2: Architecture and Description

Guide the user through a structured project description following the established pattern:

```markdown
# [Project Name]

[1-2 sentence elevator pitch]

## Architecture

* **[Component 1]** ([tech stack]) — [role]
* **[Component 2]** ([tech stack]) — [role]

## Technology Stack

* **Backend**: [languages, frameworks]
* **Frontend**: [languages, frameworks]
* **Database**: [storage systems]
* **Deploy**: [infrastructure]
```

Don't force all sections — only include what's relevant. A single-component project doesn't need an Architecture section with one bullet.

### Round 3: Labels and Milestones

**Domain labels:**
- Ask what the project's major subsystems or concern areas are
- Cross-reference with existing workspace labels — reuse where applicable
- For new labels, ask about:
  - Name (short, noun-based: "Frontend", "Capture", "Pathfinding")
  - Color (suggest one that doesn't clash with existing labels)
  - Description (one sentence explaining scope)

**Highlight reusable labels:**
- Type labels (Bug, Feature, Improvement, Refactoring) are workspace-wide — never recreate
- Common domain labels (Frontend, Backend, Database, Infrastructure, Testing, Documentation, Security) may already exist — suggest reusing
- Only create labels that are genuinely project-specific

**Milestones (optional):**
- Ask if the user wants to define initial milestones
- Keep it lean: 1-2 for new projects, 3 max
- Each milestone needs: name, description (optional), target date (optional)
- If the user doesn't have clear phases, skip milestones entirely

## Step 3: Preview Everything

Show the full plan before executing:

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

**Wait for explicit confirmation before creating anything.**

## Step 4: Execute

Execute in order:

1. **Create project** via `save_project` — name, summary, description, priority, lead, team, start date
2. **Create new labels** via `create_issue_label` — only the genuinely new ones
3. **Create milestones** via `save_milestone` — if any were defined

Report results with URLs after each step.

## Step 5: Generate CLAUDE.md Section

Output a concise Linear integration section for the project's CLAUDE.md. Follow this template:

```markdown
## Linear Issue Tracking

[Project name] tracks work in [Linear](https://linear.app/xevion-personal/) under the **[Project name]** project.

- **Team:** `Xevion's Personal`
- **Project:** `[Project name]` — filter by this when querying issues
- **Issue prefix:** `XEV-` (e.g. `XEV-###`)

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
- The section should be copy-pasteable into the project's CLAUDE.md

**Present the section to the user** — don't write it to a file automatically. The user decides where and how to integrate it.

## Label Color Guidelines

When creating new labels, avoid clashing with existing colors. Use this as a reference:

- Red spectrum: Bug (#EB5757), Combat (#F44336), Security (#FF0000)
- Orange spectrum: Error Handling (#FF9800), Coordination (#FF9800), Refactoring (#FFA500)
- Yellow: Performance (#F59E0B)
- Green spectrum: Movement (#4CAF50), SEO (#10B981)
- Blue spectrum: Rendering (#2196F3), Feature (#BB87FC), Improvement (#4EA7FC), Docker (#2496ED)
- Purple spectrum: Pathfinding (#9C27B0), Analytics (#8B5CF6), Bot AI (#9C27B0)
- Neutral: Infrastructure (#6B7280), Exploits (#9E9E9E), Documentation (#95A2B3), Configuration (#795548)

Pick colors that are visually distinct from existing labels in the workspace. When in doubt, ask the user.

## When NOT to Create a New Project

Sometimes the user wants to track work that belongs in an existing project:
- New subsystem within an existing project → add labels, not a new project
- Temporary spike or investigation → create issues in the relevant project
- Cross-cutting concern → may belong in multiple projects as individual issues

If the request sounds like it fits an existing project, say so and ask before proceeding with scaffolding.
