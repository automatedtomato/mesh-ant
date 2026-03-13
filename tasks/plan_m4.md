# MeshAnt — Milestone 4 Plan

**Date:** 2026-03-11
**Theme:** Graph diff — situated comparison of two articulations, recording what became visible or invisible between two cuts.

---

## Overview

M3 gave us two cut axes: observer position and time window. We can now articulate the same dataset from two different positions — different observers, different days, different combinations — and hold both graphs in memory. M4 answers the natural next question: what changed between them?

The key methodological commitment carries forward and deepens: a diff is not a neutral changelog. It is itself a situated comparison. It must record both cuts — the positions from which g1 and g2 were made — so that the diff is never read as an objective account of "what really changed in the mesh." What changed depends entirely on where you were standing and when.

The diff captures three kinds of movement:

1. **Nodes** entering or leaving visibility (NodesAdded, NodesRemoved, NodesPersisted)
2. **Edges** entering or leaving the cut (EdgesAdded, EdgesRemoved, matched by TraceID)
3. **Shadow shifts** — elements that moved between shadow and visible, or that remained in shadow but for different reasons (ShadowShifts)

Shadow shifts are new territory. An element that was in the shadow of g1 and is now a node in g2 has not merely "appeared" — it has *emerged* from the shadow. An element that was a node in g1 and is now in the shadow has *submerged*. An element in the shadow of both graphs, but excluded for different reasons, has shifted posture within the shadow. These distinctions are methodologically significant and must be named.

**M4 = diff function + print + E2E against longitudinal dataset + decision record.**

---

## Design Decisions

### Decision 1 — Edge identity: TraceID as the match key

When comparing edges across two graphs, the TraceID (the UUID of the source trace) is the stable identity. Two edges with the same TraceID refer to the same trace event. An edge is "added" if its TraceID appears in g2.Edges but not in g1.Edges; "removed" if the reverse.

This is the correct choice because TraceID is the canonical identifier. The alternative — matching on WhatChanged or element names — is unstable: the same logical event could have slightly different descriptions, and element names can recur across unrelated events.

Full Edge structs are stored in EdgesAdded and EdgesRemoved (not just TraceIDs), consistent with how MeshGraph.Edges already provides full context. Callers can inspect Sources, Targets, WhatChanged, Mediation, Observer, and Tags without reaching back to the original trace data.

### Decision 2 — Node identity: Name as the match key

Node identity in the diff is the element name string (the map key in MeshGraph.Nodes). This is consistent with how Node.Name is defined in the existing type. A node is "added" if it appears in g2.Nodes but not g1.Nodes; "removed" if the reverse; "persisted" if it appears in both.

Persisted nodes record AppearanceCount from both graphs. A node that persisted but changed count (became more or less active) is visible by comparing the two counts in PersistedNode. No separate "count-changed" field is needed — the diff consumer reads both values directly.

### Decision 3 — ShadowShift: three kinds, named explicitly

Elements can move across the shadow boundary in three ways:

- **emerged**: present in g1.Cut.ShadowElements, present as a Node in g2.Nodes. The element moved from invisible to visible.
- **submerged**: present as a Node in g1.Nodes, present in g2.Cut.ShadowElements. The element moved from visible to invisible.
- **reason-changed**: present in g1.Cut.ShadowElements AND in g2.Cut.ShadowElements, but with different Reasons slices. The element remained invisible but the cause shifted (e.g., from observer-excluded to time-window-excluded, or from one reason to both).

Elements that are in the shadow of both graphs with identical reasons are not reported in ShadowShifts — they are simply not visible from either cut and nothing changed about why.

Elements that are Nodes in both graphs do not appear in ShadowShifts regardless of ShadowCount changes. ShadowCount is an annotation on an already-visible node; it is captured in NodesPersisted via the count fields.

### Decision 4 — ShadowShiftKind: string type with named constants

`ShadowShiftKind` is a `string` type following the same pattern as `ShadowReason` in the existing package. Constants are defined as package-level constants of type `ShadowShiftKind`:

```
ShadowShiftEmerged       ShadowShiftKind = "emerged"
ShadowShiftSubmerged     ShadowShiftKind = "submerged"
ShadowShiftReasonChanged ShadowShiftKind = "reason-changed"
```

Using a named string type (rather than iota) keeps the values human-readable in JSON serialization and in PrintDiff output, consistent with `ShadowReason`.

### Decision 5 — From and To cuts stored verbatim in GraphDiff

