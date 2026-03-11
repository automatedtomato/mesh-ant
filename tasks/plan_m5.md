# MeshAnt — Milestone 5 Plan

**Date:** 2026-03-11
**Theme:** Graph-as-actor — the observation apparatus enters the mesh it observes.

---

## Overview

M4 established that two articulations can be compared through a diff. M5 asks the
harder question: what happens when an articulation is not just an output but an *actor*?
A deforestation map triggers a policy response. A diff report causes an agent to
re-route. A graph circulates, gets cited, is used as evidence. At that point it is not
merely an observation — it is a participant.

M5 makes this formal: a `MeshGraph` or `GraphDiff` can be assigned a stable identifier
and appear as a `Source` or `Target` in subsequent traces. The observation apparatus
becomes traceable inside the mesh it observes.

This is the milestone where MeshAnt closes its first reflexive loop.

---

## Ideological Grounding

### Latour: the principle of generalised symmetry

ANT insists that humans and non-humans be treated symmetrically. A satellite and a
government ministry are both actants — they both make differences, and those differences
are what we follow. A `MeshGraph` is no different. Once produced, it can redirect other
actors, foreclose options, trigger actions, be cited as evidence. The framework must
treat it symmetrically with the elements it contains.

This rules out a privileged "graph reference" field on `schema.Trace`. To add a separate
`GraphRefs []GraphRef` field alongside `Source` and `Target` would be to grant graphs a
different ontological status — a two-tier world where graphs are acknowledged as actors
but kept in a separate register. ANT forbids this move. Graphs go into `Source` and
`Target` as strings, the same way everything else does.

### Latour: immutable mobiles

An inscription that can travel without changing shape. The graph-reference ID must be
stable: the same ID must refer to the same graph wherever it appears, in whatever trace,
regardless of when or where that trace was written. This requires a convention that is
recognisable and durable — a string format that does not collide with ordinary element
names and does not depend on a registry to be interpreted.

The convention chosen: `meshgraph:<uuid>` and `meshdiff:<uuid>` — type-prefixed UUID
strings. They are self-describing: the prefix names what kind of actor this is, the UUID
names which one.

### Strathern: the cut carries itself

Every graph is a cut. When that cut circulates as an actor, it carries the knowledge of
what it excluded — its shadow. A graph-reference in a trace is not just a pointer to an
opaque object; it is a reference to a *situated* articulation: "this graph, from this
observer position, at this time, with this shadow." The `Cut` stored in `MeshGraph` is
part of the actor's identity, not implementation detail.

This shapes the ID assignment design. `IdentifyGraph` assigns an ID to a graph *after*
it has been fully articulated, so the ID is bound to a specific cut. The ID cannot be
assigned before articulation, because the actor does not yet exist.

### Haraway: no view from nowhere

When a graph enters a trace as a source or target, it enters from somewhere. A graph-
reference ID that stripped positional information — reducing the graph to a bare UUID
with no accessible cut — would be a "god-trick": the pretence that the observation came
from a neutral, unlocated viewpoint.

This is why `IdentifyGraph` does not take a UUID as a parameter and is not a simple
rename. The function returns a `MeshGraph` with its `Cut` intact. Anyone who holds the
identified graph can inspect its observer positions, time window, shadow elements, and
trace counts. The ID is a handle for stable reference; the `Cut` is the position that
makes the handle meaningful.

### Principle 8: the designer is inside the mesh

MeshAnt's own observation apparatus must be traceable. When a graph produced by MeshAnt
influences subsequent events — a policy decision, an agent re-routing, a human choosing
to investigate further — that influence should be recordable as a trace in which the
graph appears as a source. The graph acted. The action should be followable.

M5 makes this possible. It does not mandate it — recording is a curatorial act, not an
automatic one. The framework provides the vocabulary; the analyst decides when to use it.

### Reflexivity, not surveillance

M5 does not auto-record every articulation. Automatically appending a trace every time
`Articulate` is called would be surveillance: the framework observing itself without
discretion, generating noise rather than signal. Reflexivity in the ANT tradition is
selective: you record the moments when the apparatus made a difference, not every moment
the apparatus ran.

