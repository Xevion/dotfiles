# Raw node addendum

Companion to `SKILL.md`. Everything here is for `--node` — the raw-PM-JSON
escape hatch you reach for when `--md` can't say it (the `gloss` mark and the
widget nodes). For prose, headings, lists, code, blockquotes, and the standard
inline marks, stay in `--md`; you don't need this file.

This list is the authoritative set of raw payloads. **Keep it in sync with
`src/pm.rs`** (the node/mark allow-list and attrs) — if a node, mark, or attr
changes there, change it here and in `SKILL.md`'s widget catalogue. The names
below match `src/pm_schema.generated.json` (nodes + marks).

## Author by piping, not by file

Every authoring input takes a `-` that means "read stdin", so the document
never has to survive a round-trip through shell argument quoting or a temp file:

```bash
… | xevion projects content insert  <ref> --at <anchor> --node -
… | xevion projects content replace <ref> <locator>     --node -
… | xevion projects content insert  <ref> --at <anchor> --md   -
… | xevion projects content set     <ref> --file -
```

`set --file -` replaces the whole document from stdin; `--file <path>` still
reads a file. `--md -` / `--node -` read the block(s) from stdin.

### The quoting problem this dodges

Inlining JSON as a shell argument is where authoring goes wrong:

- **Single-quoted** (`--node '{…}'`) — the first apostrophe in your prose ends
  the string. A `gloss` note or caption with `it's` mangles the command.
- **Double-quoted** (`--node "{…}"`) — apostrophes are fine, but the shell now
  expands `$`, `` ` ``, `\`, and (in bash) `!`. JSON or prose containing any of
  them breaks or silently corrupts.
- **fish has no heredocs** (`<<` doesn't exist), so the usual bash escape hatch
  isn't available there.

Piping stdin removes the argument entirely. Pick the feed by shell:

- **bash / zsh** — single-quoted heredoc passes everything literally, apostrophes
  and `$` included:
  ```bash
  xevion projects content insert myproj --at end --node - <<'JSON'
  {"type":"callout","attrs":{"variant":"warning"},"content":[
    {"type":"paragraph","content":[{"type":"text","text":"It's fine — $VAR and `backticks` survive."}]}]}
  JSON
  ```
- **fish** — single quotes are literal for everything *except* an embedded `'`
  (no `$` expansion to worry about), so `printf` + pipe is the move; the only
  enemy is the apostrophe, which you escape as `\'`:
  ```fish
  printf '%s' '{"type":"paragraph","content":[{"type":"text","text":"plain text"}]}' \
    | xevion projects content insert myproj --at end --node -
  ```

When in doubt, the heredoc (bash/zsh) is the no-think option: nothing inside
`<<'JSON' … JSON` is interpreted.

## Node catalogue

Attrs and defaults below mirror `src/pm.rs`. ids are minted on write — never
author an `id`.

### `figure` — image/video block (atom)

Attrs: `src` (**required, a real URL** — an empty `src` renders nothing, a fake
one renders a broken image; never stub), `alt`, `caption`, `kind` (`image` |
`video`, default `image`).

```json
{"type":"figure","attrs":{"src":"https://media.example/demo.gif","alt":"…","caption":"…","kind":"image"}}
```

### `callout` — typed admonition (block)

Attr: `variant` ∈ `note` | `tip` | `warning` (default `note`). Holds `block+`.

```json
{"type":"callout","attrs":{"variant":"warning"},"content":[{"type":"paragraph","content":[{"type":"text","text":"A caveat the reader must not miss."}]}]}
```

### `details` — collapsible disclosure (block)

Attrs: `summary` (toggle label, default `"Details"`), `open` (default `false`).
Holds `block+`.

```json
{"type":"details","attrs":{"summary":"Why X?","open":false},"content":[{"type":"paragraph","content":[{"type":"text","text":"The deep-dive the main thread shouldn't carry."}]}]}
```

### `sidenote` — quiet aside (block)

No attrs. Renders as `<aside>`. Holds `block+`. Softer than a callout.

```json
{"type":"sidenote","content":[{"type":"paragraph","content":[{"type":"text","text":"A margin tangent set apart from the column."}]}]}
```

### `gloss` — inline hover annotation (mark)

A mark on a `text` node, like `link`, carrying a `note` shown on hover/focus.
For a one-line "what is X?" aside that shouldn't break the sentence. Apply it to
a single text node and inline it into a paragraph:

```json
{"type":"text","marks":[{"type":"gloss","attrs":{"note":"short explanation"}}],"text":"the glossed phrase"}
```

A whole paragraph mixing plain and glossed text:

```json
{"type":"paragraph","content":[
  {"type":"text","text":"It leans on "},
  {"type":"text","marks":[{"type":"gloss","attrs":{"note":"a fast in-memory cache crate"}}],"text":"moka"},
  {"type":"text","text":" for the page cache."}]}
```

### `listItem` — slot one item into an existing list

`--md` builds whole lists; use a raw `listItem` only to drop a single item into
a list that already exists. Anchor it `append:<list-id>` (last) or
`prepend:<list-id>` (first), or `before:`/`after:` a sibling `listItem`.

```json
{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"the new bullet"}]}]}
```

## Standard marks (reference)

These are reachable through `--md`; you only spell them as `marks` in a `--node`
payload when you're already hand-authoring a text node. Allowed marks:
`bold`, `italic`, `strike`, `code`, `underline`, `link`, `gloss`. `link` hrefs
are restricted to `http://`, `https://`, `mailto:` (see `src/pm.rs`).

```json
{"type":"text","marks":[{"type":"bold"},{"type":"link","attrs":{"href":"https://example.com"}}],"text":"bold link"}
```

The `code` mark takes an optional highlight attr (in `--md`, the `{:lang}` /
`{:.kind}` suffix; here, the attr directly). `lang` is any Shiki id (unknown →
plain); `token` ∈ `keyword`, `fn`, `type`, `string`, `number`, `const`, `var`,
`flag`, `comment` (validated). Set one, not both.

```json
{"type":"text","marks":[{"type":"code","attrs":{"lang":"rust"}}],"text":"while running { poll_events() }"}
{"type":"text","marks":[{"type":"code","attrs":{"token":"fn"}}],"text":"useState"}
```

## Not a node: `kbd` keycaps

`[[Key]]` in ordinary prose becomes `<kbd>Key</kbd>` via a render post-pass (it
skips `<pre>`). Author it in `--md` or any text node — `[[Shift]]+[[D]]` — never
as a node. The keys must be real (see SKILL.md fact-checking).
