---
description: "Session triage — auto-activates on first message to calibrate approach, research depth, and interaction style. Activate when the user starts any non-trivial task, gives an ambiguous or open-ended request, asks for help figuring something out, or begins exploratory/design/brainstorming work. Skip only for completely unambiguous single-step tasks."
---

# Task Intake

Calibrate your approach before starting work. Run this on the user's first substantive message of a session.

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

1. **ONE Question tool call.** Batch ALL questions into a single call. Never ask one question, wait for an answer, then ask another. The Question tool supports up to 4 questions — use them.

2. **Recommend, don't just list.** Put your recommended option first in each question. Explain your reasoning in the description. The user wants your judgment and creative insight, not a neutral menu.

3. **Expand scope through options.** Include at least one option per question that opens a direction the user didn't explicitly ask about — a related concern, a better approach, a risk they might not see. Options should be invitations to explore, not pre-digested conclusions. Allow scope expansion, not just narrowing.

4. **`multiSelect: true` by default.** Only use `multiSelect: false` when options genuinely conflict (e.g., "TDD or direct implementation?" — can't be both).

5. **Don't re-ask what's been stated.** If the user already said "I want TDD" or "research this first", that dimension is answered. Focus questions on what they DIDN'T specify.

6. **Labels are button text.** Keep labels under 30 characters. Put all reasoning, trade-offs, and context in descriptions.

## Question Dimensions

Pick from these based on what's actually ambiguous. You rarely need all of them.

### Research
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

1. Confirm the calibrated plan in 2-3 sentences. Not a monologue.
2. Start executing according to the chosen approach.
3. Continue using the Question tool at decision points throughout the session, scaled to the interaction style the user selected.
