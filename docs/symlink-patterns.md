# Symlink Patterns

This repository uses four distinct symlink patterns for different use cases.

## 1. Managed Configs (`.managed/`)

**Use for**: Config files that are auto-updated by tools OR shared across multiple platforms

**Location**: `chezmoi/home/.managed/<app>/<file>`  
**Template syntax**: `{{ .chezmoi.sourceDir }}/.managed/<app>/<file>`  
**Important**: `.managed` is listed in `home/.chezmoiignore` to prevent copying to home

**Examples**:
```
~/.config/mise/config.toml → .managed/mise/config.toml
~/.config/lazygit/config.yml → .managed/lazygit/config.yml
~/.config/Code/User/settings.json → .managed/vscode/settings.linux.json
~/AppData/Roaming/Cursor/User/settings.json → .managed/cursor/settings.windows.json
```

**How it works**: 
1. Tool modifies symlink target: `~/.config/mise/config.toml`
2. Changes write to: `chezmoi/.managed/mise/config.toml`
3. Run `chezmoi cd && git add .managed/ && git commit` to track changes
4. Syncs across machines via chezmoi

**Why**: Prevents source file from being copied to home. Tools can modify configs and changes are automatically tracked in the chezmoi repo.

---

## 2. Root-Level Configs

**Use for**: Simple auto-updated files at home root that don't need multi-location sharing

**Location**: `chezmoi/home/<filename>` (+ entry in `.chezmoiignore`)  
**Template syntax**: `{{ .chezmoi.sourceDir }}/<filename>`

**Examples**:
```
~/.claude/settings.json → claude-settings.json
Build/validation scripts in hooks/ directory
```

**Why**: Simpler than `.managed/` for single-purpose files at home root. Same principle (ignored source, symlinked target), just different organization.

---

## 3. Internal Code Sharing

**Use for**: Sharing code/docs between locations within this repo (DRY principle)

**Location**: Varies (typically special directories like `.claude/`)  
**Template syntax**: Relative paths like `../../../.claude/commands/file.md`

**Examples**:
```
~/.config/opencode/command/commit-staged.md → ~/.claude/commands/commit-staged.md
~/.config/opencode/AGENTS.md → ~/.claude/CLAUDE.md
```

**Why**: Single source of truth for shared commands/documentation. Changes in one location automatically reflected in all symlinked locations.

---

## 4. External Symlinks

**Use for**: Links to files outside home directory (cross-filesystem, WSL → Windows)

**Location**: External absolute paths  
**Template syntax**: Absolute paths like `/mnt/c/Users/<user>/AppData/...`  
**Conditional**: Usually wrapped in OS/environment checks

**Examples**:
```
~/.gnupg → /mnt/c/Users/Xevion/AppData/Roaming/gnupg  (WSL only)
```

**Why**: Access Windows configs from WSL without duplication. Shares actual Windows application state with WSL environment.
