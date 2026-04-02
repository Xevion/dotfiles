---
name: brand-identity
description: |
  Brand identity, project naming, and domain discovery. Auto-activate when the user is:
  starting a new project and needs a name, discussing what to call something,
  exploring domain availability, brainstorming brand names, choosing a project identity,
  asking "what should I call this", "what domains are available", "help me name this",
  "I need a name for", "brand identity", "project name ideas", or any conversation
  about naming, branding, or domain selection for a software project.
  Also activate when the user says "brand-identity" or asks about TLDs, domain pricing,
  or name availability across platforms.
---

# Brand Identity

Help the user discover a project name, verify availability across domains and platforms, and establish a visual identity (color palette + typography). The user typically does NOT come in with a name — the whole point is to explore what's available and what sounds good.

## CRITICAL: Domain Availability Rules

1. **ALWAYS use `domain-check` MCP tools.** NEVER use bash loops, curl, gh api, or any other method for domain lookups.
2. **Batch aggressively.** The `check_domains` tool accepts many domains in a single call. Pass 30-60 FQDNs per call, not 3-5. Fewer MCP calls with more domains each is always better.
3. **Go wide on TLDs from the start.** Use `check_with_preset` with the "popular" preset (11 TLDs) as the minimum. Don't start with just .com/.dev/.io/.app — that's too narrow.
5. **Include thematic TLDs for the project's culture.** Don't only check generic tech TLDs. Check `references/tld-presets.md` for niche presets and add relevant ones to your checks. Match TLDs to the project's vibe and audience — `.gg` for gaming, `.moe` for anime, `.finance` for fintech, `.fm` for music, etc.
4. **Use the CLI for massive generation.** For generating hundreds of permutations, use `domain-check` via Bash with `--prefix`, `--suffix`, `--pattern`, and `--json` flags. The CLI is better than the MCP for large-scale generation+check combos.

### RDAP False Positives (Known Bug)

