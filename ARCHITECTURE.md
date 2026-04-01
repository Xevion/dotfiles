# Architecture

This document describes how the chezmoi dotfiles repository is structured, how files are named, how templates work, and how the various subsystems fit together.

## Repository Layout

```
chezmoi/
├── home/                          # Source files (mirrors ~/)
│   ├── .chezmoi.toml.tmpl         # Chezmoi config: encryption, data vars, hooks
│   ├── .chezmoiignore             # Patterns chezmoi won't manage
│   ├── .chezmoiremove             # Target paths to delete on next apply
│   ├── .chezmoiexternal.toml      # External archive/file sources
│   ├── .chezmoitemplates/         # Shared partial templates (included by other templates)
│   │   ├── common-rules.md.tmpl   # Shared AI assistant rules (Claude Code + OpenCode)
│   │   ├── common-rules-minimal.md.tmpl  # Reduced ruleset for Gemini
│   │   ├── banner.tmpl            # "Managed by chezmoi" notice
│   │   ├── nushell/               # Shared Nushell config partials
│   │   └── scripts/               # Shared shell script partials
│   ├── .managed/                  # Config files managed as symlink targets
│   │   ├── cursor/                # Cursor IDE settings
│   │   ├── git/                   # Git delta config
│   │   ├── intellij/              # JetBrains keymap files
│   │   ├── lazygit/               # LazyGit config
│   │   ├── mise/                  # Mise tool version config
│   │   ├── share/                 # Shared data files
│   │   ├── vscode/                # VS Code settings (linux/windows variants)
│   │   └── zed/                   # Zed editor settings (linux/windows variants)
│   ├── hooks/                     # Pre-apply hooks (TypeScript via Bun)
│   │   ├── .init_pre.ts           # Runs on `chezmoi init`
│   │   └── .update_pre.ts        # Runs on `chezmoi update --init`
│   ├── dot_*/                     # Files deploying to ~/.<name>
│   ├── dot_config/                # Files deploying to ~/.config/
│   ├── dot_local/                 # Files deploying to ~/.local/
│   ├── private_dot_ssh/           # ~/.ssh/ (600 permissions, encrypted content)
│   ├── AppData/                   # Windows-only: %APPDATA% targets
│   └── Documents/                 # Windows-only: ~/Documents/ targets
├── docs/                          # Reference documentation
│   ├── platform-detection.md      # Template variable reference
│   └── symlink-patterns.md        # How .managed/ symlinks work
├── ARCHITECTURE.md                # This file
├── CLAUDE.md                      # AI assistant instructions for this repo
├── TODO.md                        # Ongoing task list
├── README.md                      # Overview and quick reference
└── FAQ.md                         # Common questions
```

## File Naming Conventions

Chezmoi transforms source file names into target paths using prefixes and suffixes:

| Source pattern | Target result | Notes |
|---|---|---|
| `dot_foo` | `.foo` | Dot-prefixed file |
| `dot_config/bar` | `.config/bar` | Directory |
| `private_dot_ssh/` | `.ssh/` | Directory with 600 permissions |
| `foo.tmpl` | `foo` | Template rendered before deploy |
| `encrypted_foo.age` | `foo` | Age-encrypted, decrypted on deploy |
| `symlink_foo.tmpl` | `foo` → (symlink) | Template containing a path |
| `run_onchange_*.sh` | (executed on hash change) | Script, not a file target |
| `run_onchange_before_*` | (executed before apply) | Order: before → apply → after |
| `executable_foo` | `foo` (chmod +x) | Sets executable bit |
| `remove_foo` | (deletes `foo` on apply) | Empty file, name matters |

Multiple modifiers stack: `private_encrypted_dot_netrc.age` → `~/.netrc` with 600 perms, decrypted from age.

## Template System

### Regular Templates

Files ending in `.tmpl` are Go templates rendered at apply time. They have direct access to chezmoi's context:

```
{{ .chezmoi.os }}          # "linux", "darwin", "windows"
{{ .chezmoi.homeDir }}     # e.g. "/home/xevion"
{{ .chezmoi.sourceDir }}   # e.g. "/home/xevion/.local/share/chezmoi/home"
{{ .wsl }}                 # bool: true when running inside WSL
{{ .chassis }}             # "laptop" or "desktop"
{{ .cpu.cores }}           # int: physical CPU cores
{{ .cpu.threads }}         # int: logical CPU threads
```

### Partial Templates (`.chezmoitemplates/`)

Partials are reusable fragments included via `{{ template "name.tmpl" . }}`. They **do not** have automatic access to chezmoi context — the calling template passes it explicitly as a parameter (conventionally called `data`). Inside a partial, access everything through that parameter:

```
{{ .data.chezmoi.os }}     # OS (inside a partial)
{{ .data.wsl }}            # WSL flag (inside a partial)
{{ .data.chassis }}        # chassis type (inside a partial)
```

Calling a partial from a regular template:
```
{{ template "common-rules.md.tmpl" . }}
```

The `.` passes the entire chezmoi context as the `data` parameter.

### Platform Conditionals

Standard pattern for platform-specific blocks:

```
{{- if and (eq .chezmoi.os "windows") (not .wsl) }}
# Windows-only (not WSL)
{{- else if .wsl }}
# WSL-only
{{- else if eq .chezmoi.os "linux" }}
# Native Linux
{{- else if eq .chezmoi.os "darwin" }}
# macOS
{{- end }}
```

See [docs/platform-detection.md](docs/platform-detection.md) for the full reference.

## The `.managed/` Pattern

Some config files need to be editable in-place without running `chezmoi apply` (e.g., IDE settings edited via GUI). These use the symlink pattern:

1. **Source**: `home/.managed/<tool>/config.toml` — the actual config, tracked by git
2. **Symlink template**: `home/dot_config/<tool>/symlink_config.toml.tmpl` — contains the target path
3. **Deployed result**: `~/.config/<tool>/config.toml` → symlink → chezmoi source file

Edits made by the application write through the symlink directly to the source file, so they're immediately tracked by git without needing `chezmoi re-add`.

> **Note**: Glob does not search hidden directories. Use `ls -la home/.managed/` to list managed subdirectories. Known subdirs: `cursor`, `git`, `intellij`, `lazygit`, `mise`, `share`, `vscode`, `zed`.

## Secret Management

Two complementary approaches:

### Age Encryption
- Encrypt files at rest in the repo using age public key
- Key: `age1s3ctpj9lafl6qwyvd89sn448us7gdzd53d8yyhsc7zny78c0k4sqerrkze`
- Identity file bootstrapped from Doppler during `chezmoi init`
- Use for files that are inherently secret (SSH config, API keys embedded in configs)
- File naming: `encrypted_<name>.age` or `private_encrypted_<name>.age`

### Doppler Integration
- Runtime secret injection via `{{ dopplerProjectJson.KEY_NAME }}`
- Project: `dotfiles`, config: `production`
- Use for secrets referenced in templates (tokens, passwords)
- The `private_key.txt.tmpl` bootstraps the age identity key from Doppler

### Decision Matrix

| Secret type | Approach |
|---|---|
| Entire file is sensitive (SSH config, API keys file) | Age-encrypted file |
| Secret value embedded in a config template | Doppler variable |
| Key/credential that unlocks other secrets | Age + Doppler (age key from Doppler) |

## Hooks

Hooks are TypeScript scripts run by Bun before chezmoi applies changes:

| Hook | Trigger | Purpose |
|---|---|---|
| `hooks/.init_pre.ts` | `chezmoi init` | Fetch age key from Doppler, write to `~/key.txt` |
| `hooks/.update_pre.ts` | `chezmoi update --init` | Same as init (workaround for `--init` flag behavior) |

`run_onchange_*` scripts run during apply when their content hash changes:

| Script | When | Purpose |
|---|---|---|
| `run_onchange_install-packages.sh.tmpl` | Package list changes | Install apt/brew packages |
| `run_onchange_after_fisher-update.sh.tmpl` | Fisher plugin list changes | Install Fish plugins |
| `run_onchange_after_install-fonts.sh.tmpl` | Font list changes | Install Nerd Fonts |
| `run_onchange_before_setup-wsl-gpg.sh.tmpl` | GPG setup script changes | Symlink Windows GPG into WSL |
| `run_onchange_before_setup-intellij-keymaps.ts.tmpl` | Keymap files change | Symlink JetBrains keymaps |

## AI Assistant Configuration

AI assistant config is split between tool-specific and shared:

```
home/dot_claude/
├── CLAUDE.md.tmpl          # Claude Code: includes common-rules + subagent guidance
├── settings.json.tmpl      # Claude Code settings
├── agents/                 # Custom subagent definitions
├── skills/                 # Skill definitions
├── commands/               # Slash command definitions (shared with OpenCode via symlinks)
└── hooks/                  # Claude Code hooks

home/dot_config/opencode/
├── AGENTS.md.tmpl          # OpenCode: includes common-rules + OpenCode-specific rules
├── opencode.jsonc.tmpl     # OpenCode settings
└── command/                # Symlinks → ../../../dot_claude/commands/*.md

home/.chezmoitemplates/
├── common-rules.md.tmpl    # Shared rules (used by Claude + OpenCode)
└── common-rules-minimal.md.tmpl  # Reduced ruleset (used by Gemini)
```

When adding a new slash command, create the definition in `dot_claude/commands/` and add a `symlink_<name>.md` in `dot_config/opencode/command/` pointing to it.

## Stale File Cleanup

Chezmoi does **not** automatically remove target files when a source file is deleted. Use `home/.chezmoiremove` to declare target paths for removal:

```
# .chezmoiremove
.config/fish/conf.d/fzf.fish
.config/opencode/agent/**
```

After adding an entry, `chezmoi diff` will show the pending removal. Always confirm before adding to `.chezmoiremove` since removal is destructive.

## Platform Coverage

| Platform | Shell | Notes |
|---|---|---|
| Linux (native) | bash, fish, nushell | Primary development environment |
| WSL2 | bash, fish, nushell | Windows-integrated: GPG, PATH, etc. |
| Windows | PowerShell, nushell | AppData targets, Windows Terminal |
| macOS | bash, fish | Less actively maintained |
