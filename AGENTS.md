## ⚠️ CRITICAL: Use the Question Tool for Planning and Decisions

**STRONGLY prefer using mcp_question when:**
- Planning how to approach a task with multiple valid options
- Facing design decisions or architectural choices
- Uncertain about user intent or requirements
- About to make assumptions that could lead to rework
- Working with ambiguous requests

**During planning phases, ASK before acting:**
- "I see 3 approaches to this. Which do you prefer: A, B, or C?"
- "Should I prioritize X or Y for this implementation?"
- "This could mean either A or B. Which did you intend?"

**It's better to ask and get it right than to guess and need to redo work.**

See detailed Question Tool guidance in CLAUDE.md below.

---

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
