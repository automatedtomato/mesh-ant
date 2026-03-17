# Decision Record: Time-Window Cut Axis v1

**Date:** 2026-03-11
**Status:** Active
**Package:** `meshant/graph`
**Branch merged:** `feat/m3-time-window`

---

## What was decided

1. **`TimeWindow` lives in the `graph` package, not `schema`**
2. **Zero `TimeWindow` = full temporal cut (named, symmetric with observer axis)**
3. **AND semantics for combined filters**
4. **Inclusive bounds on both ends**
5. **`ShadowReason` per element — single shadow list, reasons accumulated across traces**
6. **`Cut.TimeWindow` stored verbatim**
7. **`TimeWindow.Validate()` — inverted window is an error, not a silent empty result**

---

## Context

M2 established observer position as the primary cut axis. M3 adds the second axis: time.
The question was how time-window filtering should interact with observer filtering, how
the shadow should represent temporal exclusion, and how to handle degenerate inputs.

This record documents the decisions made and what was explicitly left open.

---

## Decision 1 — `TimeWindow` in the `graph` package, not `schema`

**Chosen:** `TimeWindow` defined in `meshant/graph`, alongside `ArticulationOptions` and `Cut`.

The `schema` package defines what a trace is. The `graph` package defines how traces are
cut into a graph. `TimeWindow` is an articulation concept: it parameterises a cut, not a
trace. The `schema` package already has `Trace.Timestamp time.Time` — no schema changes
were needed.

**Why not in schema:**

A `TimeWindow` is not a property of a trace. It is a constraint imposed by a caller when
rendering a view. Putting it in `schema` would conflate the data model with the rendering
layer. This separation follows the same logic as `ArticulationOptions.ObserverPositions`:
the observer filter lives in `graph`, not `schema`, even though `schema.Trace.Observer`
is what it filters on.

---

## Decision 2 — Zero `TimeWindow` = full temporal cut

`ArticulationOptions{TimeWindow: TimeWindow{}}` (both `Start` and `End` zero) means:
include all traces regardless of timestamp. This is not an error or a missing value — it
is a named state: "I am not filtering by time."

**Symmetry with the observer axis:**

`ObserverPositions: nil` = no observer filter (full cut). `TimeWindow{}` = no time filter
(full temporal cut). Both axes use zero value to mean "unbounded." A caller who sets
neither gets the full dataset. A caller who sets both gets a doubly-constrained cut.

`TimeWindow.IsZero()` reports whether both bounds are unset. `Cut.TimeWindow` stores the
zero value verbatim, so the cut is self-describing: `TimeWindow{}` means "no time
filter was applied."

---

## Decision 3 — AND semantics for combined filters

When both `ObserverPositions` and `TimeWindow` are set, a trace must pass **both** to be
included. A trace from the correct observer but outside the time window is excluded. A
trace within the time window but from a different observer is excluded.

**Why AND, not OR:**

An OR filter would let a caller accidentally include traces they did not mean to include.
AND semantics mean that adding a second constraint always narrows the result, never widens
it. This is consistent with how cuts work in ANT terms: each constraint is a further
specification of position. "From the satellite-operator's position, within March 11"
is a narrower position than either constraint alone.

---

## Decision 4 — Inclusive bounds on both ends

A trace is included if:
- `trace.Timestamp >= Start` (when `Start` non-zero)
- AND `trace.Timestamp <= End` (when `End` non-zero)

A trace at exactly `Start` or exactly `End` is included. This matches natural language:
"between March 11 and March 14" includes both March 11 and March 14.

Half-open intervals (e.g. `[Start, End)`) were considered but rejected: they require
callers to reason about sub-second precision when constructing day-boundary windows,
which is error-prone and not warranted by the current datasets.

`PrintArticulation` renders a zero bound as `"(unbounded)"` rather than an empty string,
so half-open windows (one bound set, one zero) read as:
```
Time window: (unbounded) – 2026-03-14T23:59:59Z
```

---

## Decision 5 — `ShadowReason` per element; single shadow list

