---
name: interview
description: "Discovery, design, and brainstorming before implementation. Auto-activate before creative work (features, components, behavior changes), vague/underspecified requests, or when the user says 'interview me', 'help me figure out', 'brainstorm'."
---

# Interview & Design

## Purpose

Help the user discover what they need, refine it into a design, and hand off to implementation — all through progressive questioning. Replaces separate brainstorming and planning document workflows.

## When to Activate

**Explicit triggers:**
- User invokes this skill directly
- User says "interview me", "help me figure out", "brainstorm", "what should I do about", "I'm not sure what I want"

**Auto-activate before creative work:**
- Creating features, building components, adding functionality, or modifying behavior
- Complex tasks with many possible approaches
- User expresses uncertainty or asks open-ended questions
- Vague or underspecified requests

**Skip for:**
- Completely unambiguous single-step tasks
- Mechanical operations (rename, typo fix, simple addition)
- One sanity check is enough: "Anything else to consider before I proceed?"

## Phase 1: Discovery

### Style Selection

Choose or ask about the appropriate style based on context:

| Style | When to Use |
|-------|-------------|
| **Socratic** | User needs to discover what they really need; guide through questions |
| **Requirements gathering** | User knows roughly what they want; need structured completeness |
| **Devil's advocate** | User has a plan but wants it stress-tested; challenge assumptions |
| **Collaborative brainstorm** | User wants to explore possibilities; build on ideas together |

Creative work defaults to **collaborative brainstorm**. Vague requests default to **Socratic**. If unclear, ask.

### Question Structure

- Use the Question tool for all structured questions with options/choices
- Ask **2-4 related questions** per round to reduce back-and-forth
- **Prioritize alternatives and options** — always offer choices where possible
- For open-ended topics, provide **directional choices** rather than precise options
- Use multiple-select when several options could apply simultaneously

### Handling Vague Input

When the user says "something feels wrong" or can't articulate the problem:

1. **Start with feelings:** Ask about symptoms, what triggered the concern, recent changes
2. **Light analysis:** Scan relevant code for common issues (complexity, duplication, unclear naming, etc.)
3. **Guided exploration:** Walk through areas together, ask "what about this?" style questions

### Research Phase

Before or during questioning:
- **Light scan** of relevant files to ground questions in reality
- Don't over-research upfront; let questions reveal what needs investigation
- Use codebase findings to inform follow-up questions

### Progress Tracking

After each round of questions, print a **running summary**:

```markdown
## Current Understanding

**Decided:**
- [Confirmed decisions and constraints]
- [Agreed-upon scope boundaries]

**Still exploring:**
- [Open questions]
- [Topics needing more discussion]
- [Unresolved tradeoffs]
```

Update this summary as understanding evolves.

### Scope Management

At natural boundaries (every 2-3 question rounds, or when a subtopic concludes):
- "Should we **expand** (add related concerns), **narrow** (focus on what we have), or **continue** as-is?"

When the user mentions tangential topics:
- Acknowledge the topic
- Ask: "Do you want to incorporate this into our current scope, or note it for later?"

When a user's response contradicts the current direction:
1. Identify whether the contradiction is incompatible or could be integrated
2. Confirm: "It sounds like you're shifting toward X. Should we pivot fully, or try to integrate both X and Y?"
3. Update the summary to reflect the new direction

## Phase 2: Design Presentation

When discovery converges (user signals readiness, or questions are exhausted):

1. **Present a concise design summary** — architecture, approach, key decisions, components, data flow. Keep it brief.

2. **Ask leading concern questions** using the Question tool (multi-select):
   - Identify 2-4 potential concerns or areas that might need refinement
   - Frame as "Do any of these concern you?" with specific, grounded options
   - For larger designs, use multiple questions covering different aspects

3. **If the user selects any concerns** → return to discovery (Phase 1) to resolve them, then re-present the updated design. Loop until no concerns remain.

4. **If no concerns** → move to Phase 3.

## Phase 3: Next-Step Handoff

Once the design is validated, offer the next step:

**First choice (single-select):**
- **Implement directly** — For simple/clear tasks where the conversation is the spec
- **Enter plan mode** — Use Claude Code's native plan mode to formalize into actionable steps before coding
- **Nothing — conversation is the artifact** — The interview captured everything needed

**After "implement directly" is chosen**, recommend one of:
- **Subagent-driven execution** — When there are 3+ independent tasks that can be dispatched to subagents. Recommend this and explain why when applicable.
- **Direct implementation** — When tasks are tightly coupled or few. Recommend this and explain why when applicable.

Provide a concrete reason for your recommendation (e.g., "These 4 tasks are independent and touch different files — subagent-driven will be faster").

**After "enter plan mode" is chosen:**
- Enter plan mode and begin planning actionable steps
- During planning, ask the user whether to use subagent-driven or direct implementation (recommend one with reasoning)

## Domain-Specific Guidance

### New Features

Focus questions on:
- **Requirements completeness:** What must it do? What must it NOT do?
- **Edge cases:** What happens when X? What if Y is empty/null/huge?
- **Integration:** How does this connect to existing code? What might break?
- **User impact:** Who uses this? How will they discover/learn it?

### Refactoring

Focus questions on:
- **Scope boundaries:** What's in scope? What should we explicitly leave alone?
- **Risk assessment:** What could break? How do we verify correctness?
- **Testing strategy:** Do tests exist? Do we need new ones first?
- **Approach:** Incremental small changes, or coordinated big-bang?

### Architecture Decisions

Focus questions on:
- **Tradeoffs:** What are we optimizing for? What are we willing to sacrifice?
- **Constraints:** Technical limitations? Team preferences? Timeline?
- **Future implications:** How will this age? What becomes harder/easier?
- **Reversibility:** Can we undo this if it's wrong? What's the blast radius?

## Key Principles

- **Multiple choice preferred** — Easier to answer than open-ended when possible
- **YAGNI ruthlessly** — Remove unnecessary features from all designs
- **Explore alternatives** — Always propose 2-3 approaches before settling
- **Be flexible** — Go back and clarify when something doesn't make sense
- **Don't over-question** — Watch for completion cues: "let's do it", "sounds good", "go ahead"
