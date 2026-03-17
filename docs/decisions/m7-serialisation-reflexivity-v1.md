# Decision Record: Serialisation and Reflexive Tracing v1

**Date:** 2026-03-12
**Status:** Active
**Package:** `meshant/graph` (serial.go, reflexive.go); `meshant/schema` (trace.go)
**Branch merged:** `feat/m7-reflexive`

---

## What was decided

1. **Codec scope: codec only, no file persistence**
2. **TimeWindow zero bounds serialise as null**
3. **JSON field names: snake_case**
4. **Observer for reflexive traces is caller-supplied**
5. **Source for ArticulationTrace is caller-supplied, absent by default**
6. **Source for DiffTrace is derived from the input graphs**
7. **New tag value: "articulation"**
8. **What M7 does not close**

---

## Context

M7 addresses two connected gaps left open by M2:

**First gap (M2 Decision 5):** A MeshGraph is not a neutral object. Once produced, it can enter the mesh as a force — a deforestation map that triggers policy, a carbon-credit audit that suspends a market. The graph is not just a view of the mesh; it becomes part of the mesh. This noted architecturally but not implemented.

**Second gap (M2 Exclusion):** No persistence of articulations (graphs are in-memory only). Graphs were not serializable to JSON.

M7 closes both gaps by adding:
- A JSON codec for all graph types (MeshGraph, GraphDiff, Node, Edge, Cut, ShadowElement, ShadowShift, PersistedNode)
- Reflexive tracing: explicit opt-in functions to produce Traces recording the act of articulation or diffing
- A new schema tag: "articulation"

The decisions below record what form these changes take and what remains open.

---

## Decision 1 — Codec scope: codec only, no file persistence

**Chosen:** `serial.go` provides only the JSON marshalling/unmarshalling layer. No `WriteGraph` or `ReadGraph` helpers. No file I/O in the graph package.

**Why:**

The graph package is an articulation engine, not a storage system. Encoding to JSON and persisting to disk are separate concerns:

- **Encoding** (in-package): JSON codec for MeshGraph, GraphDiff, etc. Callers can `json.Marshal` to `[]byte`.
- **Persistence** (out-of-package): Callers decide where to store the bytes — file, HTTP response, database, message queue. Storage is not the graph package's responsibility.

This follows the Unix philosophy: each tool does one thing well. The graph package articulates; storage adapters use the codec.

**Implication:** File I/O helpers may be added in a future utilities package without modifying the graph package API.

---

## Decision 2 — TimeWindow zero bounds serialise as null

**Chosen:** Custom `MarshalJSON` and `UnmarshalJSON` on `TimeWindow`. Zero `Start` or `End` is encoded as JSON `null`, not as `"0001-01-01T00:00:00Z"`.

**Why:**

A zero `TimeWindow` bound means "unbounded" — it is not a real timestamp. The Go standard library's `time.Time` JSON codec encodes zero time as `"0001-01-01T00:00:00Z"`, which is technically correct but semantically misleading. Downstream consumers might mistake this for a real date boundary in 1 AD.

JSON `null` is the honest representation: "no lower bound" is encoded as `null`, not as a pretend date.

**Implementation:**

- `timeWindowWire` internal type uses `*string` for Start and End, allowing `nil` to represent unbounded.
- `MarshalJSON`: non-zero bounds are formatted as RFC3339 strings; zero bounds become `nil` pointers.
- `UnmarshalJSON`: JSON `null` decodes to zero `time.Time`; JSON strings are parsed as RFC3339. Other token types (numbers, booleans) are rejected with a clear error.
- Public API unchanged: `TimeWindow` fields remain `time.Time` (not pointers).

**Test coverage:** See `meshant/graph/serial_test.go`.

---

## Decision 3 — JSON field names: snake_case

**Chosen:** All JSON struct tags use `snake_case` (e.g., `json:"appearance_count"`, not `json:"appearanceCount"`).

**Why:**

Consistency with the trace schema established in M1. Trace fields use `snake_case` (e.g., `what_changed`, `trace_id`, `observer_positions`). Extending this convention to graph JSON maintains cross-package format coherence.

**Fields affected:**

