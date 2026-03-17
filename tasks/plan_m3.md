# MeshAnt — Milestone 3 Plan

**Date:** 2026-03-11
**Theme:** Time-situated articulation — the mesh as it was visible from a stated position, within a stated window.

---

## Overview

M2 established observer position as the primary cut axis. Every articulation is made *from somewhere*. M3 adds the second axis: *from when*. A time-window filter lets a caller ask not just "what did the satellite operator see?" but "what did the satellite operator see on March 11, before the policy response arrived?"

The key methodological commitment is unchanged: a time-window cut is not a more precise view of an objective timeline. It is a situated observation with a temporal boundary. The shadow now has two possible causes — an element may be invisible because the observer could not see it, *or* because it fell outside the time window, *or* both. The Cut must be self-describing about which axis excluded what.

**M3 = D (longitudinal dataset) + A (time-window axis implementation).**

---

## Design Decisions

### Decision 1 — Dataset: extend deforestation across three days

Extend the existing `deforestation.json` scenario into a new file,
`data/examples/deforestation_longitudinal.json`. The deforestation scenario is already
well-understood and populated with interesting structural features (3 threads, 8 observer
positions). Extending it across time is the most legible choice: a reader can trace how the
same mesh evolves, compare what different observers saw on each day, and watch elements
enter and exit windows.

Three time slices:
- **Day 1 — 2026-03-11** (existing 20 traces): initial detection, immediate policy response, carbon market reaction
- **Day 2 — 2026-03-14** (10 new traces): policy escalation, retask satellite imagery, community legal case, carbon market delayed notification
- **Day 3 — 2026-03-18** (10 new traces): Ministry of Environment formal ruling, UNFF complaint acknowledged, carbon market recalibration, new satellite scan

Total: 40 traces. The new file is self-contained — it does not reference `deforestation.json`.
The existing `deforestation.json` is left unchanged.

### Decision 2 — `TimeWindow` lives in the `graph` package

`TimeWindow` is an articulation concept, not a schema concept. The `schema` package defines
what a trace is; `graph` defines how traces are cut into a graph. `TimeWindow` belongs
alongside `ArticulationOptions` and `Cut`. No schema changes are needed — `Trace.Timestamp`
already exists.

### Decision 3 — Shadow semantics: single shadow, reasons tracked per element

An element in the shadow is invisible from this cut. The cause — observer filter, time filter,
or both — is recorded on `ShadowElement` via a new `Reasons []ShadowReason` field.

Keeping a single shadow list preserves the existing `PrintArticulation` structure and the
conceptual clarity that "the shadow is what this cut cannot see." Reasons add nuance without
restructuring. An element excluded by both axes carries both reasons.

### Decision 4 — `Cut.TimeWindow` stores the window verbatim

`Cut` gains a `TimeWindow` field. Zero `TimeWindow` means no time filter — consistent with
`ObserverPositions == nil` meaning no observer filter. `PrintArticulation` uses
`TimeWindow.IsZero()` to decide how to render the window line.

### Decision 5 — Filtering logic: AND, inclusive on both ends, zero = unbounded

A trace passes the time filter if and only if:
- `TimeWindow.Start.IsZero()` OR `trace.Timestamp >= TimeWindow.Start`
- AND `TimeWindow.End.IsZero()` OR `trace.Timestamp <= TimeWindow.End`

Combined with observer filter: both must pass (AND semantics). Full cut = `ObserverPositions
== nil AND TimeWindow.IsZero()`.

### Decision 6 — `ShadowElement.SeenFrom` remains observer-only; temporal visibility deferred

`SeenFrom` continues to record the observer positions of shadow traces. For elements excluded
by both axes, `SeenFrom` records the observers of all shadow traces containing that element.
"Visible in which window" is not computed in M3 — deferred.

---

## New Dataset: `data/examples/deforestation_longitudinal.json`

**Traces:** 40 total
**Time span:** 2026-03-11 through 2026-03-18

