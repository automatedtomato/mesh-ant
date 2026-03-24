# Decision Record: ANT-like Knowledge Graph — Scoping Document

**Issue:** #141
**Phase:** 2 — Form 3 scoping document
**Date:** 2026-03-22
**Status:** Approved
**Follows:** v2.0.0 (LLM-assisted ingestion, `llm-boundary-v2.md`)

---

## Purpose

This document resolves all open design questions before any Phase 3 (Layer 1) or Phase 4
(Layer 3) code is written. It is a planning milestone, not an implementation. No Phase 3
implementation begins until this document is approved.

The six questions come from `tasks/plan_post_v2.md` Phase 2:

1. Schema — how are `Trace` records stored in Neo4j?
2. Adapter contract — what interface does the Go engine call?
3. Session handling — how are `SessionRecord` and `ExtractionConditions` stored?
4. Query pre-filtering — what DB queries replace `loader.Load()`?
5. Web UI contract — what JSON does `meshant serve` emit?
6. Conditions of reading — what does the server require before returning a cut?

---

## ANT constraint (carries across all six)

The graph is always already a cut. Every design choice below must be traceable to an ANT
commitment — the schema encodes analytical position, not transparent storage. The adapter
boundary is the enforcement point: if the cut is not named before the data enters the Go
analytical engine, it cannot be recovered later.

---

## 1. Schema

### 1.1 Node labels

| Label | Maps to | Key properties |
|---|---|---|
| `:Trace` | `schema.Trace` | `id`, `timestamp`, `what_changed`, `observer`, `mediation`, `tags` (list), `uncertainty_note` |
| `:Element` | Any string in `Source` or `Target` across all traces | `name` (unique) |
| `:TraceDraft` | `schema.TraceDraft` | `id`, `timestamp`, `source_span`, `source_doc_ref`, `what_changed`, `observer`, `mediation`, `extraction_stage`, `extracted_by`, `derived_from`, `criterion_ref`, `session_ref`, `uncertainty_note`, `intentionally_blank` (list), `tags` (list) |
| `:Session` | `llm.SessionRecord` + `llm.ExtractionConditions` | See §3 |

### 1.2 Relationship types

| Relationship | From | To | Meaning |
|---|---|---|---|
| `SOURCE_OF` | `:Element` | `:Trace` | This element appeared in `Trace.Source` |
| `TARGETS` | `:Trace` | `:Element` | This element appeared in `Trace.Target` |
| `IN_SESSION` | `:TraceDraft` | `:Session` | This draft was produced by this session |
| `DERIVED_FROM` | `:TraceDraft` | `:TraceDraft` | Draft revision chain (`TraceDraft.DerivedFrom`) |
| `PROMOTED_FROM` | `:Trace` | `:TraceDraft` | Canonical trace promoted from this draft |

### 1.3 ANT rationale for element normalisation

In ANT, source and target are not stable ontological categories — any element can be
source in one trace and target in another. Normalising strings into `:Element` nodes
makes that mobility traversable: a query can ask "in which traces did this element
act as source, and in which as target?" without privileging either role.

`Mediation` is stored as a property on `:Trace`, not a separate node. Mediators are
named within a specific trace — a mediation string in trace A and the same string in
trace B may refer to different mediating acts. Collapsing them into one `:Mediator`
node would falsely assert equivalence across traces. If follow-chain traversal later
shows that the same mediator appears systematically, that finding should emerge from
the data, not be imposed by the schema.

`Observer` is stored as a property on `:Trace` (same rationale as `Mediation`). The
observer is a position taken in a specific trace, not a persistent identity. A separate
`:Observer` node would suggest that observer identity is stable across traces — a
premature closure the framework explicitly refuses (see `docs/principles.md` Principle 8).

Tags are stored as a list property on `:Trace`, not separate nodes. Tags are descriptors
characterising the kind of difference; they are not actants.

**Standing tension — string equality as equivalence criterion.** The `:Element` node
deduplicates by string equality: `"incident-response-team"` in trace A and the same
string in trace B are stored as one node. This makes co-occurrence traversable, but
it is itself an equivalence criterion — and an unexamined one. Two traces sharing a
source string may be referring to different assemblages. The `:Element` node does not
assign actor status or ontological category, so this is not a violation; but it should
be revisited when the `EquivalenceCriterion` mechanism (see `docs/decisions/equivalence-criterion-v1.md`)
is applied to the graph substrate. String equality is a provisional commitment, not a
final one.

### 1.4 What is not in the graph

- API keys (never stored — `ExtractionConditions` omits them by design; see `llm/types.go`)
- Raw LLM responses (stored only in session files on disk, not in the graph)
- Intermediate prompt text (the `prompt_template` field stores the path, not the content)
- Span arrays from `meshant split` (spans are pre-trace material; they are not analytical
  objects and are not persisted in the graph)

