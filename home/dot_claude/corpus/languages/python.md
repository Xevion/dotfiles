---
name: python
category: languages
last_audited: 2026-03-26
exemplars:
  - repo: Xevion/doujin-ocr-summary
    path: sidecar/
    note: FastAPI OCR sidecar with basedpyright, ruff, pytest, uv
---

# Python

## Philosophy

Type hints on every public and internal function. Untyped signatures lose the value of static analysis and force callers to inspect the body to understand contracts.

Dataclasses over dicts for structured data. A `dict[str, object]` return type tells the caller nothing; a dataclass documents the contract at the call site and enables static analysis.

Explicit over implicit: imports are absolute, `from __future__ import annotations` appears at the top of every file, and suppression comments always include the rule code (`# type: ignore[attr-defined]`, `# noqa: F401`). Bare `# type: ignore` or bare `# noqa` are rejected.

Modern toolchain by default: **uv** for package management and virtual environments, **ruff** for formatting and linting in one pass, **basedpyright** for strict type checking. These replace the older pip/black/isort/mypy stack.

Prefer narrowing over suppression. Use `isinstance` guards, `cast()`, or `typing.Protocol` before reaching for `# type: ignore`.

## Conventions

### Toolchain

| Tool | Role | Config |
|---|---|---|
| **uv** | Package management, venvs, lockfiles | `pyproject.toml` `[tool.uv]` |
| **ruff** | Lint + format (replaces black, isort, flake8) | `pyproject.toml` `[tool.ruff]` |
| **basedpyright** | Type checking (stricter basedpyright fork) | `pyrightconfig.json` |
| **pytest** | Tests + markers | `pyproject.toml` `[tool.pytest.ini_options]` |

All checks are orchestrated by a single `just check` command (or equivalent project script) that runs typecheck, lint, format, and test in sequence.

### Type Annotations

Every file starts with `from __future__ import annotations`. This enables PEP 563 deferred evaluation, allowing forward references without quotes everywhere.

```python
from __future__ import annotations

from dataclasses import dataclass, field
from typing import ClassVar


@dataclass
class DetectionResult:
    bbox: BBox
    confidence: float
    language: str = "unknown"
    metadata: dict[str, object] = field(default_factory=dict)
```

Use `ClassVar` for class-level attributes that should not appear in `__init__`:

```python
class TextDetector(ABC):
    name: ClassVar[str]
    languages: ClassVar[list[str]] = []
```

### Exception Handling

Never bare `except:` or `except BaseException:`. Catch the narrowest exception type possible. `except Exception` is acceptable only as a top-level handler catch-all (log + 500 response), never in business logic.

Every catch block that doesn't re-raise must log with `exc_info=True` or include the exception value. Never `except SomeError: pass`.

```python
try:
    data = base64.b64decode(image_b64)
except binascii.Error as e:
    logger.warning("Invalid base64 input: %s", e)
    raise HTTPException(status_code=400, detail="Invalid base64-encoded image data.") from None
```

### Structured Logging

Use stdlib `logging` via `logger = logging.getLogger(__name__)`. Never `print()` for operational output — reserve `print()` for user-facing CLI output to stdout, with logging going to stderr.

Level discipline: `DEBUG` for per-item detail, `INFO` for request-level flow, `WARNING` for fallbacks and degraded behavior, `ERROR` for failures that will surface to the caller.

Heavyweight operations (model loading, inference, downloads) must log start and completion with timing:

```python
logger.info("Loading model: %s", name)
t0 = time.monotonic()
# ... load ...
logger.info("Model %s loaded in %.1fms", name, (time.monotonic() - t0) * 1000)
```

Centralize logging setup in a dedicated `log.py` module with a `setup_logging()` function. Suppress noisy third-party loggers to `WARNING` by default.

### basedpyright Configuration

Use basedpyright (the stricter fork) over vanilla pyright. Enable these basedpyright-exclusive rules:

```json
{
  "reportImportCycles": "warning",
  "reportMissingSuperCall": "warning",
  "reportImplicitOverride": "warning",
  "reportIgnoreCommentWithoutRule": "warning"
}
```

When working with ML libraries or other poorly-typed third-party code, disable the `reportUnknown*` family and `reportAny` — but document why in the config file, and exclude the specific library paths from checking entirely. Don't silence errors globally to hide real problems.

