---
name: go
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/doujin-ocr-summary
    path: internal/server/
    note: Chi router with writeJSON buffer pattern, dual timeouts, sentinel error mapping, slog context propagation
---

# Go

## Philosophy

Go favors simplicity and explicitness over cleverness. Errors are values returned by functions, not exceptions thrown up the stack — handle them where they occur or wrap them with context and return them. Composition via interfaces beats inheritance; keep interfaces small and define them in the consumer package, not the producer. Prefer the standard library over third-party dependencies when the stdlib solution is adequate. `context.Context` is the canonical mechanism for cancellation, deadlines, and request-scoped value propagation — pass it as the first argument to any function that may block or needs request identity.

## Conventions

### writeJSON with buffer intermediary

Always encode JSON into a `bytes.Buffer` before writing the response. Writing directly to `http.ResponseWriter` commits the 200 status before encoding is complete; an encoding error after `WriteHeader` cannot change that status.

```go
func writeJSON(w http.ResponseWriter, status int, v any) error {
    var buf bytes.Buffer
    if err := json.NewEncoder(&buf).Encode(v); err != nil {
        return err
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _, err := buf.WriteTo(w)
    return err
}
```

### Dual per-route timeouts

Set both a `context.WithTimeout` (cancels in-flight DB/RPC work) and `http.ResponseController.SetWriteDeadline` (prevents slow-write hangs). Either alone leaves a failure mode unhandled.

```go
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
defer cancel()

rc := http.NewResponseController(w)
rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
```

### Sentinel errors + MapDBError boundary

Define domain sentinel errors as package-level vars. Translate driver errors to domain errors at the persistence boundary via a single mapping function. Handlers stay free of driver imports and switch only on domain errors.

```go
var (
    ErrNotFound = errors.New("not found")
    ErrConflict = errors.New("conflict")
)

func MapDBError(err error) error {
    if errors.Is(err, pgx.ErrNoRows) {
        return ErrNotFound
    }
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23505" {
        return ErrConflict
    }
    return err
}
```

### slog for structured logging with context propagation

Attach a request-scoped `*slog.Logger` (enriched with request ID) to the context in middleware. Retrieve it via a helper rather than passing the logger through every function signature.

```go
func LoggerFromContext(ctx context.Context) *slog.Logger {
    if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
        return l
    }
    return slog.Default()
}
```

### Union-type constraints for structurally identical generated types (situational)

When code-generation tools (e.g., sqlc) produce multiple structurally identical row types that a single converter should handle, a union constraint with a type switch can avoid duplicating conversion logic. Use sparingly — it adds a type switch cost and is only justified when the alternative is near-identical duplicated functions.

```go
type RowVariant interface {
    UserRow | AdminRow
}

func toUser[T RowVariant](row T) User {
    switch r := any(row).(type) {
    case UserRow:
        return User{ID: r.ID, Name: r.Name}
    case AdminRow:
        return User{ID: r.ID, Name: r.Name}
    default:
        panic("unreachable")
    }
}
```

## Anti-Patterns

- **Naked returns in complex functions** — named return variables with bare `return` statements obscure what is being returned and make functions harder to read; use explicit returns.
- **`init()` abuse** — `init` runs implicitly at program start with no way to pass arguments or return errors; prefer explicit initialization in `main` or constructor functions so startup failures surface cleanly.
- **Interface pollution** — defining interfaces in the producer package, or defining wide interfaces with methods callers don't need, forces unnecessary coupling; define interfaces where they are consumed and keep them as narrow as the caller requires.
- **Ad-hoc `map[string]any` response shapes** — untyped response maps skip compile-time checks and make API contracts invisible; define concrete response structs even for simple payloads.

## Open Questions

- **Generic union constraint cost** — the type switch inside a union-constrained generic function is not eliminated by the compiler; for hot paths, benchmark against duplicated concrete functions before committing to the pattern.