- `MeshGraph`: `id`, `nodes`, `edges`, `cut`
- `Node`: `name`, `appearance_count`, `shadow_count`
- `Edge`: `trace_id`, `what_changed`, `mediation`, `observer`, `sources`, `targets`, `tags`
- `Cut`: `observer_positions`, `time_window`, `traces_included`, `traces_total`, `distinct_observers_total`, `shadow_elements`, `excluded_observer_positions`
- `ShadowElement`: `name`, `seen_from`, `reasons`
- `ShadowShift`: `name`, `kind`, `from_reasons`, `to_reasons`
- `PersistedNode`: `name`, `count_from`, `count_to`
- `GraphDiff`: `id`, `nodes_added`, `nodes_removed`, `nodes_persisted`, `edges_added`, `edges_removed`, `shadow_shifts`, `from`, `to`
- `TimeWindow`: `start`, `end`

---

## Decision 4 — Observer for reflexive traces is caller-supplied

**Chosen:** `ArticulationTrace(g MeshGraph, observer string, source []string)` and `DiffTrace(d GraphDiff, g1, g2 MeshGraph, observer string)` take `observer` as a required parameter. The caller provides it.

**Why:**

The framework does not have one fixed observer position. Every act of articulation is situated — the caller knows from which position they are articulating. This is consistent with `ArticulationOptions.ObserverPositions` being caller-supplied and with Principle 8: the designer is inside the mesh.

The observer who runs `Articulate` from the command line, a web endpoint, or a scheduled job is not implicit. Recording their position makes the observation apparatus traceable.

