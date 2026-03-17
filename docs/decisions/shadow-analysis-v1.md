# Decision Record: Shadow Analysis v1

**Date:** 2026-03-17
**Status:** Active
**Milestone:** M13 — Shadow analysis + observer-gap + ingestion deepening
**Packages:** `meshant/graph` (shadow.go, gaps.go), `meshant/loader` (draftchain.go), `meshant/schema` (tracedraft.go), `meshant/cmd/meshant` (shadow, gaps subcommands)
**Related:** `docs/decisions/articulation-v2.md`, `docs/decisions/tracedraft-v2.md`, `docs/decisions/equivalence-criterion-v1.md`

---

## What was decided

1. **Shadow is a cut decision, not missing data**
2. **ObserverGap takes pre-articulated graphs — the operation is composable, not re-articulating**
3. **FollowDraftChain mirrors FollowTranslation — derivation chains are first-class analytical objects**
4. **CriterionRef is citation metadata — does not affect validation, promotability, or promotion**
5. **DraftStepKind heuristics mirror StepKind — content change determines classification**
6. **shadow and gaps are CLI-first — articulation is done by the subcommand, not the caller**

---

## Context

M12 introduced the critique pipeline: re-articulation as a cut, DerivedFrom as positional vocabulary,
`cmdLineage` for reading the chain. But the analytical surface of the graph package remained primarily
about structure — what is there — rather than position — what is visible from here.

M13 opens the shadow as a first-class analytical object. Shadow analysis (`SummariseShadow`) makes
the cut's consequences visible: how many elements are shadowed, by what reason, from which positions.
ObserverGap (`AnalyseGaps`) compares two cuts directly, producing a three-way partition without
re-articulating.

Simultaneously, M13 deepens the ingestion pipeline: `FollowDraftChain` makes the DerivedFrom chain
traversable at the loader level (mirroring `FollowTranslation` at the graph level), and `CriterionRef`
names which EquivalenceCriterion governed a critique skeleton.

---

## Decision 1: Shadow is a cut decision, not missing data

`SummariseShadow` reads `MeshGraph.Cut.ShadowElements` — elements visible to excluded observers, not
included in the current articulation. These elements are not absent or unknown; they were deliberately
excluded when the cut was made.

All output language uses "shadowed" rather than "missing" or "absent". `ShadowSummary.ByReason` names
*why* elements were shadowed (observer position, tag filter, or time window) — three causes, each
corresponding to one of the three cut axes. `SeenFromCounts` names *who else* can see the shadowed
elements.

The test `TestSummariseShadow_ByReason` verifies that `ShadowReasonObserver` appears in `ByReason`
when elements are excluded by observer position — the most common shadow cause in the evacuation dataset.

This language commitment applies in `PrintShadowSummary`, `SummariseShadow`, and all documentation.
The word "missing" does not appear in shadow-analysis output or code comments.

---

## Decision 2: ObserverGap takes pre-articulated graphs

`AnalyseGaps(g1, g2 MeshGraph) ObserverGap` takes two already-articulated `MeshGraph` values.

This is the composability decision: the caller is responsible for articulation (including choosing
observer positions, time windows, and tag filters). `AnalyseGaps` only reads `g1.Nodes` and `g2.Nodes`
plus both `Cut` fields. It does not load traces, does not call `Articulate()`, and does not impose
a time window.

The alternative — `AnalyseGaps(traces, optsA, optsB)` — would be convenient but would hide
articulation inside the comparison function, making it undebuggable and non-composable. The caller
may want to inspect g1 and g2 independently before comparing them. Taking pre-articulated values
preserves that option.

`cmdGaps` does the articulation (two calls to `graph.Articulate`) before calling `AnalyseGaps`.
This is the correct place for it: the CLI is the convenient layer; the library function is the
composable layer.

Each `ObserverGap` retains both `Cut` fields (`CutA`, `CutB`) so the gap report is self-situated:
a comparison without its positions is uninterpretable. `PrintObserverGap` always names both positions.

---

## Decision 3: FollowDraftChain mirrors FollowTranslation

`FollowDraftChain(drafts []TraceDraft, from string) []TraceDraft` returns the chain of drafts in
derivation order starting from the draft with id `from`. It mirrors `FollowTranslation` in the
graph package: a traversal that produces an ordered sequence, stopping at leaves.

Implementation uses a children map (parent ID → []child IDs) and a visited set for cycle detection.
The first unvisited child at each step is followed; siblings beyond the first are not followed
(consistent with first-match branching in FollowTranslation). An empty slice is returned if `from`
is not found — not an error — because the caller may be probing membership before traversal.

