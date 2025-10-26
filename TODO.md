# Dotfiles Repository TODO

## Overview

## Critical

*Must be done first - establishes the groundwork for everything else*

### 1. Create Installation Workflow Documentation

- [ ] Document bootstrap process for fresh Linux installations
- [ ] Document bootstrap process for WSL installations
- [ ] Document bootstrap process for Windows (including Bun requirement)
- [ ] Create pre-flight checklist (required tools: Bun, age encryption, chezmoi, etc.)
- [ ] Document hook system (`.init_pre.ts`, `.update_pre.ts`) and when they execute
- [ ] Add troubleshooting guide for common installation issues
- [ ] Document the relationship between source directory and target directory
- [ ] Add step-by-step first-time setup guide
- [ ] Document Doppler setup requirements
- [ ] Create quick reference card for emergency recovery

### 2. Standardize Encryption & Authentication Workflow

- [ ] Document age encryption setup and key management
- [ ] Create guide for Doppler integration (project setup, config selection)
- [ ] Document secret rotation process and schedule
- [ ] Create checklist for adding new encrypted values
- [ ] Consolidate all authentication tokens/keys into Doppler or age-encrypted files
- [ ] Document age recipient key storage and backup strategy
- [ ] Add encryption troubleshooting guide
- [ ] Create template for encrypted file naming conventions
- [ ] Document `.private_*` file pattern usage
- [ ] Audit existing secrets and migrate to standard locations

### 3. Define Cross-Platform Strategy

- [ ] Create platform detection reference (`.data.wsl`, `.data.chassis`, OS checks)
- [ ] Document template conditionals strategy (when to use OS-specific blocks)
- [ ] Define Windows vs WSL separation of concerns
- [ ] Create decision matrix for which tools go where (Windows native vs WSL)
- [ ] Document how to handle tools that exist on both platforms
- [ ] Define file path conventions for cross-platform compatibility
- [ ] Create testing strategy for multi-platform templates
- [ ] Document Windows-specific quirks and workarounds

---

## Important

*Affects daily use and prevents frustration*

### 4. Refactor Shell Configuration Architecture

- [ ] Consolidate PATH modifications into single, organized section in `commonrc.sh.tmpl`
- [ ] Create modular PATH loading system (one block per tool with conditional checks)
- [ ] Separate PATH, environment variables, and shell completions into logical sections
- [ ] Document load order (`.bashrc` → `.zshrc` → `commonrc.sh` → `.bash_aliases`)
- [ ] Add comments explaining each tool's PATH modification
- [ ] Eliminate duplicate PATH additions
- [ ] Create standard pattern for conditional tool loading
- [ ] Add PATH deduplication function
- [ ] Move WSL-specific settings to dedicated section (currently at commonrc.sh.tmpl:109-114)
- [ ] Organize tool sections alphabetically or by category

### 5. Shell Completions Cleanup

- [ ] Audit all completion sources in `commonrc.sh.tmpl` (lines 17-22, 35-39, 71-76)
- [ ] Create consistent pattern for conditional completion loading
- [ ] Move completions to dedicated section (after PATH, before aliases)
- [ ] Add error handling for missing completion files
- [ ] Document which tools provide completions
- [ ] Test completions on bash and zsh
- [ ] Add completion loading performance optimization
- [ ] Document how to add new tool completions

### 6. Windows PATH Management

- [ ] Create PowerShell script for Windows-only PATH modifications
- [ ] Document registry-based PATH vs user PATH vs chezmoi-managed PATH
- [ ] Create chezmoi hook for Windows PATH synchronization
- [ ] Add validation script to check PATH consistency
- [ ] Document how to add new Windows PATH entries
- [ ] Create backup/restore mechanism for Windows PATH
- [ ] Add Windows environment variable management strategy
- [ ] Test PATH length limits on Windows

### 7. Core Tool Standardization

**mise (development environment manager):**
- [ ] Add mise configuration file (`.mise.toml` or per-project configs)
- [ ] Document mise setup and installation
- [ ] Integrate mise activation with shell configs
- [ ] Document tool version management strategy
- [ ] Add mise to installation checklist

