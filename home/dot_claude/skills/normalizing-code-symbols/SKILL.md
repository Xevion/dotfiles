---
name: normalizing-code-symbols
description: Use when finding or fixing non-ASCII typographic symbols in source code, comments, docs, or strings. Covers em dashes, unicode arrows, box-drawing tree chars, ellipsis, smart quotes, and similar. Also covers detecting and removing banner/decorative divider comments and bare section-label comments. Auto-invoke when the banner-comment-lint PostToolUse hook reports a banner, or when doing a cleanup pass across multiple files.
---

# Normalizing Code Symbols

## Overview

Only Unicode letters (`\p{L}`), combining marks (`\p{M}`), emoji (`\p{Emoji}`), and plain ASCII
(`\x00-\x7F`) are permitted in source code. Typographic symbols that sneak in via copy-paste from
browsers, word processors, or IDEs must be normalized -- but *how* depends on where the symbol lives.

**Core rule:** If the symbol scan finds a match, determine context first, then apply the right fix.

## Detection

### Symbol Scan

```bash
rg -P '[^\p{L}\p{M}\p{Emoji}\x00-\x7F]' -g '!migrations/**' -g '!scripts/**'
```

### Banner Comment Scan

```bash
rg \
    --context 2 \
    --heading \
    -e '^\s*(//[/!]?|/\*+|#|--|<!--|@rem|::|\*)\s*[-=*#~+._^─═/]{5,}' \
    -e '^\s*[-=*#~+._^─═/]{10,}' \
    -e '(//[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{2,}\s+\S.{0,60}\s*[-=*#~+._^─═]{2,}' \
    -e '[-=*#~+._^─═/]{5,}\s*(\*/|-->)' \
    -e '[╔╗╚╝║│┌┐└┘├┤┬┴┼]{3,}' \
    -e '(//[/!]?|#|--|/\*+|<!--)\s*[-=*#~+._^─═]{1,}\s*\[.{1,60}\]\s*[-=*#~+._^─═]{2,}' \
    --glob '!*.lock' \
    --glob '!*.min.*' \
    --glob '!node_modules' \
    --glob '!*.md' \
    --glob '!gradlew' \
    --glob '!gradlew.bat' \
    --glob '!**/migrations/*.sql'
```

## Context Decision Tree

```
Is it a comment (any language)?
  -> ASCII. Always. (// --, /* -> */, <!-- -- -->)

Is it in HTML markup / Svelte template / JSX?
  -> HTML entity (&mdash; &middot; &rarr; &plusmn; &hellip; etc.)
     The framework unescapes these natively.

Is it in a JS/TS string literal?
  Is this a build/dev script (scripts/, Justfile, similar)?
    Is it console output shown to a developer?
      -> Emoji preferred (-> ➡️, check -> ✅, x -> ❌)
    Otherwise (comments, non-output strings)?
      -> ASCII
  Is this production application code?
    Is it a string that goes into HTML (innerHTML, template)?
      -> HTML entity string: "&mdash;" / "&middot;"
    Is it any other string (aria-label, tooltip, API field, log)?
      -> JS unicode escape: \u2014, \u00B7
    Is it a log line / tracing span field?
      -> ASCII only. No emoji, ever.
```

**Key rule for emoji:** Emoji are only acceptable in developer-facing script output
(`console.log`, `process.stdout.write` in build/dev scripts). Never in production app logs,
tracing spans, API responses, or UI strings computed in application code.

## Replacement Reference

### Comments (any language/context)

| Symbol | Replace with |
|--------|--------------|
| `—` em dash | `--` |
| `–` en dash | `-` |
| `→` right arrow | `->` |
| `←` left arrow | `<-` |
| `≈` approx equal | `~=` or `approx.` |
| `≠` not equal | `!=` |
| `≤` / `≥` | `<=` / `>=` |
| `×` multiply | `x` |
| `−` minus sign | `-` |
| `…` ellipsis | `...` |
| `·` middle dot | `*` |
| `µ` micro sign | `u` |
| `±` plus-minus | `+/-` |
| `─` `═` bars | `-` / `=` |

### HTML Markup (Svelte template, JSX, HTML files)

