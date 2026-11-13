# History Surgery

Recipes for rewriting the unpushed part of a branch without an interactive editor. Every recipe
here was exercised against a scratch repo on git 2.54. On another version, skim
`man git-rebase` for changed behaviour before relying on one.

Write literal SHAs and paths into each command. Placeholders are in angle brackets; `<x7>` means
the first seven characters of X's SHA.

## Contents

1. Editors: make sure none can open
2. The todo list
3. Tree state: in place, or in a worktree
4. Recipes: insert, amend, fixup, reorder, split
5. Conflicts and aborting
6. Verifying a rewrite
7. Rewriting pushed history (ask first)

## 1. Editors: make sure none can open

Git opens the user's editor (check `git config core.editor`) at several points that are easy to
forget. Without a terminal it either fails or hangs the call, so set an editor that exits at once:

| Command | Opens an editor? | Use |
| --- | --- | --- |
| `git rebase -i` | Yes, for the todo | `GIT_SEQUENCE_EDITOR="<sed ...>"` or `GIT_SEQUENCE_EDITOR=:` |
| `git rebase --continue` after resolving a conflict | **Yes, for the commit message** | `GIT_EDITOR=true git rebase --continue` |
| `git rebase --continue` after a `break` or `edit` stop | No | plain |
| `git commit` without `-m`/`-C`/`-F`/`--no-edit` | Yes | always pass one of them |
| `git commit --amend` | Yes | `--no-edit`, or `-m` |

`GIT_EDITOR=true git rebase --continue` is harmless where no editor would open, so use it every
time.

## 2. The todo list

On git 2.54 a todo line reads `pick d9d8679 # subject`. The hash always comes first.
`rebase.instructionFormat` only changes what follows it, and `rebase.abbreviateCommands=true`
turns `pick` into `p`. So match lines with `sed -E '/^(pick|p) <x7>/...'`. The todo's hash is at
least seven characters long and always starts with the first seven of the full SHA. These
recipes use GNU `sed -i`; on macOS use `gsed`, or `sed -i ''`.

## 3. Tree state: in place, or in a worktree

| Tree | Approach |
| --- | --- |
| Clean, user not working in it | Rebase in place |
| Unstaged edits only, user not committing | In place with `--autostash`, which re-applies them afterwards |
| Anything staged | Never `--autostash`. Tested: it re-applies the changes but un-stages them, which destroys the user's review state. Use the worktree route |
| User actively editing or committing | Worktree route |

Record `<old>` (`git rev-parse HEAD`) before either.

**Worktree route.** A worktree used only for surgery is a checkout with no build, so it is cheap.
Building in it is the expensive part (fresh target dir, missing git-excluded files), so decide
that separately.

```bash
git worktree add --detach <scratch>/surgery <old>
# In a separate Bash call, cd into <scratch>/surgery and run the recipe against origin/<branch>.
git rev-parse HEAD                       # inside the worktree -> <new>
git worktree remove <scratch>/surgery    # from the main checkout
```

**Hand-back to the main checkout:**

```bash
git rev-parse <branch>                   # still <old>? if not, see below
git switch --detach <new>                # carries staged and unstaged changes across; refuses on overlap
git branch -f <branch> <new>
git switch <branch>                      # same commit, so nothing moves; upstream tracking is kept
```

Don't use `git reset --keep` or `--merge` for the hand-back. Both reset the index. Tested:
`--keep` leaves a staged new file untracked and staged edits unstaged, and `--merge` deletes a
staged new file from disk outright. A branch switch does a
two-way merge that keeps the index for every path the rewrite didn't touch. When a user change
overlaps a path the rewrite touched, `git switch` refuses with "would be overwritten" and changes
nothing. Stop and ask at that point.

When the rewrite leaves the tip tree identical (`git diff --quiet <old> <new>`, as in a pure
reorder), the hand-back can just move the ref, because no file differs:
`git update-ref refs/heads/<branch> <new> <old>`. The trailing `<old>` makes it fail if the branch
moved meanwhile. Use the switch route whenever the trees differ: an index left behind by
`update-ref` would show the rewrite's changes as staged reversions.

If the user committed on top of `<old>` while you worked, replay their commits onto `<new>` inside
the worktree first (`git cherry-pick <old>..<branch>`), then hand back that result.

`git replay` (experimental in 2.54) replays commits without touching any tree or index. It is
tempting for pure reorders in a busy checkout, but it is marked unstable. Use it only after reading
its current man page, with `--ref-action=print`, so you can inspect the result before any ref
moves.

## 4. Recipes

All of these rebase onto the upstream, so only unpushed commits are replayed.

### Insert a fix commit right after the pushed red tip

```bash
GIT_SEQUENCE_EDITOR="sed -i '1i break'" git rebase -i origin/<branch>
# stops with HEAD at the upstream tip; make the fix, then:
git add <files> && git commit -m "<message>"
GIT_EDITOR=true git rebase --continue
```

