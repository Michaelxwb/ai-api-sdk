# Project Guidelines

## Team Identity

- Team: AI Platform SDK
- Project: `github.com/Michaelxwb/ai-api-sdk`
- Language: Go (`go 1.23`)
- Positioning: Unified multi-provider AI access SDK (library-first, not server app)

## Core Principles

- SDK core must stay lightweight and composable: `auth/client/config/provider/session`.
- Keep stable contracts small (`SessionStore`, `ProviderSpec`) and extend via optional interfaces/options.
- Do not force logging framework choices on SDK callers; return structured errors instead.
- Prefer backward-compatible evolution: additive changes over breaking signature changes.
- Separate responsibilities clearly: provider protocol logic in `provider/impls/*`, orchestration in `client/`.

## Mandatory Engineering Rules

- All I/O and network paths must respect `context.Context` and timeout boundaries.
- Error strings must include source prefix (`client:`, `auth manager:`, `session store:`...); wrap external errors with `%w`.
- Limit response-body reads (`io.LimitReader` 4MB); truncate untrusted body text in error surfaces (4KB).
- Validate custom header/query values to prevent CRLF injection (`\r\n` check).
- Shared mutable state must be concurrency-safe; copy slices before persistence or mutation.

## Forbidden Patterns

- Logging (`log.*`/`slog.*`) inside SDK core packages (`auth/client/config/provider/session`).
- `panic` for recoverable runtime issues.
- Unparameterized SQL.
- Swallowing parse errors (`_ = json.Unmarshal(...)`).
- Importing `examples/` into SDK core.
- Adding methods directly to stable core interfaces (`SessionStore`, `ProviderSpec`) for one-off features.

## Change Checklist

- Provider registration complete: `init()` + `provider/provider.go` blank import.
- `remote_session` vs `local_history` semantics explicit and tested.
- If touching session persistence: verify `SessionStore` contract + `OnStoreError` callback wiring.
- If touching auth flow: check `%w` wrapping consistency + OAuth 401 retry body reset.
- Error messages follow `"package: description"` prefix convention.
- Run tests: `go test ./test/...`.

## Spec Loading

This project uses the code-flow two-tier spec system.

- Tier 0 `_map.md`: manual read for navigation context.
- Tier 1 constraint specs: auto-injected by hooks based on edited files.

### Spec Index

| Spec File | Injected When Editing |
|-----------|----------------------|
| `directory-structure.md` | Package structure, new providers, imports |
| `platform-rules.md` | Provider impl, session mode, Quick API |
| `error-handling.md` | Error wrapping, error types, sentinel errors |
| `code-quality-performance.md` | Security, concurrency, HTTP body limits |
| `logging.md` | Any logging or observability changes |
| `database.md` | SessionStore, SQL, session persistence |

### Assistant Responsibility

1. Infer domain from the task (this repo is primarily backend Go SDK).
2. Read `.code-flow/specs/backend/_map.md` before substantial code changes.
3. Rely on auto-injected Tier 1 constraints; do not manually duplicate them.
4. For error handling changes: consult `error-handling.md` for prefix conventions and sentinel error rules.
5. When changes span docs/examples/tests, anchor decisions on backend map and current code structure.

Do not ask users which spec to load; use path + task intent to decide.