`Articulate` and `Diff` continue to return values without side effects. `IdentifyGraph`
is an explicit opt-in: the caller names the graph as an actor when they intend to use it
as one.

---

## Design Decisions

### Decision 1 — `ID string` on `MeshGraph` and `GraphDiff`; zero value = not an actor

Both `MeshGraph` and `GraphDiff` gain an `ID string` field. The zero value (empty string)
means the graph has not been identified as an actor — it is an articulation output, not
yet a participant. Non-empty means the graph has been explicitly identified and can be
referenced.

`Articulate` and `Diff` leave `ID` empty. This is the default: most articulations are
produced for analysis, not circulation. The analyst explicitly opts in to actor status
by calling `IdentifyGraph` or `IdentifyDiff`.

**Why not assign automatically?** Because not every articulation needs to be an actor.
Intermediate articulations produced for exploration would pollute the mesh with IDs that
refer to nothing anyone cares about. The explicit opt-in matches the ANT principle that
actors are entities that *make a difference* — not everything that exists.

### Decision 2 — `meshgraph:<uuid>` and `meshdiff:<uuid>` as the reference convention

When a graph's ID appears in a trace's `Source` or `Target`, it is formatted as
`meshgraph:<uuid>` or `meshdiff:<uuid>`. These strings are:

- Self-describing: the prefix names the kind of actor
- Durable: no registry needed to interpret them
- Distinguishable from ordinary element names: colons do not appear in typical element
  name strings; the prefix is semantically unambiguous

**Why not a URI scheme (`meshgraph://...`)?** URI semantics carry the implication that
the reference is resolvable via a protocol handler. MeshAnt has no network layer. A bare
`meshgraph:` prefix is a type tag, not a URI.

**Why not a bracket convention (`[graph:uuid]`)?** Bracket syntax looks like a special
annotation rather than a first-class element. ANT demands symmetry — the graph reference
should look as ordinary as any other element name.

**Why two prefixes, not one?** `MeshGraph` and `GraphDiff` are distinct kinds of actors:
one is a cut of traces, the other is a comparison between two cuts. Callers and analysts
need to know which kind they are encountering. A single `meshgraph:` prefix for both
would lose this distinction.

### Decision 3 — Graph-reference strings live in `[]string Source` and `Target`

A trace's `Source` and `Target` are `[]string`. Graph-reference strings go there, not
in a new field. This is the generalised symmetry decision: a graph that acted in the
mesh is named alongside the other actants that acted, in the same structural position.

**Why not a new `GraphRefs []string` field?** A separate field grants graphs a different
ontological status. It would also require schema changes that break existing `Validate()`
logic, loader tests, and all downstream consumers. The cost is high and the benefit —
type safety at the field level — is outweighed by the methodological cost.

### Decision 4 — `IdentifyGraph` and `IdentifyDiff` are pure functions

```go
func IdentifyGraph(g MeshGraph) MeshGraph
func IdentifyDiff(d GraphDiff) GraphDiff
```

Both take a value, assign a UUID to the `ID` field, and return the new value. They do
not mutate the input. This follows the project's immutability rule: always return a new
object, never modify an existing one.

The UUID is generated by a package-private `newUUID4()` function using `crypto/rand`.
No external UUID library is introduced — MeshAnt has no external dependencies and this
milestone does not change that.

### Decision 5 — `GraphRef` and `DiffRef` format the reference string

```go
func GraphRef(g MeshGraph) (string, error)
func DiffRef(d GraphDiff) (string, error)
```

Both return the formatted reference string (`meshgraph:<id>` or `meshdiff:<id>`). They
return an error if `ID` is empty — calling `GraphRef` on an unidentified graph is a
programming error and must be surfaced explicitly rather than silently producing an
unresolvable reference.

**Why not panic?** Panic on logic errors in library code is generally unidiomatic in Go.
The caller can inspect and handle the error. The error message makes the diagnosis clear.

### Decision 6 — `IsGraphRef`, `GraphRefKind`, `GraphRefID` live in the `schema` package

These are predicate functions that inspect a string to determine whether it is a
graph-reference, and if so, what kind and which ID:

