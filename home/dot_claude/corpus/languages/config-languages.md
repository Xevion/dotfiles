---
name: config-languages
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: local/maestro
    path: config module
    note: TOML config with ktoml + kotlinx.serialization, human-readable domain type serializers
  - repo: Xevion/tempo
    path: src/config.ts
    note: TypeScript config with defineConfig, self-registration virtual modules, cross-runtime loading
---

# Config Languages (TOML / YAML / JSON)

## Philosophy

TOML is preferred for human-edited configs: it has clear semantics, inline comments, and no ambiguous indentation. YAML is acceptable when the ecosystem demands it (CI pipelines, Kubernetes). JSON is best for machine-generated output consumed programmatically. TypeScript config files are the right choice when complexity justifies it — conditional logic, type inference, IDE autocomplete, or programmatic composition (presets, hooks).

## Conventions

### TOML for human-edited config

Pair TOML with language-native serialization libraries that support human-readable types. In Rust, use `figment` for layered config with environment overrides. In Kotlin, use `ktoml` with `kotlinx.serialization`. Set `ignoreUnknownNames = true` (or the library equivalent) for forward-compatibility — unknown keys from a newer config version should not crash older software.

Serialize semantic types as human-readable strings rather than raw numbers. Duration as `"5s"` or `"2m30s"`, color as `"#FF5733"`, file sizes as `"512MB"`. This keeps configs self-documenting and avoids unit ambiguity.

```toml
[server]
timeout = "30s"       # request timeout; supports s/m/h suffixes
max_body_size = "4MB" # maximum request body; supports KB/MB/GB
accent_color = "#4A90D9"
```

### Inline comments as documentation

Document every non-obvious key with an inline TOML comment. Embed units directly in value strings for time, distance, and size values. Treat config files as primary user-facing documentation — they are often the first thing a user reads after installation. A well-commented config eliminates the need for a separate "configuration reference" page for most projects.

```toml
[cache]
ttl = "10m"           # how long entries live before eviction
max_entries = 10_000  # hard cap; eviction is LRU when exceeded
warm_on_start = true  # pre-populate from disk on startup (adds ~200ms)
```

### TypeScript config files for complex schemas

`.config.ts` files are the right choice when config benefits from type inference, IDE autocomplete, and programmatic composition. Use a `defineConfig` helper that returns the typed config object — this pattern gives users autocompletion without requiring them to import types manually.

Static formats (TOML/JSON) are better for simple key-value configs consumed by multiple tools or languages. The key tradeoff with TypeScript configs: the tool must execute arbitrary user code at load time, which is a security boundary and a startup cost.

```ts
// defineConfig provides type inference without an explicit import
export default defineConfig({
  preset: "strict",
  rules: {
    maxLineLength: 100,
    trailingComma: "all",
  },
  plugins: [myPlugin({ option: true })],
});
```

### Self-registration for TS configs (advanced/situational)

When building tools that consume TypeScript config files, consider self-registering exports as virtual modules so consumer configs can import the tool's API without installing it as a dependency. In Bun, implement this as a plugin registered before config evaluation. In Node, use `module.register()` in a loader. The virtual module name should match the package name the tool would have if installed. This adds complexity — only justified when the tool is consumed across multiple projects.

```ts
// Tool registers itself so configs can import without npm install
Bun.plugin({
  name: "my-tool-virtual",
  setup(build) {
    build.module("my-tool", () => ({
      exports: { defineConfig, builtinPresets },
      loader: "object",
    }));
  },
});
```

## Anti-Patterns

- **YAML anchors for complex logic** — anchors are not well-supported across all parsers and make configs hard to read without deep YAML knowledge. Extract the repeated values into a tool-level preset/profile instead.
- **Deeply nested configs** — more than 3 levels of nesting is a sign that config should be split into sections or that a programmatic format (TypeScript) is better suited.
- **JSON with comments hacks** — JSONC is not a standard and support is inconsistent. Use TOML or YAML if comments matter.
- **Config files without inline documentation** — a config file that requires reading external docs to understand is incomplete. Document units, defaults, and valid values at the key site.
- **Stringly-typed semantic values** — storing `timeout: 30` without documenting units creates ambiguity (seconds? milliseconds?). Either embed the unit in the key name (`timeout_ms`) or use a string with explicit units (`"30s"`).

## Open Questions

- **JSONC adoption breadth** — VS Code normalized JSONC but it is not supported by most config parsers. Worth tracking whether a standard emerges or whether TOML/YAML fully displace it.
- **KDL as a TOML alternative** — KDL has cleaner syntax for document-like configs and better support for heterogeneous lists. Ecosystem adoption is still nascent.
- **Standardization of TS config loading across runtimes** — Bun, Node (tsx/ts-node), and Deno each load TypeScript differently. No standard virtual module protocol exists yet for self-registration.