### Day 1 (2026-03-11) — 20 traces

All 20 traces from `deforestation.json`, verbatim. Same UUIDs. Same observers.

### Day 2 (2026-03-14) — 10 new traces

Threads active:
- **Satellite/policy**: retasked Landsat-9 delivers new imagery (satellite-operator);
  deforestation-detection-algorithm processes updated extent (112ha); national-forest-agency
  receives updated report
- **Community/legal**: tribunal case advances — first hearing set (ngo-field-coordinator);
  community legal aid secured (ngo-field-coordinator)
- **Carbon market**: verde-carbon-ltd receives delayed broker notification
  (carbon-credit-broker); market correction report triggers reassessment (carbon-registry-auditor)
- **Policy escalation**: Ministry of Environment receives escalated case
  (national-forest-agency); inter-ministerial protocol triggers joint working group
  (policy-enforcement-officer)
- **Cross-thread**: joint verification report updated with new satellite data
  (policy-enforcement-officer)

Structural features: absent-source ×2, multi-source ×1, all 6 tag types present.

### Day 3 (2026-03-18) — 10 new traces

Threads active:
- **Satellite/policy**: third Landsat-9 overpass confirms 112ha extent (satellite-operator);
  Ministry of Environment issues formal conservation order (policy-enforcement-officer)
- **Community/legal**: UNFF complaint formally acknowledged, case number assigned
  (international-treaty-body); community reforestation fund established
  (ngo-field-coordinator)
- **Carbon market**: buffer-pool recalculation complete; mata-viva-0031 replacement credits
  certified (carbon-registry-auditor); verde-carbon-ltd contract reissuance confirmed
  (carbon-credit-broker)
- **Cross-thread**: Paris Agreement REDD+ compliance report submitted
  (national-forest-agency); international treaty body publishes precedent notice
  (international-treaty-body)

Structural features: absent-source ×1, multi-source ×1, all 6 tag types present, all 8
original observer positions present, at least one element from day 1 reappears as
source/target in day 3 (cross-day element persistence).

### Timestamp distribution

| Day | Date | Count | Hours span |
|-----|------|-------|------------|
| 1 | 2026-03-11 | 20 | 02:14 – 17:30 |
| 2 | 2026-03-14 | 10 | 08:00 – 17:00 |
| 3 | 2026-03-18 | 10 | 09:00 – 18:00 |

Key test windows:
- `[2026-03-12, 2026-03-15]` → day 2 only (10 traces)
- `[2026-03-11, 2026-03-14]` → days 1+2 (30 traces)
- `[2026-03-15, 2026-03-20]` → day 3 only (10 traces)

---

## API Design

### New type: `TimeWindow` (in `meshant/graph`)

```go
// TimeWindow defines an inclusive time range for filtering traces.
// A zero Start means no lower bound; a zero End means no upper bound.
// A zero TimeWindow (both fields zero) means no time filter — consistent
// with empty ObserverPositions meaning no observer filter.
type TimeWindow struct {
    Start time.Time // zero = unbounded lower bound
    End   time.Time // zero = unbounded upper bound
}

// IsZero reports whether the TimeWindow applies no filter.
func (tw TimeWindow) IsZero() bool
```

### New type: `ShadowReason` (in `meshant/graph`)

```go
// ShadowReason names why an element is in the shadow of a cut.
type ShadowReason string

const (
    ShadowReasonObserver   ShadowReason = "observer"
    ShadowReasonTimeWindow ShadowReason = "time-window"
)
```

### Changed: `ShadowElement`

```go
type ShadowElement struct {
    Name     string
    SeenFrom []string       // observer positions from shadow traces (sorted)
    Reasons  []ShadowReason // why this element is in the shadow (sorted)
}
```

### Changed: `ArticulationOptions`

```go
type ArticulationOptions struct {
    ObserverPositions []string
    TimeWindow        TimeWindow // zero = no time filter
}
```