```go
func IsGraphRef(s string) bool
func GraphRefKind(s string) string   // "meshgraph", "meshdiff", or ""
func GraphRefID(s string) string     // UUID string after prefix, or ""
```

They live in `schema` because they operate on the content of `Source` and `Target`
fields — data that belongs to the schema layer. The `graph` package produces graph-
reference strings; the `schema` package defines what those strings mean to a trace.
The `loader` package uses `schema.IsGraphRef` to identify graph references in summaries.
No circular dependencies are introduced.

**Why not in `graph`?** If `schema` imported `graph` to check references, or `graph`
imported `schema` for this, the dependency would create a cycle. Keeping the predicate
in `schema` as a string-level operation avoids any import cycle.

### Decision 7 — `schema.Validate()` does not restrict Source/Target content

`Validate()` currently accepts any string in `Source` and `Target` — it validates
structure (UUID format for ID, required fields, etc.) but does not inspect individual
element name strings. `meshgraph:<uuid>` strings pass validation as-is. This is
correct: `Validate()` should not know about graph-package conventions. No change needed.

This is documented here to make the decision explicit: the schema layer's permissiveness
toward source/target content is a feature, not an oversight.

### Decision 8 — `MeshSummary.GraphRefs []string` in encounter order

```go
type MeshSummary struct {
    Elements           map[string]int
    Mediations         []string        // existing, encounter order
    MediatedTraceCount int
    FlaggedTraces      []FlaggedTrace
    GraphRefs          []string        // NEW: graph-reference strings, encounter order, deduplicated
}
```

`Summarise` adds a graph-reference string to `GraphRefs` the first time it is
encountered across all `Source` and `Target` slices, in encounter order. This is
consistent with how `Mediations` is built.

**Why encounter order, not alphabetical?** Encounter order preserves the sequence in
which graphs entered the dataset — which has methodological significance. The first
appearance of a graph-reference marks when the graph became an actor in the recorded
mesh.

### Decision 9 — No registry; no automatic recording

There is no package-level map of ID → MeshGraph. Callers retain the graphs they
identify. If a graph needs to be looked up by ID, the caller maintains that mapping
in their own scope.

Automatic recording (e.g., `IdentifyGraph` writing a trace to a dataset) is not
implemented. Recording is a curatorial act.

### Decision 10 — New `actor.go` file in `graph` package

`IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`, and `newUUID4` are added to a
new file `meshant/graph/actor.go`. This keeps the diff logic (`diff.go`) and the
articulation logic (`graph.go`) focused, and signals to readers that actor-identity is
a distinct concern.

---

## Full Type and Function Signatures

```go
// --- schema package additions (schema/trace.go or schema/graphref.go) ---

// IsGraphRef reports whether s is a graph-reference string — i.e., whether it
// begins with "meshgraph:" or "meshdiff:".
func IsGraphRef(s string) bool

// GraphRefKind returns the kind of a graph-reference string: "meshgraph",
// "meshdiff", or "" if s is not a graph-reference.
func GraphRefKind(s string) string

// GraphRefID returns the UUID portion of a graph-reference string (the part
// after the "meshgraph:" or "meshdiff:" prefix). Returns "" if s is not a
// graph-reference or if the ID portion is empty.
func GraphRefID(s string) string

// --- graph package additions (graph/actor.go) ---

// IdentifyGraph assigns a fresh, stable UUID to g.ID and returns the updated
// MeshGraph. The input g is not modified (immutable pattern). The returned
// graph's ID is non-empty and can be formatted as a graph-reference string
// via GraphRef.
//
// Call IdentifyGraph only when you intend to use the graph as an actor in the
// mesh — i.e., when you plan to reference it in subsequent traces via GraphRef.
// Most articulations do not need to be actors.
func IdentifyGraph(g MeshGraph) MeshGraph

// IdentifyDiff assigns a fresh, stable UUID to d.ID and returns the updated
// GraphDiff. The input d is not modified.
func IdentifyDiff(d GraphDiff) GraphDiff

// GraphRef returns the graph-reference string for g ("meshgraph:<g.ID>").
// Returns an error if g.ID is empty — call IdentifyGraph first.
func GraphRef(g MeshGraph) (string, error)

// DiffRef returns the graph-reference string for d ("meshdiff:<d.ID>").
// Returns an error if d.ID is empty — call IdentifyDiff first.
func DiffRef(d GraphDiff) (string, error)

// --- MeshGraph and GraphDiff gain ID field ---

type MeshGraph struct {
    ID    string          // stable actor identifier; empty = not yet an actor
    Nodes map[string]Node
    Edges []Edge
    Cut   Cut
}

type GraphDiff struct {
    ID             string          // stable actor identifier; empty = not yet an actor
    NodesAdded     []string
    NodesRemoved   []string
    NodesPersisted []PersistedNode
    EdgesAdded     []Edge
    EdgesRemoved   []Edge
    ShadowShifts   []ShadowShift
    From           Cut
    To             Cut
}

// --- loader package addition ---

type MeshSummary struct {
    Elements           map[string]int
    Mediations         []string
    MediatedTraceCount int
    FlaggedTraces      []FlaggedTrace
    GraphRefs          []string  // graph-reference strings found in Source/Target, encounter order, deduplicated
}
```

