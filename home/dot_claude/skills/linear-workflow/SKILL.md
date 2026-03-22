---
name: linear-workflow
description: Manage Linear issue lifecycle during work sessions — status transitions, comments, issue reference handling, and relevance detection. Auto-activates when issue IDs (XEV-###, MOT-###) appear in user messages.
---

# Linear Workflow Management

## Overview

Manage the lifecycle of Linear issues during active work sessions. This skill handles status transitions with context-aware gate checks, structured comments, issue reference resolution, and prompted relevance detection.

**This skill does NOT create issues** — use `linear-issue` for that. This skill manages issues that already exist.

## Activation

### Semi-automatic

This skill auto-activates based on pattern matching. The key distinction is **action-implying** vs **referential** mentions.

**Auto-activate when the user's message contains:**

1. **Issue ID + action verb** — "mark XEV-123 as done", "update MOT-45", "close XEV-67"
2. **Issue ID + work context** — "I'm working on XEV-123", "starting XEV-45", "finished MOT-12"
3. **Issue ID as the primary subject** — "XEV-123" alone on a line, or "what's the status of XEV-123?"
4. **Action keywords without ID** — "mark the issue as done", "update the Linear issue", "add a comment to the issue" (requires a previously-referenced issue in session)
5. **Relevance queries** — "are there related issues?", "does this affect any Linear issues?", "check Linear"

**Do NOT auto-activate when:**

- Issue ID appears in a **subordinate clause** providing context: "as discussed in XEV-123, we should..." → the subject is the discussion, not the issue
- Issue ID appears in a **commit message or code reference**: "this fixes XEV-123" in a commit → the user is writing a commit, not managing the issue (unless they also ask to update it)
- Issue ID appears in a **question about the codebase**: "what did XEV-123 change?" → this is a git/code question, not a Linear workflow question
- The user is **invoking `linear-issue`** — that skill handles creation; don't intercept

**When in doubt:** If the issue ID is the object of the sentence (something is being done *to* it), activate. If it's a reference or citation (providing background), don't.

**Explicit invocation:** `/linear-workflow`

## CRITICAL: Use the Question Tool for ALL Decisions

**Every decision point in this skill MUST use the Question tool.** Do not present options as plain text. If you're asking the user to choose, confirm, or weigh in — use the Question tool. Always.

**Default to `multiSelect: true`** unless options are mutually exclusive.

## Core Behaviors

### 1. Issue Reference Handling

When an issue ID is mentioned with implied action:

1. **Fetch the issue** via `get_issue` — pull title, status, priority, labels, assignee, description
2. **Present a brief summary** — title, current status, priority, assignee
3. **Offer contextual actions** via the Question tool (`multiSelect: true`):
   - Actions depend on the current status (see Status Transitions below)
   - Always include: "Add comment", "View full description", "No action needed"
   - If the issue is assigned to someone else, note this before offering status changes

### 2. Status Transitions

#### Transition Map

| From | Valid Targets |
|------|--------------|
| Backlog | Todo, Cancelled |
| Todo | In Progress, Backlog, Cancelled |
| In Progress | Done, Todo (deprioritize), Cancelled |
| Done | In Progress (reopen) |
| Cancelled | Backlog (reopen) |

#### Context-Aware Gate Checks

Before completing a status transition, run gate checks appropriate to the **issue type** (inferred from labels or title). Present warnings as soft checks via the Question tool — the user can always override.

**Bug issues — before marking Done:**
- "What caused the bug and what fixed it?" — if no fix description exists in comments or the conversation, warn and suggest adding one
- If the conversation contains the fix context, draft a brief summary and propose it as a comment

**Feature issues — before marking Done:**
- "Was the full scope implemented, or was anything descoped?" — warn if the issue description mentions scope items that weren't addressed in the session
- If descoped, suggest updating the issue description or creating a follow-up issue

**Spike / Investigation issues — before marking Done:**
- "What were the findings?" — spikes should always have a conclusion comment
- Draft a findings summary from the conversation context and propose it

**All issues — before marking In Progress:**
- Assign to "me" if unassigned
- If assigned to someone else, warn before reassigning

**All issues — before marking Cancelled:**
- Always require confirmation with a reason
- Suggest adding a cancellation comment explaining why

#### Transition Flow

1. **Show current state** — status, assignee, labels
2. **Run gate checks** — present warnings if any conditions aren't met
3. **Confirm via Question tool** (`multiSelect: false`):
   - "Move to [status]" (Recommended if gates pass)
   - "Move to [status] + add comment first"
   - "Cancel transition"
4. **Execute** — update status, reassign if needed, add comment if specified

### 3. Comments

#### Writing Comments

When adding a comment to an issue (whether prompted or as part of a transition):

1. **Draft the comment** based on conversation context
2. **Present it to the user** as plain text
3. **Confirm via Question tool** (`multiSelect: true`) — always these exact 4 options:
   - "Post as written"
   - "Skip comment"
   - First amendment suggestion — a specific alternative wording or addition you think improves the comment
   - Second amendment suggestion — a different specific revision (e.g., adding a detail, removing a section, changing tone)

The amendment suggestions must be **concrete and specific** — not "make it shorter" but an actual rewritten version or a specific change like "Add: 'Blocked by missing API endpoint'".

#### Example: Comment Confirmation Flow

Suppose the user fixed a race condition in `fetchGallery` and wants to mark XEV-89 (a bug) as Done.

**Drafted comment presented as plain text:**
> Fixed the race condition in `fetchGallery` — the resize event handler was firing multiple concurrent fetches. Added a debounce guard. Did not address the related thumbnail cache invalidation mentioned in the issue description; that's a separate concern.

**Question tool call (`multiSelect: true`):**
- **"Post as written"** — comment looks accurate and complete
- **"Skip comment"** — mark Done without commenting
- **"Add debounce duration"** — revise to: "...Added a 150ms debounce guard on the resize handler. Did not address..."
- **"Remove thumbnail note"** — revise to: "Fixed the race condition in `fetchGallery` — the resize event handler was firing multiple concurrent fetches. Added a debounce guard."

Each amendment option names the specific change and shows the resulting text, so the user can pick without rewriting anything. Selecting both amendments applies them together. "Post as written" and "Skip" are mutually exclusive with each other and with amendments — if an amendment is selected, the revised version is posted.

#### Comment Style Rules

- **Concise summaries.** What happened, what didn't. What's next, what's blocked.
- **No boilerplate.** No "Update:", "Progress Report:", or header prefixes unless the comment genuinely has multiple sections.
- **No emojis.** Ever.
- **No cheerful filler.** No "All checks passed!", "Looking good!", "Ready to go!". State facts.
- **No imperative suggestions.** No "You can now proceed with..." or "Next steps: ...". The comment is a record of what happened, not instructions to the reader.
- **Mention skipped work.** If something was intentionally skipped or descoped during the session, say so explicitly.
- **Simple markdown.** Bullet lists for multiple items, backticks for code references, bold for emphasis. No headers for short comments.
- **Match the issue's language.** If the issue is technical, the comment should be technical. Don't dilute with generic language.

#### When to Suggest Comments

Proactively suggest adding a comment when:
- A status transition is happening (especially to Done or Cancelled)
- Significant implementation decisions were made during the session
- Work was blocked or descoped
- The user explicitly asks

Do NOT suggest comments for trivial status changes (e.g., Backlog → Todo with no additional context).

### 4. Relevance Detection (Prompted)

When the user asks "are there related issues?" or "does this affect any Linear issues?":

1. **Identify what's being worked on** — gather file paths, feature names, component names from the current session context
2. **Search Linear** via `list_issues` with relevant filters:
   - Search by project (if detected from CLAUDE.md)
   - Filter to open statuses (Backlog, Todo, In Progress)
   - Search issue titles and descriptions for keywords matching the current work
3. **Present matches** with brief context — title, status, why it might be related
4. **Offer actions** via Question tool (`multiSelect: true`):
   - "Update [issue] status"
   - "Add comment to [issue]"
   - "Link [issue] to current work"
   - "None of these are related"

**Do NOT run relevance detection unprompted.** Only when the user explicitly asks or invokes this behavior.

## Project Context Detection

Check for Linear configuration in order:

1. **CLAUDE.md in current project** — look for a `## Linear Issue Tracking` section containing project name, team, and domain labels
2. **Session context** — if the user mentioned a project or issue ID earlier in the conversation
3. **If not found** — ask which project/workspace to use via the Question tool

Cache the detected project for the session.

## Workspace Awareness

- Issue prefixes are team-level: `XEV-` = "Xevion's Personal" team, `MOT-` = separate workspace
- The prefix in an issue ID tells you which MCP connection to use
- Don't assume all issues are in the same workspace — check the prefix

## Multi-Issue Sessions

When the user is working on multiple issues in one session:

- Track which issues are "active" (mentioned + acted upon)
- When suggesting comments or transitions, handle each issue separately
- At natural breakpoints, remind about other active issues: "You also have XEV-45 in progress — any updates?"
- Do NOT batch-update multiple issues without individual confirmation for each

## Error Handling

- If `get_issue` fails (issue not found, wrong workspace), say so clearly and ask for the correct ID
- If a status transition is invalid (e.g., Done → Todo, which isn't in the map), explain which transitions are valid from the current state
- If the user references an issue that's already Done/Cancelled, note the current status before offering reopen options