`GraphDiff.From` stores `g1.Cut` verbatim; `GraphDiff.To` stores `g2.Cut` verbatim. This makes the diff self-situated: a reader of a `GraphDiff` always knows the observer positions, time windows, trace counts, and shadow elements of both input graphs without needing access to the original `MeshGraph` values.

This follows the same pattern as `Cut.TimeWindow` being stored verbatim in the `Cut` struct (established in M3 Decision 4): the articulation output is self-describing.

Slice fields (`ObserverPositions`, `ShadowElements`, `ExcludedObserverPositions`) are copied defensively, consistent with the copy discipline in `Articulate`.

### Decision 6 — Sorted output: alphabetical for names, TraceID-alphabetical for edges

`NodesAdded`, `NodesRemoved`, and `NodesPersisted` are sorted alphabetically by element name. Alphabetical sort avoids implying any ranking and matches the existing sort convention on `Cut.ShadowElements`.

`EdgesAdded` and `EdgesRemoved` are sorted by TraceID alphabetically. This is consistent and deterministic without depending on dataset order, which varies between graphs.

`ShadowShifts` are sorted by Name alphabetically.

All sort decisions are deterministic to ensure test assertions against output order are stable.

### Decision 7 — Diff is directional: Diff(g1, g2) reads as "from g1 to g2"

`Diff(g1, g2)` records what changed moving from g1 to g2. From and To are directional. Swapping g1 and g2 produces the inverse diff (NodesAdded and NodesRemoved swap, EdgesAdded and EdgesRemoved swap, ShadowShift directions invert). The function does not enforce any convention about which graph is "earlier" — that is a concern for callers.

### Decision 8 — PrintDiff: mandatory From/To section; all sections always rendered

`PrintDiff` renders all sections unconditionally, including empty ones. This follows the same design principle as `PrintArticulation`'s shadow section: a section that says "(none)" is not absence of output — it is a named state. A diff that shows no node changes must still emit the nodes section so a reader knows the question was asked, not skipped.

The From/To cut metadata is always printed at the top of the diff, encoding the methodological commitment: this comparison was made from two specific positions, and those positions are named.

### Decision 9 — Diff and PrintDiff live in graph.go

Both `Diff` and `PrintDiff` are added to the existing `meshant/graph/graph.go` file. They operate on `MeshGraph` and produce `GraphDiff` — they belong in the same package and file as `Articulate` and `PrintArticulation`. No new file is created for the diff logic.

If `graph.go` approaches the 800-line limit after M4, extraction into `diff.go` is acceptable at that point. The decision is deferred until the implementation reveals the actual line count.

### Decision 10 — Tag-filter axis remains deferred

M4 does not introduce a tag-filter axis. The diff operates on fully articulated `MeshGraph` values — the filtering parameters that produced each graph are captured in their `Cut` fields. Tag filtering, if added in a future milestone, would be an `ArticulationOptions` concern and would flow through unchanged.

---

## Full Type Signatures

```go
// ShadowShiftKind names the direction of movement across the shadow boundary.
type ShadowShiftKind string

const (
    // ShadowShiftEmerged indicates an element moved from shadow to visible node.
    ShadowShiftEmerged ShadowShiftKind = "emerged"

    // ShadowShiftSubmerged indicates an element moved from visible node to shadow.
    ShadowShiftSubmerged ShadowShiftKind = "submerged"

    // ShadowShiftReasonChanged indicates an element remained in shadow in both
    // graphs but with different exclusion reasons.
    ShadowShiftReasonChanged ShadowShiftKind = "reason-changed"
)

// ShadowShift records one element's movement across or within the shadow boundary
// between two graph articulations.
//
// FromReasons is empty when the element was a visible Node in g1 (Kind == ShadowShiftSubmerged).
// ToReasons is empty when the element became a visible Node in g2 (Kind == ShadowShiftEmerged).
// Both FromReasons and ToReasons are non-empty when Kind == ShadowShiftReasonChanged.
type ShadowShift struct {
    Name        string
    Kind        ShadowShiftKind
    FromReasons []ShadowReason
    ToReasons   []ShadowReason
}

// PersistedNode records a node present in both graphs, with its appearance
// count from each. A count change indicates the element became more or less
// active between the two cuts.
type PersistedNode struct {
    Name      string
    CountFrom int
    CountTo   int
}

// GraphDiff is the result of comparing two MeshGraph articulations.
// It records what nodes and edges entered or left visibility, which elements
// moved across or within the shadow boundary, and the full cuts of both input
// graphs so the diff is self-situated.
//
// A GraphDiff is not a neutral changelog. It records what became visible or
// invisible between two specific situated cuts. The From and To fields name
// those cuts.
type GraphDiff struct {
    NodesAdded     []string        // in g2.Nodes, not g1.Nodes; sorted alphabetically
    NodesRemoved   []string        // in g1.Nodes, not g2.Nodes; sorted alphabetically
    NodesPersisted []PersistedNode // in both; sorted alphabetically by Name
    EdgesAdded     []Edge          // TraceID in g2 edges, not g1 edges; sorted by TraceID
    EdgesRemoved   []Edge          // TraceID in g1 edges, not g2 edges; sorted by TraceID
    ShadowShifts   []ShadowShift   // shadow boundary movements; sorted by Name
    From           Cut             // cut of g1, stored verbatim
    To             Cut             // cut of g2, stored verbatim
}

// Diff compares two MeshGraph articulations and returns a GraphDiff recording
// what became visible or invisible between them. The diff is directional:
// Diff(g1, g2) reads as "moving from g1 to g2."
func Diff(g1, g2 MeshGraph) GraphDiff

// PrintDiff writes a human-readable comparison of two articulations to w.
// All sections are rendered unconditionally — an empty section emits "(none)"
// rather than being skipped. The From/To cut metadata is always printed first.
func PrintDiff(w io.Writer, d GraphDiff) error
```