### Changed: `Cut`

```go
type Cut struct {
    ObserverPositions         []string
    TimeWindow                TimeWindow // NEW: stored verbatim
    TracesIncluded            int
    TracesTotal               int
    DistinctObserversTotal    int
    ShadowElements            []ShadowElement
    ExcludedObserverPositions []string
}
```

### `Articulate` — same signature, extended logic

```go
func Articulate(traces []schema.Trace, opts ArticulationOptions) MeshGraph
```

- A trace is included only if it passes **both** observer filter AND time filter
- Exclusion reason tracked per shadow element (observer / time-window / both)
- `Cut.TimeWindow` stores a copy of `opts.TimeWindow`

### `PrintArticulation` — extended output

After observer line:
```
Observer position(s): satellite-operator
Time window:          2026-03-11T00:00:00Z – 2026-03-14T23:59:59Z
```

When no time window:
```
Time window:          (none — no time filter)
```

Shadow section with reason annotation:
```
Shadow (elements invisible from this position: 5):
  enforcement-order-br-am-441 → also seen from: policy-enforcement-officer  [observer]
  mata-viva-0031-day3-credits → (no observer data)  [time-window]
  some-element → also seen from: national-forest-agency  [observer, time-window]
```

---

## Test Plan

### `meshant/graph/graph_test.go` — new groups

**Group 6: Articulate — TimeWindow filter**

| Test | Verifies |
|------|----------|
| `TestArticulate_TimeWindow_Start_ExcludesEarlierTraces` | Traces before Start excluded |
| `TestArticulate_TimeWindow_End_ExcludesLaterTraces` | Traces after End excluded |
| `TestArticulate_TimeWindow_BothBounds_IncludesOnly_WithinWindow` | Only in-window traces included |
| `TestArticulate_TimeWindow_StartAtTimestamp_InclusiveLowerBound` | Timestamp == Start is included |
| `TestArticulate_TimeWindow_EndAtTimestamp_InclusiveUpperBound` | Timestamp == End is included |
| `TestArticulate_TimeWindow_ZeroStart_NoLowerBound` | Zero Start = no lower bound |
| `TestArticulate_TimeWindow_ZeroEnd_NoUpperBound` | Zero End = no upper bound |
| `TestArticulate_TimeWindow_IsZero_FullCut` | Zero TimeWindow = full cut |
| `TestArticulate_TimeWindow_ZeroTracesInWindow` | Empty window → TracesIncluded == 0 |
| `TestArticulate_TimeWindow_CombinedWithObserver_AND_Semantics` | Both filters must pass |
| `TestArticulate_TimeWindow_StoredInCut` | Cut.TimeWindow stores Start/End verbatim |
| `TestArticulate_TimeWindow_FullCut_StoredAsZero` | No window → Cut.TimeWindow.IsZero() |

**Group 7: ShadowReason**

| Test | Verifies |
|------|----------|
| `TestArticulate_ShadowReason_ObserverOnly` | Observer-excluded element → Reasons == [observer] |
| `TestArticulate_ShadowReason_TimeWindowOnly` | Time-excluded element → Reasons == [time-window] |
| `TestArticulate_ShadowReason_Both` | Both-excluded element → Reasons == [observer, time-window] |
| `TestArticulate_ShadowReason_FullCut_NoReasons` | Full cut → no shadow elements |
| `TestArticulate_ShadowReason_ObserverCutOnly_AllReasonsAreObserver` | Observer-only filter → all shadow reasons are observer |
| `TestArticulate_ShadowReason_TimeWindowCutOnly_AllReasonsAreTimeWindow` | Time-only filter → all shadow reasons are time-window |

**Group 8: TimeWindow.IsZero**