### Insert a fix commit after an unpushed commit X

```bash
GIT_SEQUENCE_EDITOR="sed -i -E '/^(pick|p) <x7>/a break'" git rebase -i origin/<branch>
```

### Amend commit X in its own context

Use this when later commits touch the same lines. The edit happens with X's tree checked out, so
it can't collide with code that only exists later.

```bash
GIT_SEQUENCE_EDITOR="sed -i -E 's/^(pick|p) <x7>/edit <x7>/'" git rebase -i origin/<branch>
# stops after applying X
git add <files> && git commit --amend --no-edit
GIT_EDITOR=true git rebase --continue
```

### Amend X from the tip (fixup)

This is quicker when the fix overlaps nothing after X. On 2.54, `--autosquash` works without
`-i` (tested); on an older git, add `-i` with `GIT_SEQUENCE_EDITOR=:`.

```bash
git add <files> && git commit --fixup=<x>
git rebase --autosquash origin/<branch>
```

### Move a fix F earlier

To move it to just after the upstream tip:

```bash
GIT_SEQUENCE_EDITOR="sed -i -E -e '/^(pick|p) <f7>/d' -e '1i pick <f7>'" git rebase -i origin/<branch>
```

To move it to just after commit X:

```bash
GIT_SEQUENCE_EDITOR="sed -i -E -e '/^(pick|p) <f7>/d' -e '/^(pick|p) <x7>/a pick <f7>'" git rebase -i origin/<branch>
```

Predict trouble before moving anything. This read-only merge applies F onto its new parent
without touching the tree or index; exit 0 means clean and exit 1 means conflict:

```bash
git merge-tree --write-tree --quiet --merge-base=<f>^ <target> <f>
```

File overlap between F and the commits it jumps over (`git diff --name-only`) is only a hint. The
commits it jumps over are replayed on top of F afterwards, and they can conflict even when F lands
cleanly. A clean result still doesn't prove F builds in its new place, since it may use code the
skipped commits added, so check the new position locally when that is cheap.

### Split a fix out of a bundled commit B, then move it

First pass: split B into the fix and the remainder.

```bash
GIT_SEQUENCE_EDITOR="sed -i -E 's/^(pick|p) <b7>/edit <b7>/'" git rebase -i origin/<branch>
git reset HEAD^                     # B's changes return to the tree, unstaged
git add <fix files>                 # whole files; for partial hunks write a patch and
                                    # `git apply --cached <patch>` (git add -p is interactive)
git commit -m "<fix message>"
git add -A && git commit -C <b>     # -C reuses B's message, author and author date
GIT_EDITOR=true git rebase --continue
```

`git add -A` is safe here only because the tree held nothing but B's changes. That requires a
clean tree when done in place; a worktree guarantees it.

Second pass: move the new fix commit with the reorder recipe.

When the fix is woven through several commits, or depends on code they introduce, splitting stops
being clean. Batch-skip the span instead.

## 5. Conflicts and aborting

A conflict stops the rebase with `UU` paths in `git status`. When the resolution is mechanical,
resolve it, `git add` it, then run `GIT_EDITOR=true git rebase --continue`. Moving a commit past
several others can conflict once per commit it passes, so expect to repeat this. When a resolution
is not mechanical, `git rebase --abort` puts the branch back exactly at `<old>`, from any point.
Never end a turn with a rebase half done.

## 6. Verifying a rewrite

Before pushing anything rewritten:

```bash
git range-diff origin/<branch> <old> HEAD   # same as origin/<branch>..<old> vs origin/<branch>..HEAD
git diff <old> HEAD --stat                  # insert/amend: exactly the fix; pure reorder: empty
```

In range-diff, `=` marks a commit whose patch is unchanged, `!` one whose patch changed, and `<`
or `>` one that was removed or added. The tip diff is the hard check: a pure reorder must leave it
empty. Range-diff can show `!` even then, because a commit replayed after the moved one picks up
different diff context (the moved commit's lines now appear around its hunks). Read each `!` and
make sure only context lines moved; changed `+`/`-` lines mean content changed, so stop and look.

## 7. Rewriting pushed history (ask first)

Consider it when a standalone fix commit would read wrong, such as a trivial oversight in the red
commit R that ordinary local checks would have caught. Get a yes, then:

```bash
GIT_SEQUENCE_EDITOR="sed -i -E '1s/^(pick|p) /edit /'" git rebase -i <r>^
# the first todo line is R; it stops after applying it
git add <files> && git commit --amend --no-edit
GIT_EDITOR=true git rebase --continue
git push --force-with-lease=<branch>:<full old r sha> origin <new r sha>:refs/heads/<branch>
```

`--force-with-lease=<ref>:<expect>` overwrites only if the remote still holds `<expect>`, so a push
someone else made in the meantime makes this fail instead of being lost. The rewritten R runs CI
again, and deploys again if green pushes deploy.