### pytest Conventions

Tests live in a `tests/` directory. Shared fixtures go in `conftest.py`. Use `@pytest.mark.anyio` with `httpx.AsyncClient` and `ASGITransport` for async endpoint tests.

Stub/fake implementations for unit tests must never pollute production registries. Create isolated instances:

```python
class StubRecognizer(TextRecognizer):
    name = "stub-recognizer"

    def __init__(self, text: str = "hello", confidence: float = 1.0):
        super().__init__()
        self._text = text
        self._confidence = confidence

    @override
    def load(self) -> None:
        pass

    @override
    def recognize(self, image: Image.Image) -> RecognitionResult:
        return RecognitionResult(text=self._text, confidence=self._confidence)
```

Mark slow tests requiring real model weights or external resources with a custom marker (`@pytest.mark.requires_models`) and register it in `pyproject.toml`.

### uv Package Management

Use `uv` for all package operations. Declare dependencies in `pyproject.toml`, separate dev dependencies under `[dependency-groups]`:

```toml
[dependency-groups]
dev = [
    "pytest>=9",
    "ruff>=0.11",
    "basedpyright>=1.38.2",
]
```

Use `uv run` to execute scripts within the managed environment without activating it manually. Use `[tool.uv.override-dependencies]` to pin broken transitive deps without forking them.

### Ruff Rule Selection

Enable rules beyond the basic E/F set. A well-rounded selection:

```toml
[tool.ruff.lint]
select = ["E", "F", "I", "UP", "B", "SIM", "S", "PT", "RUF", "A", "C4"]
```

- `I` — isort (import ordering, replaces standalone isort)
- `UP` — pyupgrade (modernize syntax for the target Python version)
- `B` — flake8-bugbear (common bug patterns including mutable defaults)
- `SIM` — flake8-simplify (simplifiable constructs)
- `S` — flake8-bandit (security checks)
- `PT` — flake8-pytest-style (pytest conventions)
- `A` — flake8-builtins (shadowing builtins like `id`, `list`, `type`)
- `C4` — flake8-comprehensions (prefer comprehensions over `list(map(...))`)

Document every `ignore` entry with an inline comment explaining why.

## Anti-Patterns

**Bare except clauses** — `except:` and `except BaseException:` swallow `KeyboardInterrupt`, `SystemExit`, and `GeneratorExit`. Always name the exception type.

**Mutable default arguments** — `def fn(items=[])` shares state across calls. Use `None` with a guard, or let ruff's `B006` catch it. For dataclass fields use `field(default_factory=list)`.

**Untyped function signatures** — a function with no annotations produces no type-checking value and forces callers to inspect the body to understand contracts. Even private helpers should be annotated.

**Bare suppression comments** — `# type: ignore` without a rule code and `# noqa` without a rule code disable all checks instead of the specific one you intend. Both ruff and basedpyright can be configured to reject these.

**`print()` for operational output** — operational messages belong in the logging system so they can be suppressed, filtered, and redirected. Reserve `print()` for explicit user-facing CLI output.

## Open Questions

**uv vs poetry long-term** — uv has largely won on speed and workspace support, but poetry has a larger ecosystem of plugins and more mature publishing workflows. The main question is whether uv's `[dependency-groups]` (PEP 735) gains enough tooling support to fully replace poetry's dev-extras pattern.

**`typing.Protocol` adoption** — Protocols let you express structural subtyping without inheritance, which is cleaner for plugin architectures and capability checks. The open question is where to draw the line: using Protocols everywhere can make code harder to navigate compared to explicit ABCs, and IDE support for protocol-based dispatch varies.

**basedpyright vs pyright upstream convergence** — basedpyright adds useful rules (`reportImportCycles`, `reportIgnoreCommentWithoutRule`) that aren't in vanilla pyright. Whether to track upstream pyright releases or stay on the basedpyright fork depends on how quickly these rules land upstream and whether the fork stays in sync.

**Async vs sync FastAPI handlers** — FastAPI runs sync handlers in a threadpool by default, which can cause contention under load. The question is whether to default all handlers to `async def` (even when not doing I/O) to avoid threadpool overhead, or keep sync handlers for CPU-bound routes and accept the threadpool behavior.
