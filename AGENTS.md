## ⚠️ CRITICAL: File Access in Chezmoi Repository

**When working in `/home/xevion/.local/share/chezmoi`, ONLY access files within this directory.**

### WRONG (accessing deployed files):
```bash
# ❌ DO NOT access ~/.config/opencode/agent/interview.md
# ❌ DO NOT access ~/.config/opencode/AGENTS.md
# ❌ DO NOT access ~/.bashrc
```

### CORRECT (accessing source files):
```bash
# ✅ Access ./home/dot_config/opencode/agent/interview.md
# ✅ Access ./home/dot_config/opencode/AGENTS.md
# ✅ Access ./home/dot_bashrc.tmpl
```

**Why this matters:**
- This is a chezmoi SOURCE directory - files here are templates that deploy elsewhere
- Accessing `~/.config/*` files bypasses chezmoi and creates inconsistencies
- All edits must happen in the source directory (`./home/dot_*` files)
- The source directory is the single source of truth

**File mapping:**
- `~/.config/foo` → `./home/dot_config/foo` (or `.tmpl` variant)
- `~/.bashrc` → `./home/dot_bashrc.tmpl`
- `~/.ssh/config` → `./home/private_dot_ssh/config.tmpl` (or `encrypted_*.age`)

**When you need to access managed files, ALWAYS:**
1. Start from current working directory (`.` in chezmoi repo)
2. Use `./home/dot_*` path patterns
3. Check with `ls -la home/` or similar to find the right source file
4. NEVER jump to `~/.*` paths

@/CLAUDE.md
