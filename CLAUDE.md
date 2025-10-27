# Chezmoi Dotfiles Repository - AI Assistant Guidelines

## Repository Context

This is a **chezmoi source directory** for managing dotfiles across multiple machines. Files here are SOURCE files that get templated and deployed to the home directory.

### Key Concepts

**Source vs Target Pattern:**
- **Source**: `~/.local/share/chezmoi/home/dot_bashrc.tmpl` (what you edit)
- **Target**: `~/.bashrc` (what gets deployed after `chezmoi apply`)
- Edit source files only. DO NOT modify target files directly.

**File Naming Conventions:**
- `dot_` `.` (e.g., `dot_bashrc` becomes `~/.bashrc`)
- `.tmpl` suffix Go template file (rendered with platform detection)
- `private_` prefix 600 permissions
- `encrypted_*.age` age-encrypted files (safe to commit)
- `run_onchange_*` executable scripts that run during apply

**Template System:**
- Uses Go templates with platform detection
- Variables: `.chezmoi.os`, `.chezmoi.homeDir`, `.data.*`
- Conditional rendering for Windows/Linux/macOS/WSL

**Secret Management:**
- Age encryption for sensitive files (recipient: `age1s3ctpj9lafl6qwyvd89sn448us7gdzd53d8yyhsc7zny78c0k4sqerrkze`)
- Doppler integration for API keys/tokens
- Encryption key bootstrapped via hooks from Doppler

**Hooks:**
- `.init_pre.ts` and `.update_pre.ts` (TypeScript via Bun)
- Bootstrap encryption key from Doppler before apply
- Handle `chezmoi init` and `chezmoi update --init`

**GPG Configuration (WSL-only):**
- `~/.gnupg` → Symlink to Windows GPG directory (`C:\Users\Xevion\AppData\Roaming\gnupg`)
- `/usr/local/bin/gpg` → Symlink to Windows `gpg.exe` (via `run_onchange_before_setup-wsl-gpg.sh.tmpl`)
- Enables native Windows Qt5 pinentry GUI for passphrase prompts
- Automatic setup on WSL; ignored on regular Linux

## Critical Restrictions

### NEVER Do These Actions

1. **DO NOT apply changes to filesystem**
   - NO `chezmoi apply`
   - NO direct file writes to `~/.bashrc`, `~/.gitconfig`, etc.
   - Changes stay in source directory only

2. **DO NOT commit or push automatically**
   - NO `git commit` without explicit user request
   - NO `git push` on your own
   - Let user review changes first

3. **DO NOT embed secrets in plaintext**
   - NO API keys, tokens, or passwords in plain text
   - Use Doppler variables: `{{ dopplerProjectJson.KEY_NAME }}`
   - Use age encryption for sensitive files
   - Reference encryption: `encrypted_private_*.age`

4. **DO NOT verify changes yourself**
   - NO running build/test commands unless requested
   - Let user test changes with `chezmoi diff` or `chezmoi apply --dry-run`
   - Ask user to verify after making changes

### Recommended Actions

1. **Edit source files** in `home/` directory
   - Modify `.tmpl` files with proper template syntax
   - Respect platform conditionals (`{{ if eq .chezmoi.os "windows" }}`)
   - Maintain existing template structure

2. **Explain impact** of changes
   - Which target files will be affected
   - Platform-specific behavior
   - What the user should test

3. **Suggest verification commands**
   - `chezmoi diff` - preview changes
   - `chezmoi apply --dry-run` - simulate apply
   - `chezmoi status` - see what's changed

4. **Use templates correctly**
   - Platform detection: `.chezmoi.os`, `.data.wsl`, `.data.chassis`
   - Doppler secrets: `{{ dopplerProjectJson.SECRET_NAME }}`
   - Conditional logic: `{{ if }}...{{ else }}...{{ end }}`

5. **Suggest TODO list updates** (but DO NOT modify automatically)
   - When a task is completed, check if `TODO.md` exists in the repository
   - If the completed task relates to items in TODO.md, **suggest** updating the file
   - Examples of suggestions:
     - "I've completed [task]. Would you like me to update TODO.md to mark this item as complete?"
     - "This work relates to items in TODO.md. Should I update the relevant checkboxes?"
   - **NEVER** modify TODO.md without explicit user approval
   - User must explicitly approve (even if not specifically) before making changes
   - Acceptable approvals: "yes", "go ahead", "update it", "sure", etc.
   - If unclear, ask: "Should I update TODO.md to reflect this completion?"

## Common Tasks

**Add new dotfile:**
```bash
# DO NOT run - explain this to user instead
chezmoi add ~/.newconfig
# Edit: home/dot_newconfig or home/dot_newconfig.tmpl
```

**Add sensitive config:**
```bash
# DO NOT run - explain this to user instead
chezmoi add --encrypt ~/.ssh/config
# Creates: home/private_dot_ssh/encrypted_config.age
```

**Edit existing file:**
- Locate source: `home/dot_config/nushell/config.nu.tmpl`
- Make changes to source file
- User runs: `chezmoi apply` or `chezmoi apply ~/.config/nushell/config.nu`

## Platform Coverage

- **OS**: Windows, Linux (WSL/native), macOS
- **Shells**: bash, fish, nushell, PowerShell
- **Tools**: 30+ development tools configured (pyenv, bun, cargo, etc.)
- **Secrets**: Doppler + age encryption

## When Uncertain

1. **Ask before modifying** templates with complex platform logic
2. **Clarify secret handling** before adding sensitive data
3. **Let user verify** all changes before suggesting next steps
4. **Prefer explanations** over automated actions

# Extended Documentation

@README.md
@TODO.md
@FAQ.md
@ONBOARDING.md