Some TLD registries have **broken RDAP servers** that return HTTP 404 for ALL queries — registered or not. `domain-check` interprets 404 as "available," producing false positives. Known affected TLDs include `.moe` (GoDaddy Registry's `rdap.nic.moe`). Other GoDaddy Registry-operated TLDs may have the same issue.

**When a high-value domain shows as available on a niche TLD and it seems too good to be true** (e.g., `read.moe`, `scan.moe`, any short/common English word):
1. **Cross-check with WHOIS**: `whois <domain>` via Bash — WHOIS is a separate protocol and may have correct data even when RDAP is broken
2. **DNS existence check**: `dig +short <domain> NS` — if NS records exist, the domain is registered regardless of what RDAP says
3. **Flag it to the user** with a note like "⚠️ .moe RDAP is unreliable — verify before purchasing"

This does NOT mean ignore these TLDs entirely — just verify standout results before presenting them as confirmed available.

### NEVER do this:

```bash
# WRONG — bash loops for domain checks
for name in foo bar baz; do domain-check "$name" ...; done

# WRONG — gh api for domain availability
gh api repos/$name/$name

# WRONG — curl for domain availability
curl -s whois.example.com
```

---

## Available Tools

### MCP: domain-check (PRIMARY)
RDAP-first, WHOIS fallback. 1,200+ TLDs. No registrar APIs.

- `check_domains` — check multiple specific domains. **Pass 30-60 FQDNs per call.**
- `check_with_preset` — check a name against a TLD preset (startup, tech, popular, creative, etc.)
- `generate_names` — prefix/suffix/pattern permutation generation
- `list_presets` — show available TLD presets
- `domain_info` — detailed RDAP/WHOIS info for a single domain

### CLI: domain-check (for bulk generation + checking)
When you need to generate and check hundreds of permutations at once, use the CLI via Bash:

```bash
# Generate prefix/suffix permutations and check them
domain-check myapp --prefix get,try,use,go,my --suffix hub,dex,box,den,lair,nest,vault --preset popular --json

# Pattern expansion
domain-check --pattern "app\d\d" -t com,dev,io --json

# Check a name against ALL 1,200+ TLDs
domain-check myapp --all --json

# Check a name against a specific preset
domain-check myapp --preset startup --pretty
```

The CLI handles batching, concurrency (up to 100 parallel), and rate limiting internally. Use `--json` for parseable output, `--pretty` for human-readable.

### MCP: internet-names-mcp (LATE STAGE ONLY — 1-3 finalists)
Social handles via Sherlock (Instagram, Twitter/X, Reddit, YouTube, TikTok, Twitch, Threads).

### Package Registry / GitHub Checks
**OPT-IN ONLY.** Do NOT check by default. Only when the user explicitly asks or is publishing a package.

### Pricing (late stage)
Porkbun pricing (no API key needed):
```bash
curl -s -X POST https://api.porkbun.com/api/json/v3/pricing/get -H "Content-Type: application/json" -d '{}' | jq '.pricing["<tld>"] | {registration, renewal}'
```

### Reference Files
- `references/naming-strategies.md` — linguistic techniques for name generation
- `references/tld-presets.md` — TLD presets, custom groups, choosing TLDs
- `references/typography-ref.md` — personality-to-font mappings

---

## Output Format

### Domain results: use the list format

**Always use this format** — name in the first column, available TLDs listed together in the second, notes in the third:

```
| Name       | Available TLDs              | Notes                          |
|------------|-----------------------------|--------------------------------|
| snapvault  | .com .net .org .io .xyz     | .com available! snap + vault   |
| gridpulse  | .net .org .io .dev .app .xyz| No .com. grid + pulse. 9 chars |
| byteden    | .net .org .io .dev .me .xyz | No .com. byte + den. 7 chars   |
```

- **Use ✅/❌ emoji** when showing per-TLD detail (e.g., in focused comparisons of 2-3 finalists)
- **List format is default** — available TLDs grouped together, not one column per TLD
- **Exclude dead names entirely** — if nothing is available, don't show it in the table
- **Sort by quality** — names with .com first, then by number of available TLDs

### Web searches for existing projects
Only search for existing projects when narrowed to **3-5 finalists**. Don't web-search every candidate — it's slow and unnecessary early on.

---

## Workflow

### Phase 1: Discovery

Gather context, then ask what's missing via `AskUserQuestion`.

**Auto-gather** (if project context exists):
- Read README, package.json, Cargo.toml, pyproject.toml, etc.
- Extract: project purpose, tech stack, target audience, existing naming

**Ask structured questions** — batch up to 3, skip anything already answered.

Questions to ask (use AskUserQuestion with these):
1. What does the project do? (CLI / web app / library / other)
2. What vibe should the name have? (multiSelect: technical, playful/clever, clean/modern, bold/edgy, Japanese-flavored, explore all vibes broadly). **Include "Explore broadly — mix and match" as an option.** The user often wants you to try many directions, not commit to one upfront.
3. Domain budget and TLD preferences? (multiSelect: .com is a must-have, .com/.net/.org are high-value targets, open to dev-focused TLDs (.dev/.io/.ai/.app), open to creative/cheap TLDs (.xyz/.sh/.rs/.land), flexible — surprise me, budget under $15/yr, budget under $50/yr, don't care about budget yet). **List the big TLDs by name and group the rest by category.** Don't make the user type out their TLD preferences from scratch.

### Phase 2: Name Generation

Read `references/naming-strategies.md`.

**Step 1: Brainstorm a domain-specific vocabulary FIRST.**

Before generating any candidate names, build two lists of **word fragments** relevant to this specific project:

- **Prefixes/vibes** (15-30): words that capture the project's content, culture, tone, or subject matter. Think slang, loanwords, subculture terms, emotional tone, not just literal descriptions. Aim for at least 15 — if you can only think of 5, you haven't gone deep enough into the project's domain.
- **Suffixes/functions** (15-30): words that describe what the tool/site does structurally — scan, read, hub, vault, index, dex, den, lib, etc. Also include generic modifiers: -ify, -ly, -r, -x, -io.

**Self-check for gaps before proceeding.** Ask yourself: "What obvious word in this domain am I missing?" Think about synonyms, slang, abbreviations, loanwords, euphemisms, and subculture-specific terms. If a native speaker of this project's culture would look at your list and immediately say "how did you miss X?" — you missed it. Spend the extra 30 seconds filling gaps rather than surfacing an obvious word on batch 4.

**Show this vocabulary to the user** alongside the first batch of candidates. They may add words, but don't rely on them to catch your gaps — that's your job.

**Step 2: Generate 30-50 candidates** from the vocabulary, across diverse strategies:
- Direct combinations from the vocabulary lists (prefix + suffix)
- Portmanteau, truncation, invented words, domain hacks
- Names with accessible double meanings (both meanings land in English)
- Variations of popular/well-known names in the space (prefixing, suffixing, truncating, or respelling)
- **Cross-category combinations** — if the project has a cultural flavor and a functional word, smash them together (e.g., Japanese word + English tool suffix, Norse mythology + dev terminology, cooking metaphor + data word). Many of the best names blend two vibes rather than being pure examples of one.

Present candidates grouped loosely by strategy, but **always include a "hybrids" group** that combines elements across categories. Then **immediately ask** what direction appeals using AskUserQuestion.

### Phase 3: Domain Exploration (THE CORE LOOP)

This is where most time is spent. The loop is:

**Generate → Check → Show results → Ask direction → Repeat**

Use `AskUserQuestion` frequently (every 1-2 batches) to check if the direction is right. Don't run 5 batches before asking.

**Step 1: First batch — check top candidates across wide TLD set**
Use `check_with_preset` with "popular" (11 TLDs) for each top candidate, OR batch them all into one `check_domains` call with 30-60 FQDNs.

**Step 2: Present results in list format** (see Output Format above). Exclude dead names.

**Step 3: Ask direction** — what's pulling them? What to explore more? **Offer combining directions** as an explicit option (e.g., "Japanese compounds + tool suffixes", "retro-gaming aesthetic + short invented words", "Latin roots + finance terminology"). Be specific to the project's actual vibes — not generic labels like "minimal + technical." Users rarely want to go deep on one pure category — they want hybrids.

**Step 4: Explore preferred direction — but MAINTAIN BREADTH**

**CRITICAL: Don't over-drill one suffix/prefix family.** If the user liked 3 names from different categories, explore all 3 directions in parallel — don't pick the suffix from one and grind 20 variations of it. Each batch should introduce **new base words, new combination strategies, and new structural patterns** alongside any suffix/prefix variations.

A good exploration batch looks like:
- 5-8 variations on a liked suffix (e.g., "-dex" with different bases)
- 5-8 variations on a liked base word (e.g., "lewd-" with different suffixes)
- 5-8 **completely new names** inspired by the same vibe but using different strategies (portmanteau, truncation, invented words, domain hacks)

A bad exploration batch looks like:
- 20 names that are all `[word]scan`, `[word]dex`, `[word]hub` — same structure, different filler

When dispatching subagents or CLI for bulk checks, **split the budget**: half on variations of liked patterns, half on fresh names that share the vibe but not the structure.

**Subagent dispatch** (for heavy exploration):
```
Use the Agent tool to dispatch a general-purpose subagent with a prompt like:

"Use the domain-check CLI to explore names for [project description] with [vibe].
HALF variations on liked patterns: [list specific patterns to vary]
HALF fresh names using different strategies: portmanteau, truncation, invented words, compounds.
Check all against --preset popular.
Parse the JSON output. Return ONLY names that have at least 2 available TLDs.
Format as: name | available TLDs | derivation notes
Sort by: .com available first, then by TLD count."
```

**CLI for systematic generation** (when varying a specific pattern):
```bash
domain-check myapp --suffix dex,den,hub,box,vault,lair,nest,lab,sync,flow --preset popular --json
```

**Step 5: Present refined results, ask again. Repeat until the user has favorites.**

### Phase 3b: Edge Case Exploration

When the user likes a name that's taken on .com, explore variations:
- Add/remove a letter: `spotify` → `spotifyx`, `sptfy`; `nhentai` → `nhentxt`
- Prefix variations: `go-`, `get-`, `my-`, `the-`
- Suffix variations: `-app`, `-hq`, `-x`, `-2`
- Spelling hacks: drop vowels, double letters, swap letters

Check these against wide TLD sets. Present any that open up .com or other good TLDs.

### Phase 4: Finalist Verification

When narrowed to 1-3 names:
1. **Web search** for existing projects (one search per finalist)
2. **`internet-names-mcp`** for social handle availability
3. **Package registry** check (only if publishing a package)
4. **Porkbun pricing** if the user cares about cost

### Phase 5: Visual Identity

Once a name is chosen, offer color + typography.

Read `references/typography-ref.md`.

**Color palette:**
- Colormind API: `curl -s -X POST http://colormind.io/api/ -d '{"model":"default"}' | jq '.result'`
- Or generate manually using color theory

**Typography:** Recommend 2-3 pairings from the reference based on project vibe.

**Logo:** Suggest `/logo-designer` for visual work.

**Brand summary** (inline markdown, only sections that were actually explored):
```
## Brand Summary: [Name]

### Domain
- Primary: [name].[tld]
- Alternatives: [list]

### Colors
- Primary: #XXXXXX (etc.)

### Typography
- Heading: [Font], Body: [Font]
```

### Phase 6: Domain Registration (optional)

Porkbun pricing lookup, then direct to Porkbun web UI. Never register programmatically without explicit confirmation.

---

## Behavioral Notes

- **Drive the exploration aggressively.** Generate far more variations than you think necessary. The user wants breadth.
- **Check in frequently with AskUserQuestion.** After every 1-2 batches, ask what's working and what to explore more.
- **When the user likes a pattern, go deep — but not ONLY deep.** If they like "-dex", generate 15+ dex variations AND 15+ fresh names with different structures. See Step 4.
- **Explore edge cases of popular taken names.** Variations of well-known names in the space are worth checking — letter swaps, prefixes, truncations.
- **Use subagents for heavy exploration.** Dispatch a subagent to crack a namespace with hundreds of queries and return a refined list. This keeps the main context clean.
- **Get to the compiled summary format fast.** The list format (name + available TLDs + notes) is the primary output. Get there in 1-2 batches, not 5.
- **Web searches only for finalists.** Don't search for existing projects on every candidate — only the final 3-5.
- **Don't check things the user didn't ask for.** Domains are default scope. Package registries, GitHub, social handles are OPT-IN.
- **Use MCP tools for domains. Always.** Never shell out to bash for domain lookups (except the domain-check CLI for bulk generation).
- **Adapt direction quickly.** When the user gives feedback, immediately pivot. Don't keep generating in a direction they've rejected.