---

## Test Plan

All tests live in package `graph_test`. New tests are added to `meshant/graph/graph_test.go`
(groups 10–15) and `meshant/graph/e2e_test.go` (group 16). `testhelpers_test.go` gains one
new helper.

### New helper in `testhelpers_test.go`

`validTraceWithElements(id, observer string, sources, targets []string) schema.Trace` — builds
a valid trace with populated Source and Target slices, for tests that need node-producing traces
without repeating the construction inline.

---

### Group 10: Diff — empty and trivial cases

| Test function | Verifies |
|---|---|
| `TestDiff_IdenticalGraphs_NoChanges` | Diff of same graph against itself → all change slices empty, From == To |
| `TestDiff_TwoEmptyGraphs_NoChanges` | Diff of two zero-value MeshGraphs → all change slices empty |
| `TestDiff_EmptyGraphs_CutsStored` | From and To are stored correctly even on empty graphs |

### Group 11: Diff — node differences

| Test function | Verifies |
|---|---|
| `TestDiff_NodeAdded_InG2NotG1` | Node in g2 but not g1 → appears in NodesAdded |
| `TestDiff_NodeRemoved_InG1NotG2` | Node in g1 but not g2 → appears in NodesRemoved |
| `TestDiff_NodePersisted_InBothGraphs` | Node in both → appears in NodesPersisted, not in Added or Removed |
| `TestDiff_NodePersisted_CountUnchanged` | Persisted node with same count → CountFrom == CountTo |
| `TestDiff_NodePersisted_CountChanged` | Persisted node with different counts → CountFrom != CountTo |
| `TestDiff_NodesAdded_SortedAlphabetically` | NodesAdded slice is alphabetically sorted |
| `TestDiff_NodesRemoved_SortedAlphabetically` | NodesRemoved slice is alphabetically sorted |
| `TestDiff_NodesPersisted_SortedAlphabetically` | NodesPersisted slice is alphabetically sorted by Name |

### Group 12: Diff — edge differences

| Test function | Verifies |
|---|---|
| `TestDiff_EdgeAdded_TraceIDInG2NotG1` | Edge with TraceID in g2 but not g1 → appears in EdgesAdded |
| `TestDiff_EdgeRemoved_TraceIDInG1NotG2` | Edge with TraceID in g1 but not g2 → appears in EdgesRemoved |
| `TestDiff_EdgePersisted_SameTraceID_NotInEitherSlice` | Edge present in both → absent from EdgesAdded and EdgesRemoved |
| `TestDiff_EdgesAdded_FullEdgeStored` | EdgesAdded entries contain full Edge struct (WhatChanged, Mediation, Observer, Tags, Sources, Targets) |
| `TestDiff_EdgesRemoved_FullEdgeStored` | EdgesRemoved entries contain full Edge struct |
| `TestDiff_EdgesAdded_SortedByTraceID` | EdgesAdded slice is sorted by TraceID alphabetically |
| `TestDiff_EdgesRemoved_SortedByTraceID` | EdgesRemoved slice is sorted by TraceID alphabetically |

### Group 13: Diff — shadow shifts