Every `ShadowElement` now carries a `Reasons []ShadowReason` field. Two constants:
- `ShadowReasonObserver` — at least one excluding trace failed the observer filter
- `ShadowReasonTimeWindow` — at least one excluding trace failed the time-window filter

An element can carry both reasons. Reasons are accumulated across all excluded traces that
mention the element: if any such trace fails the observer filter, `ShadowReasonObserver`
is present; if any fails the time-window filter, `ShadowReasonTimeWindow` is present. An
element can have both reasons even if no single trace failed both filters simultaneously —
this is the correct union-across-traces semantics, documented explicitly in the type.

**Why a single shadow list, not two separate lists:**

Keeping one `ShadowElements []ShadowElement` preserves the existing `PrintArticulation`
structure and the conceptual clarity that "the shadow is what this cut cannot see."
Splitting into `ObserverShadow` and `TimeShadow` would suggest that temporal and observer
exclusion are different kinds of invisibility, when they are both instances of the same
thing: the element is present in the dataset but not rendered visible by this cut.

`Reasons` adds nuance to the unified shadow without fragmenting it.

---

## Decision 6 — `Cut.TimeWindow` stored verbatim

`Cut.TimeWindow` stores the `TimeWindow` value passed in `ArticulationOptions`, unchanged.
`TimeWindow` is a value type (two `time.Time` fields); copying `ArticulationOptions` copies
it automatically. No explicit deep-copy is required.

This follows the same pattern as `Cut.ObserverPositions` (stored verbatim as a copy).
The `Cut` is always self-describing: reading `Cut.TimeWindow` tells you exactly what
temporal constraint was in effect, without recomputing it from graph structure.

---

## Decision 7 — `TimeWindow.Validate()` — inverted window is an error

`TimeWindow.Validate() error` returns an error when both `Start` and `End` are non-zero
and `End.Before(Start)`. An inverted window would silently produce a zero-trace
articulation — a valid-looking `MeshGraph` with `TracesIncluded == 0` — with no
indication that the filter parameters were nonsensical.

**Why an explicit Validate() rather than silent normalization:**

Swapping `Start` and `End` silently would hide a programming error. Returning zero traces
silently would be indistinguishable from a dataset with no traces in that window. Making
the invalid state explicit — with a descriptive error message including both the bad Start
and End values — makes the programming error visible at the call site.

Callers should call `TimeWindow.Validate()` before passing a `TimeWindow` to `Articulate`.
This is the same pattern as `schema.Trace.Validate()`: the caller is responsible for
validation before use.

---

## Relation to prior decisions

- **articulation-v2.md Decision 6** (time-window axis deferred): fulfilled here. The
  dataset now has longitudinal depth (3 days); the time-window axis becomes meaningful.
- **articulation-v2.md Decision 3** (empty filter = full cut): the zero `TimeWindow`
  extends the same principle to the temporal axis.
- **articulation-v2.md Decision 2** (shadow as mandatory output): the shadow section
  is still mandatory. `ShadowReason` adds nuance but does not change the requirement
  that every articulation names what it excludes.

---

## What this cut excludes

- **Tag-filter axis**: still deferred. Tag filters highlight patterns within an already-
  positioned cut; adding them now would require deciding whether they apply before or
  after observer/time filters, which is premature.
- **Temporal visibility in shadow**: `SeenFrom` records which observer positions could
  see a shadow element; it does not record *when* the element would become visible.
  Adding `SeenInWindow []TimeWindow` per element is deferred until the axis has been
  used in practice and the need becomes evident.
- **`Articulate` returning an error**: `Articulate` still returns `MeshGraph` (no error
  return). Validation is the caller's responsibility via `TimeWindow.Validate()`. A future
  milestone could change `Articulate` to return `(MeshGraph, error)` if the validation
  surface grows.
- **Graph diff**: comparing two `MeshGraph` values (e.g., same observer, different
  windows). The longitudinal dataset makes this the natural next step; deferred to M4.
- **Graph-as-actor**: a day-1 articulation entering a day-2 trace as a source. The
  longitudinal dataset demonstrates the need; schema convention for graph-reference IDs
  remains unresolved. Deferred to M4.