---

## 2. Adapter contract

### 2.1 `TraceStore` interface

The Go analytical engine currently calls `loader.Load(path)` to get `[]schema.Trace`.
The adapter contract replaces that call with a narrow interface that both the JSON file
loader and the Neo4j adapter implement:

```go
// TraceStore is the storage interface the Go analytical engine calls.
// It is deliberately narrow: the engine needs traces filtered by cut axes;
// it does not need to know how or where traces are stored.
//
// The JSON file loader (existing behaviour) and the Neo4j adapter both
// implement this interface. Switching backends requires no changes to the
// analytical engine (Articulate, Diff, Shadow, Gaps, Follow, Bottleneck).
//
// The interface will live in meshant/store (new package, Phase 3).
type TraceStore interface {
    // Store persists traces. Idempotent on ID: storing a trace whose ID
    // already exists updates its properties.
    Store(ctx context.Context, traces []schema.Trace) error

    // Query returns traces matching the given options. The returned slice
    // is the pre-filtered substrate the analytical engine cuts from.
    // An empty result is valid (no traces match the criteria).
    Query(ctx context.Context, opts QueryOpts) ([]schema.Trace, error)

    // Get retrieves a single trace by ID. Returns (zero, false, nil) if
    // the trace does not exist, (trace, true, nil) if found.
    Get(ctx context.Context, id string) (schema.Trace, bool, error)

    // Close releases backend resources. Safe to call multiple times.
    Close() error
}

// QueryOpts specifies the pre-filtering criteria for a Query call.
// Each non-zero field adds a constraint (AND semantics).
// The analytical engine applies cut logic on the returned slice —
// the DB handles only efficient pre-filtering, not cut semantics.
type QueryOpts struct {
    // Observer filters to traces whose Observer field matches exactly.
    // Empty means no observer filter (all observers included).
    Observer string

    // Window filters to traces whose Timestamp falls within [From, To].
    // Zero values mean no bound on that end.
    Window schema.TimeWindow

    // Tags filters to traces that carry ALL of the listed tags.
    // Empty means no tag filter.
    Tags []string

    // Limit caps the number of returned traces. 0 means no limit.
    Limit int
}
```

### 2.2 Data flow

```
CLI flags
    │
    ▼
TraceStore.Query(ctx, opts)   ← narrow, swappable; returns []schema.Trace
    │
    ▼
Go analytical engine          ← unchanged; all ANT logic lives here
(Articulate, Diff, Shadow, Gaps, Follow, Bottleneck, ...)
    │
    ▼
output (text / JSON / DOT / Mermaid / Web UI endpoint)
```

### 2.3 JSON file loader implements TraceStore

The existing `loader.Load(path)` becomes the implementation body of
`JSONFileStore.Query()`. Pre-filtering options are applied in-memory (current behaviour
is to load all traces and let the engine filter; this is preserved for the JSON backend).
`JSONFileStore.Store()` writes traces as a JSON array to disk (replacing the manual JSON
editing workflow). This keeps the existing CLI fully functional during the Neo4j adapter
transition.

### 2.4 Package location

`meshant/store` — new package in Phase 3. Contains the `TraceStore` interface,
`QueryOpts`, and the `JSONFileStore` implementation. The Neo4j adapter is a separate
file in the same package (`neo4j_store.go`), guarded by a build tag or runtime config
so the binary can be built without Neo4j dependencies.

---

## 3. Session handling

`SessionRecord` and `ExtractionConditions` are stored as a single `:Session` node. The
`ExtractionConditions` fields are flattened into properties on `:Session` — they are a
flat property bag with no internal structure that warrants a separate node.

### 3.1 `:Session` node properties

| Property | Source field | Notes |
|---|---|---|
| `id` | `SessionRecord.ID` | Unique; used as the session reference in `TraceDraft.SessionRef` |
| `command` | `SessionRecord.Command` | `"extract"`, `"assist"`, `"critique"`, `"split"` |
| `timestamp` | `SessionRecord.Timestamp` | When the session ran |
| `input_path` | `SessionRecord.InputPath` | Path to source material |
| `output_path` | `SessionRecord.OutputPath` | Path to output file |
| `draft_count` | `SessionRecord.DraftCount` | Number of drafts or spans produced |
| `error_note` | `SessionRecord.ErrorNote` | Empty on success |
| `model_id` | `ExtractionConditions.ModelID` | Flattened from Conditions |
| `prompt_template` | `ExtractionConditions.PromptTemplate` | Path, not content |
| `criterion_ref` | `ExtractionConditions.CriterionRef` | May be empty |
| `system_instructions` | `ExtractionConditions.SystemInstructions` | Path or short ref |
| `source_doc_ref` | `ExtractionConditions.SourceDocRef` | Document identifier |
| `conditions_timestamp` | `ExtractionConditions.Timestamp` | When conditions were recorded |