**bun (JavaScript runtime):**
- [ ] Document global install location and version management
- [ ] Add shell completions configuration
- [ ] Document bun vs npm vs pnpm strategy
- [ ] Add bun to PATH in standard location
- [ ] Test bun hooks (`.init_pre.ts`, `.update_pre.ts`) on all platforms

**Windows Terminal:**
- [ ] Add `settings.json` template to dotfiles
- [ ] Document theme/color scheme customization
- [ ] Add font configuration (Cascadia Code, Nerd Fonts, etc.)
- [ ] Configure profiles for PowerShell, WSL, cmd
- [ ] Document Windows Terminal installation method

**Claude (AI assistant):**
- [ ] Document Claude configuration location
- [ ] Add API key management strategy (Doppler?)
- [ ] Add global CLAUDE.md to dotfiles
- [ ] Create project-specific CLAUDE.md templates
- [ ] Document sync strategy across machines

---

## Nice to Have

*Important for consistency across machines*

### 8. IDE Configuration Management

**VS Code:**
- [ ] Migrate `settings.json` to dotfiles (separate user vs workspace settings)
- [ ] Add `keybindings.json` template
- [ ] Create extensions list and installation script
- [ ] Document settings sync strategy (Settings Sync vs chezmoi)
- [ ] Add snippets directory
- [ ] Configure language-specific settings

**Cursor:**
- [ ] Determine config overlap with VS Code
- [ ] Manage Cursor-specific settings
- [ ] Add Cursor AI configuration
- [ ] Document Cursor vs VS Code decision criteria
- [ ] Add Cursor to installation workflow

**JetBrains:**
- [ ] Add IDE configs for IntelliJ IDEA
- [ ] Add IDE configs for PyCharm
- [ ] Add IDE configs for other JetBrains IDEs if needed
- [ ] Identify portable vs machine-specific settings
- [ ] Document plugin installation strategy
- [ ] Add color scheme/theme configuration

**Meta:**
- [ ] Create sync strategy for each IDE (template vs symlink vs manual)
- [ ] Document extension/plugin installation automation
- [ ] Add IDE version tracking and update strategy
- [ ] Create IDE-specific .gitignore patterns

### 9. AI Tools Configuration

- [ ] Audit existing AI tool configs (`.claude/`, API keys, prompts)
- [ ] Add Claude Desktop configuration to dotfiles
- [ ] Add Gemini configuration (if applicable)
- [ ] Document per-project vs global AI configurations
- [ ] Create templates for common AI prompts/rules
- [ ] Integrate API keys into Doppler
- [ ] Add model preferences and settings
- [ ] Document AI tool usage workflows

### 10. Development Tool Configurations

**Git:**
- [ ] Expand `.gitconfig.tmpl` (currently at `home/dot_config/git/config-ryan.tmpl`)
- [ ] Add git aliases and shortcuts
- [ ] Configure diff and merge tools
- [ ] Add commit signing configuration
- [ ] Configure git credential helpers
- [ ] Add git hooks templates

**Language Tools:**
- [ ] Add Python development configs (pyproject.toml templates, .python-version)
- [ ] Add Node.js configs (.npmrc, .nvmrc templates)
- [ ] Add Rust configs (rustfmt.toml, clippy settings)
- [ ] Add Go configs (go.env, tool preferences)
- [ ] Add language-specific linter/formatter configs

**Terminal Emulators:**
- [ ] Expand Kitty configuration (currently at `home/dot_config/kitty/`)
- [ ] Add Alacritty config (if used)
- [ ] Document terminal color schemes
- [ ] Add font configuration and Nerd Fonts setup

**Other Tools:**
- [ ] Expand Wakatime config (currently at `home/dot_wakatime.cfg.tmpl`)
- [ ] Add tmux configuration
- [ ] Add vim/neovim basic config
- [ ] Add SSH config template
- [ ] Add GPG configuration

### 11. Package Management Strategy

- [ ] Audit `run_onchange_install-packages.sh.tmpl`
- [ ] Separate Linux, WSL, and Windows package lists
- [ ] Create idempotent package installation scripts
- [ ] Document package dependencies and installation order
- [ ] Add Windows package manager integration (winget/chocolatey/scoop)
- [ ] Add Homebrew package list for Linux
- [ ] Add platform-specific package selection logic
- [ ] Test package installation on clean system
- [ ] Document manual installation steps for proprietary tools
- [ ] Add package version pinning strategy

