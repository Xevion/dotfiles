---
name: disk-reclaim
description: Find and free up disk space safely. Auto-activate when the user asks about freeing space, a full disk, "what's eating my storage", "out of space", or where their gigabytes went. Scan once with duc, then query the cached index — never re-run du or ncdu.
---

# Disk Reclaim

## Core principle

**Scan the filesystem once. Query the cached result many times.**

A naive cleanup re-reads the disk for every question (`du -sh this`, `ncdu that`, `du -sh the-other`). On a large or slow disk that's minutes of I/O per question. Instead, build a single index with `duc` and answer every follow-up from that cached database — instantly, no re-reads.

> Never reach for `du -sh` or `ncdu` here. If you catch yourself re-scanning a subtree, you already have it in the index — query it.

## 1. Scan once

```sh
duc index -x -p -d /path/to/scan.duc /
```

- `-x` stays on one filesystem (won't wander into network mounts or other drives).
- `-p` shows progress.
- `-d` is the database path. **Store it off the disk you're cleaning** (another drive, `/tmp` if it has room) so the index itself doesn't compete for the last free bytes.

Confirm it landed:

```sh
duc info -d /path/to/scan.duc
```

## 2. Inspect from the cache

```sh
duc ls   -d scan.duc -F /              # top-level, sorted-ish
duc ls   -d scan.duc -F /some/dir | sort -rh -k1   # force descending
duc ui   -d scan.duc                   # interactive browse, still no re-scan
```

**The one gotcha — apparent vs real size.** `--apparent` reports logical file size; without it you get actual blocks on disk. They diverge hard on **sparse files** (VM/container disk images, database files). A "32G" image may occupy 11G. Decide what to delete based on **real** usage; compare against `--apparent` to detect sparseness.

**Hardlinks/dedup.** `duc` (like `du`) counts a shared inode once. If two trees look big but are hardlinked, you are *not* double-paying. Verify with the link count (`ls -l` — the number after permissions) before assuming duplication or "fixing" it.

## 3. Triage by category

| Category | Examples | Treatment |
|---|---|---|
| Package/build caches | apt, npm, bun, pip, cargo, flatpak caches | **Safe** — clear; they regenerate |
| Build artifacts | `target/`, `node_modules/`, `build/`, `dist/` | Rebuildable — delete freely, expect regrowth |
| VM / container images | qcow2, `.img`, Docker Desktop VM | Check sparseness first; `prune` inside before deleting the image |
| Toolchain / runtime managers | mise, flatpak, snap | Prune **unused** versions/runtimes, not the live ones |
| Backup pileups | timestamped snapshots accumulating | Keep newest N, delete the rest |
| Hard-to-replace data | datasets, built databases, archives | **Compress, don't delete** — see below |
| Filesystem reserve | ext4 reserved blocks (default 5%) | `tune2fs -m` to lower on data disks |

### Compress hard-to-replace data instead of deleting

For data that took real effort to produce and would be painful to rebuild — but you rarely touch — compress it in place rather than moving or deleting it. Modern heavy compression buys large savings on most structured/columnar data:

```sh
zstd --long=27 -19 bigfile            # strong, still reasonable speed
zstd --ultra -22 --long=27 bigfile    # maximum zstd ratio (slow, RAM-hungry)
# decompress later: zstd -d bigfile.zst
```

Keep the source of truth, recover the space, expand on demand. Verify the `.zst` before removing the original.

### Filesystem reserved blocks

ext4 reserves ~5% for root by default — real space that's invisible and unusable to your user. On a data/home disk you don't need that cushion:

```sh
sudo tune2fs -m 1 /dev/sdXN           # 5% -> 1%; reclaims meaningful space on large disks
```

This is non-destructive. Leave the default on the root/system disk if you want emergency headroom for root processes.

## 4. Safety tiers — act in this order

1. **Safe** — package caches, build artifacts, reserved-block reduction. No data loss, no decision needed.
2. **Low-risk** — old backups beyond newest N, unused toolchain versions/runtimes, crash/trace dirs.
3. **Decision-required** — datasets, games, anything the user might miss. **Always show the user the exact targets and a dry-run first. Never bulk-delete without confirmation.**

Prefer showing `duc ls` of a subtree + the proposed command over deleting blind. Delete in tiers, smallest-risk first.

## 5. Clean up

The index is a normal file. Re-query it anytime, or remove it when done:

```sh
duc info -d scan.duc      # re-check / re-query later
rm scan.duc               # done
```
