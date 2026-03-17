# Reflection — Decision Records and Milestone Plans

**Date:** 2026-03-17
**Scope:** Full read of all 13 decision records (staleness audit) + 12 milestone plan files (forward-looking review)

---

## What the audit found

Reading all decision records in sequence — from `trace-schema-v2.md` (M1) through
`rearticulation-v2.md` (M12) — revealed two categories of drift:

1. **Deferred items that were resolved but not marked** — at least 15 items across
   6 documents were listed as open when the resolution had already landed in code.
2. **Type signatures that diverged significantly from the final implementation** —
   primarily in `translation-chain-v2.md`, where the chain traversal design was
   substantially refactored before merge.

The divergence in `translation-chain-v2.md` is the most instructive case. The original
design embedded a `ChainStep` inside `ChainBreak` (to carry the alternative edge in full),
used a named `ChainBreakKind` type, and stored `Direction`, `Observer`, and `GraphID`
separately on `TranslationChain`. The final implementation moved to `AtElement string` +
`Reason string` (plain strings, not a named type) and replaced the scalar fields with the
full `Cut` struct. This is a richer, more consistent design — but the decision record
described the discarded design.

---

## Design insights from reading the records in sequence

### 1. The shadow philosophy extended further than the code

The chain's `branch-not-taken` break mechanism (Decision 6, translation-chain-v2.md)
is explicitly modeled on the shadow element pattern from articulation (Decision 2,
articulation-v2.md): *named absence is methodologically significant*. But the decision
record never draws the conclusion fully: every `ChainBreak` is, in ANT terms, a shadow of
the chain — the path that was present but not followed.

**Extension idea:** A future `SummariseBranchNotTaken()` function could produce a
`BranchSummary` parallel to `MeshSummary` — counting how often each element appears as
a branch-not-taken target across a dataset. This would answer: "Which actants are
systematically excluded from the chains analysts follow?" Elements that frequently appear
as branch-not-taken shadows may be more analytically significant than the paths that were
followed.

### 2. `ClassifyOptions.Criterion` is provenance, not a cut — a meaningful asymmetry

From `graph-as-actor-v2.md` (Decision 3): the `Cut` struct is stored verbatim on
`MeshGraph` because it situates the articulation. The equivalent in classification would
be `ClassifyOptions.Criterion` situating the classification pass.

The current design (Decision 9, `translation-chain-v2.md`) marks `Criterion` as
"envelope metadata only — does not alter v1 step heuristics." This is epistemologically
honest: the criterion declares the analyst's interpretive frame without retroactively
changing what edges say. But it creates a productive tension: if the criterion names what
counts as a translation boundary ("only juridical–scientific crossings count"), should
steps that don't meet that criterion be reclassified as `StepIntermediary` rather than
`StepMediator`?

**Extension idea (M13+):** `ClassifyOptions.CriterionMode: enum(envelope|filter)`. In
`filter` mode, the criterion conditions step classification — a mediation that does not
satisfy the criterion is classified as `StepIntermediary` even if `Edge.Mediation` is
non-empty. In `envelope` mode (current default), the criterion is stored for provenance
only. This would make the criterion analytically active rather than decorative.

### 3. The `GraphRef` + `TranslationChain.Cut` gap

From `graph-as-actor-v2.md` (Decision 8): `MeshSummary.GraphRefs` collects
graph-reference strings in encounter order. From `translation-chain-v2.md` (Decision 1):
`TranslationChain.Cut` situates the chain within its articulation.