---

## Test Plan

### M5.1 — schema additions

Tests added to `meshant/schema/trace_test.go` (or a new `meshant/schema/graphref_test.go`).

| Test function | Verifies |
|---|---|
| `TestIsGraphRef_MeshgraphPrefix_True` | `meshgraph:<uuid>` → true |
| `TestIsGraphRef_MeshdiffPrefix_True` | `meshdiff:<uuid>` → true |
| `TestIsGraphRef_PlainElement_False` | `"landsat-9-satellite"` → false |
| `TestIsGraphRef_Empty_False` | `""` → false |
| `TestIsGraphRef_PartialPrefix_False` | `"meshgraph"` (no colon) → false |
| `TestGraphRefKind_MeshgraphPrefix` | returns `"meshgraph"` |
| `TestGraphRefKind_MeshdiffPrefix` | returns `"meshdiff"` |
| `TestGraphRefKind_PlainElement_Empty` | returns `""` |
| `TestGraphRefID_ExtractsUUID` | returns the UUID portion after the colon |
| `TestGraphRefID_PlainElement_Empty` | returns `""` for non-ref string |
| `TestGraphRefID_EmptyAfterColon_Empty` | `"meshgraph:"` with no UUID → returns `""` |
| `TestValidate_GraphRefInSource_Valid` | trace with `meshgraph:<uuid>` in Source passes Validate() |
| `TestValidate_GraphRefInTarget_Valid` | trace with `meshdiff:<uuid>` in Target passes Validate() |

### M5.2 — graph actor additions

New file `meshant/graph/actor_test.go`.

| Test function | Verifies |
|---|---|
| `TestIdentifyGraph_AssignsNonEmptyID` | returned graph has non-empty ID |
| `TestIdentifyGraph_DoesNotMutateInput` | original graph ID remains empty after call |
| `TestIdentifyGraph_IDIsUnique` | two calls on same graph produce different IDs |
| `TestIdentifyGraph_PreservesNodes` | Nodes map unchanged |
| `TestIdentifyGraph_PreservesCut` | Cut unchanged |
| `TestIdentifyDiff_AssignsNonEmptyID` | returned diff has non-empty ID |
| `TestIdentifyDiff_DoesNotMutateInput` | original diff ID remains empty |
| `TestIdentifyDiff_IDIsUnique` | two calls produce different IDs |
| `TestGraphRef_FormatsCorrectly` | returns `"meshgraph:<id>"` |
| `TestGraphRef_EmptyID_ReturnsError` | error when ID is empty |
| `TestGraphRef_ErrorMessage_Descriptive` | error message mentions IdentifyGraph |
| `TestDiffRef_FormatsCorrectly` | returns `"meshdiff:<id>"` |
| `TestDiffRef_EmptyID_ReturnsError` | error when ID is empty |
| `TestArticulate_IDEmpty_ByDefault` | Articulate returns graph with empty ID |
| `TestDiff_IDEmpty_ByDefault` | Diff returns diff with empty ID |

### M5.3 — loader additions

Tests added to `meshant/loader/loader_test.go`. New file `meshant/loader/graphref_test.go`
for E2E tests against `data/examples/graph_ref_traces.json`.

