#!/usr/bin/env python3
"""Find file paths in terminal output, then act on one of them.

Serves two entry points, both wired from kitty.conf:

* ``kitten hints --customize-processing screen_paths.py`` calls :func:`mark` to
  label paths in place.
* ``kitty-paths list`` reads captured screen text on stdin and offers the same
  paths through fzf. ``kitty-paths action PATH`` runs the second-stage menu.

Extraction excludes brackets and quotes from the path character class rather
than trimming them afterwards, so ``Read(/tmp/x.md)`` yields ``/tmp/x.md``
without the wrapper ever becoming part of the candidate. A candidate survives
when it exists on disk, or failing that when it still reads unambiguously as a
path: an explicit root/home/relative prefix, or a plausible extension. That
second test is what rejects statusline debris such as ``152.3`` or ``$0.64``.
"""
import os
import re
import shutil
import subprocess
import sys

# Path characters: anything but whitespace, brackets, quotes, and the separators
# that routinely abut a path in prose or log output.
_SEG = r"[^\s/()\[\]{}<>'\"`,;|]"
_CANDIDATE = re.compile(
    # A path with at least one slash, including its first segment and a braced
    # ${VAR} root (whose braces the segment class deliberately excludes) ...
    rf"(?:\$\{{\w+\}})?{_SEG}*(?:/{_SEG}+)+/?"
    # ... or a bare dotted filename.
    rf"|{_SEG}*\w\.\w{_SEG}*"
)
_LINECOL = re.compile(r"^:(\d+)(?::(\d+))?")
_TRAILING = re.compile(r"[.!?:]+$")
_EXTENSION = re.compile(r"\.[A-Za-z][A-Za-z0-9]{0,8}$")
_ROOTED = re.compile(r"^(?:/|~/|\./|\.\./|\$\{?\w+\}?/)")


def unwrap(text):
    """Drop kitty's soft-wrap markers, returning (clean, offsets).

    The hints kitten marks a wrap with a null byte, ``launch
    --stdin-add-line-wrap-markers`` with a carriage return; both may be followed
    by the newline they introduced. ``offsets[i]`` is the index of ``clean[i]``
    in ``text``, so spans found in the cleaned string map back onto the screen
    for highlighting.
    """
    out, offsets, i, n = [], [], 0, len(text)
    while i < n:
        if text[i] in "\0\r":
            i += 1
            if i < n and text[i] == "\n":
                i += 1
            continue
        out.append(text[i])
        offsets.append(i)
        i += 1
    offsets.append(n)
    return "".join(out), offsets


def _split_linecol(core):
    """Peel a trailing ``:12`` or ``:12:34`` off a path, returning (path, suffix)."""
    at = core.find(":")
    if at == -1:
        return core, ""
    m = _LINECOL.match(core[at:])
    return (core[:at], core[at:at + m.end()]) if m else (core, "")


def looks_like_path(core):
    """True when the token reads as a path on its own, without touching disk.

    Deliberately strict: this is the only gate for candidates that are not
    present locally, and for every candidate during in-kitty hint marking where
    no working directory is available.
    """
    if not core or core in {".", "..", "/"}:
        return False
    if "@" in core and "/" not in core:
        return False
    has_extension = bool(_EXTENSION.search(core))
    if _ROOTED.match(core):
        depth = core.strip("/").count("/") + 1
        return has_extension or depth >= 2
    return has_extension


def resolve(core, cwd):
    """Expand ~ and $VARs, then anchor a relative path to cwd."""
    p = os.path.expanduser(os.path.expandvars(core))
    return p if os.path.isabs(p) else os.path.join(cwd, p)


def extract(text, cwd=None):
    """Yield (start, end, core, suffix, exists) for every path-like span in text.

    Offsets index ``text`` as given. With no cwd, existence is only consulted
    for paths that resolve absolutely.
    """
    for m in _CANDIDATE.finditer(text):
        # A match butted against / or : is the tail of a URL, not a path.
        if m.start() and text[m.start() - 1] in "/:":
            continue
        core = _TRAILING.sub("", m.group())
        core, suffix = _split_linecol(core)
        if not core:
            continue
        target = resolve(core, cwd) if cwd else os.path.expanduser(os.path.expandvars(core))
        exists = os.path.isabs(target) and os.path.lexists(target)
        if not exists and not looks_like_path(core):
            continue
        yield m.start(), m.start() + len(core), core, suffix, exists


def mark(text, args, Mark, extra_cli_args, *a):
    """kitty hints entry point: label every path visible on screen."""
    clean, offsets = unwrap(text)
    for idx, (start, end, core, suffix, _) in enumerate(extract(clean)):
        yield Mark(idx, offsets[start], offsets[end], core + suffix, {})


def _clip(text):
    if shutil.which("kitten"):
        r = subprocess.run(["kitten", "clipboard"], input=text, text=True)
        if r.returncode == 0:
            return
    subprocess.run(["xclip", "-selection", "clipboard"], input=text, text=True, check=True)