| Test function | Verifies |
|---|---|
| `TestDiff_ShadowShift_Emerged_ShadowToNode` | Element in g1.ShadowElements, in g2.Nodes → ShadowShift with Kind == "emerged" |
| `TestDiff_ShadowShift_Emerged_FromReasons_Populated` | Emerged shift → FromReasons copied from g1 shadow element; ToReasons empty |
| `TestDiff_ShadowShift_Submerged_NodeToShadow` | Element in g1.Nodes, in g2.ShadowElements → ShadowShift with Kind == "submerged" |
| `TestDiff_ShadowShift_Submerged_ToReasons_Populated` | Submerged shift → ToReasons copied from g2 shadow element; FromReasons empty |
| `TestDiff_ShadowShift_ReasonChanged_ShadowInBoth_DifferentReasons` | Element in shadow of both, different Reasons → ShadowShift with Kind == "reason-changed" |
| `TestDiff_ShadowShift_ReasonChanged_FromAndToReasons_Populated` | reason-changed shift → both FromReasons and ToReasons non-empty |
| `TestDiff_NoShadowShift_ShadowInBoth_SameReasons` | Element in shadow of both with identical Reasons → not in ShadowShifts |
| `TestDiff_NoShadowShift_NodeInBoth` | Element that is a Node in both graphs → not in ShadowShifts |
| `TestDiff_ShadowShifts_SortedAlphabetically` | ShadowShifts slice is alphabetically sorted by Name |

### Group 14: Diff — cut metadata

| Test function | Verifies |
|---|---|
| `TestDiff_From_StoresG1Cut_Verbatim` | GraphDiff.From equals g1.Cut exactly (ObserverPositions, TimeWindow, TracesIncluded, TracesTotal, DistinctObserversTotal) |
| `TestDiff_To_StoresG2Cut_Verbatim` | GraphDiff.To equals g2.Cut exactly |
| `TestDiff_From_And_To_AreIndependent` | Modifying g1.Cut.ShadowElements after Diff does not affect GraphDiff.From |

### Group 15: PrintDiff — output

| Test function | Verifies |
|---|---|
| `TestPrintDiff_EmptyDiff_AllSectionsPresent` | PrintDiff on zero GraphDiff emits all section headers |
| `TestPrintDiff_EmptyDiff_NodeSections_ShowNone` | Empty NodesAdded/Removed/Persisted → "(none)" in output |
| `TestPrintDiff_EmptyDiff_EdgeSections_ShowNone` | Empty EdgesAdded/Removed → "(none)" in output |
| `TestPrintDiff_EmptyDiff_ShadowShifts_ShowNone` | Empty ShadowShifts → "(none)" in output |
| `TestPrintDiff_NodeAdded_AppearsInOutput` | Added node name appears in the nodes-added section |
| `TestPrintDiff_NodeRemoved_AppearsInOutput` | Removed node name appears in the nodes-removed section |
| `TestPrintDiff_NodePersisted_BothCounts_Shown` | Persisted node shows CountFrom and CountTo values |
| `TestPrintDiff_EdgeAdded_TraceID_Shown` | Added edge TraceID appears in output |
| `TestPrintDiff_EdgeRemoved_TraceID_Shown` | Removed edge TraceID appears in output |
| `TestPrintDiff_ShadowShift_Emerged_KindShown` | Emerged shift shows "emerged" in output |
| `TestPrintDiff_ShadowShift_Submerged_KindShown` | Submerged shift shows "submerged" in output |
| `TestPrintDiff_ShadowShift_ReasonChanged_KindShown` | reason-changed shift shows "reason-changed" in output |
| `TestPrintDiff_CutMetadata_FromAndTo_Shown` | Output contains observer positions and time windows for both cuts |
| `TestPrintDiff_WriteError_Propagated` | Write failure returns wrapped error |

### Group 16: E2E (e2e_test.go)

