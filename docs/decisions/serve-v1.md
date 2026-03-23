# Decision Record: `meshant serve` HTTP server (v1)

**Issue:** #145
**Branch:** `145-meshant-serve`
**Phase:** 4 — Layer 3: Interactive graph output

---

## Problem

Phase 3 completed the trace substrate (TraceStore, Neo4j adapter, `meshant store`).
The substrate is queryable from the CLI, but has no interactive surface. The Web UI
(issue #146) requires a server that exposes analytical endpoints — every response
must carry cut metadata so the client can never receive a graph without naming its
observer position.

---

## Decision: `meshant serve` + `meshant/serve` package

Two additions:

1. **`meshant serve [--db bolt://...] [--port 8080] [<traces.json>]`** — starts a
   localhost HTTP server backed by either a Neo4j database or a JSON file.

2. **`meshant/serve` package** — contains the `Server` struct and four HTTP handlers.
   The CLI is thin glue (`cmd_serve.go`); all handler logic lives in the package and
   is testable via `httptest.NewRecorder` without a live server.

---

## Design decisions

### D1: Standard library only — `net/http` ServeMux

Four endpoints do not justify a third-party router. Go 1.22+ `http.ServeMux` supports
method+path patterns (`GET /articulate`) and returns 405 for wrong-method requests on
registered routes. The binary remains dependency-lean.

### D2: `meshant/serve` package, `cmd_serve.go` as thin glue

Handler logic lives in `meshant/serve`, not in `cmd/meshant`. This enables unit testing
via `httptest.NewRecorder` without spinning up a real HTTP listener. `cmd_serve.go`
parses flags, opens the store, creates a `serve.Server`, and manages the HTTP lifecycle.
The `serve` package has no knowledge of CLI flags.

### D3: Query per-request (no caching)

The `Server` holds a `store.TraceStore` and calls `ts.Query(ctx, store.QueryOpts{})` on
each HTTP request. No pre-filtering at the store layer; the analytical engine applies all
cut logic on the full substrate. This is consistent with `loadTraces` design decision D1
from `store-cli-v1.md` and preserves ANT correctness for Phase 4 (localhost, small
datasets). Phase 5 may need caching or cursor support.

### D4: `/diff` envelope `cut` uses `observer-a` as the named position

The `GraphDiff` has two full cuts (From, To) inside `data`. The envelope `cut` field
uses observer-a's cut — the diff is read *from* observer-a's position *toward* observer-b.
Both cuts are available in `data.from` and `data.to`. This is ANT-defensible: every
output is read from somewhere, and the envelope names that somewhere.

### D5: Observer enforced per-handler, not in middleware

Each handler performs its own observer check and returns a specific, ANT-reasoning
error message. Middleware would centralise the check but loses per-endpoint specificity
(`/diff` requires `observer-a`/`observer-b`, not a single `observer`). Four handlers
is the right granularity for four distinct messages.

### D6: Graceful shutdown via `signal.NotifyContext`

`cmdServe` registers SIGINT/SIGTERM via `signal.NotifyContext` and calls
`http.Server.Shutdown(ctx)` on receipt. The `TraceStore.Close()` is deferred after the
server shuts down. This is the idiomatic Go pattern; no custom process management needed.

### D7: `/element/:name` endpoint deferred

The kg-scoping-v1.md (§5.2) specifies `GET /element/:name`, which requires Neo4j
`:Element` node traversal. This endpoint is more naturally a Phase 4.2 (Web UI)
feature — it serves a node-click handler, not a standalone analytical query. It is
omitted from this issue and documented here as a deferred item.

---

## Response envelope

Every endpoint returns:

```json
{
  "cut": {
    "observer":    "string (required)",
    "from":        "RFC3339 or null",
    "to":          "RFC3339 or null",
    "tags":        ["string"] or null,
    "trace_count": 42,
    "shadow_count": 7
  },
  "data": { ... }
}
```

Missing observer → `400 Bad Request`:
```json
{"error": "observer is required — every graph is a positioned reading"}
```

Missing `observer-a`/`observer-b` on `/diff` → `400 Bad Request`:
```json
{"error": "diff requires two observer positions"}
```

---

## ANT tensions

**T1: Query-per-request loads full substrate on each HTTP call.**
Acceptable for Phase 4 (localhost, small datasets). Phase 5 may need caching or
server-side pagination. The tension is between ANT correctness (full substrate,
engine applies all cut logic) and HTTP response latency at scale.

**T2: `/traces` shadow_count is approximate.**
The `/traces` endpoint returns raw traces filtered by observer without calling
`graph.Articulate`. The `shadow_count` in the envelope is computed as:
`total traces − observer-filtered traces`. This counts *traces*, not *elements*.
The real element-level shadow requires articulation. Documented clearly in the
handler comment. The `/shadow` endpoint provides the element-level shadow.

**T3: `meshant serve` is a mediator not recorded as a Trace.**
Running the server is an observation act — the framework observes the mesh but
does not observe itself observing. Named here (consistent with kg-scoping-v1.md §6.4);
intentionally deferred.

**T4: `/diff` envelope `cut` is directional.**
The diff is read from observer-a toward observer-b. The envelope names observer-a's
position; observer-b's full cut is in `data.to`. This is a positional reading, not
a symmetric comparison.

---

## Files

| File | Purpose |
|------|---------|
| `meshant/serve/response.go` | `CutMeta`, `Envelope`, `ErrorBody`; `writeJSON`, `writeError`, `cutMetaFromGraph`, `parseQueryTime`, `parseLimit` |
| `meshant/serve/server.go` | `Server` struct, `NewServer`, `ServeHTTP` (ServeMux routing) |
| `meshant/serve/handlers.go` | `handleArticulate`, `handleDiff`, `handleShadow`, `handleTraces`, `filterTraces` |
| `meshant/serve/handlers_test.go` | Black-box tests via `httptest.NewRecorder`; 82.3% coverage |
| `meshant/cmd/meshant/cmd_serve.go` | `cmdServe` — CLI flag parsing, store opening, graceful shutdown |
| `meshant/cmd/meshant/cmd_serve_test.go` | Flag/error tests for `cmdServe` |

Modified:

| File | Change |
|------|--------|
| `meshant/cmd/meshant/main.go` | Added `"serve"` case + usage line |

---

## Definition of done

- [x] `meshant serve --port 8080 traces.json` starts server
- [x] `meshant serve --db bolt://...` accepted (real Neo4j path via build tag)
- [x] All four endpoints return `Envelope` with `cut` metadata
- [x] Missing observer → 400 with ANT-reasoning error
- [x] Unknown routes → 404; non-GET methods → 405
- [x] Graceful shutdown on SIGINT/SIGTERM
- [x] `go test ./...` passes, `go vet ./...` clean
- [x] 82.3% test coverage on `meshant/serve` package (≥ 80%)
- [x] ANT tensions documented (T1–T4)
- [x] Decision record committed
