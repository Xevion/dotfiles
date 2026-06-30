"""Custom kitty hints processor: trim stray wrapping punctuation from matches.

kitty's builtin path/url matchers treat brackets and quotes as valid characters,
so a path printed as ``(~/.config/kitty/kitty.conf)`` is matched *with* the
trailing ``)``. This processor keeps a closing bracket only when it is balanced
(e.g. the Wikipedia URL ``.../Foo_(bar)``) and drops it otherwise, also peeling a
leading wrapper and any trailing sentence punctuation. Only *matching* is
customized; the action (clipboard / default-open / launch) defers to kitty.

Wired up from kitty.conf via ``--customize-processing hints-trim.py``. Honors
``--type url`` (require a URL scheme) vs anything else (require a path signal).

For open-style actions (any ``--program`` that isn't pure copy/paste) a path
candidate is dropped only when it is *provably* absent on disk: an absolute or
``~``/``$VAR`` path that doesn't exist. Relative paths can't be resolved here (no
window cwd), so they are kept; copy actions are never filtered.
"""
import os
import re

_TOKEN = re.compile(r'\S+')
_URL_SCHEME = re.compile(r'(?:https?|ftp|ftps|file|ssh|git)://', re.IGNORECASE)
_PATH_SIGNAL = re.compile(r'/|\w\.\w')          # a slash, or a dotted filename
_LINENUM = re.compile(r':\d+(?::\d+)?$')        # trailing :line or :line:col
_CLOSERS = {')': '(', ']': '[', '}': '{', '>': '<'}
_OPENERS = '([{<'
_QUOTES = "'\"`"
_SENTENCE = '.,;:!?'
_COPY_LIKE = {'@', '*', '-'}                    # --program values that copy/paste text


def _strip(s):
    """Return (left_offset, core): drop leading wrappers, then trailing sentence
    punctuation, quotes, and *unbalanced* closing brackets."""
    left = 0
    while s and (s[0] in _OPENERS or s[0] in _QUOTES):
        s = s[1:]
        left += 1
    changed = True
    while changed and s:
        changed = False
        ch = s[-1]
        if ch in _CLOSERS:
            if s.count(_CLOSERS[ch]) < s.count(ch):   # unbalanced -> drop
                s = s[:-1]
                changed = True
        elif ch in _QUOTES or ch in _SENTENCE:
            s = s[:-1]
            changed = True
    return left, s


def _opens_files(args):
    """True for open/launch actions (not pure copy/paste), where filtering to
    paths that exist on disk is useful."""
    progs = getattr(args, 'program', None) or ['default']
    return any(p not in _COPY_LIKE and not p.startswith('@') for p in progs)


def _provably_absent(core):
    """True only when we can resolve the path to an absolute location and it is
    confirmed missing. Relative/unresolvable paths return False (kept)."""
    p = os.path.expanduser(os.path.expandvars(_LINENUM.sub('', core)))
    if not os.path.isabs(p):
        return False
    try:
        return not os.path.lexists(p)
    except OSError:
        return False


def mark(text, args, Mark, extra_cli_args, *a):
    is_url = getattr(args, 'type', '') == 'url'
    filter_missing = not is_url and _opens_files(args)
    idx = 0
    for tm in _TOKEN.finditer(text):
        left, core = _strip(tm.group().replace('\0', ''))
        if not core:
            continue
        if is_url:
            if not _URL_SCHEME.match(core):
                continue
        elif not _PATH_SIGNAL.search(core):
            continue
        elif filter_missing and _provably_absent(core):
            continue
        start = tm.start() + left
        yield Mark(idx, start, start + len(core), core, {})
        idx += 1


# Intentionally no handle_result: defining one hijacks the action entirely (see
# kitty hints main.py). Omitting it lets kitty run its builtin --program handling
# (@ clipboard, default open, launch ...) on our trimmed matches.
