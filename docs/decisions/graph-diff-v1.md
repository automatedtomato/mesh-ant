# Decision Record: Graph Diff v1

**Date:** 2026-03-11
**Status:** Active
**Package:** `meshant/graph`
**Branch merged:** `feat/m4-diff`

---

## What was decided

1. **Diff is directional: Diff(g1, g2) reads as "from g1 to g2"**
2. **Edge identity: TraceID as the match key**
3. **Node identity: element Name as the match key**
4. **ShadowShift: three kinds (emerged, submerged, reason-changed), string type**
5. **No ShadowShift for identical shadow reasons in both graphs**
6. **ShadowCount changes on persisted nodes are not shadow shifts**
7. **From and To cuts stored verbatim (defensive copies)**
8. **Sort discipline: alphabetical for names, TraceID-alphabetical for edges**
9. **PrintDiff: all sections always rendered; empty sections show "(none)"**
10. **Diff and PrintDiff in diff.go (separate from graph.go)**
11. **What M4 explicitly defers**

---

## Context

M3 gave MeshAnt two cut axes (observer position, time window) and a longitudinal dataset
spanning three days. The natural next question was: what changed between two articulations?

A diff is not a neutral changelog. Like an articulation, it is a situated observation. It
must record both positions it compares — otherwise a diff could be read as an objective
account of "what really changed," which contradicts the project's methodological stance.
The `GraphDiff.From` and `GraphDiff.To` fields enforce this commitment structurally.

---

## Decisions

### 1. Diff is directional: Diff(g1, g2) reads as "from g1 to g2"

`NodesAdded` means "elements in g2 but not g1"; `NodesRemoved` means "in g1 but not g2".
The function does not enforce any convention about which graph is "earlier" — that is a
concern for callers. Swapping g1 and g2 produces the inverse diff.

**Why not symmetric?** A symmetric API (e.g., `DiffSymmetric(a, b)`) implies both
directions are equally valid and equivalent. They are not: "from day-1 to day-3" and
"from day-3 to day-1" are different questions with different interpretations. The
directional API makes the asymmetry explicit at the call site.

### 2. Edge identity: TraceID as the match key

An edge in `MeshGraph.Edges` represents one trace. Two edges with the same TraceID
refer to the same trace event regardless of observer position or time window. An edge
is "added" if its TraceID appears in g2 but not g1; "removed" if the reverse.

**Why not WhatChanged or element names?** These are unstable: the same logical event could
have slightly different descriptions, and element names recur across unrelated events.
TraceID is a UUID — stable, unique, canonical.

Full Edge structs (not just TraceIDs) are stored in `EdgesAdded` and `EdgesRemoved` so
callers have full context without reaching back to the original trace data. This follows
the same defensive-copy pattern established for `Edge.Tags` in M2.

### 3. Node identity: element Name as the match key

Nodes are identified by their element name string — the map key in `MeshGraph.Nodes`.
This is consistent with how `Node.Name` is defined in the existing type and how
`ShadowElement.Name` works.

### 4. ShadowShift: three kinds, ShadowShiftKind as a named string type

An element can move across or within the shadow boundary in three ways:

- **emerged** (`ShadowShiftEmerged`): in g1 shadow, visible as a Node in g2. The element
  came out of the shadow between the two cuts.
- **submerged** (`ShadowShiftSubmerged`): visible as a Node in g1, in g2 shadow. The
  element entered the shadow between the two cuts.
- **reason-changed** (`ShadowShiftReasonChanged`): in the shadow of both g1 and g2, but
  with different `Reasons` values. The element remained invisible, but the cause of its
  invisibility shifted — e.g., from observer-excluded to additionally time-window-excluded.

`ShadowShiftKind` is a named `string` type following the same pattern as `ShadowReason`.
Using a named string (rather than `iota`) keeps values human-readable in printed output
and in any future JSON serialization, consistent with the existing vocabulary.

`ShadowShift.FromReasons` is empty when `Kind == ShadowShiftSubmerged` (element was
visible in g1). `ToReasons` is empty when `Kind == ShadowShiftEmerged` (element became
visible in g2). Both are non-empty when `Kind == ShadowShiftReasonChanged`.

### 5. No ShadowShift for identical shadow reasons in both graphs

An element in the shadow of both g1 and g2 with identical Reasons slices is not reported
in `ShadowShifts`. Nothing changed about its invisibility between the two cuts. Reporting
it would add noise without adding methodological information.

**What counts as "identical"?** The `shadowReasonsEqual` function compares element-by-element
in order. Reasons in a `ShadowElement` are always sorted deterministically by `Articulate`
(observer before time-window), so order is stable across calls.

### 6. ShadowCount changes on persisted nodes are not shadow shifts

`Node.ShadowCount` counts how many shadow traces mention an element that is also visible
from the current cut. A persisted node whose `ShadowCount` increased or decreased between
g1 and g2 is still a visible node in both — it did not cross the shadow boundary. Its
count change is visible via `PersistedNode.CountFrom` and `CountTo`.

