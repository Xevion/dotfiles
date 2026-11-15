#!/usr/bin/env python3
"""Print a repository's versioning state: tags, manifests, past bumps, unreleased work, release files.

Usage: survey.py [REPO]   (defaults to the current directory)

Read-only. Run `git fetch --tags` first, or tags made by CI or on another machine are missing.
"""

import collections
import json
import re
import subprocess
import sys
from pathlib import Path

repo = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()


def git(*args: str) -> str:
    r = subprocess.run(["git", "-C", str(repo), *args], capture_output=True, text=True)
    return r.stdout.strip() if r.returncode == 0 else ""


def section(title: str) -> None:
    print(f"\n## {title}")


if not git("rev-parse", "--git-dir"):
    sys.exit(f"{repo}: not a git repository")

section("Repository")
branch = git("rev-parse", "--abbrev-ref", "HEAD")
upstream = git("rev-parse", "--abbrev-ref", "@{u}")
print(f"branch {branch}, upstream {upstream or 'none'}")
if upstream:
    ahead, behind = git("rev-list", "--left-right", "--count", "HEAD...@{u}").split()
    print(f"unpushed {ahead}, behind {behind}")
for line in git("remote", "-v").splitlines():
    if line.endswith("(fetch)"):
        name, url, _ = line.split()
        opt = git("config", f"remote.{name}.tagOpt")
        print(f"remote {name} {url}{'  tagOpt=' + opt if opt else ''}")
gh = subprocess.run(["gh", "repo", "view", "--json", "visibility,isFork,parent", "--jq",
                     '"\\(.visibility) fork=\\(.isFork) parent=\\(.parent.nameWithOwner // "")"'],
                    cwd=repo, capture_output=True, text=True)
print(f"github {gh.stdout.strip() if gh.returncode == 0 else 'unknown'}")
roots = git("log", "--reverse", "--format=%cs", "--max-parents=0").splitlines()
print(f"commits {git('rev-list', '--count', 'HEAD')}, first {roots[0] if roots else '-'}, last {git('log', '-1', '--format=%cs')}")

section("Tags")
fmt = "%(refname:short)\t%(objecttype)\t%(taggerdate:iso-strict)\t%(*committerdate:iso-strict)\t%(committerdate:iso-strict)"
tags = []
for line in git("for-each-ref", "--sort=creatordate", f"--format={fmt}", "refs/tags").splitlines():
    # git() strips the output, which eats the last line's empty trailing fields.
    name, kind, tagger, peeled, direct = (line.split("\t") + [""] * 4)[:5]
    commit_date = peeled or direct
    tags.append((name, kind, tagger, commit_date))
if not tags:
    print("none")
by_prefix = collections.defaultdict(list)
for name, *_ in tags:
    m = re.match(r"^(.*?)v?\d+\.\d+\.\d+", name)
    by_prefix[m.group(1) if m else "(non-version)"].append(name)
for prefix, names in by_prefix.items():
    print(f"prefix {prefix or '(none)'!r}: {len(names)} tags, latest {names[-1]}")
reachable = set(git("tag", "--merged", "HEAD").splitlines())
for name, kind, tagger, commit_date in tags[-15:]:
    lag = ""
    if kind == "tag" and tagger[:10] != commit_date[:10]:
        lag = f"  tagged {tagger[:10]}"
    print(f"  {name:<32} {'annotated' if kind == 'tag' else 'lightweight':<11} commit {commit_date[:10]}{lag}{'' if name in reachable else '  NOT IN HEAD'}")
if len(tags) > 15:
    print(f"  ({len(tags) - 15} older tags not shown)")
# Several unit tags on one release commit are normal, so count distinct tagged commits per minute.
stamp = collections.Counter(m for m, _ in {(t[2][:16], t[3]) for t in tags if t[1] == "tag"})
bursts = [(k, v) for k, v in stamp.items() if v >= 3]
for minute, n in bursts:
    print(f"BURST: {n} annotated tags created in the minute {minute}")
per_day = collections.Counter((t[3][:10], re.sub(r"v?\d.*", "", t[0])) for t in tags)
dense = sorted(((n, d) for (d, _), n in per_day.items() if n >= 3), reverse=True)
if dense:
    print(f"DENSE: {len(dense)} days with 3+ tags of one prefix; worst {dense[0][1]} with {dense[0][0]}")
remote = upstream.split("/")[0] if upstream else ("origin" if "origin" in git("remote").split() else "")
if remote:
    r = subprocess.run(["git", "-C", str(repo), "ls-remote", "--tags", "--refs", remote],
                       capture_output=True, text=True, timeout=30)
    if r.returncode == 0:
        remote_tags = {line.split("refs/tags/", 1)[1] for line in r.stdout.splitlines()}
        local_tags = {t[0] for t in tags}
        if remote_only := sorted(remote_tags - local_tags):
            print(f"ONLY ON {remote} (fetch first): {' '.join(remote_only)}")
        if local_only := sorted(local_tags - remote_tags):
            print(f"ONLY LOCAL (never pushed, or from another remote): {' '.join(local_only[:20])}"
                  f"{f' (+{len(local_only) - 20})' if len(local_only) > 20 else ''}")

