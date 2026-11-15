# TODO

Open work only. Finished items are deleted rather than ticked off; the history lives in git.

## screen-pick

Go rewrite of the kitty path pickers in `src/screen-pick`.

- [ ] Check the overlay inside real kitty: `KITTY_LISTEN_ON` reaching it, `@active-kitty-window-id` substitution, and the trailing-newline behavior of `@screen` text (known from kitty's master source, not the installed version)
- [ ] Resolve cost for many distinct directory spans on one screen is still quadratic (`resolve.basesWith`, per-call `above` copy); about 70 ms at 16 KB
- [ ] "Open in browser" hands any URL scheme to `xdg-open`; decide whether to limit it to http(s) and label the rest separately
- [ ] OSC 8 links act on the visible label, not the link target (unreachable with the current binding, which captures no formatting)
- [ ] Mouse input is not handled, and hint labels freeze after the first key, so enlarging the window leaves new matches unlabelled
- [ ] `scan -index` still uses an unguarded `os.Stat`
- [ ] Retire `screen_paths.py` and the `kitty-paths` shim once the new pickers have been lived with

## AI tooling configuration

Permissions, rules and Linear conventions (audit of 2026-09-16 and after).

- [ ] **Architecture (XEV-1684): move the permission table into guard and generate the Claude Code allow list from it.** The flat matrix cannot express recursion, subcommand nesting, verb-changing flags, global options before a subcommand, or ecosystem wrappers, and patching holes into it is unbounded work. guard already parses and normalizes properly, so it should own the model and emit the list as an artifact. Needs a ~100-tool audit to find the right shape, including recursive wrapper parsing (`uv run toolA` evaluating `toolA`) and aliases for the same tool under different ecosystem names
- [ ] Apply the tool-hoist proposal: 54 allow, 6 split, 17 absent from Group A; compilers (`rustc`/`gcc`/`clang`/`cc`) allowed; `make`/`cmake` split, not bare (`cmake -E rm -rRf` is a plain recursive delete); 16 `uv` subcommands added with `uv publish` as the one ask
- [ ] Hoist recurring per-project tool grants into `meta/permissions.ts` (XEV-1681). Needs an allow / absent / ask decision per tool; prefer `absent` over `ask`, since an `ask` entry can never be overridden afterward
- [ ] Decide the split for write-capable tools still bare-allowed in the static table: `sed` (`-i`), `sort` (`-o`), `awk` (`-i inplace`), `tee`, `script` (`-c`), `rsync` (`--delete`), `mv`, `cp`. Removing them outright is too costly since `sed -n` and `jq .` are everyday reads; a per-tool split is likely, but flag position varies (XEV-1677)
- [ ] Resolve the interpreter-class boundary in `permissions.ts`: it allows `python`, `python3`, `bun`, `npx`, `bunx`, `uv run`, `uvx` while `node`, `deno`, `bash`, `perl`, `ruby` are absent. Either interpreters are absent (those lines come out) or scripting runtimes are fine (node/deno go in); the wording of the hoist proposal's comment depends on which
- [ ] Linear labels: rename `Engine` to something unambiguous or accept it as project-bound, since it sits one word from the retired `Game Engine`
- [ ] Linear labels: `BF4` is on 73% of its project's issues (threshold is 40%), but the project also hosts `AC` and `Hired Ops` work. Recommendation: keep it and stop applying it to unambiguously-BF4 new issues
- [ ] Encode the labeling rules where labels are applied (`linear-issue`, `linear-project`): at most 3 labels, exactly one type label, and never apply a subsystem label already on more than 40% of a project's issues
- [ ] Linear label Phase 1 as specified is impossible, since a label's team cannot be changed; promoting the 5 team-scoped labels to workspace scope means create + relabel + retire. Team-scoped labels are also invisible to a workspace-level listing (107 of 112 visible), so any future audit must enumerate per team
- [ ] Design corpus follow-ups: flag over-specific existing claims, verify tempo exemplars against the task-graph model, populate the `shell` stub, profile the recently active projects not yet audited
- [ ] Cross-machine sync for untracked project rule files. `CLAUDE.local.md` plus a global ignore line would remove the per-repo exclude bookkeeping, but neither it nor the auto-memory directory (machine-local by design) syncs. Deferred until it bites
- [ ] Grafana/VictoriaMetrics/VictoriaLogs MCP servers are duplicated in `local.config` and `railway-collector` project settings, pointing at `tower`. The token is likely dead and the servers may never have been used; set them up properly or remove them
- [ ] Linear housekeeping: Game Hacking has 18 issues In Progress, 12 stale since May-June 2026; Figsaw (0 issues, Cancelled), WebSAM (0 issues, marked Completed) and Pac-Man (1 issue) carry rules governing almost nothing
- [ ] Audit existing AI tool configs (`.claude/`, API keys, prompts); add Claude Desktop and Gemini configuration if applicable; move API keys into Doppler; document per-project vs global configuration and project-specific `CLAUDE.md` templates

## linear-proxy

Local MCP proxy built from `~/projects/linear-local` (see its `docs/proxy-plan.md`).

- [ ] Replace the borrowed Firefox session cookie (`src/cookies.rs`) with something long-lived: stored login credentials, or better, the account's passkey, which signs in instantly with no second factor. The cookie expires or rotates and only works while Firefox is signed in
- [ ] Drop the `claude-code-cache` token source once `linear-proxy login` has been exercised through a token refresh
- [ ] Decide whether opencode and Codex should also use the proxy; `override` in `meta/mcp-servers.ts` is per-format, so it is one line each

## Bootstrap and documentation

- [ ] Installation guide covering fresh Linux, WSL and Windows (including the Bun requirement), with a pre-flight checklist of required tools (Bun, age, chezmoi; `ONBOARDING.md` lists the apps that are not installed automatically)
- [ ] First-time setup walkthrough: how the source directory relates to the target, Doppler setup, and how to add a symlink (including the `.chezmoiignore` entry)
- [ ] Encryption guide: age setup and key storage/backup, the `.private_*` file pattern, a checklist for adding a new encrypted value, secret rotation, troubleshooting
- [ ] Troubleshooting guide for install problems, plus a quick emergency-recovery card
- [ ] Document the template-conditional strategy (when to use OS-specific blocks), Windows vs WSL separation of concerns, which tools live where, and file path conventions across platforms
- [ ] Mise: document setup and the version-management strategy, add it to the install checklist. Decided: shims stay rejected (about 22 ms per call against 1.6 ms direct)
- [ ] Bun: document global install location, shell completions, and the bun vs npm vs pnpm choice
- [ ] Windows Terminal: document theme and font configuration, profiles for PowerShell/WSL/cmd, and install method

## Cross-platform

- [ ] Windows PATH management: a PowerShell script for Windows-only PATH changes, a chezmoi hook to keep it in sync, a validation script, and a backup/restore path. Document registry PATH vs user PATH vs chezmoi-managed PATH, and test the PATH length limit
- [ ] Windows environment variable management strategy
- [ ] Test strategy for multi-platform templates
- [ ] Test bash completions in a real shell

## Tool configuration

- [ ] Mise upgrade policy: `latest` pins never move on their own (36 tools were behind); decide on a periodic `mise upgrade` or an apply hook
- [ ] Duplicate-binary check in `meta/` that flags one name resolving to two installers
- [ ] Git: credential helpers, hook templates
- [ ] Rust: `rustfmt.toml` and clippy settings
- [ ] Python: `pyproject.toml` templates and `.python-version`
- [ ] Language linter and formatter configs
- [ ] Kitty: expand the config; add the dracula theme to micro (kitty already has it); document terminal color schemes
- [ ] Alacritty config, if it is used at all
- [ ] Expand the Wakatime config
- [ ] Firefox: confirm the Windows deploy script from an elevated shell; decide whether Android needs anything beyond Firefox Sync
- [ ] btop: decide which keys are volatile for the reconciler (`shown_boxes`, `proc_sorting`, `proc_reversed`, `show_disks` flip from the UI); add a YAML adapter once lazygit grows a volatile key
- [ ] Replace earlyoom with a Rust service in `~/projects`. Requirements, in priority order:
  - Config file, never argv. earlyoom takes all config as flags, so its `--prefer`/`--avoid`/`--ignore` regexes sit in `/proc/<pid>/cmdline` and every `pgrep -f java` matches the daemon. Not fixable upstream: build tools are transient shell children, so the policy cannot move to systemd unit properties
  - PSI triggers: `poll()` on `/proc/pressure/memory` instead of polling `MemAvailable`, which measures stall time rather than estimating headroom
  - Split monitor and notifier over a pre-connected socket, so the kill path never allocates, forks, or blocks on a thrashing notification daemon
  - Post-kill cooldown: wait for reclaim before re-evaluating. earlyoom has none, which is why one spike on 2026-09-01 escalated into 45 signals across 25 processes in 8 seconds
  - Warn before killing, not only after; the useful notification fires while there is still headroom to render it

## Editors and IDEs

VS Code, Cursor and Zed settings and keybindings are managed; JetBrains has keymaps only.

- [ ] VS Code: extension list with an install script, snippets, per-language settings
- [ ] Cursor: work out the overlap with VS Code and what belongs only to Cursor (AI configuration, install workflow)
- [ ] JetBrains: IDE settings beyond keymaps, color scheme, plugin installation strategy
- [ ] Pick a sync strategy per IDE (template vs symlink vs manual) and a way to track versions

## Packages

- [ ] Audit `run_onchange_install-packages.sh.tmpl`; split the Linux, WSL and Windows package lists; make installs idempotent
- [ ] Windows package manager integration (winget, chocolatey or scoop)
- [ ] Homebrew package list for Linux
- [ ] Document package dependencies, install order, manual steps for proprietary tools, and a version-pinning strategy
- [ ] Test package installation on a clean system

## Workflow tooling

- [ ] Drift check: a script that shows what differs between source and target, with fuzzy search over config diffs
- [ ] Tell me at terminal startup when configs are out of date, checking on a timer so startup stays fast
- [ ] "Unmanaged files" workflow (see the README's Status section)
- [ ] Safer apply: change preview, a snapshot before applying, a rollback mechanism, and documented rollback procedures
- [ ] Interactive wizard for new machines, and device-specific configuration
- [ ] Script to list all managed files
- [ ] Profile shell startup time
- [ ] rbw: lock automatically (8 hour sessions, and on sleep, idle or screen saver)
- [ ] GitHub language attributes script: a pre-commit hook that updates `.gitattributes`
- [ ] Neovim and VS Code automatic setup improvements

## Testing and validation

`tests/` holds only the template-include and claude-usage tests so far.

- [ ] Fresh-install simulation script
- [ ] Age decrypt test and Doppler integration validation
- [ ] Pre-commit hook for template syntax, and a template rendering test suite
- [ ] CI (GitHub Actions) for template validation, shellcheck on shell scripts, and hook execution on every platform
- [ ] Test matrix across Linux, WSL and Windows; try it in GitHub Codespaces

## Cleanup

- [ ] Standardize file naming conventions across the repo
- [ ] Archive old or unused configuration files and clean up temporary or test files
- [ ] Review `.gitattributes` for proper file type detection
- [ ] Reorganize the directory structure if it still needs it
- [ ] Documentation extras: a `CONTRIBUTING.md` for future me, a chezmoi commands cheat sheet