But `TranslationChain` has no `ID` field. A chain cannot yet appear as an actant in traces
— it cannot be cited, referenced, or used as a source or target. This means the moment
when a chain influenced a decision ("we articulated the supply-chain graph and followed the
translation chain to X, which triggered policy Y") is not currently recordable in MeshAnt's
own vocabulary.

**Extension idea:** `IdentifyChain(c TranslationChain) TranslationChain` (following the
`IdentifyGraph` pattern from M5). Chain-reference string format: `"meshchain:<uuid>"`. The
`ChainRef(c TranslationChain) (string, error)` function would then allow chains to appear
in `Source`/`Target` slices — consistent with generalised symmetry.

The decision record (`translation-chain-v2.md`, "What M10.5 explicitly defers") already
names this as "chain-as-actor: a `TranslationChain` does not receive an ID and cannot
appear in traces." The infrastructure from M5 is ready to extend.

### 4. `DerivedFrom` as the missing link between ingestion and articulation

From `tracedraft-v2.md` (Decision 5): `DerivedFrom` links drafts in a derivation chain.
From `rearticulation-v2.md`: `cmdRearticulate` builds skeleton critique drafts with
`DerivedFrom` set. The decision record explicitly frames re-articulation as a *second cut*
on the extraction chain.

But the decision records never connect this back to `FollowTranslation`. A derivation
chain is structurally equivalent to a translation chain: each node is a draft, each link
is a derivation step, each step has a potential mediation (did the critic transform the
original?). The machinery exists; it has not been assembled.

**Extension idea:** A `DraftChainOptions` and `FollowDraftChain()` function in
`meshant/loader` that traverses `DerivedFrom` links and produces a `[]TraceDraft` chain
in derivation order — analogous to `FollowTranslation` but operating on drafts, not graph
edges. `ClassifyDraftChain()` could then classify each step: did the critique step produce
an intermediary revision (same content, added `uncertainty_note`) or a mediator revision
(reformulated actor framing)?

This would make the critique pipeline *analytically traceable* in MeshAnt's own vocabulary
rather than an external pipeline that feeds data in.

### 5. The observer-position gap in the ingestion pipeline

From `articulation-v2.md` (Decision 1): the observer position is the primary cut axis
because it asks *who is doing the seeing*, not just *what is being selected*.

`TraceDraft` has `ExtractedBy string` — who produced the draft. But the canonical `Trace`
that emerges from `Promote()` has `Observer string` — who *observed* the event being
traced. These are not the same. A draft extracted by `"llm-pass1"` may record an event
observed by `"audit-team"`. The extraction chain collapses this distinction.

**Design tension (not an extension idea):** MeshAnt's ingestion pipeline currently treats
`ExtractedBy` and `Observer` as parallel provenance fields without confronting their
different ANT roles. `ExtractedBy` names the inscription device (the LLM, the human
analyst). `Observer` names the actant in the mesh who witnessed the event. Conflating or
ignoring the gap risks producing traces whose provenance is ambiguous at the point of
articulation. This tension warrants a future decision record.

### 6. The "sorted alphabetically" decision and temporal meaning

Multiple decision records (graph-diff-v2.md Decision 8, graph-as-actor-v2.md Decision 8,
articulation-v2.md) choose alphabetical sort for stable test assertions and to avoid
implying rank. But `MeshSummary.Mediations` and `GraphRefs` are encounter-order for
explicit ANT reasons: "the first appearance marks when the actant entered the mesh."

**Insight:** The two sort disciplines encode two different analytical questions:
- *Who/what is present?* (alphabetical: comparable, ranked-free)
- *When did something become significant?* (encounter order: temporal, sequenced)

These could be made more explicit in a future `ArticulationOptions.SortMode` or in the
`MeshSummary` documentation. The current state is correct but the distinction is not
surfaced to callers.

---

## Insights from milestone plan review

Reading all 12 plan files (M1–M12) surfaced additional findings not visible from the
decision records alone. Plans capture design thinking before it hardens into decisions,
and contain tensions and deferred items that were never escalated to formal records.

### 7. Temporal visibility in shadow is permanently incomplete

`ShadowElement.SeenFrom` records which observer positions would make an element visible —
but not *when*. The plan_m3.md explicitly deferred "SeenInWindow" and subsequent milestones
never returned to it. The gap is real: given a shadowed element, you cannot ask "in which
time window does this element become visible again?" from the data alone.

This is deeper than a missing field. A shadow element may be invisible from a given
observer because they were absent during the relevant window — so observer and time reasons
are entangled. The current `ShadowReason` enum (`observer`, `time-window`, `tag-filter`)
records *which axis caused the shadow* but not *what the boundary looks like*.

**Extension idea:** `ShadowElement.VisibleWindow TimeWindow` — the minimal time interval
in which this element becomes visible from the current cut's observer positions. Would
require a secondary pass over the excluded traces. Answers the analyst's natural question:
"if I expand my time window, when does this shadow lift?"

### 8. Identified graphs are invisible in printed output

`IdentifyGraph` and `IdentifyDiff` assign stable IDs (deferred item from `graph-as-actor-v2.md`
Decision 5: "PrintArticulation and PrintDiff do not output the graph's own ID when non-empty").
This deferral was never lifted. A consumer reading `PrintArticulation` output cannot see
whether the graph has been identified, what its ID is, or whether it is available as a
`meshgraph:<uuid>` reference.

This breaks the immutable-mobile principle: the graph travels as a printed document but
loses its identity in transit. A reader of the output cannot cite the graph in a subsequent
trace without going back to code.

**Extension idea:** A one-line ID header in `PrintArticulation` / `PrintDiff` output when
`ID` is non-empty: `Graph ID: meshgraph:<uuid>`. Optional flag `--id` on `cmdArticulate`
could trigger `IdentifyGraph` automatically and print the reference. Makes the graph
citable directly from its own printed output.

### 9. Chain traversal is sensitive to dataset load order

`FollowTranslation` uses first-match branching — "first by dataset encounter order." The
encounter order is determined by the order edges appear in `MeshGraph.Edges`, which
reflects the order traces were loaded. If two traces are loaded in a different sequence
(e.g., after a dataset edit), the chain changes.

This is not documented as a limitation in any decision record or output. A user who edits
`traces.json` (adding a trace between two existing ones) will silently get a different
chain result without any warning.

**Oversight (medium severity):** The decision record (`translation-chain-v2.md`, Decision 4)
says "encounter order preserves the observational sequence" as a methodological
justification, but this conflates *load order* with *observation order*. A future dataset
reordering (for editorial reasons) is not a change in observation order, yet it changes
chains. Should be documented explicitly as a known reproducibility caveat.

### 10. TraceDraft has no structural signal for "deliberately blank"

`TraceDraft` fields are optional. A blank `WhatChanged` could mean:
(a) the extractor didn't find a candidate, or
(b) the analyst deliberately chose to leave it blank as an epistemic commitment.

Only `UncertaintyNote` can disambiguate — but it's optional and free-form. There is no
structural way to ask "was this field intentionally left blank, or simply not extracted?"

This matters for the critique pass: `cmdRearticulate` produces skeletons with blank content
fields. A downstream consumer cannot distinguish a critique skeleton from an unfinished
extraction without reading the `ExtractionStage` field, which is also optional.

**Design tension:** `plan_m11.md` acknowledged this as a design choice ("empty preferred
over fabricated") but did not provide a structural mechanism for the analyst to record *why*
a field is blank. A minimal fix: a `BlankFields []string` field (or `IntentionallyBlank
[]string`) on `TraceDraft` — a list of field names that were deliberately not filled,
distinct from fields that simply weren't attempted.

### 11. EquivalenceCriterion and the critique pass are orthogonal — a missed integration

`EquivalenceCriterion` (M10.5+) declares what the analyst considers preserved or altered
across a passage. The critique pass (M12) declares what the analyst considers over-actorized
or under-specified in a draft.

These are the same analytical gesture applied to different objects: one to graph edges
(does this edge preserve what I care about?), the other to extraction drafts (does this
draft reflect what I care about?). They were developed in parallel milestones and never
connected.

A critique skeleton produced by `cmdRearticulate` has no field for declaring an
equivalence criterion. An analyst running a critique pass operates without any structured
declaration of their interpretive frame — which undercuts the criterion's purpose of making
interpretation visible.

**Extension idea:** `CriterionRef string` field on `TraceDraft` (optionally set during
`cmdRearticulate`). Value could be a path to a `.criterion.json` file or an inline
`EquivalenceCriterion` struct. This would make the critique pass self-situated: the
skeleton knows under what interpretive conditions it was generated.

### 12. "From whose interests" is not a cut axis

`articulation-v2.md` Decision 1 frames observer position as "who is doing the seeing"
and time window as "from when." But Haraway's situated knowledge goes further: *from whose
interests* and *with what framing* shapes what is seen.

The current cut axes (observer, time, tags) record *positions* and *categories* but not
*agendas* or *framings*. Two observers in the same position at the same time with the same
tags but different professional frames (an economist and an ecologist) would produce
identical cuts in MeshAnt.

This is not easily fixable at the schema level — but it surfaces in the `ExtractionStage`
and `ExtractedBy` pattern. The LLM's framing (what it was trained on, what the prompt
emphasizes) shapes extraction, but the resulting `TraceDraft` carries only
`extracted_by: "llm-pass1"`, not any characterization of the LLM's analytical frame.

**Design tension (no concrete fix):** The framework currently has no vocabulary for
"interpretive frame" distinct from "observer position." The equivalence criterion is the
closest approximation — it declares what the analyst considers worth preserving — but it
operates on edges, not on the articulation cut itself. Long-term, a `FramingDeclaration`
parallel to `EquivalenceCriterion` might situate articulations more fully.

### 13. Multi-step diff chains remain unaddressed since M4

`graph-diff-v2.md` Decision 11 deferred "multi-step diff chains" as still open. No
subsequent milestone has addressed this. A user cannot currently ask "how did the graph
evolve across three time periods?" — they can only compare two cuts pairwise.

The infrastructure supports this: `GraphDiff` carries `From` and `To` cuts, and `GraphDiff`
can itself be identified as an actor (M5). A chain of diffs would be: `Diff(g1, g2)`,
`Diff(g2, g3)`, then comparing the two diffs — showing how the *rate or pattern of change*
evolved.

**Extension idea:** `DiffChain(diffs []GraphDiff) DiffEvolution` — a type analogous to
`TranslationChain` but for sequential diffs. `PrintDiffEvolution` would show: which nodes
persisted across all cuts, which shadow shifts were stable vs. ephemeral, and where the
boundary of visibility was most volatile. This is the "graph evolution timeline" that M4
explicitly deferred.

---

## Summary table: open extension ideas

| # | Idea | Prerequisite | Complexity | ANT grounding |
|---|------|-------------|-----------|---------------|
| 1 | `SummariseBranchNotTaken()` | None | Low | Shadow philosophy; named absence |
| 2 | `ClassifyOptions.CriterionMode` (filter mode) | M10.5+ Criterion | Medium | Criterion as active cut, not just provenance |
| 3 | `IdentifyChain` / chain-as-actor (`meshchain:<uuid>`) | M5 infrastructure | Low | Generalised symmetry; direct M5 extension |
| 4 | `FollowDraftChain` / `ClassifyDraftChain` | M11 `DerivedFrom` | Medium | Ingestion chain as translation chain |
| 5 | `ExtractedBy` vs `Observer` distinction | Conceptual | Design only | Observer position = primary cut axis |
| 6 | `ShadowElement.VisibleWindow` (temporal shadow lifting) | M3 shadow structure | Medium | Strathern: shadow has its own structure |
| 7 | Graph ID in `PrintArticulation` / `PrintDiff` header | M5 `IdentifyGraph` | Low | Immutable mobile must carry its identity |
| 8 | `IntentionallyBlank []string` on `TraceDraft` | M11 `TraceDraft` | Low | Blank as epistemological claim, not absence |
| 9 | `CriterionRef` on `TraceDraft` / critique skeletons | M10.5+ Criterion + M12 | Low | Critique pass must be self-situated |
| 10 | `DiffChain` / diff evolution timeline | M4 `GraphDiff` + M5 identity | Medium | Latour: associations change over time |

---

## What the records do well

The decision records form a coherent design history. The methodological grounding
(Latour, Strathern, Haraway, Principle 8) is consistently applied and explains *why*
decisions were made, not just what. The "explicitly defers" sections at the end of each
record are particularly valuable — they name what was not done and often forecast exactly
what the next milestone will address.

The `translation-chain-v2.md` record demonstrates the best practice: it documents not just
the accepted design but a significant design revision (the shift from embedded `ChainStep`
in `ChainBreak` to plain `AtElement`+`Reason`). After the type signature corrections in
this audit, the record is a reliable guide to the implementation.
