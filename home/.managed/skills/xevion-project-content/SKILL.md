---
name: xevion-project-content
description: Author, update, improve, and polish project detail-page bodies on xevion.dev using the `xevion projects content` CLI. Covers the TipTap/ProseMirror document model, every CLI verb and its traps, the widget catalogue (figure, callout, gloss, details, sidenote, kbd), the house voice, and fact-checking project claims against source. Auto-activate when authoring or editing the long-form content under /projects/[slug], or when working inside the xevion.dev repo or a project being written up for the portfolio.
user-invocable: true
argument-hint: "[project slug or task]"
---

# Detail-Page Authoring Guide

How to write a project's detail-page body — the long-form content under
`/projects/[slug]` — with the `xevion projects content` CLI. Most of this is
about the writing: the voice, the wording, and keeping the facts honest. The
rest is the mechanics of the CLI you author through.

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

### Authoring inputs: `--md` vs `--node` vs `--file`

- **`--md '<markdown>'`** — the default and the one you'll use for nearly
  everything. CommonMark (plus strikethrough) is parsed to PM. Handles prose,
  headings, lists, blockquotes, fenced code, links, inline marks. Cannot express
  the `gloss` mark or the widget nodes.
- **`--node '<pm-json>'`** — the escape hatch: one raw PM node. Needed only for
  what `--md` can't say — the `gloss` mark, the widget nodes (`figure`, `callout`,
  `details`, `sidenote`), or slotting a single `listItem` into an existing list.
- **`--file <doc.json>`** (via `set`) — replace the whole document. Atomic,
  validated, round-trips byte-for-byte. Use for a from-scratch author or a big
  restructure; for surgical edits prefer targeted `insert`/`replace`.

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

- **A single-quoted argument breaks on an embedded apostrophe.** The shell ends
  the string at the first `'` inside your text, mangling the command. Worst with
  `--node`, where the JSON forces single-quoting the whole argument. Fixes, in
  order of preference: for `--md`, wrap the argument in double quotes instead
  (apostrophes are then fine); for `--node`, reword to avoid the apostrophe ("the
  axes of a tensor" not "a tensor's axes"); for anything genuinely complex, write
  the document to a file and `set --file`.
- **A `figure` with an empty `src` renders nothing; a fake one renders broken.**
  The renderer returns an empty string for an empty `src`, and a placeholder URL
  shows a broken image on the live page. Don't author figure stubs to fill in
  later — author a figure only once you have a real asset URL.
- **No scripts.** Every edit is one CLI call. If you're reaching for a jq/Bun
  transform to mutate the doc, you're overcomplicating it — use
  `insert`/`replace`/`move`, or `set --file` for a full rewrite.

## Widget catalogue

Every widget below is in the schema. Use one because the content calls for it, not
to decorate — a page that reaches for a callout, a details, and a sidenote in
three consecutive paragraphs reads as a feature demo, not an essay.

### Headings → TOC + anchors
Author ordinary Markdown headings via `--md`. Levels shift **down one** on the way
in: the page title owns the only h1, so your top-level `#` renders as h2, `##` as
h3, `###` as h4 (deeper clamps to h4). So start sections at `#` and nest from
there — don't write `##` thinking you're "starting at h2"; you'll skip a level.
Every heading is auto-slugged into a shareable anchor and collected into the
on-page table of contents.

### Code blocks
Fenced Markdown with a language tag (` ```rust `). Highlighted server-side by
Shiki; the language label shows in a header bar. Author via `--md`.

### Inline code & the standard marks
`bold`, `italic`, `strike`, inline `code`, `underline`, and `link` (http/https/
mailto only). Express through `--md` where Markdown allows, or as `marks` in a
`--node` payload.

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
{"type":"text","marks":[{"type":"gloss","attrs":{"note":"short explanation, no apostrophes"}}],"text":"the glossed phrase"}
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

The body is a portfolio piece — not a README, not marketing copy. The reader is
technical and skeptical; write for them.

- **Lead with the constraint, not the feature.** Open on the thing that made the
  project worth building or hard to build, not a feature list. *Why* was this
  interesting — what tension, limit, or surprise shaped it?
- **Specifics over adjectives.** Real numbers (`145 MB`, not "large"), real names
  (the actual API, format, file, or function), real commands. A concrete detail is
  worth ten superlatives. Never "blazingly fast," "cutting-edge," "robust."
- **No hype, no pandering, no cringe.** No exclamation points, no "we're excited
  to," no first-person-plural for a solo project, no inviting the reader to "dive
  in."
- **Plain, technical, declarative.** Short sentences. Active voice. Explain the
  mechanism, then the consequence.
- **Don't restate the structured fields.** Type, status, tags, accent, links, and
  the gallery live in dedicated fields — don't repeat them in the body.
- **Honesty reads as confidence.** A real limitations section ("the largest
  weights are a sizeable download — a real commitment on first load") lands as
  self-assurance, not weakness. Hiding the rough edges is what reads as insecure.

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
2. **Draft against the facts**, in the house voice.
3. **Edit with targeted ops.** `list` for the map and ids, then
   `insert`/`replace`/`move`. Use `set --file` only for a full rewrite.
4. **Verify structure** with `list` — confirm blocks landed where intended.
5. **Eyeball the rendered page.** Widgets (gloss popover, callout, details,
   sidenote) only exercise their styles against real prose once they're live.
6. **Fix forward.** Edits are cheap and atomic; there's no penalty for a follow-up
   `replace`.
