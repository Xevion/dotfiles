#!/usr/bin/env python3
"""PreToolUse hook: enforce bash command discipline.

Blocks Bash tool calls that use patterns better served by dedicated tools
(Read, Grep, Glob) or that include unnecessary complexity (error suppression,
excessive chaining, small head/tail truncation).

Exit codes:
  0 — allow the command
  2 — block the command (stderr is shown to Claude as the reason)
"""

import json
import re
import sys

# Threshold: pipeline head/tail with N <= this value is blocked.
# Values above this are considered reasonable output caps.
SMALL_TRUNCATION_LIMIT = 100


def check_command(cmd: str) -> list[str]:
    """Return list of violation messages. Empty list = command is allowed."""
    issues: list[str] = []
    stripped = cmd.strip()

    # ── Dedicated tool replacements ────────────────────────────────────

    # grep as primary command (not in a pipeline)
    if re.match(r"^\s*grep\b", stripped):
        issues.append(
            "Use the Grep tool instead of the grep command. "
            "It handles permissions correctly and doesn't require approval."
        )

    # head/tail as primary command on files
    if re.match(r"^\s*(head|tail)\s+", stripped):
        issues.append(
            "Use the Read tool with offset/limit instead of head/tail on files."
        )

    # cat as primary command
    if re.match(r"^\s*cat\s+\S", stripped):
        issues.append(
            "Use the Read tool instead of cat. "
            "It provides line numbers and supports offset/limit."
        )

    # find -name when Glob exists
    if re.match(r"^\s*find\s+\S+\s+(-name|-iname|-type)", stripped):
        issues.append("Use the Glob tool instead of find. It's faster and doesn't need approval.")

    # ls to check file existence
    if re.match(r"^\s*ls\s+\S+.*2>/dev/null", stripped):
        issues.append(
            "Use the Glob tool to check if files exist instead of ls with error suppression."
        )

    # ── Error suppression ──────────────────────────────────────────────

    # 2>/dev/null (swallows errors entirely)
    if re.search(r"2>\s*/dev/null", cmd):
        issues.append(
            "Do not suppress stderr with 2>/dev/null. "
            "Error output often contains the diagnosis, and suppression "
            "makes commands unrecognizable to the permission system."
        )

    # || echo "not found" / || echo "error" style fallbacks
    if re.search(r'\|\|\s*echo\s+["\']?(not found|NOT FOUND|error|no |none)', cmd):
        issues.append(
            "Do not suppress errors with '|| echo ...'. "
            "Let commands fail naturally so errors are visible."
        )

    # ── Small pipeline truncation (wasteful re-run pattern) ────────────

    # | head -N where N is small
    for m in re.finditer(r"\|\s*head\s+-n?\s*(\d+)", cmd, re.IGNORECASE):
        n = int(m.group(1))
        if n <= SMALL_TRUNCATION_LIMIT:
            issues.append(
                f"Avoid '| head -{n}' — small truncation values encourage "
                f"wasteful re-runs with incrementally larger limits. "
                f"Let the Bash tool handle output naturally, or use the "
                f"Read tool on saved output. If truncation is truly needed, "
                f"use a limit above {SMALL_TRUNCATION_LIMIT}."
            )

    # | tail -N where N is small (but NOT tail -n +N which is offset, not truncation)
    for m in re.finditer(r"\|\s*tail\s+-n?\s*(\d+)", cmd, re.IGNORECASE):
        # Skip tail -n +N (offset pattern, perfectly fine)
        offset_match = re.search(r"\|\s*tail\s+-n\s*\+", cmd)
        if offset_match and offset_match.start() == m.start() - len(m.group(0)) + len(m.group(0)):
            continue
        n = int(m.group(1))
        if n <= SMALL_TRUNCATION_LIMIT:
            issues.append(
                f"Avoid '| tail -{n}' — use the Read tool with offset/limit instead."
            )

    # ── 2>&1 (unnecessary stream merging) ──────────────────────────────
    # Block 2>&1 when combined with piping (the problematic pattern).
    # Bare 2>&1 without piping is less harmful but still pointless since
    # the Bash tool captures both streams.
    if re.search(r"2>&1", cmd):
        issues.append(
            "Remove '2>&1' — the Bash tool captures both stdout and stderr automatically. "
            "Adding it is unnecessary and can interfere with permission pattern matching."
        )

    return issues


def main():
    try:
        data = json.load(sys.stdin)
    except (json.JSONDecodeError, EOFError):
        sys.exit(0)  # Can't parse input, allow through

    if data.get("tool_name") != "Bash":
        sys.exit(0)

    command = data.get("tool_input", {}).get("command", "")
    if not command:
        sys.exit(0)

    issues = check_command(command)
    if issues:
        for issue in issues:
            print(f"• {issue}", file=sys.stderr)
        sys.exit(2)


if __name__ == "__main__":
    main()
