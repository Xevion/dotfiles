# Meta-Configs Pattern

This repository uses a **meta-config pattern** for configuration files that drive imperative actions during `chezmoi apply`, rather than being deployed directly to the filesystem.

## What are Meta-Configs?

Meta-configs are configuration files that:

1. **Are NOT deployed** to the target system
2. **Drive scripts** that perform imperative actions (downloads, installations, modifications)
3. **Live in `meta/`** at the repository root (outside `home/`)
4. **Trigger `run_onchange_*` scripts** when their content changes

This pattern separates "what to configure" from "configuration files that get deployed."

## Directory Structure

```
chezmoi/
├── meta/                    # Meta-configs (NOT deployed)
│   ├── fonts.toml           # Drives font installation
│   └── ...                  # Future meta-configs
├── home/                    # Deployed to $HOME
│   ├── .config/
│   ├── .local/
│   └── ...
└── docs/
    └── meta-configs.md      # This file
```

## Current Meta-Configs

### `meta/fonts.toml`

Drives the `install-fonts.ts` script to download and install fonts.

```toml
[ui]
primary = "Inter"
fallback = "Noto Sans"

[mono]
primary = "Geist Mono"
fallback = "JetBrains Mono"

[extras]
# Fonts to install without category assignment
fonts = ["ZedMono NF"]
```

**Triggered by:** `run_onchange_after_install-fonts.sh.tmpl`

**Script:** `~/.local/bin/install-fonts.ts`

## How It Works

1. Edit a meta-config in `meta/` (e.g., `meta/fonts.toml`)
2. Run `chezmoi apply`
3. Chezmoi detects the file changed via `{{ include "../meta/fonts.toml" | sha256sum }}`
4. The corresponding `run_onchange_*` script executes
5. The script reads the meta-config and performs imperative actions

## Why This Pattern?

### Problem

Some configuration requires imperative actions:
- Downloading files from the internet
- Installing packages
- Modifying system state

These don't fit the declarative "deploy this file" model of chezmoi.

### Solution

Meta-configs provide:
- **Centralized configuration** - Easy to edit, version-controlled
- **Imperative execution** - Scripts perform the actual work
- **Change detection** - `run_onchange_*` only runs when config changes
- **Clear separation** - Meta-configs are clearly not "files to deploy"

## Adding a New Meta-Config

1. Create `meta/<name>.toml` with your configuration schema
2. Create a script in `home/dot_local/bin/` to process the config
3. Create `home/run_onchange_after_<name>.sh.tmpl` that:
   - Includes a hash comment: `# hash: {{ include "../meta/<name>.toml" | sha256sum }}`
   - Calls your processing script
4. Document the meta-config in this file

## Comparison with `.managed/`

| Aspect       | `meta/`                            | `.managed/`                             |
| ------------ | ---------------------------------- | --------------------------------------- |
| Deployed     | No                                 | Yes (via symlinks)                      |
| Purpose      | Drive imperative scripts           | Source of truth for app configs         |
| Consumed by  | Chezmoi scripts                    | Applications (via symlinks)             |
| Location     | Repo root                          | Inside `home/`                          |
