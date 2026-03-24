# Decision Record: `meshant store` + `--db` flag (v1)

**Issue:** #144
**Branch:** `144-store-cli-db-flag`
**Phase:** 3 — Layer 1: Trace substrate

---

## Problem

The `Neo4jStore` (issue #143) implements `TraceStore` but was unreachable from the CLI.
Users had no way to ingest traces into the database or run analytical commands against it.
The analytical engine was coupled to `loader.Load` (JSON file on disk) at every call site.

---

## Decision: `meshant store` + `--db` flag on analytical commands

Two additions:

1. **`meshant store [--db bolt://...] <traces.json>`** — reads canonical traces from a
   JSON file and writes them to the connected DB via `TraceStore.Store()`. Idempotent on
   trace ID.

2. **`--db bolt://...`** flag on `articulate`, `diff`, `shadow`, `gaps`, `follow`,
   `bottleneck` — switches the trace source from a JSON file to the database. The JSON
   file positional argument and `--db` are mutually exclusive.

---

## Design decisions

### D1. No pre-filtering at the store layer for analytical commands

When `--db` is used, `loadTraces` calls `ts.Query(ctx, store.QueryOpts{})` with no
filters. The full substrate is returned to the analytical engine, which performs all cut
logic (observer filter, time window, tags) via `graph.Articulate`.

**Why:** Pre-filtering at the store layer partially commits a cut before the analytical
engine sees any data (the T1 tension documented in both #142 and #143). Loading the full
substrate preserves the same semantics as `loader.Load` and avoids deepening this tension.
The cost is efficiency at scale; that is a Phase 4+ concern.

**Implication:** `QueryOpts.Observer` and `QueryOpts.Tags` remain unused by `loadTraces`.
They exist for callers that explicitly want server-side pre-filtering (future Phase 4 work).

### D2. Build-tag factory (`db_factory.go` / `db_factory_neo4j.go`)

Two mutually exclusive files implement `openDB(ctx, dbURL) (store.TraceStore, error)`:

- `db_factory.go` (`//go:build !neo4j`): returns a clear error "rebuild with -tags neo4j"
  for any non-empty `dbURL`. Default binary is unaffected.
- `db_factory_neo4j.go` (`//go:build neo4j`): creates and returns `Neo4jStore`.

This follows the same build-tag isolation pattern as the `Neo4jStore` itself (#143).

### D3. Credentials from environment, URL from flag

`--db` accepts the Bolt URL only. Credentials are never exposed as CLI flags
(they would appear in `ps` output and shell history). The env convention:

```
MESHANT_DB_URL   — Bolt URL (also the default for --db)
MESHANT_DB_USER  — username (default: "neo4j" if absent)
MESHANT_DB_PASS  — password
MESHANT_DB_NAME  — target database (empty = driver default "neo4j")
```

`MESHANT_DB_URL` is also read as the default value for the `--db` flag, so users
who configure it once via env do not need to pass `--db` every invocation.

### D4. Injected `TraceStore` in `cmdStore`

```go
func cmdStore(w io.Writer, ts store.TraceStore, args []string) error
```

`ts` may be nil (real path: `openDB` is called) or pre-built (test path: injected
`JSONFileStore`). This follows the same pattern as `cmdExtract(w, client, args)` with
`llm.LLMClient`. Integration tests with a real Neo4j instance are behind the `neo4j`
build tag; unit tests use the injected `JSONFileStore`.

### D5. `loadTraces` shared helper in `main.go`

Rather than duplicating the DB/file branching in six commands, `loadTraces` centralises it:

```go
func loadTraces(ctx context.Context, dbURL string, fileArgs []string) ([]schema.Trace, func(), error)
```

Returns a cleanup function (closes the store) that callers must defer. For JSON file
loading the cleanup is a no-op. Mutual exclusion (both `--db` and file present) is
validated per-command before `loadTraces` is called, so each command can produce a
command-specific error message.

---

## ANT tensions

**T1 (carried from #142, #143): Full substrate load trades efficiency for ANT correctness.**
Loading the full trace set avoids premature cut commitment but may be expensive for large
databases. Accepted for Phase 3; Phase 4 (interactive graph output) will reconsider with
server-side pagination or cursor support.

**T2: The name "store" encodes a storage metaphor.**
Naming the command "store" frames the database as a container for pre-existing objects,
which slightly contradicts ANT's relational ontology (traces exist in relation to the
observation act, not independently). The name was chosen for CLI legibility; the tension
is acknowledged but not resolvable at the CLI layer without sacrificing usability.

---

## Files

| File | Build tag | Purpose |
|------|-----------|---------|
| `cmd/meshant/db_factory.go` | `!neo4j` | `openDB` stub — returns "rebuild" error for non-empty URL |
| `cmd/meshant/db_factory_neo4j.go` | `neo4j` | `openDB` — creates `Neo4jStore` from env credentials |
| `cmd/meshant/cmd_store.go` | — | `meshant store` subcommand |
| `cmd/meshant/cmd_store_test.go` | — | Unit tests (injected `JSONFileStore`) |
| `cmd/meshant/cmd_db_test.go` | — | `--db` error-path tests for analytical commands |
| `cmd/meshant/main.go` | — | `loadTraces`, `noop`, "store" dispatch |
| `cmd/meshant/cmd_{articulate,diff,shadow,gaps,follow,bottleneck}.go` | — | `--db` flag added |

---

## Definition of done

- [x] `meshant store` functional and idempotent
- [x] `--db` flag on all six analytical commands
- [x] `--db` and `<file>` mutually exclusive with clear error
- [x] `!neo4j` build gives clear "rebuild" error for `--db` usage
- [x] `go test ./...` passes (no tag)
- [x] `go build -tags neo4j ./...` and `go vet -tags neo4j ./...` clean
- [x] Unit tests use injected store (no Neo4j required)
- [x] Decision record written
- [x] ANT tensions documented
