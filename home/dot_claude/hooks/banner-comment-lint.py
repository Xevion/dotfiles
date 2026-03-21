#!/usr/bin/env python3
"""PostToolUse hook: detect banner/decorative comments in written files.

Fires after Write or Edit tool calls. Scans written content for banner-style
comment patterns (decorative dividers, box-drawing chars) and reports matches
to Claude via stderr with file:line context.

Exit codes:
  0 — no banner patterns found (or file excluded)
  2 — banner pattern(s) found (stderr is shown to Claude as feedback)
"""

import json
import re
import sys
from pathlib import Path

# Banner patterns mirror the normalizing-code-symbols skill's detection scan.
BANNER_PATTERNS = [
    # Comment delimiter followed by 5+ repeated decoration chars
    re.compile(r'^\s*(?://[/!]?|/\*+|#|--|<!--|@rem|::|\*)\s*[-=*#~+._^─═/]{5,}', re.MULTILINE),
    # Standalone line of 10+ decoration chars (no comment prefix needed)
    re.compile(r'^\s*[-=*#~+._^─═/]{10,}', re.MULTILINE),
    # Decorative header: delimiter, chars, content, chars (// === Header ===)
    re.compile(r'(?://[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{2,}\s+\S.{0,60}\s*[-=*#~+._^─═]{2,}', re.MULTILINE),
    # Closing comment with decoration (====== */)
    re.compile(r'[-=*#~+._^─═/]{5,}\s*(?:\*/|-->)', re.MULTILINE),
    # Box-drawing characters (3+ consecutive)
    re.compile(r'[╔╗╚╝║│┌┐└┘├┤┬┴┼]{3,}', re.MULTILINE),
    # Bracket-style banner (// --- [Section Name] ---)
    re.compile(r'(?://[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{1,}\s*\[.{1,60}\]\s*[-=*#~+._^─═]{2,}', re.MULTILINE),
]

EXCLUDED_SUFFIXES = {'.md', '.lock'}
EXCLUDED_FILENAME_PARTS = ['node_modules', 'migrations', '.min.']
EXCLUDED_FILENAMES = {'gradlew', 'gradlew.bat'}


def is_excluded(file_path: str) -> bool:
    path = Path(file_path)
    if path.name in EXCLUDED_FILENAMES:
        return True
    if path.suffix in EXCLUDED_SUFFIXES:
        return True
    path_str = str(path)
    return any(part in path_str for part in EXCLUDED_FILENAME_PARTS)


def find_banner_lines(content: str) -> list[tuple[int, str]]:
    """Return sorted (line_number, line_content) pairs for each matched line."""
    seen: set[int] = set()
    matches: list[tuple[int, str]] = []
    lines = content.splitlines()

    for pattern in BANNER_PATTERNS:
        for m in pattern.finditer(content):
            line_no = content[: m.start()].count('\n') + 1
            if line_no not in seen:
                seen.add(line_no)
                line_text = lines[line_no - 1].rstrip() if line_no <= len(lines) else m.group(0)
                matches.append((line_no, line_text))

    return sorted(matches)


def main() -> None:
    try:
        data = json.load(sys.stdin)
    except (json.JSONDecodeError, EOFError):
        sys.exit(0)

    tool_name = data.get('tool_name', '')
    if tool_name not in ('Write', 'Edit'):
        sys.exit(0)

    tool_input = data.get('tool_input', {})
    file_path = tool_input.get('file_path', '')

    if is_excluded(file_path):
        sys.exit(0)

    content = tool_input.get('content') if tool_name == 'Write' else tool_input.get('new_string')
    if not content:
        sys.exit(0)

    banner_lines = find_banner_lines(content)
    if not banner_lines:
        sys.exit(0)

    # For Edit, new_string is a snippet so line numbers are relative to it,
    # not the full file — note this in the output.
    location_note = '' if tool_name == 'Write' else ' (line numbers relative to replacement text)'

    out = [f'Banner comment(s) detected in {file_path}{location_note}:']
    for line_no, line_text in banner_lines[:5]:
        out.append(f'  {file_path}:{line_no}: {line_text.strip()}')
    if len(banner_lines) > 5:
        out.append(f'  ... and {len(banner_lines) - 5} more')
    out.append('')
    out.append('Remove decorative dividers. If an adjacent label adds context, keep it as a plain comment.')
    out.append('If this is a false positive (generated file, intentional delimiter), disregard.')

    print('\n'.join(out), file=sys.stderr)
    sys.exit(2)


if __name__ == '__main__':
    main()