| Test function | Verifies |
|---|---|
| `TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesAdded` | satellite-operator day-1 vs day-3 cut from longitudinal dataset → elements appearing only on day 3 are in NodesAdded |
| `TestE2E_Diff_SatelliteOperator_Day1VsDay3_EdgesAdded` | satellite-operator day-1 vs day-3 → day-3 trace edges are in EdgesAdded |
| `TestE2E_Diff_SatelliteOperator_Day1VsDay3_NodesPersisted` | satellite-operator day-1 vs day-3 → elements present on both days appear in NodesPersisted |
| `TestE2E_Diff_SatelliteOperator_Day1VsDay3_ShadowShifts` | satellite-operator day-1 vs day-3 → at least one element demonstrates a shadow shift |
| `TestE2E_Diff_SatelliteOperator_Day1VsDay3_CutsStored` | From cut has day-1 TimeWindow; To cut has day-3 TimeWindow |
| `TestE2E_Diff_SameDay_TwoObservers_DisjointCuts` | Same day (day 1), two different observer positions → NodesAdded and NodesRemoved both non-empty |
| `TestE2E_Diff_SameDay_TwoObservers_ShadowShifts` | Same-day diff across two observers → shadow shifts present (elements visible to one, shadow to other) |
| `TestE2E_PrintDiff_Day1VsDay3_RoundTrip` | PrintDiff runs without error on a day-1 vs day-3 diff; output contains From/To cut summaries |

---

## Task Breakdown

### M4.1 — `Diff()` implementation and unit tests (groups 10–14)

**Branch:** `feat/m4-diff` (cut from `develop`)

**Files:**
- `meshant/graph/graph_test.go` — add groups 10–14 (~25 tests)
- `meshant/graph/testhelpers_test.go` — add `validTraceWithElements` helper
- `meshant/graph/graph.go` — add `ShadowShiftKind`, constants, `ShadowShift`, `PersistedNode`, `GraphDiff` types, and `Diff()` function

