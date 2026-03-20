---
description: "Session triage — auto-activates on first message to calibrate approach, research depth, and interaction style. Activate when the user starts any non-trivial task, gives an ambiguous or open-ended request, asks for help figuring something out, or begins exploratory/design/brainstorming work. Skip only for completely unambiguous single-step tasks."
---

# Task Intake

Calibrate your approach before starting work. Run this on the user's first substantive message of a session.

## CRITICAL: Question Tool FIRST

**You MUST call the Question tool BEFORE doing ANY of the following:**
- Reading or exploring code
- Searching the codebase (Glob, Grep, etc.)
- Dispatching subagents
- Diagnosing problems
- Proposing solutions

The entire point of intake is to ask the user how they want to work BEFORE you start working. If you explore the codebase first, you've already committed to an approach without consulting the user — that defeats the purpose.

**"Already explored" is NOT an excuse to skip intake.** If the user invokes intake after you've already done work, that means you should have done intake earlier. Do not say "intake is retroactive" or "I already explored so let me just ask about next steps." Reset. Ask the calibration questions now. The work you already did doesn't change the user's right to choose the approach going forward.

**Anti-pattern 1 (WRONG) — skipping intake because you already explored:**
> User: "The canvas zoom is broken"
> AI: *reads 5 files, diagnoses root cause, proposes solution* "Want me to plan this or just build it?"

This is NOT intake. This is skipping intake entirely and asking a token question after you've already decided the approach.

**Anti-pattern 2 (WRONG) — omitting the Research dimension:**
> User: "Look into the deployment setup"
> AI: *asks about scope and implementation priorities, but no question about research depth*

If the user's task involves understanding something, the Research dimension is MANDATORY. Don't assume they want to jump to building.

**Anti-pattern 3 (WRONG) — asking questions as plain text instead of the Question tool:**
> User: "I want you to research this"
> AI: "What aspects do you want researched? For example: - bullet 1 - bullet 2 - bullet 3"

This is NOT using the Question tool. Plain-text lists of options are NOT a substitute. If you're asking the user to choose between options, USE THE QUESTION TOOL. Always.

**Correct pattern:**
> User: "The canvas zoom is broken"
> AI: *calls Question tool with 2-3 calibration questions about approach, scope, research depth*
> User: *selects options*
> AI: *NOW explores codebase according to chosen approach*

## When to Activate

Scale triage complexity to the task's ambiguity:

**Skip entirely** — task is unambiguous AND the user specified exactly what to do:
- "Fix the typo on line 42 of src/foo.ts"
- "Follow this plan exactly: ./path/to/plan.md"
- "Run the test suite"

**Light (1 question, 2-3 options)** — slight ambiguity, clear domain:
- "Add retry logic to the API client"
- "Refactor this component"
- Clear task, 2-3 valid approaches

**Medium (2-3 questions)** — real design decisions:
- "Implement caching for the gallery images"
- "Fix the auth flow" (which part? what's broken?)
- Bug with unclear root cause, tasks touching multiple subsystems

**Full (3-4 questions)** — exploratory or open-ended:
- "Build the notification system"
- "I'm not sure how to approach this"
- Architecture, design, brainstorming, research-heavy tasks

## Rules

1. **Question tool BEFORE exploration.** Do not read files, search code, or dispatch subagents before calling the Question tool. You may read the skill itself, but that's it. Your first substantive action after loading this skill must be the Question tool call.

2. **ONE Question tool call.** Batch ALL questions into a single call. Never ask one question, wait for an answer, then ask another. The Question tool supports up to 4 questions — use them.

3. **Recommend, don't just list.** Put your recommended option first in each question. Explain your reasoning in the description. The user wants your judgment and creative insight, not a neutral menu.

4. **Expand scope through options.** Include at least one option per question that opens a direction the user didn't explicitly ask about — a related concern, a better approach, a risk they might not see. Options should be invitations to explore, not pre-digested conclusions. Allow scope expansion, not just narrowing.

5. **`multiSelect: true` by default.** Only use `multiSelect: false` when options genuinely conflict (e.g., "TDD or direct implementation?" — can't be both).

6. **Don't re-ask what's been stated.** If the user already said "I want TDD" or "research this first", that dimension is answered. Focus questions on what they DIDN'T specify.

7. **Labels are button text.** Keep labels under 30 characters. Put all reasoning, trade-offs, and context in descriptions.

8. **NEVER substitute plain text for the Question tool.** If you're presenting options for the user to choose from — at intake or any later decision point — use the Question tool. Bullet-point lists in prose are not interactive, not structured, and not what the user wants. The Question tool exists for a reason. Use it.

## Question Dimensions

Pick from these based on what's actually ambiguous. Research is almost always relevant — default to including it.

### Research (include by default)

**Always include the Research dimension** unless the task is purely mechanical (e.g., "rename X to Y"). When the user says "look into," "investigate," "figure out," "understand," or describes a problem they haven't diagnosed — Research is mandatory, not optional.

- **Research first** — look up docs, patterns, prior art before touching code
- **Research as needed** — start working, look things up when you hit unknowns
- **Skip research** — enough context exists in the codebase and conversation
- **Deep exploration** — dispatch subagents for thorough investigation before proposing anything

### Interaction Style
- **Key decisions only** — ask on major forks, handle details autonomously
- **Frequent check-ins** — question tool at each phase transition
- **Full autonomy** — implement and present results, minimize interruption
- **Brainstorm together** — high-interaction, creative, iterative back-and-forth

### Approach
- **Direct implementation** — read the relevant code, make the changes
- **TDD** — write failing tests first, then implement until green
- **Plan first** — written plan with phases, get approval, then execute
- **Subagent exploration** — use subagents to investigate/review, implement in main session
- **Exploratory** — read broadly, understand the landscape, then decide direction together

### Scope
- **As described** — scope looks right, proceed as stated
- **Broader than stated** — related issues are worth addressing (describe what you see)
- **Narrow first** — task is too broad, let's pick one piece to start
- **Needs investigation** — can't scope this without reading the codebase first

## After Triage

1. Wait for the user's selections. Do NOT start exploring while waiting.
2. Confirm the calibrated plan in 2-3 sentences. Not a monologue.
3. NOW begin codebase exploration and work, following the chosen approach.
4. Continue using the Question tool at decision points throughout the session, scaled to the interaction style the user selected.
