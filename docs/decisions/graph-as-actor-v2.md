# Decision Record: Graph-as-Actor v1

**Date:** 2026-03-11
**Status:** Active
**Packages:** `meshant/schema`, `meshant/graph`, `meshant/loader`
**Branch merged:** `feat/m5-loader`

---

## What was decided

1. **`ID string` on `MeshGraph` and `GraphDiff`; zero value = not an actor**
2. **`meshgraph:<uuid>` and `meshdiff:<uuid>` as the reference string convention**
3. **Graph-reference strings live in `[]string Source` and `Target` (generalised symmetry)**
4. **`IdentifyGraph` and `IdentifyDiff` are pure functions (immutable, explicit opt-in)**
5. **`GraphRef` and `DiffRef` return `(string, error)` — error on unidentified input**
6. **`IsGraphRef`, `GraphRefKind`, `GraphRefID` live in the `schema` package**
7. **`schema.Validate()` does not restrict Source/Target content**
8. **`MeshSummary.GraphRefs []string` in encounter order, deduplicated**
9. **No registry; no automatic recording**
10. **New `actor.go` file in the `graph` package**

---

## Ideological grounding

### ANT: generalised symmetry

Latour's principle of generalised symmetry requires treating human and non-human
actants with the same analytical vocabulary. A `MeshGraph` that triggers a policy
response, is cited as evidence, or causes an agent to re-route is making a
difference — it is an actant. ANT forbids placing it in a separate register.

This rules out a `GraphRefs []GraphRef` field alongside `Source` and `Target`. A
separate field would grant graphs different ontological status. Graph-reference
strings go into `Source` and `Target` as strings, exactly as every other actant
does.

### ANT: immutable mobiles

An inscription that travels without changing shape. The graph-reference ID must
be stable: the same ID, wherever it appears, refers to the same graph. The
`meshgraph:<uuid>` / `meshdiff:<uuid>` convention is self-describing and durable —
no registry is needed to interpret a reference. The prefix names the kind; the UUID
names the instance.

### Strathern: the cut carries itself

Every graph is a cut. When a cut circulates as an actor, it carries the knowledge
of what it excluded — its shadow. A graph-reference is not a pointer to an opaque
object; it is a reference to a situated articulation: a specific observer position,
time window, and shadow. The `Cut` stored in `MeshGraph` is part of the actor's
identity, not implementation detail.

This shapes `IdentifyGraph`: the ID is assigned after full articulation, so the
actor's identity is bound to a specific cut. IDs cannot be assigned before
articulation, because the actor does not yet exist.

### Haraway: no view from nowhere

`IdentifyGraph` does not strip positional information. The returned `MeshGraph`
carries its `Cut` intact. Anyone holding the identified graph can inspect its
observer positions, time window, shadow elements, and trace counts. The ID is a
handle for stable reference; the `Cut` is the position that makes the handle
meaningful. A bare UUID with no accessible cut would be a "god-trick" — pretending
the observation came from a neutral, unlocated viewpoint.

### Principle 8: the designer is inside the mesh

When a graph produced by MeshAnt influences subsequent events, that influence is
now recordable as a trace in which the graph appears as a source. The observation
apparatus is traceable inside the mesh it observes.

M5 makes this possible. It does not mandate it. Recording is a curatorial act:
the analyst decides which moments of graph influence are worth recording. Reflexivity
is selective, not automatic.

---

## Decision details

### Decision 1 — `ID string` on `MeshGraph` and `GraphDiff`; zero value = not an actor

Both `MeshGraph` and `GraphDiff` gain an `ID string` field. The zero value (empty
string) means the graph has not been identified as an actor — it is an output of
articulation, not yet a participant. Non-empty means the graph was explicitly
identified and can be referenced in traces.

`Articulate` and `Diff` leave `ID` empty. This is the default: most articulations
are produced for analysis, not circulation. The caller explicitly opts in to actor
status by calling `IdentifyGraph` or `IdentifyDiff`.

**Why not assign automatically?** Not every articulation needs to be an actor.
Automatically assigning IDs to every articulation would pollute the mesh with
references to graphs that nobody intended to circulate. The explicit opt-in matches
the ANT principle that actors are entities that *make a difference* — not everything
that exists.

### Decision 2 — `meshgraph:<uuid>` and `meshdiff:<uuid>` as the reference convention

Graph-reference strings use a type-prefixed UUID format. Properties:

- **Self-describing**: the prefix names the kind of actor
- **Durable**: no registry is needed to interpret the reference
- **Distinguishable from element names**: colons do not appear in typical element
  name strings; the prefix is semantically unambiguous
- **Symmetrical**: the string looks as ordinary as any other element name in
  a `Source` or `Target` slice — it is just a string with a known prefix

**Why not a URI scheme (`meshgraph://...`)?** URI semantics imply resolvability
via a protocol handler. MeshAnt has no network layer. A bare `meshgraph:` prefix
is a type tag, not a URI.

**Why two prefixes, not one?** `MeshGraph` and `GraphDiff` are distinct kinds of
actors: one is a cut of traces, the other is a comparison between two cuts. Callers
and analysts need to know which kind they are encountering.

### Decision 3 — Graph-reference strings live in `[]string Source` and `Target`

The principle of generalised symmetry: a graph that acted in the mesh is named
alongside the other actants that acted, in the same structural position. No new
field is introduced on `schema.Trace`.

**Why not a new `GraphRefs []string` field?** A separate field grants graphs a
different ontological status and would break existing `Validate()` logic and all
downstream consumers. The methodological cost exceeds any benefit.