**Unit tests (loader_test.go, new group):**

| Test function | Verifies |
|---|---|
| `TestSummarise_GraphRefs_Empty_WhenNonePresent` | GraphRefs is nil/empty when no graph-refs in dataset |
| `TestSummarise_GraphRefs_SingleRef` | one graph-ref string → GraphRefs contains it |
| `TestSummarise_GraphRefs_Deduplication` | same ref in two traces → appears once |
| `TestSummarise_GraphRefs_EncounterOrder` | refs appear in order of first encounter |
| `TestSummarise_GraphRefs_MixedWithElements` | graph-refs extracted; plain elements still in Elements map |
| `TestSummarise_GraphRefs_BothPrefixes` | meshgraph: and meshdiff: refs both collected |
| `TestPrintSummary_GraphRefs_Section_Present` | output contains GraphRefs section header |
| `TestPrintSummary_GraphRefs_Empty_ShowsNone` | empty GraphRefs → "(none)" in output |
| `TestPrintSummary_GraphRefs_ShowsRef` | non-empty GraphRefs → ref string in output |

**E2E tests (graphref_test.go):**

| Test function | Verifies |
|---|---|
| `TestGraphRef_Load_Count` | dataset loads 6 traces without error |
| `TestGraphRef_Summarise_GraphRefsCount` | Summarise finds ≥2 distinct graph-refs |
| `TestGraphRef_Summarise_KnownRefPresent` | specific known meshgraph: ref appears in GraphRefs |
| `TestGraphRef_Summarise_DiffRefPresent` | specific known meshdiff: ref appears in GraphRefs |
| `TestGraphRef_Summarise_ElementsStillCounted` | non-ref elements still appear in Elements |

---

## Task Breakdown

### M5.1 — Schema additions

**Branch:** `feat/m5-schema` (cut from `develop`)

**Files:**
- `meshant/schema/trace.go` (or new `meshant/schema/graphref.go`) — add constants and three functions
- `meshant/schema/trace_test.go` (or `meshant/schema/graphref_test.go`) — add 13 tests

**Steps:**
1. Write tests — RED
2. Add `const graphRefPrefixGraph = "meshgraph:"` and `const graphRefPrefixDiff = "meshdiff:"` (unexported constants)
3. Implement `IsGraphRef`, `GraphRefKind`, `GraphRefID`
4. Run tests — GREEN

**Note:** Verify that `Validate()` already accepts any string in Source/Target (check existing tests). If so, `TestValidate_GraphRefInSource_Valid` and `TestValidate_GraphRefInTarget_Valid` should pass immediately after implementing the schema functions.

### M5.2 — Graph actor additions

**Branch:** `feat/m5-actor` (cut from `develop` after M5.1 merges, or same branch)

**Files:**
- `meshant/graph/graph.go` — add `ID string` as first field of `MeshGraph`
- `meshant/graph/diff.go` — add `ID string` as first field of `GraphDiff`
- `meshant/graph/actor.go` (new file) — `newUUID4`, `IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`
- `meshant/graph/actor_test.go` (new file) — 15 tests

**Steps:**
1. Add `ID string` to `MeshGraph` and `GraphDiff` — existing tests must still pass (zero value)
2. Write `actor_test.go` — RED (actor.go does not exist yet)
3. Implement `newUUID4` using `crypto/rand` (version 4 UUID, hyphenated lowercase format)
4. Implement `IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`
5. Run full test suite — GREEN
6. Check that `TestArticulate_IDEmpty_ByDefault` and `TestDiff_IDEmpty_ByDefault` pass without any change to `Articulate` or `Diff`

**`newUUID4` format:** `xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx` where `y` is `8`, `9`, `a`, or `b`. Uses only `crypto/rand` — no external library.

**Existing struct literal check:** existing tests use named field initialisation — adding `ID string` as the first field does not break any existing struct literals. Confirm before coding.

### M5.3 — Loader addition and new dataset

**Branch:** `feat/m5-loader` (cut from `develop` after M5.1 merges)

