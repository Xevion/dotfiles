---
name: search-index-tfidf
category: architecture
last_audited: 2026-04-03
exemplars:
  - repo: Xevion/rustdoc-mcp
    path: src/search/
    note: "Custom inverted TF-IDF index with stemming, tokenization, multi-token coverage penalties"
  - repo: Xevion/recall
    path: src/db/fts.ts
    note: "DuckDB built-in FTS extension, three managed indexes, rebuild via CLI command"
---

# Search Index & TF-IDF

## Philosophy

Two approaches on opposite ends of the build-vs-buy spectrum: custom inverted indexes for domain-specific ranking control, and managed FTS extensions for rapid integration. Choose based on how much ranking behavior needs customization.

## Conventions

### Custom Inverted Index (Build)

- **Tokenization pipeline**: split on word boundaries, lowercase, stem (e.g., `rust-stemmers`), hash terms (e.g., xxh3) for compact storage. Stopword removal is optional — stemming handles most noise
- **TF-IDF scoring**: term frequency × inverse document frequency with per-document normalization. Multi-token queries apply coverage penalties — results matching fewer query terms are ranked lower even if individual term scores are high
- **Serialization with postcard**: serialize the inverted index to disk using a compact binary format (postcard, bincode) for fast cold-start loading. Rebuild on content hash change

### Managed FTS Extension (Buy)

- **DuckDB FTS extension**: `PRAGMA create_fts_index('table', 'id', 'col1', 'col2')` creates a managed full-text index. Queries use `fts_main_table.match_bm25(id, 'query')`. Rebuild via a CLI command (`recall fts rebuild`) when schema or content changes
- **Multiple indexes for different content types**: create separate FTS indexes for distinct content domains (messages, analysis summaries, research artifacts). Each index covers different columns and may have different query patterns

## Anti-Patterns

- Building a custom search index when the database's built-in FTS is sufficient
- Using LIKE/ILIKE queries for full-text search (no ranking, poor performance)
- Forgetting to rebuild indexes after schema changes (stale results)

## Open Questions

- When to graduate from FTS to a dedicated search engine (Meilisearch, Typesense)
- Hybrid search combining TF-IDF with semantic/embedding search
