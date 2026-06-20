---
name: xevion-project-content
description: Author, update, improve, and polish project entries on xevion.dev — the long-form detail-page body and the structured fields it leans on — using the `xevion projects` CLI. Covers the TipTap/ProseMirror document model, every content CLI verb and its traps, the widget catalogue (figure, callout, gloss, details, sidenote, kbd), the structured fields (accent, demo URL, type, tags, status, related), the house voice, and fact-checking project claims against source. Auto-activate when authoring or editing a project under /projects/[slug], or when working inside the xevion.dev repo or a project being written up for the portfolio.
user-invocable: true
argument-hint: "[project slug or task]"
---

# Project Authoring Guide

How to author a project's portfolio entry under `/projects/[slug]` — chiefly the
long-form detail-page body, plus the structured fields it leans on — with the
`xevion projects` CLI. Most of this is about the writing: the voice, the wording,
and keeping the facts honest. The rest is the mechanics of the CLI you author
through, and the fields that frame the body.

Read it **before** authoring, not after. The voice preferences here are still
settling; treat them as the current house style, not eternal law.

## What you're writing

A detail body is a single TipTap/ProseMirror document — faithful PM JSON — stored
in `projects.detail_content`:

```json
{ "type": "doc", "content": [ /* top-level blocks */ ] }
```

You rarely touch that JSON directly. You write Markdown, the CLI converts it, and
the server validates and stores it. What's worth knowing:

- **Top-level children are "blocks":** a paragraph, heading, list, callout, etc.
  Each carries a stable 8-char id (e.g. `rz6lqp1t`). You address blocks by that id
  or by a positional path.
- **ids are minted for you.** Any block you insert without an id gets one stamped
  on write. Never hand-author ids.
- **The schema is an allow-list, enforced on write.** `src/pm.rs` validates node
  types, marks, link schemes, and which children each node permits. The full batch
  is validated before anything is stored, so a bad edit is rejected whole — you
  can't half-apply one. The node/mark allow-list is generated into
  `src/pm_schema.generated.json`; if it isn't in there, you can't use it.
- **Rendering is Bun's** (`web/src/lib/tiptap/render.server.ts`): PM JSON →
  sanitized HTML via the static renderer + Shiki, plus post-passes for keycaps and
  heading slugs/TOC.

## The body is one field among many

The detail body is only the long-form prose. A complete project entry also carries
**structured fields**, set separately with `xevion projects update <ref>` (not the
content CLI), and the body leans on them: the voice rule "don't restate the
structured fields" only holds if those fields are actually *populated*. Authoring
the body without checking them is how a page ships with a neutral-grey accent, no
demo link, and no type label — the gap that makes the body look orphaned.

Audit them before you start, and again before you call it done. Read the current
state with `xevion projects get <ref> --json` — keys are camelCase, and **null/unset
fields are omitted entirely**, so an absent key means unset, not an error. The
fields worth specifying:

| Field | Flag | Notes |
|---|---|---|
| Accent color | `--accent 6366F1` | Feeds the page's `--accent` CSS variable; unset falls back to neutral zinc `#71717a`. An aesthetic call — ask, don't guess. |
| Demo URL | `--demo-url https://…` | The live site/app, if one exists (`""` clears). |
| Source repo | `--github-repo owner/repo` | The public source, if any (`""` clears). |
| Project type | `--project-type "Web App"` | Primary type label shown in the page header. |
| Status | `--status active\|maintained\|archived\|hidden` | |
| Tags | `-t "+rust,-old"` | `+`/`-` deltas, comma-separated. |
| Related | `--related slug-a,slug-b` | Full replace (`""` clears). |
| Terminal cast | `--terminal-cast cast.json` | Optional asciinema-style transcript. |

The image gallery (`media`) is its own upload pipeline, not an `update` flag — leave
it to that flow.

Unset fields are not a default to accept silently. **Prompt for any that are
missing** — especially the accent color, which is the author's choice — rather than
shipping the body alone and leaving the entry half-specified.

## The CLI

```
xevion projects content <verb> <ref> [args]
```

`<ref>` is a project slug or UUID. Verbs:

| Verb | Purpose |
|---|---|
| `list` | Table of every block: id, path, type, preview. Your map — run it first. |
| `get` | Print the whole doc as JSON, or one block by locator. (Always JSON; `--json` is a no-op here.) |
| `insert` | Add block(s) at an anchor. |
| `replace` | Swap a block's content, keeping its position and id. |
| `rm` | Delete a block. |
| `move` | Relocate a block to a new anchor. |
| `set` | Replace the **entire** document from a JSON file. |