The API key is absent by design (see `llm/types.go`).

### 3.2 Relationships involving `:Session`

- `(:TraceDraft)-[:IN_SESSION]->(:Session)` — links each draft to the session that
  produced it. `TraceDraft.SessionRef` maps to `Session.id`.
- `(:Trace)-[:DERIVED_FROM_SESSION]->(:Session)` — when a draft is promoted to a
  canonical trace, this relationship records the ingestion lineage. The promoted
  `Trace` does not carry `session_ref` as a property (canonical traces record
  analytical content, not ingestion mechanics — see `tracedraft.go` `Promote()`
  comment), but the relationship makes the lineage traversable.

### 3.3 Split sessions

`meshant split` produces a `:Session` node with `command = "split"`. It has no
`:TraceDraft` children (spans are pre-trace material). `DraftCount` records the span
count; `DraftIDs` is null. The `:Session` node is the provenance record for the split
act — it is the LLM mediating act that proposed candidate boundaries.

---

## 4. Query pre-filtering

### 4.1 Cypher for each QueryOpts field

**No filter (load all traces):**
```cypher
MATCH (t:Trace)
RETURN t
```

**Observer filter:**
```cypher
MATCH (t:Trace)
WHERE t.observer = $observer
RETURN t
```

**Time window filter:**
```cypher
MATCH (t:Trace)
WHERE t.timestamp >= $from AND t.timestamp <= $to
RETURN t
```

**Tag filter (ALL tags must be present):**
```cypher
MATCH (t:Trace)
WHERE ALL(tag IN $tags WHERE tag IN t.tags)
RETURN t
```

**Combined (observer + window + tags):**
```cypher
MATCH (t:Trace)
WHERE t.observer = $observer
  AND t.timestamp >= $from AND t.timestamp <= $to
  AND ALL(tag IN $tags WHERE tag IN t.tags)
RETURN t
ORDER BY t.timestamp ASC
LIMIT $limit
```

### 4.2 What the DB does and does not do

The DB applies pre-filtering only: observer match, time window, tag membership, and
row limit. It does not apply cut logic. Cut logic (shadow calculation, ShadowReason
assignment, element inclusion/exclusion, graph construction) stays entirely in the
Go analytical engine. The DB is used for what it is genuinely good at — indexed
retrieval over a large corpus — and nothing more.

This is the "Choice B hybrid" from `tasks/plan_post_v2.md`: the ANT logic lives in Go
and is never re-expressed in Cypher. Re-expressing cut semantics in a graph query
language would introduce drift from the carefully aligned Go implementation.

### 4.3 Element traversal queries (for the Web UI)

The Web UI needs to ask "what other traces involve this element?" — a query the JSON
loader cannot answer efficiently at scale. These queries use the normalised `:Element`
nodes:

**Traces where an element is source:**
```cypher
MATCH (e:Element {name: $name})-[:SOURCE_OF]->(t:Trace)
RETURN t
```

**Traces where an element appears in any role:**
```cypher
MATCH (t:Trace)
WHERE ((:Element {name: $name})-[:SOURCE_OF]->(t))
   OR (t)-[:TARGETS]->(:Element {name: $name})
RETURN t
```

These queries are called by the Web UI's node-click handler, not by the analytical
engine. The analytical engine always receives `[]schema.Trace` from `TraceStore.Query`.

---

## 5. Web UI contract

### 5.1 Response envelope

Every endpoint response uses this envelope. The `cut` field is mandatory; no endpoint
omits it. The server returns `400 Bad Request` if the required cut context is absent
(see §6).

```json
{
  "cut": {
    "observer": "string (required)",
    "from":     "RFC3339 timestamp or null",
    "to":       "RFC3339 timestamp or null",
    "tags":     ["string", "..."] or null,
    "trace_count":  42,
    "shadow_count": 7
  },
  "data": { ... }
}
```

`shadow_count` is the number of elements in shadow for this cut. It is always included,
never suppressed — shadow is a first-class output of the cut, not a footnote.

### 5.2 Endpoints

All endpoints require `?observer=<string>`. All return the envelope above.

| Endpoint | Required params | Optional params | `data` shape |
|---|---|---|---|
| `GET /articulate` | `observer` | `from`, `to`, `tags` | `MeshGraph` JSON (same schema as `--format json`) |
| `GET /diff` | `observer-a`, `observer-b` | `from`, `to`, `tags` | `GraphDiff` JSON |
| `GET /shadow` | `observer` | `from`, `to`, `tags` | `[]ShadowElement` JSON |
| `GET /traces` | `observer` | `from`, `to`, `tags`, `limit` | `[]Trace` JSON (raw substrate, cut applied) |
| `GET /element/:name` | `observer` (in query) | — | `[]Trace` JSON (all traces involving element, observer-filtered) |

