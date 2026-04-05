---
name: develop-workflow
description: Develop a proper workflow for a tool, CLI, or repeated process — explore, test, evaluate alternatives, fix pain points, and capture it in the right place
---

# Develop Workflow

## Overview

Don't document assumptions. Don't document how something _should_ work. Document how it _actually_ works after you've proven it.

**Core principle:** A workflow you haven't validated is a hypothesis. A hypothesis written into CLAUDE.md is misinformation.

## When to Use

- User explicitly invokes `/develop-workflow`
- A new tool or CLI is being set up, integrated, or configured
- User asks to "add X to CLAUDE.md", document a workflow, or capture how something works
- You've just figured something out through trial and error and it should be remembered

**Also activate when:**
- The task involves a tool you had to probe to understand
- You notice you're doing something repetitive that could be codified
- The user mentions recurring friction ("I always have to do X before Y")

## The Iron Law

```
NO DOCUMENTATION WITHOUT LIVE TESTING FIRST
```

If you haven't run the commands and seen the output, you cannot write the workflow.

## Phase 0: Ask About the End Goal

**Before doing anything else**, clarify where the workflow artifacts should land. Use the Question tool.

Common destinations:

| Destination | When |
|---|---|
| **Project `CLAUDE.md`** | Workflow is project-specific; future Claude sessions need it |
| **`docs/` file** | Human-readable reference; user wants to find it themselves |
| **Project-level skill** | Reusable procedure for this project, invocable as `/name` |
| **User-level skill** (`~/.claude/skills/`) | Cross-project workflow; reusable everywhere |
| **`Justfile` recipe** | User or AI needs a runnable command, not just documentation |
| **Multiple** | Often CLAUDE.md + Justfile, or docs + CLAUDE.md — ask |

Ask explicitly. Do not assume. Wrong destination = wasted effort.

## Phase 1: Reconnaissance

Before touching the tool:

1. **Identify what already exists**
   - Does the project already have a workflow for this? Check CLAUDE.md, `docs/`, Justfile
   - Has this been partially solved before? Check git log, existing scripts
   - Are there prior notes, failed attempts, TODOs?

2. **Understand the tool's surface area**
   - `<tool> --help` for flags, subcommands, output modes
   - What does the tool produce? (files, stdout, exit codes, JSON, plain text?)
   - What does the tool consume? (config files, env vars, flags?)

3. **Check for external tooling**
   - Is there a more purpose-built CLI that does this better?
   - Are there community wrappers, formatters, or viewers for this output?
   - Search for `<tool> workflow`, `<tool> cli tips`, `<tool> CI integration`
   - Don't build something that already exists. If better tooling exists, prefer it.

Narrate what you found: "The project has no existing workflow for X. The tool produces JSON via `--json` flag. I found two community tools worth evaluating: ..."

## Phase 2: Exploration

**Run the tool. Actually use it.**

1. **Start with the simplest useful invocation** and observe output
   - Note: format, verbosity, exit codes, errors
   - Note what's useful vs. what's noise

2. **Try flags and modes** that seem relevant
   - JSON output modes (usually `--json`, `--format json`, `-json`)
   - Verbose vs. quiet modes
   - Filter/query flags
   - Machine-readable vs. human-readable variants

3. **Find the interesting data**
   - What does a good result look like? A bad result?
   - What fields are actually useful vs. decorative?
   - What do you need to extract, parse, or post-process?

4. **Probe edge cases**
   - Empty results, errors, large outputs
   - Missing files, config, dependencies

Narrate each test: "Running `cargo llvm-cov --json` gives me a 2000-line JSON object. The interesting field is `.data[0].totals.lines.percent`. Let me now check if `--summary-only` reduces that..."

## Phase 3: Evaluation

**Identify friction and decide whether to fix it or find something better.**

1. **List pain points explicitly**
   - Output too verbose? Too noisy?
   - No machine-readable format?
   - Requires multiple steps that could be one?
   - Output requires non-trivial parsing?

2. **For each pain point, decide:**
   - Fix it with flags/options you haven't tried yet
   - Fix it with a small wrapper (jq query, shell alias, Justfile recipe)
   - Replace it with better tooling you found in Phase 1
   - Accept it and document the workaround

3. **Validate the fix**
   - Run the improved version
   - Confirm the pain point is actually gone
   - Don't document pain points as "known issues" unless they're truly unavoidable

Narrate decisions: "The default output is 400 lines of ANSI-colored text. `--json | jq '.data[0].totals'` reduces it to exactly the summary we need. That's the winner."

## Phase 4: End-to-End Validation

**Run the full workflow from scratch as if you know nothing.**

1. **Walk through every step in sequence**
   - Every prerequisite, every command, every flag
   - Note anything that could fail or require setup

2. **Verify it works from a clean state**
   - Does it require a prior step (build, test run, data generation)?
   - Does it fail if that prior step is missing? If so, document it.

3. **Confirm the output is correct and complete**
   - Is the output actually useful for the stated goal?
   - Does it cover the whole workflow, or just a fragment?

If anything fails during this phase, fix it before documenting. Don't document broken workflows.

## Phase 5: Capture

Write the workflow to the destination(s) agreed in Phase 0.

### For CLAUDE.md entries

Structure:
```markdown
## <Tool/Workflow Name>

**When:** <trigger — when does the AI use this?>

**How it works:** <1-3 sentence summary of what the tool does>

**Key commands:**
\`\`\`bash
# Primary workflow
<command with flags>

# Common variations
<variation 1>
<variation 2>
\`\`\`

**Output:** <what the output looks like, what fields matter>

**Gotchas:** <prerequisites, failure modes, non-obvious requirements>
```

### For Justfile recipes

Keep recipes minimal. One recipe per logical operation. Include comments only for non-obvious behavior.

```
# Generate coverage report and print line/branch summary
coverage:
    cargo llvm-cov --json | jq '.data[0].totals | {lines: .lines.percent, branches: .branches.percent}'
```

### For skill files

Follow the structure of existing skills: YAML frontmatter, overview, phases, examples, red flags.

### For docs/

Write for a human who will read it out of context. Include: why, when, prerequisites, commands, example output, troubleshooting.

## What Done Looks Like

A workflow is done when:

- [ ] Commands have been run and output observed (not theorized)
- [ ] Pain points have been identified and resolved (not just noted)
- [ ] The full workflow runs end-to-end without error from a clean state
- [ ] Artifacts are written to the agreed destination(s)
- [ ] Any Justfile recipes have been tested by actually running them
- [ ] Gotchas and prerequisites are documented, not assumed

## Red Flags — Stop and Re-evaluate

- Writing documentation before running any commands
- "This should work" without testing it
- Documenting a command you haven't verified produces the right output
- Noting a pain point as a "known issue" instead of fixing it
- Skipping Phase 3 because "it seems fine"
- Using a workaround when better tooling exists and would take 5 minutes to find
- Writing a Justfile recipe you haven't run

## Common Rationalizations

| Excuse | Reality |
|---|---|
| "I know how this tool works" | Your training data is stale. Run it. |
| "The docs say it works this way" | Docs lie. Run it. |
| "It'll take too long to test" | Untested docs waste more time than testing. |
| "The user just wants the command" | The user wants a command that works. |
| "I'll note the pain point and move on" | Unresolved pain points become permanent. Fix it now. |
| "There's probably no better tool" | You didn't look. Look. |
