# Dotfiles Repository TODO

## Overview

## Critical

*Must be done first - establishes the groundwork for everything else*

### 1. Create Installation Workflow Documentation

- [ ] Document bootstrap process for fresh Linux installations
- [ ] Document bootstrap process for WSL installations
- [ ] Document bootstrap process for Windows (including Bun requirement)
- [ ] Create pre-flight checklist (required tools: Bun, age encryption, chezmoi, etc.)
- [x] Document hook system (`.init_pre.ts`, `.update_pre.ts`) and when they execute
- [ ] Add troubleshooting guide for common installation issues
- [ ] Document the relationship between source directory and target directory
- [ ] Add step-by-step first-time setup guide
- [ ] Document Doppler setup requirements
- [ ] Create quick reference card for emergency recovery
- [ ] Document proper process of adding symlinks (mention updating chemzoiignore, etc.)

### 2. Standardize Encryption & Authentication Workflow

- [ ] Document age encryption setup and key management
- [ ] Create guide for Doppler integration (project setup, config selection)
- [ ] Document secret rotation process and schedule
- [ ] Create checklist for adding new encrypted values
- [x] Consolidate all authentication tokens/keys into Doppler or age-encrypted files (R2 and private domain migrated)
- [ ] Document age recipient key storage and backup strategy
- [ ] Add encryption troubleshooting guide
- [ ] Create template for encrypted file naming conventions
- [ ] Document `.private_*` file pattern usage
- [ ] Audit existing secrets and migrate to standard locations

### 3. Define Cross-Platform Strategy

- [x] Create platform detection reference (`.data.wsl`, `.data.chassis`, OS checks)
- [ ] Document template conditionals strategy (when to use OS-specific blocks)
- [ ] Define Windows vs WSL separation of concerns
- [ ] Create decision matrix for which tools go where (Windows native vs WSL)
- [ ] Document how to handle tools that exist on both platforms
- [ ] Define file path conventions for cross-platform compatibility
- [ ] Create testing strategy for multi-platform templates
- [x] Document Windows-specific quirks and workarounds (WSL GPG integration documented)

---

## Important

*Affects daily use and prevents frustration*

### 4. Refactor Shell Configuration Architecture