Treating a `ShadowCount` change as a shadow shift would conflate two different things:
the element being visible (a node) versus the element being invisible (a shadow element).
The distinction matters: a node with a high `ShadowCount` is "partially in the shadow"
in an informal sense, but it is not in the shadow in the technical sense the diff tracks.

### 7. From and To cuts stored verbatim (defensive copies)

`GraphDiff.From` stores `g1.Cut` verbatim; `GraphDiff.To` stores `g2.Cut` verbatim. This
makes a `GraphDiff` self-situated: a reader always knows the observer positions, time
windows, trace counts, and shadow elements of both input graphs without needing the
original `MeshGraph` values.

Slice fields (`ObserverPositions`, `ShadowElements`, `ExcludedObserverPositions`,
`ShadowElement.SeenFrom`, `ShadowElement.Reasons`) are copied defensively via `copyCut`,
matching the copy discipline established for `Edge.Tags` in M2 and `Cut.ObserverPositions`
in M3.

This follows M3 Decision 4 (Cut.TimeWindow stored verbatim) as a direct precedent.

### 8. Sort discipline: alphabetical for names, TraceID-alphabetical for edges

`NodesAdded`, `NodesRemoved`, and `NodesPersisted` are sorted alphabetically by element
name. `EdgesAdded` and `EdgesRemoved` are sorted alphabetically by TraceID.
`ShadowShifts` are sorted alphabetically by Name.

Alphabetical sort avoids implying any ranking (e.g., by recency, importance, or dataset
order) and matches the existing sort convention on `Cut.ShadowElements` (M2) and
`Cut.ExcludedObserverPositions` (M2). Test assertions against slice ordering are stable.

### 9. PrintDiff: all sections always rendered; empty sections show "(none)"

Every section (Nodes added, Nodes removed, Nodes persisted, Edges added, Edges removed,
Shadow shifts) is always emitted, even if empty. An empty section renders as `"  (none)"`.

This follows the same principle as `PrintArticulation`'s mandatory shadow section (M2
Decision 2): a section that says "(none)" is a named, affirmative state — "this question
was asked, and the answer was nothing." A missing section would be ambiguous: did nothing
change, or was the question not asked?

The From/To cut metadata is always printed first, structurally encoding the methodological
commitment that every comparison is situated.

### 10. Diff and PrintDiff in diff.go (separate from graph.go)

After M4, `graph.go` would have exceeded 1000 lines. The diff types and functions
(`ShadowShiftKind`, `ShadowShift`, `PersistedNode`, `GraphDiff`, `Diff`, `PrintDiff`,
and their unexported helpers) were extracted into `diff.go` in the same package. Both
files are within the 800-line guideline (`graph.go`: 634 lines, `diff.go`: 425 lines).

No package-boundary change was needed: `diff.go` is `package graph` and shares all
unexported types with `graph.go` (including `timeWindowLabel`, used by `cutSummaryLines`
in `diff.go`).

### 11. What M4 explicitly defers

The following were considered and deliberately not implemented in M4:

- **Tag-filter axis**: articulation-level concern; a future `ArticulationOptions.Tags`
  filter would flow transparently into `Diff` via the `Cut` fields.
- **Weighted diff**: all edges contribute equally; no recency or frequency weighting.
- **Temporal visibility in shadow shifts**: `ShadowShift` records reasons but not "which
  time window would make this element visible again."
- **Diff-as-actor** (M5): a `GraphDiff` does not receive a trace ID and cannot appear as
  a source/target in traces. See `docs/decisions/articulation-v1.md` Decision 5 and the
  Provisional M5 Note in `tasks/plan_m4.md`.
- **Persistence**: diffs remain in-memory only.
- **CLI**: form factor deliberately left open; see `docs/potential-forms.md`.
- **Multi-step diff chains**: no "diff of diffs" or timeline of graph evolution. Each
  `Diff` call is a single pairwise comparison.

---

## Files added or modified

- `meshant/graph/diff.go` — `GraphDiff`, `PersistedNode`, `ShadowShift`,
  `ShadowShiftKind`, `Diff`, `PrintDiff`, unexported helpers
- `meshant/graph/diff_test.go` — 47 unit tests (groups 10–15)
- `meshant/graph/diff_e2e_test.go` — 8 E2E tests (group 16) against longitudinal dataset
- `meshant/graph/testhelpers_test.go` — added `validTraceWithElements` helper
- `meshant/graph/graph.go` — diff code removed (now in diff.go); no API changes

---

## Relation to earlier decisions

- M2 Decision 2 (shadow mandatory): `PrintDiff` follows the same unconditional-section
  principle; shadow shifts section is always present.
- M2 Decision 4 (`ExcludedObserverPositions` stored verbatim): `GraphDiff.From/To` extend
  this pattern to the full `Cut` struct.
- M3 Decision 3 (single shadow, reasons per element): `ShadowShift.FromReasons` and
  `ToReasons` carry the same `ShadowReason` values produced by `Articulate`.
- M3 Decision 4 (TimeWindow stored verbatim in Cut): `copyCut` preserves this guarantee
  across the diff boundary.
- articulation-v1.md Decision 5 (graph-as-actor noted): remains deferred; Diff produces
  a `GraphDiff` value but it has no trace ID. M5 addresses this.
