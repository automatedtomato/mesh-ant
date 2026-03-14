# Plan: M10.5+ — Equivalence Criterion (Classification with Grounds)

**Date**: 2026-03-13
**Status**: Draft — awaiting user confirmation
**Source**: Design dialogue (2026-03-13), `docs/reviews/notes_on_mediator.md`,
`docs/reviews/equivalence_criterion_design_note.md`

---

## Problem

M10.5 introduced chain traversal and step classification. But the v1
classification heuristics outsource the mediator/intermediary judgment to
the trace author (Mediation field presence). The framework adds no
analytical value beyond reading a field.

What is missing: the **equivalence criterion** — an explicit declaration
of what counts as preserved, altered, or consequential across a passage.
Without it, classification is mechanical, not situated.

---

## Goal

Allow MeshAnt to **declare and carry the conditions of a reading**
before it tries to compute them. The classification output should name
its criterion, acknowledge it is conditional, and make the grounds
inspectable.

This is Layers 1–2 of the three-layer design. Layer 3 (comparison
function) is explicitly deferred.

---

## Non-goals

- Automatic classification based on the criterion (Layer 3)
- Comparing two readings under different criteria
- Reflexive criterion-tracing (recording the criterion as a trace)
- Shadow-criterion interaction analysis

---

## Phases

### Phase 1: Define EquivalenceCriterion type

**File**: `meshant/graph/criterion.go` (new)

```go
// EquivalenceCriterion declares the conditions under which a chain
// reading is being conducted. It is an interpretive object, not a
// computational one. The criterion governs any future comparison
// function — the function must not define the criterion.
type EquivalenceCriterion struct {
    // Name is a short identifier for this criterion.
    Name string

    // Declaration is the interpretive criterion in natural language.
    // This is the primary layer (Layer 1) — the grounds for the reading.
    // Without it, the other fields have no grounding.
    Declaration string

    // Preserve is an optional list of aspects treated as
    // continuity-bearing under this criterion (Layer 2).
    Preserve []string

    // Ignore is an optional list of aspects treated as irrelevant
    // to equivalence under this criterion (Layer 2).
    Ignore []string
}
```

Add `IsZero() bool` method (all fields empty → zero value).

**Tests**: `meshant/graph/criterion_test.go`
- Zero value detection
- Non-zero with only Name
- Non-zero with Declaration
- Preserve/Ignore populated

### Phase 2: Wire criterion into classification

**Files**: `meshant/graph/classify.go`, `meshant/graph/chain_print.go`

Changes:
1. Add `Criterion EquivalenceCriterion` to `ClassifyOptions`
2. Add `Criterion EquivalenceCriterion` to `ClassifiedChain`
3. When `ClassifyOptions.Criterion` is non-zero:
   - Copy criterion into `ClassifiedChain.Criterion`
   - Prepend criterion name to each `StepClassification.Reason`
     (e.g., `"[operational-meaning] no mediation observed — ..."`)
   - Classification logic remains v1 heuristics (Layer 3 is deferred)
4. When criterion is zero: existing v1 behaviour unchanged
5. `PrintChain` renders criterion block when non-zero:
   ```
   Criterion: operational-meaning
   Declaration: Preserve operational meaning, ignore representational variation
   Preserve: [target, obligation_level]
   Ignore: [display_format, wording]
   ```
6. `PrintChainJSON` includes criterion in JSON envelope when non-zero

**Tests**: `meshant/graph/classify_test.go`, `meshant/graph/chain_print_test.go`
- Existing tests pass unchanged (zero criterion = v1)
- New test: criterion carried through classification
- New test: reason includes criterion name
- New test: two different criteria on same chain → same classification
  but different reasons (v1 logic unchanged; criterion is carried, not
  yet used for classification)
- New print test: criterion rendered in text output
- New print test: criterion in JSON envelope

### Phase 3: CLI support

**File**: `meshant/cmd/meshant/main.go`

Add flags to `follow` subcommand:
- `--criterion-name` — short identifier
- `--criterion-declaration` — natural-language declaration
- `--criterion-preserve` — repeatable, aspects to preserve (Layer 2)
- `--criterion-ignore` — repeatable, aspects to ignore (Layer 2)

Alternative (simpler, possibly better):
- `--criterion-file <path>` — reads a JSON file containing an
  `EquivalenceCriterion` object

Both approaches should be considered. The file approach is simpler for
complex criteria; the flag approach is better for quick one-liners.
Recommend implementing both.

**Tests**: `meshant/cmd/meshant/main_test.go`
- Follow with criterion flags → criterion in output
- Follow with criterion file → criterion in output
- Follow without criterion → v1 (existing tests unchanged)
- Invalid criterion file → error

### Phase 4: Decision record + codemap

- `docs/decisions/equivalence-criterion-v1.md`
- `docs/CODEMAPS/meshant.md` updated

---

## Key design rules

1. **The criterion governs the function, never the reverse.** If a
   future comparison function disagrees with the interpretive criterion,
   the interpretive criterion governs.

2. **Zero value = v1 heuristics.** All existing code paths remain
   unchanged when no criterion is provided. No breaking changes.

3. **The criterion is carried, not yet computed.** This milestone
   attaches the criterion as an interpretive object. It does not change
   classification logic. That is a separate, later milestone.

4. **Layer order is strict.** Layer 1 (declaration) must exist before
   Layer 2 (preserve/ignore) has meaning. Layer 2 must exist before
   Layer 3 (comparison function) can be grounded.

---

## What this enables (future milestones, not planned here)

- **Criterion-aware classification**: Layer 3 — a comparison function
  that uses Preserve/Ignore to judge steps differently from v1.
- **Multi-criterion comparison**: showing the same chain under two
  different criteria and comparing the readings.
- **Criterion as trace**: recording the analyst's criterion choice as
  a trace in the mesh (reflexive criterion-tracing).
- **Shadow-criterion interaction**: how shadow elements behave under
  different equivalence criteria.

---

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Criterion becomes decorative (carried but never used) | Low | Acknowledged — this is intentional for this milestone. Layer 3 is a separate step. |
| Over-engineering the struct before real usage | Medium | Keep struct minimal (4 fields). Only add fields when real criteria demand them. |
| CLI flag proliferation | Low | `--criterion-file` absorbs complexity; flags are for quick use. |
| Confusion between "criterion carried" and "criterion applied" | Medium | Document clearly in code, tests, and decision record. |

---

## Estimated scope

- Phase 1: small (new file, ~50 lines + tests)
- Phase 2: medium (modify 2 files, ~80 lines changes + tests)
- Phase 3: medium (CLI flags + file parsing + tests)
- Phase 4: docs only

Total: moderate scope, conceptually significant.