| Test | Verifies |
|------|----------|
| `TestTimeWindow_IsZero_BothZero` | Both zero → true |
| `TestTimeWindow_IsZero_StartSet` | Start non-zero → false |
| `TestTimeWindow_IsZero_EndSet` | End non-zero → false |
| `TestTimeWindow_IsZero_BothSet` | Both set → false |

**Group 9: PrintArticulation — TimeWindow output**

| Test | Verifies |
|------|----------|
| `TestPrintArticulation_TimeWindow_LinePresent_WhenSet` | "Time window:" line with values |
| `TestPrintArticulation_TimeWindow_LinePresent_WhenZero` | "Time window:" line with "(none — no time filter)" |
| `TestPrintArticulation_TimeWindow_ShadowReasonAnnotated` | Shadow output includes reason annotations |
| `TestPrintArticulation_TimeWindow_ShadowReason_Observer_Annotation` | [observer] in output |
| `TestPrintArticulation_TimeWindow_ShadowReason_TimeWindow_Annotation` | [time-window] in output |
| `TestPrintArticulation_TimeWindow_ShadowReason_Both_Annotation` | [observer, time-window] in output |

### `meshant/graph/e2e_test.go` — new e2e tests

| Test | Verifies |
|------|----------|
| `TestE2E_LongitudinalDataset_FullCut` | 40 traces, empty shadow, 8+ distinct observers |
| `TestE2E_LongitudinalDataset_Day1Window` | [2026-03-11, 2026-03-11] → 20 traces |
| `TestE2E_LongitudinalDataset_Day2Window` | [2026-03-14, 2026-03-14] → 10 traces |
| `TestE2E_LongitudinalDataset_Day3Window` | [2026-03-18, 2026-03-18] → 10 traces |
| `TestE2E_LongitudinalDataset_Days1And2Window` | [2026-03-11, 2026-03-14] → 30 traces |
| `TestE2E_LongitudinalDataset_ShadowContainsDay3Elements` | Day 1+2 window → day 3 elements in shadow |
| `TestE2E_LongitudinalDataset_ObserverAndTimeWindow_Combined` | Observer + day-1 window → combined shadow |
| `TestE2E_LongitudinalDataset_ShadowReason_TimeWindow_Day3Element` | Day-3-only element → Reasons == [time-window] |
| `TestE2E_LongitudinalDataset_PrintArticulation_TimeWindowLine` | Filtered output contains time window line |

### `meshant/loader/longitudinal_test.go` — new file

| Test | Verifies |
|------|----------|
| `TestLongitudinal_Load_Count` | 40 traces |
| `TestLongitudinal_Load_AllValid` | All pass schema.Validate() |
| `TestLongitudinal_Load_AllHaveObserver` | No empty Observer |
| `TestLongitudinal_Load_AllHaveTimestamp` | No zero Timestamp |
| `TestLongitudinal_Timestamps_ThreeDays` | Exactly 3 distinct calendar dates |
| `TestLongitudinal_Timestamps_Day1Count` | 20 traces on 2026-03-11 |
| `TestLongitudinal_Timestamps_Day2Count` | 10 traces on 2026-03-14 |
| `TestLongitudinal_Timestamps_Day3Count` | 10 traces on 2026-03-18 |
| `TestLongitudinal_Tags_AllTypesPresent` | All 6 tag types across 40 traces |
| `TestLongitudinal_Tags_AllTypesInDay2` | All 6 tag types in day 2 alone |
| `TestLongitudinal_Tags_AllTypesInDay3` | All 6 tag types in day 3 alone |
| `TestLongitudinal_Observers_DistinctCount` | At least 8 distinct observer strings |
| `TestLongitudinal_Observers_AllPresentOnDay3` | All 8 observers appear in at least one day-3 trace |
| `TestLongitudinal_AbsentSource_Count` | At least 5 nil/empty Source traces |
| `TestLongitudinal_MultiSource_Count` | At least 4 traces with len(Source) >= 2 |
| `TestLongitudinal_CrossDay_ElementPersistence` | At least one element appears in day-1 and day-3 traces |
| `TestLongitudinal_IDs_AllUnique` | All 40 IDs distinct |
| `TestLongitudinal_Summarise_ElementCount` | At least 40 distinct elements |
| `TestLongitudinal_Summarise_MediatedTraceCount` | At least 25 traces with non-empty Mediation |