**Files:**
- `data/examples/graph_ref_traces.json` — new dataset (6 traces with graph-ref strings)
- `meshant/loader/loader.go` — add `GraphRefs []string` to `MeshSummary`; extend `Summarise` and `PrintSummary`
- `meshant/loader/loader_test.go` — add unit tests for GraphRefs
- `meshant/loader/graphref_test.go` (new file) — E2E tests against the new dataset

**Steps:**
1. Write unit tests and E2E test stubs — RED
2. Add `GraphRefs []string` to `MeshSummary`
3. Extend `Summarise`: for each source/target string, check `schema.IsGraphRef`; dedup in encounter order using a seen-map
4. Extend `PrintSummary`: add GraphRefs section (same pattern as Mediations)
5. Write `graph_ref_traces.json` (6 traces; 3 reference a `meshgraph:<uuid>`, 1 references a `meshdiff:<uuid>`, 2 have no graph refs)
6. Run full test suite — GREEN

**`graph_ref_traces.json` design:**
- 6 traces, valid schema, all with observer and WhatChanged
- 2 traces whose Source contains `meshgraph:a1b2c3d4-bbbb-4ccc-dddd-eeeeeeeeee01`
- 1 trace whose Target contains `meshgraph:a1b2c3d4-bbbb-4ccc-dddd-eeeeeeeeee01` (duplicate — dedup test)
- 1 trace whose Source contains `meshgraph:b2c3d4e5-bbbb-4ccc-dddd-eeeeeeeeee02`
- 1 trace whose Target contains `meshdiff:c3d4e5f6-bbbb-4ccc-dddd-eeeeeeeeee03`
- 1 trace with no graph refs (plain element names only)
- Distinct graph-refs: 3 total; deduped GraphRefs should have 3 entries in encounter order

### M5.4 — Decision record

**Branch:** alongside M5.2 or M5.3

**File:** `docs/decisions/graph-as-actor-v1.md`

10 decisions documented (see Design Decisions above), plus explicit deferred items and
relation to `articulation-v1.md` Decision 5 (graph-as-actor noted there, fulfilled here).

---

## What M5 Explicitly Defers

- **Graph reference validation in `Validate()`** — `schema.Validate()` does not inspect source/target content; `meshgraph:`-formatted strings pass as-is. Content-level validation of graph-ref format (e.g., checking the UUID portion is well-formed) is deferred.

- **Registry or resolution service** — no facility to look up a `MeshGraph` by its ID. Callers retain the graphs they identify.

- **Automatic trace recording of articulation events** — `IdentifyGraph` assigns an ID; it does not write a trace. Recording is a curatorial act, not an automatic one.

- **Graph-ref semantic validation in the loader** — `Load` accepts `meshgraph:` strings without cross-referencing them against any known graph. Whether the ID refers to a real graph is not the loader's concern.

- **`PrintArticulation` and `PrintDiff` output of the graph's own ID** — when `MeshGraph.ID` or `GraphDiff.ID` is non-empty, `PrintArticulation`/`PrintDiff` do not output it. This is a display decision deferred to avoid changing output format mid-milestone.

- **JSON serialisation of `MeshGraph` and `GraphDiff`** — graphs remain in-memory objects. Serialisation to disk is deferred.

- **Tag-filter cut axis** — deferred since M2, still deferred.

---

## Provisional M6 Note

Two directions emerge from M5's shadow:

**M6-A: Reflexive tracing — recording articulation events in the mesh**

M5 enables a graph to appear as a source/target in traces. A natural follow-on is a
helper for recording the *act* of articulation itself: at time T, observer O produced
graph G (now identified as `meshgraph:<id>`). This would close the reflexive loop
described in Principle 8 more completely. It requires a decision about where the
resulting traces go and whether MeshAnt ships a helper or leaves it to the caller.

**M6-B: Graph serialisation and restoration**

For a graph-reference ID to be genuinely useful across sessions, the referenced
`MeshGraph` must be serialisable. M6-B would add `json.Marshal`/`Unmarshal` support
for `MeshGraph` and `GraphDiff`, enabling a caller to save an identified graph alongside
its traces. Requires decisions about the JSON schema for graphs and whether serialisation
lives in `graph` or a new `store` package.

Both are provisional. Neither is committed. They are what M5's shadow points toward.
