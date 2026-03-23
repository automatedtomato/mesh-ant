# Decision Record: Neo4j Adapter (v1)

**Issue:** #143
**Branch:** `143-neo4j-adapter`
**Phase:** 3 — Layer 1: Trace substrate

---

## Problem

`JSONFileStore` implements `TraceStore` for small-to-medium local datasets.
At scale, it loads all traces into memory for every query. An indexed graph
database unlocks efficient traversal queries — "all traces involving element X",
"traces in time window W from observer O" — without loading the full corpus.

---

## Decision: `Neo4jStore` implementing `TraceStore`

`Neo4jStore` in `meshant/store/neo4j_store.go` implements the `TraceStore`
interface (defined in #142) against a Neo4j-compatible graph database using
the official `neo4j-go-driver/v5`.

---

## Design decisions

### D1. Build tag `neo4j`

All Neo4j files carry `//go:build neo4j`. The default binary does not link
against the driver. Users who want the Neo4j backend build with:

```
go build -tags neo4j ./...
go test -tags neo4j ./store/
```

**Why:** The go.mod currently has two dependencies. Adding the neo4j driver
(with transitive deps) to every build — even when only the JSON backend is
needed — adds weight for no gain. A build tag keeps the default binary lean
and the dev loop fast for non-DB work.

**Operational note:** `go mod tidy` without the tag would prune the driver
from go.mod since it cannot see the conditional import. Always run:

```
go mod tidy -tags neo4j
```

after modifying go.mod in a neo4j context.

### D2. Driver: `github.com/neo4j/neo4j-go-driver/v5` (v5.28.4)

The current maintained line (v4 is deprecated). Compatible with Neo4j 4.4+
and 5.x servers. Also compatible with Memgraph and other Neo4j-protocol
backends — only the connection string changes.

### D3. Single transaction for `Store()`

All traces in one `ExecuteWrite` transaction. All-or-nothing, matching the
`JSONFileStore` atomicity guarantee. Trace batches from `meshant store` are
small (typically 10–50 traces per ingestion session); batching is not needed
at this scale.

Validation runs before the transaction opens: if any trace is invalid, the
database is never touched.

### D4. `Neo4jConfig` struct constructor

```go
type Neo4jConfig struct {
    BoltURL  string
    Username string
    Password string
    Database string // empty = driver default
}
```

A struct is forward-compatible (future fields: TLS config, pool size, etc.)
without breaking callers. Positional args would not be.

### D5. Timestamps as RFC3339Nano strings

Timestamps are stored as property strings on `:Trace` nodes using
`time.RFC3339Nano` in UTC. Cypher comparisons (`t.timestamp >= $from`) use
lexicographic ordering, which is correct because all timestamps are UTC and
ISO 8601 UTC strings sort chronologically.

**Alternative rejected:** Neo4j native `datetime()`. The driver's `time.Time`
↔ Neo4j `DateTime` mapping has historically had timezone edge cases in
complex drivers. String storage gives full round-trip fidelity and is simpler
to reason about.

### D6. Integration tests behind the same build tag

Tests live in `neo4j_store_test.go` with `//go:build neo4j`. They require:

```
MESHANT_NEO4J_TEST_URL=bolt://localhost:7687  # required
MESHANT_NEO4J_TEST_USER=neo4j                 # default: "neo4j"
MESHANT_NEO4J_TEST_PASS=neo4j                 # default: "neo4j"
```

Without the env var, the test file is not compiled. `go test ./...` always
passes. `go test -tags neo4j ./store/` runs the full integration suite.

Each test clears the database before and after via a dedicated driver
connection (not through the store under test, so cleanup cannot mask store
bugs).

### D7. Graph schema follows kg-scoping-v1.md §1 (overrides issue description)

The issue listed `Observer` and `Element` as separate nodes with `OBSERVED_BY`
and `INVOLVES` relationships. The approved scoping document (kg-scoping-v1.md)
takes precedence:

- `Observer` is a **property** on `:Trace`, not a node. The observer is a
  position taken in a specific trace, not a persistent identity. A separate
  `:Observer` node would falsely assert stable observer identity across traces
  — a premature closure the framework explicitly refuses (Principle 8).
- Relationships are `SOURCE_OF` (Element→Trace) and `TARGETS` (Trace→Element),
  not `INVOLVES`.

---

## Graph schema

```
(:Element {name})-[:SOURCE_OF]->(:Trace {
    id, timestamp, what_changed, observer, mediation, tags
})
(:Trace)-[:TARGETS]->(:Element {name})
```

No `:Observer` node. No `:Session` or `:TraceDraft` nodes (future phases).

---

## Cypher patterns

### Store (one per trace, in a single transaction)

```cypher
MERGE (t:Trace {id: $id})
SET t.timestamp    = $timestamp,
    t.what_changed = $what_changed,
    t.observer     = $observer,
    t.mediation    = $mediation,
    t.tags         = $tags
WITH t
FOREACH (srcName IN $sources |
  MERGE (src:Element {name: srcName})
  MERGE (src)-[:SOURCE_OF]->(t)
)
WITH t
FOREACH (tgtName IN $targets |
  MERGE (tgt:Element {name: tgtName})
  MERGE (t)-[:TARGETS]->(tgt)
)
```

`FOREACH` (not `UNWIND`) is used for element relationships because `UNWIND []`
produces zero rows and would silently drop the trace from further processing.
`FOREACH []` iterates zero times and leaves the trace visible.

### Query (combined with all filters)

```cypher
MATCH (t:Trace)
WHERE t.observer = $observer
  AND t.timestamp >= $from AND t.timestamp <= $to
  AND ALL(tag IN $tags WHERE tag IN t.tags)
OPTIONAL MATCH (src:Element)-[:SOURCE_OF]->(t)
WITH t, collect(DISTINCT src.name) AS sources
OPTIONAL MATCH (t)-[:TARGETS]->(tgt:Element)
RETURN t, sources, collect(DISTINCT tgt.name) AS targets
ORDER BY t.timestamp ASC
LIMIT $limit
```

WHERE clauses are added dynamically from non-zero `QueryOpts` fields. The
two-stage OPTIONAL MATCH (with intermediate WITH) avoids the source × target
Cartesian product.

---

## ANT tensions

These tensions are carried into the Neo4j adapter. None are resolved here.

**T1 (carried from #142): Observer pre-filter partially commits a cut.**
The Cypher `WHERE t.observer = $observer` clause performs a real cut inside
the database, before the analytical engine sees any data. The engine receives
only traces matching that observer, with no knowledge of excluded traces. This
is more concrete than the JSONFileStore equivalent — it happens server-side.

**T2 (carried from #142): `LIMIT` truncates the substrate.**
`LIMIT $limit` in Cypher caps results without the engine knowing what was
excluded. The engine operates on a truncated substrate and cannot know it.

**T3: String equality as Element equivalence criterion.**
`MERGE (e:Element {name: $name})` collapses all occurrences of the same
string into one `:Element` node across all traces. This makes co-occurrence
traversable, but the equivalence criterion is unexamined string equality. Two
traces sharing a source string may refer to different assemblages. Per
kg-scoping-v1.md §1.3: this is a provisional commitment, not a final one.
It should be revisited when `EquivalenceCriterion` is applied to the graph
substrate (#95).

**T4: Relationship direction encodes analytical position.**
`SOURCE_OF` goes Element→Trace and `TARGETS` goes Trace→Element. This
direction is not neutral — it reads source as origin and target as effect. The
scoping document mandates this schema, and this adapter implements it, but the
directional commitment should be named. ANT does not privilege source over
target in causal terms.

---

## Files

| File | Build tag | Purpose |
|------|-----------|---------|
| `meshant/store/neo4j_cypher.go` | `neo4j` | Cypher builders and Go↔Neo4j type conversion |
| `meshant/store/neo4j_store.go` | `neo4j` | `Neo4jStore`, `Neo4jConfig`, `NewNeo4jStore`, all 4 interface methods |
| `meshant/store/neo4j_store_test.go` | `neo4j` | Integration tests (requires `MESHANT_NEO4J_TEST_URL`) |

No existing files modified.

---

## Definition of done

- [x] `Neo4jStore` satisfies `TraceStore` interface (compile-time check)
- [x] All JSONFileStore behavioral tests have Neo4j equivalents
- [x] Element deduplication (MERGE semantics) tested
- [x] `SOURCE_OF`/`TARGETS` relationship directions tested
- [x] `go test ./...` (without tag) passes — Neo4j code not compiled
- [x] `go build -tags neo4j ./...` and `go vet -tags neo4j ./...` clean
- [x] Decision record written
- [x] Codemap and todo.md updated
- [x] ANT tensions documented (T1–T4)