**Semantics:** The reflexive trace records "who articulated this" (the observer parameter) at what time (Timestamp), not "who this articulation is about" (which is the graph's subject matter).

**Error handling:** Both functions return an error if `observer` is empty. This fails fast: the caller cannot accidentally produce a trace that fails `schema.Validate()`.

---

## Decision 5 — Source for ArticulationTrace is caller-supplied, absent by default

**Chosen:** `ArticulationTrace(g MeshGraph, observer string, source []string)` accepts `source` as a parameter. If `nil` or empty, the trace's `Source` field is `nil` (encoded as omitted in JSON, due to `omitempty` tag).

**Why:**

The input `[]schema.Trace` to `Articulate` has no collective identity — traces are not an actor that initiated the articulation. The articulation emerges from the framework's process of rendering traces from a given observer position. There is no "source" that precedes the act.

An absent `Source` is the honest representation: the trace was not produced by a named prior actor in the mesh. It was produced by a process (Articulate) that has no entry point of its own.

**Caller flexibility:** If a caller has a stable dataset identifier (e.g., a Git commit hash, a dataset UUID, a filename), they may supply it via the `source` parameter. This makes the dependency traceable. But the default — `nil` — is the correct default.

**Consistency:** This respects M1's convention that `Source` is optional when attribution is unknown or unattributable.

---

## Decision 6 — Source for DiffTrace is derived from the input graphs

**Chosen:** `DiffTrace(d GraphDiff, g1, g2 MeshGraph, observer string)` does NOT accept a `source` parameter. `Source` is derived automatically from `g1.ID` and `g2.ID`: `["meshgraph:<g1.ID>", "meshgraph:<g2.ID>"]`.

**Why:**

Unlike `ArticulationTrace`, a diff has two identified prior actors: the two input MeshGraphs. Once both graphs have been identified (via `IdentifyGraph`), their graph-ref strings are the natural source of the diff. The diff did not emerge from nowhere; it is the result of comparing two specifically-named articulations.

Deriving the source automatically ensures the caller cannot get this wrong. If they forget to identify a graph, `DiffTrace` fails fast with a clear error message.

**Error handling:** All three parameters (`d`, `g1`, `g2`) must have non-empty `ID` fields. If any is empty, `DiffTrace` returns an error without producing a partial trace.

**Implication:** A diff is always traceable back to its input graphs. This closes the loop: the diff's inputs are visible in its reflexive trace.

---

## Decision 7 — New tag value: "articulation"

**Chosen:** Added `TagValueArticulation = "articulation"` to the schema tag vocabulary in `meshant/schema/trace.go`.

**Why:**

This is the first schema vocabulary addition since M1. The tag marks a trace that records the act of articulation itself — i.e., a reflexive trace produced when `ArticulationTrace` or `DiffTrace` are called.

**Why "articulation" and not "translation"?** ANT uses "translation" to describe how mediators transform action between actors. MeshAnt's `Articulate` function is indeed performing a translation — rendering the mesh from a different position. But the tag name should reflect what the framework does at the API level, not the theoretical term. "articulation" is more precise: it names the specific act the framework performs (creating a positioned rendering). "translation" is the deeper theoretical insight.

**The vocabulary remains open:** `TagValueArticulation` is a constant, but `Tags` is `[]string`. Callers can supply any string as a tag, not just the predefined constants. This preserves the open vocabulary established in M1.

**Use in reflexive traces:** Both `ArticulationTrace` and `DiffTrace` set `Tags: []string{string(schema.TagValueArticulation)}`. Consumers of the mesh can filter for all reflexive traces by looking for this tag.

---

## Decision 8 — What M7 does not close

**File persistence:** The codec (MarshalJSON/UnmarshalJSON) is provided. File I/O helpers (WriteGraph, ReadGraph) are not. This is intentional.

**Auto-recording:** The framework does not automatically call `ArticulationTrace` after `Articulate`. Recording is an explicit curatorial act. Not every articulation needs to be observed. The caller decides when an act enters the mesh record.

**Registry:** No package-level map of ID → MeshGraph. No auto-recording means no registry. Graph identity is stable (UUID-based) but location is not managed by the framework.

**Ephemeral graph-refs in traces:** Graph-ref strings (e.g., `"meshgraph:abc-123"`) in reflexive traces point to in-process MeshGraph objects unless the caller persists the JSON. If the JSON is stored, the reference is a promise: "this data is about a graph with this ID." But MeshAnt does not enforce or track that promise.

These closures are deferred intentionally. As the project follows more traces and more reflexive articulations, the needs for persistence, registry, and graph identity management will emerge from the work.

---

## Design principles evident in M7

- **Immutability:** `ArticulationTrace` copies the `source` parameter (`sourceCopy`). Callers can safely mutate their original slice.
- **Explicit opt-in:** Reflexive tracing is not automatic. The caller imports `graph` and calls `ArticulationTrace` when they choose to.
- **Fail-fast validation:** Both `ArticulationTrace` and `DiffTrace` validate preconditions (non-empty ID, non-empty observer) before producing a trace that would fail `schema.Validate()`.
- **Self-situated output:** The reflexive traces record their own position via `whatChanged` fields derived from the Cut parameters. They are not neutral records.

---

## Relation to prior decisions

- **trace-schema-v2.md:** The `Observer` field on `Trace` is required. M7 enforces this for reflexive traces: both functions error if observer is empty.
- **graph-as-actor-v2.md (M5):** Graphs are identified as actors via `IdentifyGraph` and `IdentifyDiff`. M7 makes the graph's entry into the mesh explicit: `ArticulationTrace` and `DiffTrace` are the acts of recording.
- **articulation-v2.md (M2):** Every articulation names what it excludes. M7 extends this: reflexive traces include the Cut parameters in their `whatChanged` field, making the position explicit.
- **Principle 8 (designer inside the mesh):** M7 embodies this fully. The observation apparatus (the caller performing the articulation) is named as the observer in the reflexive trace. The cut and its shadow are recorded.

---

## What M7 makes possible

With M7 in place:

1. A caller can articulate a graph, identify it, and record the act of articulation in a new trace.
2. That trace can be added to a trace dataset and articulated again, creating a layered record of observations.
3. The reflexive traces are marked with `"articulation"` tag, making them queryable.
4. The JSON codec allows graphs to be persisted and passed between systems.
5. Graph-refs allow traces to point to graphs and diffs as actors in the mesh.

This closes the loop: MeshAnt's own output is subject to MeshAnt's own tracing.

---

## Test coverage

- `meshant/graph/serial_test.go`: TimeWindow codec tests (MarshalJSON, UnmarshalJSON)
- `meshant/graph/reflexive_test.go`: ArticulationTrace and DiffTrace tests, edge cases
- Full integration via `meshant/cmd/demo/main_test.go` (M6.3)

See test files for detailed test cases and coverage metrics.

---

## Relation to Milestones

- **M2:** Observer-position cut axis; shadow as mandatory output.
- **M5:** Graph-as-actor; IdentifyGraph/IdentifyDiff.
- **M7:** Serialisation and reflexive tracing; ArticulationTrace/DiffTrace.
- **Future:** Persistence layer, registry, distributed tracing.
