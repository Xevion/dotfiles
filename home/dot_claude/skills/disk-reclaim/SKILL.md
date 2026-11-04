---
name: disk-reclaim
description: Find and free up disk space safely. Auto-activate when the user asks about freeing space, a full disk, "what's eating my storage", "out of space", or where their gigabytes went. Drill down with gdu, answer analytical questions from a DuckDB index, never re-run du or ncdu.
---

# Disk Reclaim

## Core principle

**Pay for the scan once, then ask many questions cheaply.** Two tools, each for what it is fast at:

| Question | Tool | Cost |
|---|---|---|
| "What is big under X?" (drill down, fresh numbers) | `gdu` live | 1.7 s per 1.6M entries, 11 s for all of `/` |
| "Biggest files / by extension / by age / across trees" | DuckDB index | 5 s to build per 1.4M files, 30-270 ms per query |

Measured and rejected, do not retry:

- **JSON output queried with jq, `pdu --json-input` or `gdu -f`**: 2-4 s per query and 1.2-1.6 GiB RAM at 1.6M entries.
- **`pdu`**: slower than gdu and it lists a hardlink in every directory it appears in, so per-directory numbers are inflated (`-H` fixes only the grand total).
- **`gdu -D file.sqlite`**: the database is good (parent_id tree, precomputed usage, 90 ms queries) but writing it takes minutes against 1.7 s for a normal scan. Rebuilding it per session is not viable.
- **Parallel find by directory**: only 1.9x, a few huge directories dominate.
- `du`, `ncdu`, `duc`: no reason to use them here.

## 1. Drill down with gdu

```sh
gdu -n -p -x --no-prefix --no-color --depth 1 /some/dir
```

- `-n` non-interactive, `-p` plain text (no progress bar), `-x` one filesystem.
- `--no-prefix` prints raw bytes, which sort and diff cleanly.
- `--depth N` limits printed levels. Start at 1, descend into the heavy child.
- Add `--min-age`/`--max-age`/`--since`/`--until` to look at only old or recent files, `-t N` for the top N.
- Hardlinks are counted once. Real (block) usage is reported, not apparent size.
- Run under `pkexec` for root-only trees (`/var/lib/docker`, `/root`). A user-run scan silently omits them.

Re-scan the subtree you are about to act on: it is seconds, and the numbers then reflect what `df` will feel.

## 2. Build the index for analytical questions

```sh
~/.claude/skills/disk-reclaim/index.sh / /mnt/storage/scan.duckdb
```

- The second argument must live **off the disk being cleaned** (the index is ~1 GB for 8M entries).
- It runs `find -xdev -printf` into a temporary TSV, loads it into DuckDB, and removes the TSV. Scan errors (permission denied) go to `<db>.errors.log`; read that file.
- `files` columns: `dev, ino, nlink, type, size, bytes, mtime, path`. `bytes` is real blocks (`%b * 512`), `size` is apparent. Hardlinked inodes appear once.
- Two persistent macros ship in the file: `kids(dir)` and `under(path)`.

**Reconcile before trusting it.** Index total and `df` used should roughly agree. Over-count: hardlinks (this index already dedupes). Under-count: unreadable directories, the usual blind spot, so check the errors log and re-run as root for `/var/lib/docker`. If they do not reconcile, find out why before recommending deletions. Expect small drift against a live gdu scan on busy trees.

### Query it

Run with `duckdb -readonly DB -c "..."`. In agent mode DuckDB prints a compact markdown table; do not pipe through `head`.

