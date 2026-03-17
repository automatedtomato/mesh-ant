# Decision Record: Articulation v1

**Date:** 2026-03-11
**Status:** Active
**Package:** `meshant/graph`
**Branch merged:** `feat/m2-graph`

---

## What was decided

1. **Observer position as the primary cut axis**
2. **Shadow as mandatory output**
3. **Empty filter = full cut (named, not silent default)**
4. **`ExcludedObserverPositions` stored in `Cut`, not recomputed in `PrintArticulation`**
5. **Graph-as-actor noted for future milestones**
6. **Time-window and tag-filter axes explicitly deferred**

---

## Context

MeshAnt needed a first articulation layer: a way to render a graph from a
trace dataset without producing a god's-eye view. The question was what
the primary cut axis should be, how to represent what a cut cannot see,
and what to defer.

This record documents the decisions made and what was explicitly left open.

---

## Decision 1 — Observer position as the primary cut axis

**Chosen:** `ArticulationOptions.ObserverPositions []string`

A graph in MeshAnt is not a description of the network as it really is. It is
a rendering from a particular observer position. Different observers in the
same dataset literally compose different worlds: the satellite operator, the
NGO field coordinator, and the carbon registry auditor are each present at
different traces, and a graph articulated from each position shows a different
subset of elements, edges, and relations.

This is the first implication of ANT's rejection of the god's-eye view: there
is no view from nowhere. The observer is always inside the mesh.

**Why observer position first (not time window, not tag filter):**

- Observer position is the most fundamental axis: it determines *which traces
  count*, not just which elements are emphasised. A tag filter would still see
  all traces and then select; an observer filter asks who is doing the seeing.
- A time-window filter would be more useful once a dataset has longitudinal
  depth. The first dataset is a single day.
- Tag filters are analysis tools — useful for highlighting patterns within a
  cut, not for constituting the cut itself.

**What this does not mean:**

Observer position is not the only axis. Future milestones will add time-window
and tag-filter axes. Multiple axes can be combined. This decision is about
which axis to implement first, not about privileging observer position above
all others permanently.

---

## Decision 2 — Shadow as mandatory output

Every articulation must name what it excludes. This follows Marilyn Strathern's
insight that every representation requires a cut — and that the cut has a shadow:
what was present but not rendered visible.

**`ShadowElement`** names an element that exists in the full dataset but is
invisible from the chosen observer position. Its `SeenFrom` field records which
observer positions would make it visible. The shadow has its own trace: it is
not absence, it is invisible-from-here.

**Why mandatory and not optional:**

Making the shadow optional would let callers take a god's-eye view silently —
producing output that looks definitive without naming what it excludes.
Requiring the shadow section in `PrintArticulation` output (even when empty)
encodes the methodological commitment at the code level.

When the shadow is empty (full cut taken), the output reads:
```
Shadow (elements invisible from this position: 0):
  (none — full cut taken)
```
This is not silence. It is a named state: "I looked at everything, and this
is what I saw." The full cut is a position too.

---

## Decision 3 — Empty filter = full cut (named, not error)

`ArticulationOptions{ObserverPositions: nil}` (or `[]string{}`) means: include
all traces. This is not an error or a default fallback — it is a valid and
explicit choice to occupy a god's-eye position.

The API makes this choice nameable: the caller must pass an empty (or nil)
`ObserverPositions` to get the full cut. They cannot get it accidentally by
forgetting to specify a filter. The empty filter is itself a stated position.

`Cut.ObserverPositions` stores the filter verbatim (as a copy), so the cut is
always self-describing: `[]string(nil)` means full cut; `["observer-a"]` means
filtered to one position.

---

## Decision 4 — `ExcludedObserverPositions` stored in `Cut`

`PrintArticulation` includes a footer naming observer positions not included
in the cut. This list could be reconstructed from the graph structure — but
only approximately: if an excluded observer's traces have elements that *also*
appear in included traces, those elements go into `Nodes` (with `ShadowCount > 0`)
rather than `ShadowElements`. The excluded observer's name would then not appear
in any `SeenFrom` list and would be invisible to `PrintArticulation`.

**Decision:** Compute `ExcludedObserverPositions` in `Articulate`, where the
full observer set is available, and store it in `Cut`. `PrintArticulation`
reads it directly.

This respects the separation of concerns: `Articulate` knows the full picture;
`PrintArticulation` renders a graph it was given.

---

## Decision 5 — Graph-as-actor: noted, not implemented *(resolved in M5)*

A graph produced by `Articulate` is not a neutral object. Once produced, it
can enter the mesh as a force — a deforestation map that triggers policy, a
carbon-credit audit that suspends a market. The graph is not just a view of
the mesh; it becomes part of the mesh.

This is consistent with ANT: a scientific paper, a map, a report — these are
actors, not just descriptions of actors.

**Decision:** Noted architecturally in M2; implemented in M5 (`docs/decisions/graph-as-actor-v1.md`).
`IdentifyGraph` assigns a UUID to `MeshGraph.ID`; `GraphRef` produces a `"meshgraph:<uuid>"`
string that can appear in any `Source` or `Target` slice. Graph-reference strings travel
via the same `[]string` fields as all other element names — no new field type was needed.
M7 extended this to reflexive tracing: `ArticulationTrace` records the act of articulation
as a `schema.Trace` in the mesh.

---

## Decision 6 — Time-window and tag-filter axes: deferred *(both resolved)*

Both are useful cut axes:

- **Time-window filter** *(resolved in M3)*: `ArticulationOptions.TimeWindow` added in M3.
  AND semantics with inclusive bounds. Zero value = no filter. Reason tracking per shadow
  element (`ShadowReasonTimeWindow`). See `docs/decisions/time-window-v1.md`.
- **Tag filter** *(resolved in M10)*: `ArticulationOptions.Tags` added in M10. Any-match
  (OR) semantics — a trace passes if it carries any of the specified tags. Third shadow
  reason: `ShadowReasonTagFilter`. See `docs/decisions/m10-tag-filter-diff-export-cli-v1.md`.

The interaction question was resolved: all three axes use AND semantics across filters —
a trace must pass observer, time-window, AND tag filters simultaneously to be included.

---

## What this cut excludes (M2 shadow — many since resolved)

- ~~No composite filters (time + observer + tag in one call)~~ — resolved in M10: all three axes combined with AND semantics
- ~~No longitudinal articulation~~ — resolved in M3: `TimeWindow` in `ArticulationOptions`
- ~~No graph diff~~ — resolved in M4: `Diff(g1, g2) GraphDiff` in `diff.go`
- No weighted edges (still open: all traces contribute equally regardless of recency or frequency)
- ~~No graph-as-actor implementation~~ — resolved in M5: `IdentifyGraph`, `GraphRef`; M7: `ArticulationTrace`
- ~~No persistence of articulations~~ — resolved in M8: `persist.WriteJSON`, `ReadGraphJSON`

These exclusions were the shadow of M2's cut. Naming them made the subsequent milestones legible.

---

## Relation to prior decisions

- **trace-schema-v1.md**: The `Observer` field on `Trace` is required, which
  makes observer-position filtering possible. This was the correct decision.
  The schema's open-vocabulary `Tags` and optional `Mediation` fields mean
  future axes (tag filter, mediation filter) are available without schema
  changes.
- **Principle 8 (designer inside the mesh)**: Every articulation names its
  position. The schema requires `observer`; the graph requires
  `ObserverPositions` to be named. Both encode the same commitment at different
  levels.
