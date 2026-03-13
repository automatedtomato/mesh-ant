# Design Note: Equivalence Criterion in MeshAnt

**Date**: 2026-03-13
**Status**: Design draft — not yet implemented
**Companion to**: `docs/reviews/notes_on_mediator.md`

---

## What problem this addresses

MeshAnt can follow a translation chain through an articulated graph
(`FollowTranslation`) and classify each step as intermediary-like,
mediator-like, or translation (`ClassifyChain`). But the current
classification uses only the presence or absence of the `Mediation`
field on the edge — a user-authored property of the trace.

This means:

- The classification outsources the judgment to whoever wrote the trace.
- The framework adds no analytical value beyond reading a field.
- The distinction between mediator and intermediary is not situated —
  it does not depend on how the chain is being read.

What is missing is the **equivalence criterion**: an explicit statement
of what counts as preserved, altered, or consequential across a passage.

---

## What an equivalence criterion is

An equivalence criterion is a **declaration of how a chain reading is
being conducted**. It answers:

- What is being treated as preserved across a step?
- What kinds of change are being ignored?
- What kinds of change are being treated as consequential?
- What level of continuity or discontinuity matters under this reading?

An equivalence criterion is not a measurement. It is an interpretive
act — the analyst declares the conditions under which something counts
as "the same" or "different enough to matter."

This is consistent with MeshAnt's design orientation: traces before
actors, articulation before ontology, cuts before essence. The
equivalence criterion extends this principle to classification itself:
**readings before classifications**.

---

## How it relates to cuts

MeshAnt's existing cut axes are:

| Axis | What it selects | Introduced in |
|------|-----------------|---------------|
| Observer position | Which traces are included | M2 |
| Time window | Which temporal range is included | M3 |
| Tags | Which tag-bearing traces are included | M10 |

The equivalence criterion is not another trace-selection axis. It
operates at a different level: it governs **how the included traces
are read**, not which traces are included.

A cut selects what is visible.
An equivalence criterion declares what counts as continuity within
what is visible.

Together they constitute the full conditions of a reading:

- **Cut**: observer + time + tags → what is seen
- **Criterion**: equivalence + relevance → how what is seen is interpreted

---

## How it relates to mediator / intermediary readings

Under the current v1 heuristics:

- Intermediary = no `Mediation` field → "nothing was observed to transform"
- Mediator = `Mediation` field present → "something transformed"
- Translation = `Mediation` + `"translation"` tag → "regime boundary crossed"

These heuristics are derivative of the trace author's choices. They
do not constitute an analytical reading.

Under an equivalence criterion, the classification would become:

- **Intermediary-like**: under criterion E, what entered this step
  and what exited it are treated as equivalent. The step preserved
  what matters (according to E).

- **Mediator-like**: under criterion E, what entered and what exited
  are not equivalent. Something was altered, redirected, or deformed
  in a way that matters (according to E).

- **Translation**: under criterion E, not only is equivalence broken,
  but the operational regime itself has changed — what counts, who
  acts, or how alignment is maintained has been reformulated.

The classification is now explicitly conditional on the criterion.
A different criterion applied to the same chain may yield a different
classification. This is not a bug — it is what MeshAnt should be
able to show.

---

## How it differs from automatic classification

Automatic classification computes a result and presents it as a fact.
The grounds are hidden in the algorithm.

Classification with grounds does something different:

1. **Names the criterion** — what is being compared, under what
   assumptions of equivalence
2. **Produces a reading** — intermediary-like / mediator-like /
   translation, relative to that criterion
3. **Acknowledges alternatives** — a different criterion would yield
   a different reading
4. **Exposes the conditions** — the cut, the criterion, and their
   interaction are all part of the output

The framework does not claim to know what something "really is." It
claims to show what something looks like under explicitly stated
conditions. This is the difference between a god's-eye classification
and a situated reading.

---

## Three layers of the equivalence criterion

The criterion has three layers. The order is strict: each layer
depends on the one above it, not the reverse.

### Layer 1: Interpretive criterion (curatorial declaration)