```sql
-- children of a directory, biggest first (bytes, human size, file count)
SELECT * FROM kids('/home/xevion/projects') LIMIT 20;

-- total under a path
SELECT * FROM under('/home/xevion/.cache');

-- biggest files
SELECT format_bytes(bytes::BIGINT) h, path FROM files WHERE type = 'f' ORDER BY bytes DESC LIMIT 30;

-- by extension (empty extension means no extension)
SELECT lower(regexp_extract(path, '\.[^./]+$')) ext, format_bytes(sum(bytes)::BIGINT) h, count(*) n
FROM files WHERE type = 'f' GROUP BY 1 ORDER BY sum(bytes) DESC LIMIT 20;

-- old and large (not touched in 180 days)
SELECT format_bytes(bytes::BIGINT) h, mtime::date d, path
FROM files WHERE type = 'f' AND mtime < now() - interval 180 day ORDER BY bytes DESC LIMIT 30;

-- every directory of one name (all target/ build trees), biggest first
SELECT regexp_extract(path, '^(.*?/target)(/|$)', 1) AS dir, format_bytes(sum(bytes)::BIGINT) h
FROM files WHERE regexp_matches(path, '/target(/|$)') GROUP BY 1 ORDER BY sum(bytes) DESC LIMIT 20;
```

Notes for writing your own queries:

- `sum(bytes)` is HUGEINT; cast `::BIGINT` before `format_bytes`.
- Filter a subtree with `starts_with(path, '/dir/')`, never `LIKE '/dir%'` (matches siblings such as `/dir2`).
- A directory's own row has `type = 'd'` and only its block size; use `kids`/`under` for recursive totals.
- Paths containing a tab or newline break the TSV parse; `ignore_errors` drops those rows silently, which is negligible but real.

## 3. Gotchas that distort numbers

- **Apparent vs real size.** Sparse files (VM and container images, databases) report a large logical size and occupy far less. Decide on real usage. gdu shows real by default; `size` in the index is apparent for comparison.
- **Hardlinks.** Deleting one link frees nothing until the last link goes, so a tree can vanish from the index while `df` barely moves. This matters for `uv`/`pnpm`/Nix stores and Steam/game installs. To size a tree the way `df` will feel it, sum only single-link files: `find DIR -type f -links 1 -printf '%s\n'`. Trace survivors with `find ROOT -inum N`.
- **Root-owned bind mounts** (Docker volumes) cannot be removed by the user; delete them through a container (`docker run --rm -v DIR:/x IMAGE rm -rf /x/...`).

## 4. Triage by category

| Category | Examples | Treatment |
|---|---|---|
| Package/build caches | apt, npm, bun, pip, cargo, flatpak caches | **Safe**, they regenerate |
| Build artifacts | `target/`, `node_modules/`, `build/`, `dist/` | Rebuildable, delete freely, expect regrowth |
| VM / container images | qcow2, `.img`, Docker Desktop VM | Check sparseness first; `prune` inside before deleting the image |
| Toolchain / runtime managers | mise, flatpak, snap, rustup | Prune **unused** versions, not live ones |
| Backup pileups | timestamped snapshots | Keep newest N |
| Hard-to-replace data | datasets, built databases, archives | **Compress, do not delete** |
| Filesystem reserve | ext4 reserved blocks (default 5%) | `tune2fs -m` on data disks only |

### Compress hard-to-replace data instead of deleting

```sh
zstd --long=27 -19 bigfile            # strong, reasonable speed
zstd --ultra -22 --long=27 bigfile    # maximum ratio, slow, RAM-hungry
# decompress later: zstd -d bigfile.zst
```

Verify the `.zst` before removing the original.

### Filesystem reserved blocks

```sh
sudo tune2fs -m 1 /dev/sdXN           # 5% -> 1%, non-destructive
```

Leave the default on the root/system disk if you want emergency headroom.

## 5. Safety tiers, act in this order

1. **Safe**: package caches, build artifacts, reserved-block reduction.
2. **Low-risk**: old backups beyond newest N, unused toolchain versions, crash/trace dirs.
3. **Decision-required**: datasets, games, anything the user might miss. **Show the exact targets and a dry run first. Never bulk-delete without confirmation.** Never delete logs, screenshots, captures or exports.

Prefer showing the `kids()` listing and the proposed command over deleting blind. Remember that package-manager removals (apt, brew) can pull or drop large dependency chains: simulate (`apt -s`, `brew uninstall --dry-run`) and report what leaves.

## 6. Clean up

The index and `<db>.errors.log` are ordinary files. `rm` them when the session's cleanup is done.