### Decision 4 — `IdentifyGraph` and `IdentifyDiff` are pure functions

```go
func IdentifyGraph(g MeshGraph) MeshGraph
func IdentifyDiff(d GraphDiff) GraphDiff
```

Both take a value, assign a UUID to the `ID` field, and return the new value.
Inputs are not mutated. This follows the project's immutability rule.

The UUID is generated by `newUUID4()` using `crypto/rand`. No external UUID
library is introduced — MeshAnt has no external dependencies and this milestone
does not change that.

### Decision 5 — `GraphRef` and `DiffRef` return `(string, error)`

```go
func GraphRef(g MeshGraph) (string, error)
func DiffRef(d GraphDiff) (string, error)
```

Both return an error if `ID` is empty. Calling `GraphRef` on an unidentified
graph is a programming error and must be surfaced explicitly rather than silently
producing an unresolvable reference.

**Why not panic?** Panic on logic errors in library code is unidiomatic in Go.
The caller can inspect and handle the error. The error message names the remedy
(`"call IdentifyGraph first"`).

### Decision 6 — `IsGraphRef`, `GraphRefKind`, `GraphRefID` live in the `schema` package

These predicate functions inspect a string to determine whether it is a
graph-reference, and if so, what kind and which ID. They live in `schema`
because they operate on the content of `Source` and `Target` fields — data that
belongs to the schema layer.

**Why not in `graph`?** If `schema` imported `graph`, or `graph` imported `schema`
for this purpose, a circular dependency would result (`graph` already imports
`schema`). Keeping the predicates in `schema` as string-level operations avoids
any import cycle.

**Implementation:** a private `parseGraphRef(s string) (kind, id string)` helper
using `strings.Cut` on `:` performs detection and extraction in a single pass.
`IsGraphRef`, `GraphRefKind`, and `GraphRefID` delegate to it — adding a third
prefix in future requires editing only one place.

### Decision 7 — `schema.Validate()` does not restrict Source/Target content

`Validate()` accepts any string in `Source` and `Target`. It validates structure
(UUID format for ID, required fields) but does not inspect individual element name
strings. `meshgraph:<uuid>` strings pass validation as-is.

This is documented here to make the decision explicit: the schema layer's
permissiveness toward source/target content is a feature, not an oversight.
`Validate()` should not know about graph-package conventions.

### Decision 8 — `MeshSummary.GraphRefs []string` in encounter order, deduplicated

```go
type MeshSummary struct {
    Elements           map[string]int
    Mediations         []string   // encounter order, deduplicated
    MediatedTraceCount int
    FlaggedTraces      []FlaggedTrace
    GraphRefs          []string   // NEW: graph-ref strings, encounter order, deduplicated
}
```

`Summarise` adds a graph-reference string to `GraphRefs` the first time it is
encountered across all `Source` and `Target` slices, in encounter order. This is
consistent with how `Mediations` is built.

**Why encounter order, not alphabetical?** The first appearance of a graph-reference
marks when the graph became an actor in the recorded mesh. That sequence has
methodological significance.

**Note:** graph-reference strings are also counted in `Elements` (they are actants
making a difference, even if their structure is a reference string). This is
consistent with ANT's symmetry principle.

### Decision 9 — No registry; no automatic recording

There is no package-level map of ID → MeshGraph. Callers retain the graphs they
identify. If a graph needs to be looked up by ID, the caller maintains that mapping
in their own scope.

Automatic recording is not implemented. `IdentifyGraph` assigns an ID; it does not
write a trace. Recording is a curatorial act.

### Decision 10 — New `actor.go` file in the `graph` package

`IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`, and `newUUID4` are added to
`meshant/graph/actor.go`. This keeps the diff logic (`diff.go`) and the articulation
logic (`graph.go`) focused, and signals to readers that actor-identity is a distinct
concern.

---

## What M5 explicitly defers

- **Graph reference validation in `Validate()`** — `schema.Validate()` does not
  inspect source/target content; `meshgraph:`-formatted strings pass as-is. Content-
  level validation of the UUID portion is deferred.

- **Registry or resolution service** — no facility to look up a `MeshGraph` by its
  ID. Callers retain the graphs they identify.

- **Automatic trace recording of articulation events** — `IdentifyGraph` assigns an
  ID; it does not write a trace.

- **Graph-ref semantic validation in the loader** — `Load` accepts `meshgraph:`
  strings without cross-referencing them against any known graph.

- **`PrintArticulation` and `PrintDiff` output of the graph's own ID** — when
  `MeshGraph.ID` or `GraphDiff.ID` is non-empty, `PrintArticulation`/`PrintDiff` do
  not output it. Deferred to avoid changing output format mid-milestone.

- ~~**JSON serialisation of `MeshGraph` and `GraphDiff`**~~ — resolved in M7: custom `TimeWindow` codec in `serial.go`; `PrintGraphJSON`/`PrintDiffJSON` in M8 `export.go`; `persist.WriteJSON`/`ReadGraphJSON`/`ReadDiffJSON` in M8.

- ~~**Tag-filter cut axis**~~ — resolved in M10: `ArticulationOptions.Tags` with any-match semantics; `ShadowReasonTagFilter` as third shadow reason.

---

## Relation to earlier decisions

- `articulation-v2.md` Decision 5: "graph-as-actor noted architecturally" — fulfilled
  here. `MeshGraph` is now a first-class actant, not just an analytical output.
- `graph-diff-v2.md` is not changed by M5. `GraphDiff` gains `ID string` (zero value,
  backward-compatible) and `IdentifyDiff`/`DiffRef` functions.