A human-declared reading condition expressed in natural language.

Examples:
- "Format changes do not matter"
- "Institutional form matters"
- "Preserve operational meaning, ignore representational variation"
- "What matters is whether the obligation structure changed"

This is the primary layer. Without it, the other layers have no
grounding. The interpretive criterion is what makes the reading
situated rather than mechanical.

### Layer 2: Operational register (structured indication)

A structured indication of which aspects are treated as
continuity-bearing under the criterion.

Example shape:
```
preserve: [target, obligation_level]
ignore:   [display_format, wording]
```

This layer translates the interpretive criterion into something
the framework can potentially act on, while remaining explicit
about what is being preserved and what is being ignored.

### Layer 3: Comparison function (optional, executable)

An optional function that approximates the criterion computationally.

```
same_under_criterion(step, criterion) → bool or score
```

**Critical design rule**: the function serves the criterion. The
criterion does not derive from the function. If the function and the
interpretive criterion disagree, the interpretive criterion governs.

---

## Minimal data structure

A first sketch. This is intentionally small — it should be able to
exist as a carried object on a `ClassifiedChain` without requiring
any automated comparison logic.

```
EquivalenceCriterion {
    // Name is a short identifier for this criterion.
    Name        string

    // Declaration is the interpretive criterion in natural language.
    // This is the primary layer — the grounds for the reading.
    Declaration string

    // Preserve is an optional list of aspects treated as
    // continuity-bearing under this criterion (Layer 2).
    Preserve    []string

    // Ignore is an optional list of aspects treated as irrelevant
    // to equivalence under this criterion (Layer 2).
    Ignore      []string
}
```

No comparison function at this stage. The criterion exists as an
interpretive object, not a computational one.

---

## What this means for ClassifyChain

The current `ClassifyOptions{}` is an empty struct designed as an
extension point. The equivalence criterion is what fills it:

```
ClassifyOptions {
    Criterion EquivalenceCriterion  // zero = v1 heuristics (provisional)
}
```

When `Criterion` is zero-value, `ClassifyChain` falls back to the
v1 heuristics (Mediation field presence). When a criterion is
provided, the classification should:

1. Carry the criterion in the output (`ClassifiedChain.Criterion`)
2. Reference the criterion in each `StepClassification.Reason`
3. Acknowledge that the reading is conditional on the criterion

The actual classification logic when a criterion is present is a
separate design question. The first step is simply: **allow the
criterion to be declared and carried**.

---

## Recommended implementation path

1. **Keep v1 stable** — current heuristics remain as the default
   fallback when no criterion is provided.

2. **Define `EquivalenceCriterion`** — the struct above, in
   `meshant/graph/classify.go` or a new file.

3. **Add `Criterion` to `ClassifyOptions`** — zero value means
   "use v1 heuristics."

4. **Carry the criterion through the output** — `ClassifiedChain`
   gains a `Criterion EquivalenceCriterion` field. `PrintChain`
   renders it.

5. **CLI support** — `--criterion-name` and `--criterion-declaration`
   flags on the `follow` subcommand. Or a `--criterion-file` that
   reads a JSON criterion object.

6. **Only later**: operationalise the `Preserve`/`Ignore` fields
   into actual comparison logic. This is Layer 2→3 and should not
   be rushed.

---

## What this does not address

- How to define comparison functions (Layer 3) — deferred.
- How to compare two readings under different criteria — deferred.
- Whether the criterion should itself be recorded as a trace
  (reflexive criterion-tracing) — conceptually natural but deferred.
- How equivalence criteria interact with shadow analysis — deferred.

---

## Relation to MeshAnt's design principles

- **Principle 1 (follow the traces)**: the criterion is itself a
  trace of how the analyst chose to read the network.
- **Principle 3 (mediation before intention)**: the criterion governs
  how mediation is read, not what mediation "is."
- **Principle 5 (plural observers)**: different criteria are plural
  readings of the same chain — no one is privileged.
- **Principle 8 (the analyst is inside)**: the criterion makes the
  analyst's interpretive stance explicit, not hidden.