---

## Task Breakdown

### M3.1 — Longitudinal dataset

**Branch:** `feat/m3-dataset`
**Files:** `data/examples/deforestation_longitudinal.json`,
`meshant/loader/longitudinal_test.go`

1. Write tests in `longitudinal_test.go` — RED
2. Design day 2 and day 3 traces: UUIDs, timestamps, observers, tags, source/target, mediation
3. Write `deforestation_longitudinal.json` (all 40 traces, self-contained)
4. Run tests — GREEN

### M3.2 — Time-window axis

**Branch:** `feat/m3-time-window` (from `develop` after M3.1 merges)
**Files:** `meshant/graph/graph.go`, `meshant/graph/graph_test.go`,
`meshant/graph/e2e_test.go`

1. Write unit tests (Groups 6–9) — RED
2. Write e2e tests (use `t.Skip` for dataset-dependent tests until M3.1 merges)
3. Implement:
   a. `TimeWindow` struct + `IsZero()` method
   b. `ShadowReason` type + constants
   c. `Reasons []ShadowReason` on `ShadowElement`
   d. `TimeWindow` on `ArticulationOptions` + `Cut`
   e. Extend `Articulate`: time-filter pass + reason tracking
   f. Extend `PrintArticulation`: time-window line + reason annotations
4. Remove `t.Skip` from e2e tests
5. Run full suite — GREEN, 80%+ coverage

**No schema changes required.** `Trace.Timestamp` already exists.

### M3.3 — Decision record

**Branch:** `feat/m3-decision` (alongside M3.2)
**File:** `docs/decisions/time-window-v1.md`

Decisions to record:
1. `TimeWindow` in `graph`, not `schema`
2. Zero TimeWindow = full cut (symmetry principle)
3. AND semantics for combined filters
4. Inclusive bounds
5. `ShadowReason` per element (not separate shadow lists)
6. `Cut.TimeWindow` stored verbatim
7. What M3 explicitly defers
8. Relation to articulation-v2.md Decision 6 (fulfilled here)

---

## What M3 Explicitly Defers

- **Tag-filter axis**: useful for pattern highlighting within a cut; deferred until longitudinal cuts demonstrate the natural need
- **Temporal visibility in shadow**: `SeenFrom` records who could see an element, not when it would become visible; adding `SeenInWindow` is premature before the axis has been used in practice
- **Graph diff**: comparing two `MeshGraph` values; architecturally ready but semantics and types need their own milestone
- **Graph-as-actor**: noted in articulation-v2.md Decision 5; longitudinal dataset makes it more urgent but schema convention for graph-reference IDs still unresolved
- **Weighted edges by recency**: all edges contribute equally
- **Persistence**: articulations remain in-memory
- **CLI**: form factor still deliberately open

---

## Provisional M4 note

M4 is not scoped here, but two directions are visible:

**M4-A: Graph diff**
`Diff(g1, g2 MeshGraph) GraphDiff` — structured comparison of two articulations. The
longitudinal dataset is the natural test case: diff the day-1 cut against the day-3 cut from
the same observer position to see how the mesh evolved. `GraphDiff` would itself record both
input cuts (with their time windows and observer positions), making the diff a self-situated
object.

**M4-B: Graph-as-actor**
Assign a trace ID to a produced `MeshGraph` so that it can appear as a source or target in
subsequent traces. Requires a schema convention for graph-reference IDs and a decision about
how `Validate()` handles them. The longitudinal dataset — where a day-1 articulation could
become a source in a day-2 trace — is the motivating use case.

Both are provisional. Neither is committed. They are what M3's shadow points toward.
