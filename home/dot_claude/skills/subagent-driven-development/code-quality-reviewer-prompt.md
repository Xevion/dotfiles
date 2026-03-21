# Code Quality Reviewer Prompt Template

Use this template when dispatching a code quality reviewer subagent.

**Purpose:** Verify implementation is well-built (clean, tested, maintainable)

**Only dispatch after spec compliance review passes.**

```
Task tool (code-reviewer agent):
  description: "Code quality review for Task N"
  prompt: |
    Review the code changes for Task N.

    WHAT_WAS_IMPLEMENTED: [from implementer's report]
    PLAN_OR_REQUIREMENTS: Task N from [plan-file]
    CHANGED_FILES: [list of files changed]
    DESCRIPTION: [task summary]

    Review for: code quality, test coverage, maintainability,
    naming, patterns consistency, edge cases, error handling.

    Do NOT re-review spec compliance — that's already verified.
```

**Code reviewer returns:** Strengths, Issues (Critical/Important/Minor), Assessment