`GET /traces` requires `observer` — there is no "view all without position" endpoint.
The raw trace list is still a positioned cut; the server enforces this at the boundary.

### 5.3 Web UI rendering requirements

- **Observer position selector required before any graph is shown.** The UI does not
  render until the user has selected an observer. This is not a UX guardrail — it is
  the ANT commitment made operational: every graph is a positioned reading.
- **Cut metadata always visible.** Observer, time window, trace count, shadow count
  are displayed in a persistent header, not hidden in a tooltip.
- **Shadow panel.** Named elements in shadow, with their `ShadowReason`s. Shadow is
  not a footnote; it is part of the graph.
- **Node click: full trace detail.** Clicking any node shows the full `Trace` record,
  including `observer`, `mediation`, `tags`, and — if `session_ref` is present — a
  link to the `SessionRecord` provenance chain.
- **Provenance panel (for LLM-produced traces).** For traces with a `session_ref`:
  `session_ref → SessionRecord → ExtractionConditions`. For traces with a `DerivedFrom`
  chain: show the revision chain back to the root draft.
- **Export.** Download current cut as JSON or DOT. The downloaded file includes the
  cut metadata; it is not a "raw graph" without position.

### 5.4 Frontend library

D3.js or Cytoscape.js. The choice is deferred to Phase 4 implementation. Both can
render the graph schema above. The constraint is that the observer-position selector
and cut-metadata header are non-negotiable — the library must accommodate them, not
the other way around.

---

## 6. Conditions of reading: what the server requires before returning a cut

### 6.1 What the server requires

| Missing | Response |
|---|---|
| `observer` on any endpoint | `400 Bad Request` — `{"error": "observer is required — every graph is a positioned reading"}` |
| `observer-a` or `observer-b` on `/diff` | `400 Bad Request` — `{"error": "diff requires two observer positions"}` |

The server never returns a graph without an observer position. This is the adapter
boundary enforcement — if the cut is not named before the data enters the engine, it
cannot be recovered. The error message names the ANT reason, not just the validation
failure.

### 6.2 What is never suppressed

- **Shadow elements and their reasons.** The `shadow_count` in the envelope and the
  `ShadowElement` list in `/shadow` responses are always present. A client that hides
  shadow is hiding the cut.
- **Observer in every trace.** The `observer` field is required on `schema.Trace`
  (`Trace.Validate()` rejects traces without it). The DB never stores a trace without
  `observer`. The analytical engine never receives a trace without `observer`. The Web
  UI always displays it.
- **Cut metadata in the envelope.** See §5.1.

### 6.3 What the provenance chain must show

For any trace reachable from the Web UI:
1. `Trace.observer` — the position from which the trace was made
2. If `tags` includes `"draft"` — link to the `TraceDraft` via `PROMOTED_FROM`
3. If the draft has `session_ref` — link to `:Session` (model, prompt, source doc)
4. If the draft has `derived_from` — show the full revision chain to the root

The provenance chain is never truncated. A user who clicks through to any trace must
be able to follow the full chain back to the source material and the conditions under
which the LLM mediated it.

### 6.4 The designer is inside the mesh

`meshant serve` is a participant in the mesh it serves. Running the server is not
currently recorded as a `Trace` (that gap is named here for later resolution, consistent
with the note in `tasks/plan_post_v2.md`). The gap is intentional — naming it is part
of the evidence trail.

---

## Open questions deferred to Phase 3

These questions cannot be fully resolved without implementation experience:

1. **Neo4j transaction scope.** `Store()` with a large trace slice: one transaction
   or batched? Decision deferred to `neo4j_store.go` implementation.
2. **Element deduplication on Store.** When two traces share a source string, the
   `:Element` node must be created once (MERGE semantics in Cypher). The exact MERGE
   key and conflict strategy is an implementation detail.
3. **`--db` flag format.** The connection string format (Bolt URI vs config file)
   is deferred. `bolt://localhost:7687` is the assumed default for local development.
4. **Build tag vs runtime config for Neo4j dependency.** Avoiding the Neo4j driver
   import when the JSON backend is sufficient. Deferred to `store` package design.
5. **Frontend library selection** (D3.js vs Cytoscape.js). Deferred to Phase 4.

---

## Definition of done

- [x] All six questions answered
- [x] This document committed as `docs/decisions/kg-scoping-v1.md`
- [ ] User confirms the document before Phase 3 implementation begins