section("Manifests")
files = git("ls-files").splitlines()
manifests = [f for f in files if Path(f).name in {"Cargo.toml", "package.json", "pyproject.toml", "gradle.properties", "go.mod", "build.gradle", "build.gradle.kts"}]
for f in manifests:
    text = (repo / f).read_text(errors="ignore")
    if f.endswith("package.json"):
        try:
            data = json.loads(text)
        except ValueError:
            data = {}
        print(f"{f}: name={data.get('name')} version={data.get('version')} private={data.get('private', False)}")
    elif f.endswith("go.mod"):
        print(f"{f}: Go module (the tag is the version)")
    else:
        version = re.search(r'^\s*(?:mod_)?version\s*=\s*"?([^"\s]+)"?', text, re.M)
        publish = re.search(r"^\s*publish\s*=\s*(\S+)", text, re.M)
        workspace = "[workspace]" in text
        print(f"{f}: version={version.group(1) if version else '-'}"
              f"{' publish=' + publish.group(1) if publish else ''}{' (workspace root)' if workspace else ''}")
pins = [f"{f}: {line.strip()}" for f in manifests if f.endswith("Cargo.toml")
        for line in (repo / f).read_text(errors="ignore").splitlines()
        if "path =" in line and 'version = "=' in line]
for p in pins:
    print(f"exact pin {p}")

section("Past version bumps (commits that changed a version line)")
paths = [f for f in manifests if not f.endswith("go.mod")]
if paths:
    out = git("log", "--format=%h %cs %s", r"-G^\s*(\"version\"\s*:|(mod_)?version\s*=)", "--", *paths)
    lines = out.splitlines()
    print("\n".join(lines[:20]) or "none")
    if len(lines) > 20:
        print(f"({len(lines) - 20} older)")

section("Unreleased work since the latest tag of each prefix")
package_dirs = {}
for f in manifests:
    text = (repo / f).read_text(errors="ignore")
    if f.endswith("package.json"):
        try:
            name = json.loads(text).get("name")
        except ValueError:
            name = None
    else:
        m = re.search(r'^\[package\][^\[]*?^name\s*=\s*"([^"]+)"', text, re.M | re.S)
        name = m.group(1) if m else None
    if name:
        package_dirs[name] = str(Path(f).parent)
for prefix, names in by_prefix.items():
    if prefix == "(non-version)":
        continue
    scope = package_dirs.get(prefix.rstrip("-_/@"))
    since = git("log", "--format=%s", f"{names[-1]}..HEAD", *(["--", scope] if scope else []))
    if scope:
        print(f"(limited to {scope}/)", end=" ")
    counts = collections.Counter()
    for s in since.splitlines():
        m = re.match(r"^(\w+)(\([^)]*\))?(!)?:", s)
        counts["BREAKING" if m and m.group(3) else (m.group(1) if m else "other")] += 1
    print(f"{names[-1]}: {sum(counts.values())} commits " + " ".join(f"{k}={v}" for k, v in counts.most_common()))
if not tags:
    print(f"no tags; {git('rev-list', '--count', 'HEAD')} commits in total")

section("Release machinery and docs")
interesting = [f for f in files if not f.startswith(".github/workflows/") and re.search(r"(^|/)(CHANGELOG|RELEASING|RELEASE)[^/]*$|release-please|\.changeset/|cliff\.toml|release\.toml|knope", f, re.I)]
for f in files:
    if f.startswith(".github/workflows/"):
        text = (repo / f).read_text(errors="ignore")
        # Event triggers count only inside the top-level `on:` block, not job steps' `tags:` inputs.
        on = re.search(r"^['\"]?on['\"]?:(.*?)(?=^\S)", text + "\nend:", re.M | re.S)
        events = on.group(1) if on else ""
        patterns = {"tags:": r"^\s+tags:", "release:": r"^\s+release:|[\[,\s]release\s*[\],]", "workflow_dispatch": r"\bworkflow_dispatch\b"}
        triggers = [t for t, p in patterns.items() if re.search(p, events, re.M)]
        triggers += [t for t in ("release-please", "publish") if t in text]
        if any(t in triggers for t in ("tags:", "release:", "release-please", "publish")):
            interesting.append(f"{f}  [{', '.join(triggers)}]")
for doc in ("CLAUDE.md", "AGENTS.md", "docs/RELEASING.md"):
    p = repo / doc
    if p.is_file() and re.search(r"^#+\s+(releases?|releasing|release process|versioning)\s*$", p.read_text(errors="ignore"), re.I | re.M):
        interesting.append(f"{doc}  (has a release section)")
print("\n".join(dict.fromkeys(interesting)) or "none")
