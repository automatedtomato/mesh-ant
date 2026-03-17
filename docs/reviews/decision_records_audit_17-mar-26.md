# Decision Records Audit — Insights and Extension Ideas

**Date:** 2026-03-17
**Scope:** Full read of all 13 decision records, staleness resolution, and synthesis

---

## What the audit found

Reading all decision records in sequence — from `trace-schema-v1.md` (M1) through
`rearticulation-v1.md` (M12) — revealed two categories of drift:

1. **Deferred items that were resolved but not marked** — at least 15 items across
   6 documents were listed as open when the resolution had already landed in code.
2. **Type signatures that diverged significantly from the final implementation** —
   primarily in `translation-chain-v1.md`, where the chain traversal design was
   substantially refactored before merge.

The divergence in `translation-chain-v1.md` is the most instructive case. The original
design embedded a `ChainStep` inside `ChainBreak` (to carry the alternative edge in full),
used a named `ChainBreakKind` type, and stored `Direction`, `Observer`, and `GraphID`
separately on `TranslationChain`. The final implementation moved to `AtElement string` +
`Reason string` (plain strings, not a named type) and replaced the scalar fields with the
full `Cut` struct. This is a richer, more consistent design — but the decision record
described the discarded design.

---

## Design insights from reading the records in sequence

### 1. The shadow philosophy extended further than the code

The chain's `branch-not-taken` break mechanism (Decision 6, translation-chain-v1.md)
is explicitly modeled on the shadow element pattern from articulation (Decision 2,
articulation-v1.md): *named absence is methodologically significant*. But the decision
record never draws the conclusion fully: every `ChainBreak` is, in ANT terms, a shadow of
the chain — the path that was present but not followed.

**Extension idea:** A future `SummariseBranchNotTaken()` function could produce a
`BranchSummary` parallel to `MeshSummary` — counting how often each element appears as
a branch-not-taken target across a dataset. This would answer: "Which actants are
systematically excluded from the chains analysts follow?" Elements that frequently appear
as branch-not-taken shadows may be more analytically significant than the paths that were
followed.

### 2. `ClassifyOptions.Criterion` is provenance, not a cut — a meaningful asymmetry

From `graph-as-actor-v1.md` (Decision 3): the `Cut` struct is stored verbatim on
`MeshGraph` because it situates the articulation. The equivalent in classification would
be `ClassifyOptions.Criterion` situating the classification pass.

The current design (Decision 9, `translation-chain-v1.md`) marks `Criterion` as
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

From `graph-as-actor-v1.md` (Decision 8): `MeshSummary.GraphRefs` collects
graph-reference strings in encounter order. From `translation-chain-v1.md` (Decision 1):
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

The decision record (`translation-chain-v1.md`, "What M10.5 explicitly defers") already
names this as "chain-as-actor: a `TranslationChain` does not receive an ID and cannot
appear in traces." The infrastructure from M5 is ready to extend.

### 4. `DerivedFrom` as the missing link between ingestion and articulation

From `tracedraft-v1.md` (Decision 5): `DerivedFrom` links drafts in a derivation chain.
From `rearticulation-v1.md`: `cmdRearticulate` builds skeleton critique drafts with
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

From `articulation-v1.md` (Decision 1): the observer position is the primary cut axis
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

Multiple decision records (graph-diff-v1.md Decision 8, graph-as-actor-v1.md Decision 8,
articulation-v1.md) choose alphabetical sort for stable test assertions and to avoid
implying rank. But `MeshSummary.Mediations` and `GraphRefs` are encounter-order for
explicit ANT reasons: "the first appearance marks when the actant entered the mesh."

**Insight:** The two sort disciplines encode two different analytical questions:
- *Who/what is present?* (alphabetical: comparable, ranked-free)
- *When did something become significant?* (encounter order: temporal, sequenced)

These could be made more explicit in a future `ArticulationOptions.SortMode` or in the
`MeshSummary` documentation. The current state is correct but the distinction is not
surfaced to callers.

---

## Summary table: open extension ideas

| Idea | Prerequisite | Complexity | ANT grounding |
|------|-------------|-----------|---------------|
| `SummariseBranchNotTaken()` | None | Low | Shadow philosophy; named absence |
| `ClassifyOptions.CriterionMode` (filter mode) | M10.5+ Criterion already exists | Medium | Criterion as active cut, not just provenance |
| `IdentifyChain` / chain-as-actor | M5 infrastructure ready | Low | Generalised symmetry; M5 pattern |
| `FollowDraftChain` / `ClassifyDraftChain` | M11 DerivedFrom ready | Medium | Ingestion chain as translation chain |
| ExtractedBy vs Observer distinction | Conceptual only | Design | Observer position = primary cut axis |

---

## What the records do well

The decision records form a coherent design history. The methodological grounding
(Latour, Strathern, Haraway, Principle 8) is consistently applied and explains *why*
decisions were made, not just what. The "explicitly defers" sections at the end of each
record are particularly valuable — they name what was not done and often forecast exactly
what the next milestone will address.

The `translation-chain-v1.md` record demonstrates the best practice: it documents not just
the accepted design but a significant design revision (the shift from embedded `ChainStep`
in `ChainBreak` to plain `AtElement`+`Reason`). After the type signature corrections in
this audit, the record is a reliable guide to the implementation.