**Steps:**
1. Add `validTraceWithElements` helper to `testhelpers_test.go` — RED
2. Write tests for groups 10–14 in `graph_test.go` — RED (won't compile until types exist)
3. Add type definitions to `graph.go` — tests now compile
4. Implement `Diff()`:
   a. Build TraceID sets for g1 and g2 edges
   b. Compute NodesAdded, NodesRemoved, NodesPersisted from map key difference
   c. Compute EdgesAdded, EdgesRemoved from TraceID set difference
   d. Build shadow element lookup maps for both graphs (by Name)
   e. Compute ShadowShifts: iterate union of shadow element names across both graphs
   f. Apply sort discipline to all output slices
   g. Store From = g1.Cut, To = g2.Cut (defensive slice copies)
5. Run full test suite — GREEN, verify 80%+ coverage

**Defensive copy note:** `Cut` is a struct value but its slice fields (`ObserverPositions`,
`ShadowElements`, `ExcludedObserverPositions`) must be copied explicitly to match the copy
discipline in `Articulate`.

### M4.2 — `PrintDiff()` implementation and output tests (group 15)

**Branch:** `feat/m4-diff` (same branch, continued)

**Files:**
- `meshant/graph/graph_test.go` — add group 15 (~14 tests)
- `meshant/graph/graph.go` — add `PrintDiff()` function

**Steps:**
1. Write group 15 tests — RED
2. Implement `PrintDiff()`:
   - Header: `"=== Mesh Diff (situated comparison) ==="`
   - From/To section: observer positions, time windows, traces included/total for each cut
   - Nodes Added section: sorted names or "(none)"
   - Nodes Removed section: sorted names or "(none)"
   - Nodes Persisted section: `name  xN → xM` or "(none)"
   - Edges Added section: abbreviated TraceID + WhatChanged or "(none)"
   - Edges Removed section: abbreviated TraceID + WhatChanged or "(none)"
   - Shadow Shifts section: Name, Kind, FromReasons→ToReasons per shift or "(none)"
   - Footer: `"Note: this diff is a comparison between two situated cuts, not an objective account of change."`
3. Run full test suite — GREEN

**Output format (non-normative sketch):**

```
=== Mesh Diff (situated comparison) ===

From cut:
  Observer position(s): satellite-operator
  Time window:          2026-03-11T00:00:00Z – 2026-03-11T23:59:59Z
  Traces included: 5 of 40

To cut:
  Observer position(s): satellite-operator
  Time window:          2026-03-18T00:00:00Z – 2026-03-18T23:59:59Z
  Traces included: 3 of 40

Nodes added (2):
  deforestation-confirmation-scan-3
  ministry-conservation-order-br-am-441

Nodes removed (1):
  initial-alert-landsat9

Nodes persisted (3):
  landsat-9-satellite            x4 → x2
  national-forest-agency-report  x2 → x2
  satellite-tasking-protocol     x1 → x1

Edges added (3):
  c3d4e5f6...  [translation]  third Landsat-9 overpass confirms 112ha cleared

Edges removed (5):
  a1b2c3d4...  [translation]  Landsat-9 overpass detects anomaly in AM state

Shadow shifts (2):
  enforcement-order-br-am-441  emerged        [observer] → (visible)
  deforestation-extent-report  reason-changed  [observer] → [observer, time-window]

---
Note: this diff is a comparison between two situated cuts, not an objective account of change.
```

### M4.3 — E2E tests (group 16)

**Branch:** `feat/m4-diff` (same branch, continued)

**Files:**
- `meshant/graph/e2e_test.go` — add group 16 (~8 tests)

**Steps:**
1. Write group 16 tests using `deforestation_longitudinal.json` (path already established:
   `../../data/examples/deforestation_longitudinal.json`)
2. Tests use `Articulate` with `TimeWindow` to produce day-1 and day-3 cuts for
   `satellite-operator`, then call `Diff`
3. Run full suite — GREEN

**Time windows for E2E:**
- Day-1: `TimeWindow{Start: mustParseTime(t, "2026-03-11T00:00:00Z"), End: mustParseTime(t, "2026-03-11T23:59:59Z")}`
- Day-3: `TimeWindow{Start: mustParseTime(t, "2026-03-18T00:00:00Z"), End: mustParseTime(t, "2026-03-18T23:59:59Z")}`
- Two-observer same-day: day-1 window with `satellite-operator` vs `ngo-field-coordinator`

### M4.4 — Decision record

**Branch:** `feat/m4-diff` (same branch, or separate `feat/m4-decision` if preferred)

**File:** `docs/decisions/graph-diff-v1.md`

**Decisions to record:** all 10 listed above, plus explicit deferred items (tag-filter axis,
weighted diff, temporal visibility in shadow shifts, diff-as-actor, persistence, CLI, multi-step
diff chains).

---

## What M4 Explicitly Defers

- **Tag-filter axis**: articulation-level concern, not yet implemented; does not affect Diff API
- **Weighted diff**: edges in both graphs contribute equally; no recency or frequency weighting
- **Temporal visibility in shadow shifts**: ShadowShift records reasons but not "which window would make this element visible again"
- **Diff-as-actor**: a `GraphDiff` is not yet a traceable object; it does not receive a trace ID; see Provisional M5 note
- **Persistence**: diffs remain in-memory only
- **CLI**: form factor remains deliberately open
- **Multi-step diff chains**: no "diff of diffs" or timeline of graph evolution; each Diff is a single pairwise comparison

---

## Provisional M5 Note: Graph-as-Actor

M4 produces `GraphDiff`, a comparison between two situated cuts. Both `MeshGraph` (from M2/M3)
and `GraphDiff` (from M4) are now outputs that can *do things* in the mesh: a deforestation map
triggers policy, a diff report prompts escalation. These outputs are actors, not just observations.

M5 should implement the graph-as-actor principle noted in `docs/decisions/articulation-v1.md`
Decision 5 and carried through M3's provisional note.

**The core requirement:** a produced `MeshGraph` or `GraphDiff` can be assigned a stable
identifier and appear as a `Source` or `Target` in subsequent traces. Once a graph or diff is
itself traceable, MeshAnt can record its own observation apparatus as part of the mesh it is
observing.

**What M5 must resolve:**

1. **Schema convention for graph-reference IDs.** `schema.Trace.Source` and `.Target` are
   `[]string`. A graph-reference ID must be representable as a string and distinguishable from
   a plain element name. The convention could be a URI scheme (`meshgraph://articulation/<uuid>`),
   a structural prefix (`[graph:<uuid>]`), or a new convention defined in a decision record. The
   choice must not break `schema.Validate()` for traces that do not reference graphs.

2. **ID assignment for MeshGraph.** `Articulate` would need to produce a stable identifier for
   each `MeshGraph` it returns — either caller-supplied or generated. A generated ID (e.g., UUID
   derived from Cut parameters) is reproducible; a random UUID is simpler but not idempotent.

3. **ID assignment for GraphDiff.** A `GraphDiff` produced from two identified graphs could
   derive its own ID from the From and To cut identifiers.

4. **Validate() treatment.** If a trace's source or target is a graph-reference ID,
   `schema.Validate()` must either accept the format as a valid element name or apply a new
   validation rule. The decision must not silently accept graph-reference strings as ordinary
   element names when validation is the wrong context.

5. **Loader treatment.** `loader.Load` returns `[]schema.Trace` and calls `schema.Validate()`
   on each. If traces can reference graph IDs, the loader may need to surface graph-reference
   strings distinctly in `MeshSummary`, or leave them transparent (treat as ordinary element
   strings in the summary).

M5 is not committed here. These are the questions the shadow of M4 points toward.