---

## Polishing

*Nice-to-have improvements and quality of life*

### 12. Enhanced Documentation

- [ ] Expand README.md with comprehensive sections
- [ ] Add ARCHITECTURE.md explaining repo structure
- [ ] Create CONTRIBUTING.md for your future self
- [ ] Add inline comments to all template files
- [ ] Document all chezmoi hooks and their purposes
- [ ] Create REFERENCE.md with template variable reference
- [ ] Add FAQ.md with common issues and solutions
- [ ] Document chezmoi commands cheat sheet
- [ ] Add diagrams for config flow and file organization
- [ ] Create video walkthrough or screenshots

### 13. Quality of Life Improvements

- [ ] Add chezmoi status checking script (detect drift)
- [ ] Create update reminder system (terminal startup notification)
- [ ] Implement "unmanaged files" workflow (mentioned in README.md:48-54)
- [ ] Add fuzzy search for config diffs (fzf integration)
- [ ] Create backup/snapshot system before applying changes
- [ ] Add interactive configuration wizard for new machines
- [ ] Create script to list all managed files
- [ ] Add change preview before applying updates
- [ ] Create rollback mechanism
- [ ] Add performance profiling for shell startup time

### 14. Advanced Features from README "Intended Goals"

- [ ] rbw auto-lock implementation (8hr sessions) (README.md:58-60)
- [ ] Lock rbw when computer sleeps, idle, or screen saver activates
- [ ] GitHub language attributes script (pre-commit hook for .gitattributes) (README.md:71-74)
- [ ] Terminal startup checks for out-of-date configs (README.md:69-70)
- [ ] VS Code automatic setup improvements (README.md:68)
- [ ] Neovim configuration management (README.md:67)
- [ ] Add timer-based update checking with low-latency startup
- [ ] Implement device-specific configuration system (README.md:65)

### 15. Testing & Validation

- [ ] Create test script for fresh installation simulation
- [ ] Add validation for encrypted secrets (age decrypt test)
- [ ] Create pre-commit hooks for template syntax checking
- [ ] Add CI/CD for template validation (GitHub Actions)
- [ ] Document rollback procedures
- [ ] Create test matrix for different platforms (Linux/WSL/Windows)
- [ ] Add shellcheck integration for shell scripts
- [ ] Test hook execution on all platforms
- [ ] Validate Doppler integration
- [ ] Add template rendering test suite

### 16. Cleanup & Maintenance

- [ ] Commit deletion of removed files (nushell/env.nu, dot_gitconfig, etc.)
- [ ] Clean up commented-out code in commonrc.sh.tmpl (lines 122-130)
- [ ] Review and update `.chezmoiignore` patterns
- [ ] Audit and remove unused templates
- [ ] Standardize file naming conventions across repo
- [ ] Remove deprecated hooks (old shell-based hooks if fully migrated to TS)
- [ ] Archive old/unused configuration files
- [ ] Update .gitattributes for proper file type detection
- [ ] Clean up temporary/test files
- [ ] Reorganize directory structure if needed

---

## Legacy Items from Original TODO

These items were in the original TODO.md and need to be categorized/completed:

- [ ] **age executable not installed on init** - Add to Priority 1, Item 1 (installation docs)
- [ ] **rbw executable needs better configuration** - Related to Priority 4, Item 14 (rbw auto-lock)
- [ ] **rbw config not tracked** - Add rbw config to dotfiles
- [ ] **hishtory executable not available on init** - Add to installation checklist
- [ ] **add GPG key to bw** - Add to Priority 1, Item 2 (authentication workflow)
- [ ] **dracula theme tracking, kitty/micro** - Add to Priority 3, Item 10 (dev tools)
- [ ] **lazygit fix difftool** - Add lazygit config to dotfiles
- [ ] **testing in github codespaces** - Add to Priority 4, Item 15 (testing)

---

## Notes

- Items can be worked on in any order within their priority level
- Some items have dependencies (e.g., documentation should reflect implemented changes)
- Use `chezmoi cd` to navigate to source directory when working on configs
- Test changes in WSL/Linux before applying to Windows (or vice versa)
- Keep encrypted secrets out of git history - use age encryption or Doppler