`ClassifyDraftChain(chain []TraceDraft) []DraftStepClassification` applies heuristic classification
to each consecutive pair. The heuristics are:

- `DraftTranslation` — content fields changed AND ExtractionStage changed (framing + position)
- `DraftMediator` — content fields changed, stage unchanged (reformulated framing, same pipeline position)
- `DraftIntermediary` — only UncertaintyNote added (relayed faithfully with one annotation)

These mirror `StepKind` from the graph package (`StepTranslation`, `StepMediator`, `StepIntermediary`).
The heuristics are labelled v1 and acknowledged as provisional — the same framing used for
`ClassifyChain`.

---

## Decision 4: CriterionRef is citation metadata

`TraceDraft.CriterionRef string (json:"criterion_ref,omitempty")` names the EquivalenceCriterion
under which a draft was produced or reviewed. It carries the criterion's `Name` field as a string.

CriterionRef is deliberately metadata-only:
- `Validate()` does not require it
- `IsPromotable()` does not check it
- `Promote()` does not copy it to the canonical Trace (it belongs to the draft pipeline, not the
  canonical record)

The import-cycle prevention is structural: `schema` cannot import `graph`. Storing the criterion
`Name` as a string (not the `EquivalenceCriterion` struct) avoids a new dependency. The criterion
file is loaded in `cmd/meshant/main.go` (which already imports both packages) and its `Name` is
passed to `CriterionRef` as a plain string.

The `--criterion-file` flag on `cmdRearticulate` reuses `loadCriterionFile()` already defined for
`cmdFollow`. Setting `CriterionRef` on every skeleton makes critique skeletons self-situated: the
reader can trace which criterion governed the critique pass.

When `--criterion-file` is absent, `CriterionRef` is empty. Empty is correct: a skeleton produced
without a named criterion is honest about that absence.

---

## Decision 5: DraftStepKind heuristics are provisional (v1)

`DraftStepKind` has three values: `DraftIntermediary`, `DraftMediator`, `DraftTranslation`.
The heuristics that assign them are acknowledged as v1 and will be revisited when criteria are
applied to draft chains.

`classifyDraftStep` checks content fields (WhatChanged, Source, Target, Mediation, Observer, Tags)
and ExtractionStage changes. The ordering — Translation > Mediator > Intermediary — follows the
same priority logic as `classifyChainStep`: the most transformative kind wins when multiple
conditions are met.

`Reason` strings in `DraftStepClassification` are human-readable justifications of the heuristic,
not machine-parseable codes. This mirrors `StepClassification.Reason` in the graph package.

---

## Decision 6: shadow and gaps are CLI-first

`cmdShadow` and `cmdGaps` follow the existing CLI flag patterns exactly:
- `stringSliceFlag` for repeatable observer flags
- `parseTimeWindow` for RFC3339 time boundaries
- `outputWriter` / `confirmOutput` for optional file output
- No format flag (shadow and gap reports are text-only in v1)

Neither subcommand exposes a format flag in v1. The reports have natural tabular structure that
benefits from text rendering; JSON export of `ShadowSummary` and `ObserverGap` is deferred to a
future milestone if needed.

`cmdShadow` requires `--observer` (at least one). A shadow summary without a named observer
position would not be self-situated — the report would not name what the cut cannot see from where.
`cmdGaps` requires both `--observer-a` and `--observer-b`.

---

## What M13 does NOT do

- **Shadow mode for `cmdArticulate`**: shadow summary is a separate subcommand, not a mode flag on
  articulate; keeps each command focused on one operation
- **Interactive chain review**: `FollowDraftChain` is a library function; no CLI subcommand wraps
  it in v1 — the lineage reader (`cmdLineage`) already provides chain output
- **Criterion-driven draft classification**: `ClassifyDraftChain` uses v1 heuristics; criterion
  application to draft chains is deferred
- **Layer 3 comparison**: EquivalenceCriterion's comparison function (deferred since M10.5+) remains
  deferred; CriterionRef is annotation only
- **JSON export for shadow/gap**: text output only in v1

---

## Related

- `docs/decisions/articulation-v2.md` — observer position as primary cut axis; shadow mandatory
- `docs/decisions/translation-chain-v2.md` — FollowTranslation and ClassifyChain patterns mirrored
  by FollowDraftChain and ClassifyDraftChain
- `docs/decisions/equivalence-criterion-v1.md` — CriterionRef stores criterion.Name; import cycle
  prevention via string, not struct
- `docs/decisions/tracedraft-v2.md` — DerivedFrom as positional vocabulary; SourceSpan as invariant
- `docs/decisions/rearticulation-v1.md` — critique skeleton design; IntentionallyBlank (M12.5)
- `tasks/todo.md` — M13 section