- [x] Consolidate PATH modifications into single, organized section in `commonrc.sh.tmpl`
- [x] Create modular PATH loading system (one block per tool with conditional checks)
- [x] Separate PATH, environment variables, and shell completions into logical sections
- [x] Document load order (`.bashrc` → `commonrc.sh` → `.bash_aliases`)
- [x] Add comments explaining each tool's PATH modification
- [x] Eliminate duplicate PATH additions
- [x] Create standard pattern for conditional tool loading
- [x] Add PATH deduplication function (using pnpm's case pattern)
- [x] Move WSL-specific settings to dedicated section (Section 7 in commonrc.sh.tmpl)
- [x] Organize tool sections by category (Core → Languages → Package Managers → Dev Tools)
- [x] Add mise integration as primary version manager with legacy fallbacks
- [x] Sync tool order between Bash and Fish configs for consistency
- [x] Move SDKMAN from .bashrc to commonrc.sh for centralization
- [x] Remove duplicate keychain initialization from .bashrc

### 5. Shell Completions Cleanup

- [x] Audit all completion sources in `commonrc.sh.tmpl`
- [x] Create consistent pattern for conditional completion loading
- [x] Move completions to dedicated section (Section 5 in commonrc.sh.tmpl)
- [x] Add error handling for missing completion files
- [x] Document which tools provide completions (hishtory, asdf, chatgpt)
- [ ] Test completions on bash (requires actual shell testing by user)
- [x] Add completion loading comments for performance awareness
- [x] Document how to add new tool completions (follow Section 5 pattern)

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
- [x] Add mise configuration file (`.mise.toml` or per-project configs)
- [ ] Document mise setup and installation
- [x] Integrate mise activation with shell configs
- [x] One installer per tool: duplicate brew/cargo/standalone copies of mise-managed tools removed (railway, lsd, zoxide, bun, deno, pnpm, delta, lazygit, bat, fd, ripgrep, starship, atuin, doppler, eza, hyperfine and others)
- [ ] Upgrade policy: `latest` pins never move on their own (36 tools were behind); decide on a periodic `mise upgrade` or an apply hook
- [x] Remaining cargo-installed tools moved to the mise `cargo:` backend; `kache`, `lspmux` and `xremap` stay in cargo (systemd units and wiring point at `~/.cargo/bin`), as do `cargo-update` and the path or git installs
- [x] `cargo.binstall = false` in the mise config: prebuilt cargo binaries need a newer glibc than this host has (apkeep, cargo-mutants, trunk broke, cavif installed empty), so `cargo:` tools compile on install and upgrade
- [ ] Duplicate-binary check in `meta/` that flags one name resolving to two installers
- [ ] Shims were measured and rejected: ~22 ms per call after updating mise (was ~90 ms), against ~1.6 ms direct
- [ ] Document tool version management strategy
- [ ] Add mise to installation checklist

**bun (JavaScript runtime):**
- [ ] Document global install location and version management
- [ ] Add shell completions configuration
- [ ] Document bun vs npm vs pnpm strategy
- [ ] Add bun to PATH in standard location
- [ ] Test bun hooks (`.init_pre.ts`, `.update_pre.ts`) on all platforms

**Windows Terminal:**
- [x] Add `settings.json` template to dotfiles
- [ ] Document theme/color scheme customization
- [ ] Add font configuration (Cascadia Code, Nerd Fonts, etc.)
- [ ] Configure profiles for PowerShell, WSL, cmd
- [ ] Document Windows Terminal installation method

**Claude (AI assistant):**
- [ ] Document Claude configuration location
- [ ] Add API key management strategy (Doppler?)
- [x] Add global CLAUDE.md to dotfiles
- [ ] Create project-specific CLAUDE.md templates
- [ ] Document sync strategy across machines
- [x] Consolidate rules/permissions across projects (2026-09-16 audit, below)

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
- [x] Expand `.gitconfig.tmpl` (currently at `home/dot_config/git/config-ryan.tmpl`)
- [x] Add git aliases and shortcuts
- [x] Configure diff and merge tools
- [x] Add commit signing configuration
- [ ] Configure git credential helpers
- [ ] Add git hooks templates

**Language Tools:**
- [ ] Add Python development configs (pyproject.toml templates, .python-version)
  - [x] `~/.config/uv/uv.toml` — python-preference=managed, compile-bytecode
  - [x] Python CLIs declared in mise `[tools]` via `pipx:` backend (extras survive reinstall)
- [x] Add Node.js configs (.npmrc) — fund=false via modify script (preserves authTokens)
- [ ] Add Rust configs (rustfmt.toml, clippy settings)
  - [x] `~/.cargo/config.toml` — mold linker on Linux, net.retry=3
- [x] Add Go configs (`~/.config/go/env`) — GOBIN → ~/.local/bin
- [x] Add Ruby configs (`~/.gemrc`) — --no-document
- [ ] Add language-specific linter/formatter configs

**Terminal Emulators:**
- [ ] Expand Kitty configuration (currently at `home/dot_config/kitty/`)
  - [x] Two-stage path pickers (`ctrl+shift+d` hints, `ctrl+shift+space` fzf list) via `screen_paths.py`
  - [x] Restore native `ctrl+shift+e`/`x`/`u` and the `ctrl+shift+p` hints chord (scratch pad → `ctrl+alt+p`)
  - [ ] Go rewrite of the pickers in `src/screen-pick`: one overlay per binding with its own labels and action menu (no hints kitten, no fzf), and URL/host/SHA detection
    - [x] Detection test corpus (`detect/detect_test.go`, fake filesystem)
    - [x] Detection engine to green (direct checks: screen directories, process/root/window bases, Wine prefixes)
    - [x] Streaming resolution: git-index suffix search, on-demand deep walk
    - [x] TUI, kitty bindings, build script
    - [x] Layout pass: side detail pane when wide, inline expansion when narrow, hints menu away from its target, Nerd Font icons with a legend, grouped menu with file previews
    - [x] Type-aware actions (no editor for binaries; Wine, IDA, Ghidra), repo root tab and `git log`, `git show` run in the checkout that has the commit
    - [x] Picking among copies is per-menu and never rewrites the match; a disk search only adds copies
    - [ ] Check the overlay inside real kitty: `KITTY_LISTEN_ON` reaching it, `@active-kitty-window-id` substitution
    - [ ] Retire `screen_paths.py` and the `kitty-paths` shim once the new pickers have been lived with
- [ ] Add Alacritty config (if used)
- [ ] Document terminal color schemes
- [ ] Add font configuration and Nerd Fonts setup

**Other Tools:**
- [ ] Expand Wakatime config (currently at `home/dot_wakatime.cfg.tmpl`)
- [ ] Add tmux configuration
- [ ] Add vim/neovim basic config
- [ ] Add SSH config template
- [x] Add GPG configuration
- [x] Add earlyoom configuration (upstream v1.9.0 built to `/usr/local/bin`, `--sort-by-rss`, systembus-notify relay for desktop notifications)
- [ ] Replace earlyoom with a Rust service in `~/projects`. Requirements, in priority order:
  - [ ] Config file, never argv — earlyoom takes all config as command-line flags, so its `--prefer`/`--avoid`/`--ignore` regexes sit in `/proc/<pid>/cmdline` and every `pgrep -f java` matches the daemon. Not fixable upstream: build tools are transient shell children, so the policy cannot move to systemd unit properties the way `--ignore` partly can via `OOMScoreAdjust=-1000`
  - [ ] PSI triggers — `poll()` on `/proc/pressure/memory` instead of polling `MemAvailable`, which measures stall time rather than estimating headroom
  - [ ] Split monitor and notifier over a pre-connected socket, so the kill path never allocates, forks, or blocks on a thrashing notification daemon
  - [ ] Post-kill cooldown — wait for reclaim before re-evaluating. earlyoom has none, which is why one spike on 2026-09-01 escalated into 45 signals across 25 processes in 8 seconds
  - [ ] Warn before killing, not only after; the useful notification fires while there is still headroom to render it

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

- [x] Expand README.md with comprehensive sections (Major refactor completed)
- [x] Add ARCHITECTURE.md explaining repo structure
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
- [x] Volatile-setting reconciler for app-written configs (`meta/reconcile.ts`): git clean filter for symlinked files, `modify_` for OBS
- [ ] Decide which btop keys are volatile (`shown_boxes`, `proc_sorting`, `proc_reversed`, `show_disks` flip from the UI)
- [ ] YAML adapter for the reconciler, once lazygit grows a volatile key
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

- [x] Convert removed files to templates (nushell/env.nu → env.nu.tmpl, dot_gitconfig → dot_gitconfig.tmpl)
- [x] Clean up commented-out code in commonrc.sh.tmpl (removed dead zsh/xset/todoist code)
- [x] Review and update `.chezmoiignore` patterns
- [x] Audit and remove unused templates
- [ ] Standardize file naming conventions across repo
- [x] Remove deprecated hooks (old shell-based hooks if fully migrated to TS) - Migrated to TypeScript
- [ ] Archive old/unused configuration files
- [ ] Update .gitattributes for proper file type detection
- [ ] Clean up temporary/test files
- [ ] Reorganize directory structure if needed

---

## AI Configuration Consolidation

Audit run 2026-09-16 across ~100 projects, 8,189 recorded sessions, and 85 project settings files.

**Done:**
- [x] Prune project permission bloat — 2,746 entries down to 1,901 (845 removed: 23 malformed shell fragments, 11 stale tool names, 811 already granted globally)
- [x] Fix `Read(/tmp/**)`/`Edit(/tmp/**)` in `meta/permissions.ts` — a single leading slash anchors to the settings file's directory, not the filesystem root, so these never matched. Now `//tmp/**`
- [x] Fix the guard hook grant — Bash rules match command text literally, so the `~/` form never matched an absolute-path invocation. Both forms now emitted
- [x] Gate `git checkout`, `git reset`, `git revert` as ask (`checkout -- <path>` destroys uncommitted work with no reflog, same class as `restore`)
- [x] Drop `ssh`/`scp` from ask; leave absent so dynamic approval can decide
- [x] Remove dead commented-out `rm` ask block
- [x] Broaden the git rule past committing to all state mutation, in both full and minimal rulesets
- [x] Add the missing autonomous-commit prohibition to the Gemini minimal ruleset
- [x] Fix `bun -e` self-contradiction in `common-rules.md.tmpl`
- [x] Correct stale subagent model claims (investigate/research are sonnet, not opus; 24 agents, not 3)
- [x] Add rules backed by session history: verify-before-asserting, no unrequested plan documents, silent-omission scope failure, log-message design
- [x] Ban commit references and suite counts in issue-tracker comments (they bitrot; commits get reworded and bundled here)
- [x] Replace `linear-project` Step 5's CLAUDE.md template — it was generating the nine near-identical Linear sections
- [x] Add `In Review` and `Duplicate` to the `linear-workflow` transition map
- [x] Global git ignore now covers `.claude/`, `.opencode/`, `.cursor/`, with per-repo negations where tracked
- [x] Retire `recall` from chezmoi sync (CLI broken, session DB stale since 2026-03-27; project kept locally)
- [x] Document `dot_codex/` in ARCHITECTURE.md

**Done (continued):**
- [x] Remove blanket tool grants from project settings — 105 across 42 projects (arbitrary-exec vectors, plus blankets for tools the generator already curates). Entries down to 1,796 from 2,746
- [x] Linear label Phase 2 — 17 of 18 retirements (`Debugger` deliberately kept: a precise, distinct idakit subsystem on an active project, and the analysis contradicted itself by listing it as both promote and retire)
- [x] Linear label Phase 0 — 27 description rewrites so cross-project-usable labels stop reading as one project's vocabulary. Zero issues touched, fully reversible
- [x] File guard defects found by analysis: XEV-1675 through XEV-1683
- [x] Remove `xargs` and `env` from the general-CLI bare allow list — both take a command as their argument, so prefix matching made `Bash(xargs:*)` cover `xargs rm -rf build` and `Bash(env:*)` cover `env FOO=1 rm -rf build`, overriding every other rule in the table

**Open:**
- [x] Linear label Phase 3 — all merges done, 28 labels retired total, ~79 workspace-visible remain. Verified: every loser label is empty, unrelated labels on touched issues survived. `Tech Debt` came to 143 issues (94 already carried `Refactoring` and dropped a label, 50 gained it), and `Refactoring`'s description now covers debt paydown
- [ ] `Engine` vs the retired `Game Engine` — two labels one word apart, one now gone. Rename `Engine` to something unambiguous or accept it as project-bound, but don't leave the near-collision
- [ ] `BF4` sits on 73% of its project's issues, over the 40% saturation threshold, but the project genuinely hosts `AC` and `Hired Ops` work too. Recommendation was keep it and stop applying it to unambiguously-BF4 new issues
- [ ] Encode the labeling rules where labels are actually applied (`linear-issue`, `linear-project`): the ≤3 cap, exactly one type label, and the 40% saturation rule (never apply a subsystem label already on >40% of a project's issues)
- [ ] Linear label Phase 1 as originally specified is impossible — a label's team cannot be changed, so "promote the 5 team-scoped labels to workspace scope" is really create + relabel + retire. Note: team-scoped labels are invisible to a workspace-level label listing (107 of 112 visible), so any future audit must enumerate per-team
- [ ] Decide the split for write-capable tools still bare-allowed in the static table: `sed` (`-i`), `sort` (`-o`), `awk` (`-i inplace`), `tee`, `script` (`-c`), `rsync` (`--delete`), `mv`, `cp`. Straight removal is too costly since `sed -n` and `jq .` are everyday reads; a per-tool split is likely, but flag position varies. Noted on XEV-1677
- [ ] Resolve the interpreter-class boundary in `permissions.ts`: it currently allows `python`, `python3`, `bun`, `npx`, `bunx`, `uv run`, `uvx` while `node`, `deno`, `bash`, `perl`, `ruby` would be absent. Either interpreters are absent (those lines come out) or scripting runtimes are fine (node/deno go in) — the hoist proposal's comment wording depends on which
- [ ] **Architecture (XEV-1684): move the permission table into guard and generate the Claude Code allow list from it.** The flat matrix cannot express recursion, subcommand nesting, verb-changing flags, global options before a subcommand, or ecosystem wrappers, and patching holes into it is unbounded work. guard already parses and normalizes properly, so it should own the model and emit the list as an artifact. Needs a ~100-tool audit to find the right shape, including recursive wrapper parsing (`uv run toolA` evaluating `toolA`) and aliases for the same tool under different ecosystem names
- [ ] Apply the tool-hoist proposal: 54 allow, 6 split, 17 absent from Group A; compilers (`rustc`/`gcc`/`clang`/`cc`) allowed; `make`/`cmake` split not bare (`cmake -E rm -rRf` is a plain recursive delete); 16 `uv` subcommands added with `uv publish` as the one ask
- [ ] Hoist recurring per-project tool grants into `meta/permissions.ts` (XEV-1681). Needs the three-way allow / absent / ask decision per tool, where `absent` is preferred over `ask` because an `ask` entry can never be overridden afterward
- [ ] Cross-machine sync for untracked project rule files. `CLAUDE.local.md` plus a global ignore line would remove the per-repo exclude bookkeeping, but neither it nor the auto-memory directory (machine-local by design) actually syncs. Deferred: the problem has not bitten
- [ ] Grafana/VictoriaMetrics/VictoriaLogs MCP servers, duplicated in `local.config` and `railway-collector` project settings, pointing at `tower`. Token is likely dead and the servers may never have been used; set them up properly or remove
- [ ] Game Hacking has 18 issues In Progress, 12 stale since May-June 2026
- [ ] Figsaw (0 issues, Cancelled), WebSAM (0 issues, marked Completed), Pac-Man (1 issue) carry Linear rules governing almost nothing

---

## Legacy Items from Original TODO

These items were in the original TODO.md and need to be categorized/completed:

- [ ] **age executable not installed on init** - Add to Priority 1, Item 1 (installation docs)
- [ ] **rbw executable needs better configuration** - Related to Priority 4, Item 14 (rbw auto-lock)
- [ ] **rbw config not tracked** - Add rbw config to dotfiles
- [ ] **hishtory executable not available on init** - Add to installation checklist
- [x] **add GPG key to bw** - GPG configuration completed with WSL integration
- [ ] **dracula theme tracking, kitty/micro** - Add to Priority 3, Item 10 (dev tools)
- [x] **lazygit fix difftool** - Add lazygit config to dotfiles
- [ ] **testing in github codespaces** - Add to Priority 4, Item 15 (testing)

---

## Notes

- Items can be worked on in any order within their priority level
- Some items have dependencies (e.g., documentation should reflect implemented changes)
- Use `chezmoi cd` to navigate to source directory when working on configs
- Test changes in WSL/Linux before applying to Windows (or vice versa)
- Keep encrypted secrets out of git history - use age encryption or Doppler