### Target server

Commands run against the config's `default` target, which is **production** — and
that's correct. You author directly against the live site; there is no staging
copy of the content. A write takes effect the moment the command returns (the
page's ISR cache is invalidated on write). That's the point, not a hazard: edits
are atomic and cheap to revise, so author, look at the live page, and fix forward.

`local` exists only for exercising the CLI itself against a dev server
(`xevion --api local projects content …`). Reach for it when you're testing CLI
*behaviour*, never for real content.

### Locators: path vs id

A `<LOCATOR>` is either:

- a **block id**, optionally with a leading `#` so you can paste straight from
  `list` output: `rz6lqp1t` or `#rz6lqp1t`. A bare token without a leading dot is
  always read as an id — so `3` is the id `"3"`, not the 4th block.
- a **positional path**: a leading dot then dotted indices. `.3` is the 4th
  top-level block; `.6.0.0` descends into children (list → item → paragraph).
  (jq forms like `[3][0]` and `.content[3]` parse too.)

Prefer ids. Paths shift when you insert/remove neighbours; ids are stable for the
life of the block. Run `list`, copy ids from it.

### Anchors (`--at`)

`insert` and `move` place relative to an anchor, by id or path:

`start` · `end` · `before:<loc>` · `after:<loc>` · `prepend:<loc>` · `append:<loc>`

`start`/`end` address the document's top-level list. `before:`/`after:` land
beside a block, in its parent. `prepend:`/`append:` land *inside* a container as
its first/last child — that's how you add into a list or an empty callout.
(`into:` is an alias for `append:`.)

**Ordering, when it bites.** A single `insert` call lands its blocks in document
order, even several at once. But each CLI call is its own batch, and a fresh
block's server-minted id isn't known until the call returns — so you can't
reference it within the same batch. Repeated *separate* `insert --at after:<id>`
calls therefore stack in **reverse** (each lands immediately after the same
anchor). To add several blocks across calls in order, either append with
`end`/`append:<container>`, or `list` after each insert and anchor the next one
`after:<the-new-id>`.

### Authoring inputs: `--md` vs `--node` vs `set`

- **`--md '<markdown>'`** — the default and the one you'll use for nearly
  everything. CommonMark (plus strikethrough) is parsed to PM. Handles prose,
  headings, lists, blockquotes, fenced code, links, inline marks. Cannot express
  the `gloss` mark or the widget nodes.
- **`--node '<pm-json>'`** — the escape hatch: one raw PM node. Needed only for
  what `--md` can't say — the `gloss` mark, the widget nodes (`figure`, `callout`,
  `details`, `sidenote`), or slotting a single `listItem` into an existing list.
  The full catalogue of ready-to-pipe raw payloads lives in
  [`NODES.md`](NODES.md) beside this file — author from there, don't hand-spell
  PM JSON.
- **`set --file <doc.json>`** — replace the whole document. Atomic, validated,
  round-trips byte-for-byte. Use for a from-scratch author or a big restructure;
  for surgical edits prefer targeted `insert`/`replace`.

**Prefer stdin over files and over inlined JSON.** Every input accepts a `-`
that means "read stdin": `--md -`, `--node -`, and `set --file -`. Pipe the
content in rather than fighting the shell to quote it or staging a temp file:

```bash
… | xevion projects content insert <ref> --at end --node -
… | xevion projects content set    <ref> --file -
```

The robust feeds, by shell: a **single-quoted heredoc** in bash/zsh
(`--node - <<'JSON' … JSON`) passes everything literally — apostrophes, `$`,
backticks all survive; in **fish** (no heredocs) use `printf '%s' '…' | …`,
where single quotes are literal except an embedded `'`. See
[`NODES.md`](NODES.md) for the full quoting matrix and per-node examples.

`replace` keeps the target block's id and slot whether you author with `--md` or
`--node`. If your replacement Markdown expands to several blocks, the first keeps
the id; the rest are inserted after it, in order, with fresh ids.

Add `-q`/`--quiet` to print just the `✓` line instead of the re-rendered doc.

### What Markdown supports — and what it rejects

`--md` covers the common constructs and silently maps them to schema nodes. A few
things have no schema equivalent and **error out** rather than degrade:

- **Images** (`![alt](url)`) — rejected. Use a `figure` node (`--node`).
- **Tables**, **raw/inline HTML**, **footnotes**, **task lists**, **math**,
  **definition lists**, **super/subscript** — all rejected.

If `--md` complains a construct "is not supported," that's why — restructure, or
reach for a widget node.

### CLI traps (each of these has actually bitten)

- **Inlining content as a shell argument is the quoting trap — pipe stdin
  instead.** A single-quoted argument ends at the first embedded `'` (an `it's`
  in a `gloss` note mangles the command); switching to double quotes then exposes
  `$`, `` ` ``, `\`, and bash's `!`. Don't reword around the shell. Feed the
  content through `-` (stdin): a single-quoted heredoc in bash/zsh, or
  `printf '%s' '…' | …` in fish. The apostrophe/`$`/backtick problems vanish
  because there's no argument to quote. (See the authoring-inputs section and
  [`NODES.md`](NODES.md).)
- **A `figure` with an empty `src` renders nothing; a fake one renders broken.**
  The renderer returns an empty string for an empty `src`, and a placeholder URL
  shows a broken image on the live page. Don't author figure stubs to fill in
  later — author a figure only once you have a real asset URL.
- **No scripts.** Every edit is one CLI call. If you're reaching for a jq/Bun
  transform to mutate the doc, you're overcomplicating it — use
  `insert`/`replace`/`move`, or `set --file` for a full rewrite.

## Widget catalogue

Every widget below is in the schema. Use them freely, as the content calls for —
a `gloss` to define a term inline, a `details` to fold away a deep-dive, a
`callout` for a caveat that must land. There is no quota: a page that genuinely
needs several is correct, not over-decorated. The only thing to avoid is reaching
for a widget when plain prose would say it better — decoration for its own sake,
not the widget itself.

### Headings → TOC + anchors
Author ordinary Markdown headings via `--md`. Levels shift **down one** on the way
in: the page title owns the only h1, so your top-level `#` renders as h2, `##` as
h3, `###` as h4 (deeper clamps to h4). So start sections at `#` and nest from
there — don't write `##` thinking you're "starting at h2"; you'll skip a level.
Every heading is auto-slugged into a shareable anchor and collected into the
on-page table of contents. For how to *word* a heading, see "Plain headers" under
[Voice & style](#voice--style) — quiet topic labels, never cute ones.

### Code blocks
Fenced Markdown with a language tag (` ```rust `). Highlighted server-side by
Shiki; the language label shows in a header bar. Author via `--md`.

### Inline code & the standard marks
`bold`, `italic`, `strike`, inline `code`, `underline`, and `link` (http/https/
mailto only). Express through `--md` where Markdown allows, or as `marks` in a
`--node` payload. `code` now coexists with `link` (a linked code span) and
emphasis.

**Inline-code highlighting.** A `code` span can carry an optional highlight hint,
written in Shiki's tailing-curly-colon convention in `--md`:

- `` `let x = (a: int) => a * 2{:ts}` `` — `{:lang}` highlights the span as that
  grammar, an *inline code block*. Use a real, balanced expression; a bare
  fragment tokenizes poorly. `lang` is any Shiki id the code-block picker offers;
  an unknown id degrades to plain text.
- `` `Arc{:.type}` `` — `{:.kind}` paints one author-declared token color, for a
  bare identifier a grammar can't disambiguate. `kind` ∈ `keyword` · `fn` ·
  `type` · `string` · `number` · `const` · `var` · `flag` · `comment` (validated
  — an unknown kind is rejected on the write path).

The two are mutually exclusive: a span is grammar-highlighted *or* token-painted.
A trailing `{:…}` that isn't a well-formed hint stays literal; to keep a literal
`{:…}`, author the span via `--node` with no `lang`/`token`. In a `--node` payload
the hint is an attr on the `code` mark:

```json
{"type":"text","marks":[{"type":"code","attrs":{"lang":"rust"}}],"text":"while running { poll_events() }"}
{"type":"text","marks":[{"type":"code","attrs":{"token":"fn"}}],"text":"useState"}
```

### `figure` — image/video block
Atom node. Attrs: `src` (required, real URL), `alt`, `caption`, `kind`
(`image` | `video`, default `image`). Renders `<figure><img|video><figcaption>`.
`--node` only:

```json
{"type":"figure","attrs":{"src":"https://media.example/x.gif","alt":"…","caption":"…","kind":"image"}}
```

### `gloss` — inline hover annotation
A mark, like `link`, carrying a `note` shown on hover/focus. For a one-line "what
is X?" aside that shouldn't break the sentence. `--node` only:

```json
{"type":"text","marks":[{"type":"gloss","attrs":{"note":"short explanation"}}],"text":"the glossed phrase"}
```

### `callout` — typed admonition
Block node, `variant` ∈ `note` | `tip` | `warning` (default `note`). For a caveat
the reader must not miss (e.g. a hard requirement). `--node`:

```json
{"type":"callout","attrs":{"variant":"warning"},"content":[{"type":"paragraph","content":[{"type":"text","text":"…"}]}]}
```

### `details` — collapsible disclosure
Block node, `summary` (toggle label, default "Details") + `open` (default state,
default false). For a deep-dive the main thread shouldn't be forced to carry.
`--node`:

```json
{"type":"details","attrs":{"summary":"Why X?","open":false},"content":[ /* block+ */ ]}
```

### `sidenote` — quiet aside
Block node (`block+`), renders as `<aside>`. A margin tangent set apart from the
main column — softer than a callout. `--node`:

```json
{"type":"sidenote","content":[{"type":"paragraph","content":[{"type":"text","text":"…"}]}]}
```

### `kbd` — keycaps
Not a node or mark: write `[[Key]]` in ordinary prose and a render post-pass turns
it into `<kbd>Key</kbd>`. It skips `<pre>` blocks, so code samples are safe.
Author in `--md` or any text node: `[[Shift]]+[[D]]`. **The keys must be real** —
see fact-checking.

## Voice & style

The body is a portfolio piece — not a README, not marketing copy. Write for a
**layered** reader: the opening lines should land the "why care" for anyone who
skims them, and the depth further down should reward the engineer who keeps
reading. Both are real readers; serve them in that order.

- **Rank, don't flatten.** The rule the page lives or dies on. Not every subsystem
  deserves equal wordcount. Find the two or three genuinely clever or surprising
  decisions and give them room; everything else is supporting cast. When the cache
  eviction math, the compression formats, and the socket-path detection sit at the
  same altitude as the actual ideas, the page reads as an engineering manual — the
  reader can't tell the awesome from the bookkeeping because the prose ranks
  nothing.
- **Lead with the constraint, not the feature.** Open each section on the thing
  that made it worth building or hard to build. *Why* was this interesting — what
  tension, limit, or surprise shaped it? The hook comes before the mechanism.
- **Mechanism is earned, not automatic.** Explain *how* something works only when
  the how is the interesting part. When a detail is accurate but dull — exact
  timeout thresholds, format enumerations (`Brotli, gzip, and zstd`), token
  prefixes, code alphabets, session lengths — cut it, or demote it into a
  `details`/`sidenote` so the main thread stays sharp. Genuinely interesting depth
  belongs off the main column, not deleted; trivia belongs gone.
- **Specifics over adjectives.** Real numbers (`145 MB`, not "large"), real names
  (the actual API, format, file, or function), real commands — *when the specific
  is one that matters*. A concrete detail that earns its place is worth ten
  superlatives; a concrete detail nobody cares about is just clutter. Never
  "blazingly fast," "cutting-edge," "robust."
- **Plain headers, never cute ones.** Section headers are quiet topic labels — noun
  phrases like "The trust boundary," "In-process page cache," "Tradeoffs." Never
  the anthropomorphic / listicle construction ("A CLI that logs in like a browser,"
  "X that does Y like a Z"): it reads as AI-generated filler. Name the topic; don't
  narrate it.
- **No hype, no pandering, no cringe.** No exclamation points, no "we're excited
  to," no first-person-plural for a solo project, no inviting the reader to "dive
  in."
- **Plain, technical, declarative.** Short sentences. Active voice.
- **Don't restate the structured fields.** Type, status, tags, accent, links, and
  the gallery live in dedicated fields — don't repeat them in the body.
- **Honesty reads as confidence.** A real limitations section ("the largest
  weights are a sizeable download — a real commitment on first load") lands as
  self-assurance, not weakness. Hiding the rough edges is what reads as insecure.

## Links & SEO

A portfolio page should be findable, and should give the curious reader somewhere
to go. Both come from the same few habits.

- **Link the non-obvious, once.** Link the first mention of a named library,
  runtime, format, or concept the reader might not know — moka, satori, ts-rs, the
  device-authorization flow (RFC 8628), a "tarpit" — to its canonical home or
  spec. Skip household names (Rust, Docker, HTTP) and skip repeat mentions. A
  handful of high-signal links beats a sea of blue. Anchor text is the thing's
  name, never "here" or "this link."
- **Internal links must be absolute.** The link allow-list is http/https/mailto
  only (`src/pm.rs`), so a relative `/projects/foo` is rejected. To cross-link
  another project, write the full `https://xevion.dev/projects/<slug>`. Prefer the
  `related` field for the formal "see also"; reach for a prose link only when one
  project genuinely references another.
- **The meta description is a structured field, not the body.** The detail page
  sets `<title>` to `{name} | Xevion` and `<meta name="description">` to the
  project's `shortDescription` verbatim
  (`web/src/routes/projects/[slug]/+page.svelte`). The body prose never feeds the
  search snippet — so make `shortDescription` a real one-sentence description with
  the key terms in it, set deliberately with `xevion projects update <ref> -s "…"`.
- **Lead with the searchable terms.** The first paragraph should say plainly what
  the project is and the core technologies behind it — the phrases someone would
  actually search. Don't bury the "what" under three sentences of throat-clearing.
- **Clean heading hierarchy.** Section headings render as `<h2>` beneath the page's
  single `<h1>`; nest sub-points as `<h3>` (markdown `##`), never skipping a level.
  Headings are auto-slugged into anchors and a table of contents, so a clear,
  keyword-bearing heading is both an SEO signal and a usable deep link.

## Fact-checking project details — do not skip

Most authoring errors are *factual*, not stylistic, and a portfolio page asserts
them in the author's name on the live site. Every concrete claim must be verified
against the project's own source before it ships.

**Pull facts from the repo, never from memory or the planning notes.** The plan is
a sketch; the code is the truth. For a project on GitHub, work in this order:

1. `README.md` and any `DEPLOY.md` / `docs/` — the intended architecture.
2. The **config/registry files** that hold the real numbers and names — a model
   registry, a route table, a feature store. Quote these verbatim.
3. The **feature source** for anything you describe behaviourally — the keymap
   file for keyboard shortcuts, the worker for the threading model, the handler for
   what a control actually does.

Fetch directly: `gh api repos/<owner>/<repo>/contents/<path> -q .content | base64
-d`, or `raw.githubusercontent.com`.

### Claim checklist

- **Numbers** — sizes, counts, versions, percentages. Quote the registry/config;
  don't estimate.
- **Names & commands** — exact tool names, file formats, CLI invocations.
- **Where artifacts live** — git vs object storage vs CDN. Big binaries are almost
  never committed; they're built/converted offline and uploaded, then fetched at
  runtime. Check `scripts/`, upload tooling, and `.gitignore` before claiming
  anything was "committed."
- **Behavioural claims** — keyboard shortcuts, modes, defaults. Read the handler. A
  classic miss: the plan says a key toggles one mode, but the keymap shows it does
  something else entirely — and the hotkey the plan described doesn't exist.
- **Don't omit the defining architecture.** If the system's whole shape is "X runs
  client-side; the server only signs storage URLs," that belongs on the page even
  if the first draft skipped it.
- **When you can't verify, don't assert.** Hedge, gloss, or ask — never publish a
  confident guess.

## Recommended workflow

1. **Gather facts first.** Read the repo's README/deploy doc, its registry, and the
   source for any behaviour you'll describe. Write the claims down.
2. **Audit the structured fields.** `xevion projects get <ref> --json` shows what's
   set (unset fields are omitted). Prompt for any that are missing — accent, demo
   URL, project type, tags, status — and set them with `xevion projects update`.
   See [The body is one field among many](#the-body-is-one-field-among-many); the
   body assumes these exist.
3. **Draft against the facts**, in the house voice.
4. **Edit with targeted ops.** `list` for the map and ids, then
   `insert`/`replace`/`move`. Use `set --file` only for a full rewrite.
5. **Verify structure** with `list` — confirm blocks landed where intended.
6. **Eyeball the rendered page.** Widgets (gloss popover, callout, details,
   sidenote) only exercise their styles against real prose once they're live.
7. **Fix forward.** Edits are cheap and atomic; there's no penalty for a follow-up
   `replace`.