| Symbol | HTML entity |
|--------|-------------|
| `—` em dash | `&mdash;` |
| `–` en dash | `&ndash;` |
| `→` right arrow | `&rarr;` |
| `←` left arrow | `&larr;` |
| `…` ellipsis | `&hellip;` |
| `·` middle dot | `&middot;` |
| `±` plus-minus | `&plusmn;` |
| `×` multiply | `&times;` |
| `≈` approx | `&asymp;` |
| `≠` not equal | `&ne;` |
| `≤` / `≥` | `&le;` / `&ge;` |
| `µ` micro | `&micro;` |
| `"` `"` smart quotes | `&ldquo;` / `&rdquo;` |
| `'` `'` smart apostrophe | `&lsquo;` / `&rsquo;` |

```svelte
<!-- Before -->
{score.toFixed(2)} ± {ciHalf} · {confidencePct}% confidence

<!-- After -->
{score.toFixed(2)} &plusmn; {ciHalf} &middot; {confidencePct}% confidence
```

### JS/TS String Literals (non-HTML context)

Use JS unicode escapes when the string is NOT directly placed in an HTML context:

| Symbol | JS escape |
|--------|-----------|
| `—` em dash | `\u2014` |
| `–` en dash | `\u2013` |
| `→` right arrow | `\u2192` |
| `·` middle dot | `\u00B7` |
| `±` plus-minus | `\u00B1` |
| `µ` micro | `\u00B5` |
| `…` ellipsis | `\u2026` |

```typescript
// aria-label, tooltip text, API field -- not rendered as HTML markup
const label = `${score.toFixed(2)} \u00B1 ${ci} \u00B7 ${pct}% confidence`;
```

### Return Values / Display Placeholders

Plain ASCII. These return values flow into JS and are often passed to HTML later:

```typescript
// Before
if (creditHours == null) return "—";
return `${low}–${high}`;

// After
if (creditHours == null) return "--";
return `${low}-${high}`;
```

### Box-Drawing Characters (Tree Diagrams)

Replace Unicode tree art with plain ASCII. Applies in comments and docs.

```
# Before (Unicode)          # After (ASCII)
├── src/                    +-- src/
│   ├── main.rs             |   +-- main.rs
│   └── lib.rs              |   +-- lib.rs
└── Cargo.toml              +-- Cargo.toml
```

## Banner Comments

Remove decorative divider comments -- comment lines composed primarily of repeated symbols used as
visual separators or section headers. Keep any contextual description; drop the decorative borders.

```rust
// Before
// ============================================================
// Section: Data Processing
// ============================================================

// After
// Data Processing
```

Standalone divider lines with no meaningful adjacent label can simply be deleted.

### Section Labels

Bare section-label comments -- standalone comment lines containing only a short title-case phrase,
used to divide code into named regions -- are also in scope. They provide no information the code
structure doesn't already express and should be removed.

**Detection scan (produces candidates for review):**

```bash
rg \
    --context 2 \
    --heading \
    -e '^\s*(//[/!]?|#)\s+(?:MARK:\s*-?\s*)?[A-Z][a-zA-Z]*(?:\s+[A-Za-z]+){0,3}\s*$' \
    --glob '!*.lock' \
    --glob '!*.min.*' \
    --glob '!node_modules' \
    --glob '!*.md'
```

This regex is a heuristic -- it finds candidates. Verify each match is genuinely an organizational
label (surrounded by blank lines or code blocks, no explanatory content) before removing.

```rust
// Before
fn handle_connection() { ... }

// Handlers

fn on_request() { ... }
fn on_response() { ... }

// After (label removed)
fn handle_connection() { ... }

fn on_request() { ... }
fn on_response() { ... }
```

Common forms to recognize: `// Handlers`, `// Private`, `// Helpers`, `// MARK: - Section`,
`# Utilities`, `// Public API`. If the label carries context not obvious from the code, convert
it to a real doc comment (`///` or `/** */`) instead of deleting it outright.

## What's Allowed

- Unicode **letters and combining marks** in actual content strings: `"Aguirre Mesa"`, `"Müller"`, `"João"`
- **Emoji** in developer-facing script output only: `✅`, `❌`, `➡️`
- HTML entities in markup: `&mdash;`, `&middot;`, `&rarr;` etc.
- JS unicode escapes in string literals: `\u2014`, `\u00B7` etc.
- All standard ASCII: `\x00-\x7F`

## Exclusions

| Glob | Reason |
|------|--------|
| `migrations/**` | SQL migrations may contain legitimate Unicode |
| `scripts/**` | Dev scripts use emoji in console output intentionally |
| `*.lock`, `*.min.*`, `node_modules` | Generated/vendored files |
| `*.md` | Excluded from banner scan only (not symbol scan) |
| `gradlew`, `gradlew.bat` | Generated Gradle wrappers |
| `**/migrations/*.sql` | Excluded from banner scan |