def _detach(argv):
    """Launch a GUI handler that must outlive this overlay window."""
    subprocess.Popen(
        argv, start_new_session=True,
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, stdin=subprocess.DEVNULL,
    )


def _fzf(lines, prompt, header):
    """Run fzf over tab-delimited lines, returning the chosen line or None.

    Only stdout is captured. fzf draws its interface on stderr, so piping that
    too leaves the window blank while fzf waits for a keypress it can't show.
    """
    if not shutil.which("fzf"):
        sys.exit("fzf is not installed")
    r = subprocess.run(
        ["fzf", "--height=100%", "--layout=reverse", "--no-multi", "--no-info",
         "--delimiter=\t", "--with-nth=2..", f"--prompt={prompt}", f"--header={header}"],
        input="\n".join(lines), text=True, stdout=subprocess.PIPE,
    )
    return r.stdout.strip() or None


def _pause(message):
    """In list mode our own stdin is the captured screen text, so a prompt has to
    read the keyboard directly or it returns instantly on EOF."""
    print(message)
    try:
        with open("/dev/tty") as tty:
            print("Press Enter to close...", end="", flush=True)
            tty.readline()
    except (OSError, KeyboardInterrupt):
        pass


def action_menu(raw, cwd):
    """Second stage: pick what to do with an already-chosen path."""
    # Re-run extraction on the argument rather than trusting it: hints hands over
    # clean text, but anything else calling in should not be able to smuggle a
    # wrapper like Read(...) through to xdg-open.
    found = next(iter(extract(unwrap(raw)[0], cwd)), None)
    if found:
        _, _, core, suffix, _ = found
    else:
        core, suffix = _split_linecol(_TRAILING.sub("", raw))
    target = resolve(core, cwd)
    parent = os.path.dirname(target) or "/"
    is_dir = os.path.isdir(target)
    exists = os.path.lexists(target)
    line = suffix.lstrip(":").split(":")[0] if suffix else ""

    actions = []
    if exists and not is_dir:
        actions.append(("open", "open file", f"xdg-open {os.path.basename(target)}"))
    if is_dir:
        actions.append(("open", "open folder", target))
    elif exists:
        actions.append(("dir", "open containing folder", parent))
    if exists and not is_dir:
        actions.append(("edit", "edit here" + (f" at line {line}" if line else ""), target))
    actions += [
        ("copy", "copy path", target),
        ("copy-shown", "copy path as shown", core + suffix),
        ("copy-dir", "copy containing folder", parent),
    ]
    if not exists:
        actions.append(("dir", "open nearest existing folder", _nearest(parent)))

    status = "" if exists else "  (not found on disk)"
    lines = [f"{key}\t{label:<30}{value}" for key, label, value in actions]
    chosen = _fzf(lines, "action> ", f"{core}{suffix}{status}")
    if not chosen:
        return
    key = chosen.split("\t", 1)[0]

    if key == "open":
        _detach(["xdg-open", target])
    elif key == "dir":
        _detach(["xdg-open", _nearest(parent) if not exists else parent])
    elif key == "edit":
        editor = os.environ.get("EDITOR") or "micro"
        argv = [editor, f"+{line}", target] if line and editor in ("nvim", "vim", "micro") else [editor, target]
        with open("/dev/tty") as tty:
            subprocess.run(argv, stdin=tty)
    elif key == "copy":
        _clip(target)
    elif key == "copy-shown":
        _clip(core + suffix)
    elif key == "copy-dir":
        _clip(parent)


def _nearest(path):
    """Walk up until an existing directory is found, so 'open folder' never no-ops."""
    while path and path != "/" and not os.path.isdir(path):
        path = os.path.dirname(path)
    return path or "/"


def list_menu(text, cwd):
    """First stage: every path on the captured screen, newest occurrence first."""
    clean, _ = unwrap(text)
    seen, rows = {}, []
    for _, _, core, suffix, exists in extract(clean, cwd):
        key = resolve(core, cwd) + suffix
        if key in seen:
            rows[seen[key]][2] += 1
            continue
        seen[key] = len(rows)
        rows.append([core + suffix, exists, 1])

    if not rows:
        _pause("No file paths found on screen.")
        return
    rows.reverse()

    lines = []
    for shown, exists, count in rows:
        badge = " " if exists else "?"
        tally = f" x{count}" if count > 1 else ""
        lines.append(f"{shown}\t{badge} {shown}{tally}")
    chosen = _fzf(lines, "path> ", "? = not found on disk    Enter to choose an action")
    if chosen:
        action_menu(chosen.split("\t", 1)[0], cwd)


# Invoked by the kitty-paths shim, never as a script: kitty's hints kitten runs
# this file with __name__ set to "__main__", so a self-dispatch block here would
# fire during hint marking and abort the kitten.
def main(argv):
    cwd = os.getcwd()
    if len(argv) >= 2 and argv[0] == "action":
        action_menu(argv[1], cwd)
    elif argv and argv[0] == "list":
        list_menu(sys.stdin.read(), cwd)
    else:
        sys.exit("usage: kitty-paths list < screen-text | kitty-paths action PATH")
