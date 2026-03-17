# Plan: M10.5+ — Equivalence Criterion (Classification with Grounds)

**Date**: 2026-03-13
**Status**: Confirmed — reviewed by ant-theorist, architect, philosophical reviewer; user-reviewed 2026-03-13
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
- Per-step criteria (uniform criterion across chain is a known
  simplification — see review notes). This milestone is intentionally
  **pre-translation-aware**: a single criterion is applied uniformly
  across the entire chain, even where translation steps cross regime
  boundaries. Translation was previously identified as a regime-shifting
  operation; a criterion that spans such a crossing may not be coherent.
  This is a deliberate limitation of the current design, not the final
  conceptual model.

---

## Review findings incorporated

Three-reviewer process (ant-theorist, architect, philosophical reviewer)
produced these binding resolutions:

### C1: Do NOT prepend criterion name to Reason strings

The criterion belongs on the `ClassifiedChain` envelope, not grafted
onto individual step reasons. v1 heuristics produce the reasons; the
criterion did not govern them. Rendering composes criterion context in
`PrintChain`/`PrintChainJSON`, not in `ClassifyChain`.

### C2: Name the v1 implicit criterion (decision record)

The zero-value fallback is not "no criterion." It is the implicit
criterion: "trust trace-author Mediation annotation." Name this in the
decision record.

### C3: Ignore is a second-order shadow

The `Ignore` field creates a shadow of *aspects*, not elements. Add a
doc comment acknowledging this. Future milestone: extend shadow apparatus
to criterion-excluded aspects.

### C4: Name is a handle, not an identity

A declaration-only criterion (no name) is valid and encouraged as the
primary mode. Name is a convenience for repeated use. `Declaration` is
the grounding layer (Layer 1).

A name-only criterion (no declaration) is structurally valid (non-zero,
passes `Validate()`) but analytically weak — it is a handle without
grounds. It must not quietly become the normal authored form. Code
comments and the decision record should document this: name-only
criteria are accepted as transport handles, but the conceptually strong
usage always includes a Declaration.

### A1: Add Validate() method

Error if Preserve/Ignore non-empty but Declaration empty. Enforces
layer ordering at the type boundary. Follows existing `Trace.Validate()`
and `TimeWindow.Validate()` pattern.

### A2: Omit criterion from JSON when zero

Use conditional inclusion in `chainJSONEnvelope` to avoid noisy empty
fields in v1 output.

### A3: Implement --criterion-file first

Defer individual `--criterion-*` flags to reduce flag proliferation.
Add them later if real usage demands quick one-liners.

---

## Phases

### Phase 1: Define EquivalenceCriterion type

**File**: `meshant/graph/criterion.go` (new)

```go
// EquivalenceCriterion declares the conditions under which a chain
// reading is being conducted. It is an interpretive object, not a
// computational one. The criterion governs any future comparison
// function — the function must not define the criterion.
//
// The act of declaring a criterion is itself a cut that may eventually
// be recorded as a trace (reflexive criterion-tracing, deferred).
//
// A single criterion applied uniformly across all steps is a known
// simplification. This design is intentionally pre-translation-aware:
// translation steps cross regime boundaries, and a criterion that spans
// such a crossing may not be coherent. A future version may allow
// per-step or per-regime criteria.
type EquivalenceCriterion struct {
    // Name is a short identifier (handle) for this criterion.
    // Optional — a declaration-only criterion (no name) is valid and
    // encouraged as the primary mode. Name is a convenience for
    // repeated use, not an identity. Two criteria with different names
    // may declare the same grounds; two criteria with the same name
    // may drift in their declarations over time.
    Name string

    // Declaration is the interpretive criterion in natural language.
    // This is the primary layer (Layer 1) — the grounds for the reading.
    // Without it, the other fields have no grounding.
    Declaration string

    // Preserve is an optional list of aspects treated as
    // continuity-bearing under this criterion (Layer 2).
    // Values are free-text human vocabulary, not schema field names.
    // Requires Declaration to be non-empty (layer ordering).
    Preserve []string

    // Ignore is an optional list of aspects treated as irrelevant
    // to equivalence under this criterion (Layer 2).
    // Values are free-text human vocabulary, not schema field names.
    // Requires Declaration to be non-empty (layer ordering).
    //
    // These aspects are not absent — they are declared irrelevant
    // under this criterion. A different criterion might treat them
    // as decisive. This is a second-order shadow: what the reading
    // conditions exclude from relevance.
    Ignore []string
}
```

Methods:
- `IsZero() bool` — all fields empty (len-based, treats nil and empty
  slice equally)
- `Validate() error` — returns error if Preserve or Ignore are non-empty
  but Declaration is empty (layer ordering violation). Note: `Validate()`
  does not reject name-only criteria (they are structurally valid), but
  the CLI's `--criterion-file` loader additionally rejects zero-value
  criteria after unmarshal — silent fallback to v1 is not acceptable
  when the analyst explicitly requested a criterion.

**Tests**: `meshant/graph/criterion_test.go`
- Zero value detection
- Non-zero with only Name
- Non-zero with only Declaration
- Non-zero with Preserve or Ignore
- Zero with empty (not nil) slices
- Validate: Preserve without Declaration → error
- Validate: Ignore without Declaration → error
- Validate: Declaration + Preserve → ok
- Validate: all empty → ok (zero is valid)
- Validate: Name only → ok (name is a handle, not grounding)
- Fields accessible (structural stability test)

### Phase 2: Wire criterion into classification + output

**Files**: `meshant/graph/classify.go`, `meshant/graph/chain_print.go`

Changes:
1. Add `Criterion EquivalenceCriterion` to `ClassifyOptions`
2. Add `Criterion EquivalenceCriterion` to `ClassifiedChain`
3. When `ClassifyOptions.Criterion` is non-zero:
   - Copy criterion into `ClassifiedChain.Criterion`
   - Classification logic remains v1 heuristics unchanged (C1)
   - Step reasons remain pure v1 text — no criterion name prepended
4. When criterion is zero: existing v1 behaviour unchanged
5. `PrintChain` renders criterion block when non-zero:
   ```
   Criterion: operational-meaning
   Declaration: Preserve operational meaning, ignore representational variation
   Preserve: [target, obligation_level]
   Ignore: [display_format, wording]
   (criterion carried — classification uses v1 heuristics)
   ```
   Note the explicit "(criterion carried — classification uses v1
   heuristics)" line, making the gap visible.
6. `PrintChainJSON` includes criterion in JSON envelope when non-zero;
   omits entirely when zero (A2)

**Tests**: `meshant/graph/classify_test.go`, `meshant/graph/chain_print_test.go`
- Existing tests pass unchanged (zero criterion = v1)
- New test: criterion carried through classification
- New test: step reasons unchanged regardless of criterion (C1)
- New test: two different criteria on same chain → same classification,
  same reasons (v1 logic unchanged; criterion is carried, not applied)
- New print test: criterion block rendered in text output
- New print test: "(criterion carried — classification uses v1
  heuristics)" line present
- New print test: criterion in JSON envelope when non-zero
- New print test: criterion absent from JSON when zero (A2)

### Phase 3: CLI support

**File**: `meshant/cmd/meshant/main.go`

Add to `follow` subcommand:
- `--criterion-file <path>` — reads a JSON file containing an
  `EquivalenceCriterion` object. Loads, unmarshals, calls `Validate()`,
  fails fast with clear error on validation failure. (A3)
- File-based input is intentional, not a deferral of convenience.
  The criterion is an interpretive declaration that benefits from
  materialisation: it becomes reusable, version-controllable, and
  inspectable. Flags would make it ephemeral.

Individual flags (`--criterion-name`, `--criterion-declaration`,
`--criterion-preserve`, `--criterion-ignore`) deferred unless real
usage shows file-based input is too cumbersome.

**ANT resolutions (from philosophical review, 2026-03-14):**

### T1: Zero-value error message must name the Declaration/Name asymmetry

The error when a file decodes to a zero criterion must NOT say
"at least one of: name, declaration, preserve, ignore" — that flattens
the hierarchy. Declaration is the grounding layer; Name is a handle.
Required error message:
  `"zero-value criterion — file must contain at least a declaration
   (or a name as a handle)"`

### T2: Name-only criterion output must signal analytical weakness

When a criterion has Name but no Declaration, `printChainCriterion`
must add a second line making the incompleteness visible:
  `(handle only — no declaration grounds this reading)`
This means `chain_print.go` also changes in Phase 3.
Without this, name-only and declaration-grounded criteria are
indistinguishable in output, which misrepresents the reading.

### T3: Use DisallowUnknownFields() in loadCriterionFile

The criterion is an interpretive declaration — precision matters more
than forward-compatibility tolerance. An analyst who misspells
`"declaration"` as `"declarations"` must receive a "unknown field"
error, not a misleading "zero-value criterion" error. The JSON decoder
for criterion files must use `DisallowUnknownFields()`. Unknown fields
are not silently dropped. If forward-compatibility is needed later, this
can be relaxed when the schema is more stable.

**Tests**: `meshant/cmd/meshant/main_test.go`
- Follow with criterion file → criterion in output (text + JSON)
- Follow without criterion → v1 (existing tests unchanged)
- Non-existent criterion file path → error naming the path
- Malformed JSON criterion file → "malformed JSON" error
- Valid JSON but zero-value ({}) → "zero-value criterion" hard error
  naming the Declaration/Name asymmetry (T1)
- Empty file → hard error (EOF caught as malformed JSON)
- Criterion file with Preserve but no Declaration → validation error
- Criterion file with Ignore but no Declaration → validation error
- Name-only criterion → accepted (C4), output shows
  `(handle only — no declaration grounds this reading)` (T2)
- Name-only criterion in JSON format → criterion key in JSON
- Unknown JSON field only (e.g. {"declarations":"..."}) → unknown
  field error from DisallowUnknownFields (T3)
- --criterion-file with --output → criterion in written file
- Name-only in output: no Declaration/Preserve/Ignore lines, but
  handle-only signal present

### Phase 4: Decision record + codemap

- `docs/decisions/equivalence-criterion-v1.md`
  - Name the v1 implicit criterion (C2)
  - Acknowledge criterion as second-order cut (C3)
  - Acknowledge uniform-criterion-across-chain as simplification (P1)
  - Note Preserve/Ignore are free-text for now (P3)
  - Note criterion is designed to be traceable following
    IdentifyGraph/GraphRef pattern (future, P2)
  - Seed criterion-shadow in future milestones section
- `docs/CODEMAPS/meshant.md` updated

---

## Key design rules

1. **The criterion governs the function, never the reverse.** If a
   future comparison function disagrees with the interpretive criterion,
   the interpretive criterion governs.

2. **Zero value = v1 heuristics.** All existing code paths remain
   unchanged when no criterion is provided. No breaking changes. The v1
   heuristics embody an implicit, unnamed criterion: "trust the trace
   author's mediation annotation."

3. **The criterion is carried, not yet computed.** This milestone
   attaches the criterion as an interpretive object. It does not change
   classification logic. That is a separate, later milestone. The output
   must make this gap visible, not paper over it.

4. **Layer order is strict and enforced.** Layer 1 (declaration) must
   exist before Layer 2 (preserve/ignore) has meaning. `Validate()`
   enforces this at the type boundary. Layer 2 must exist before
   Layer 3 (comparison function) can be grounded.

5. **Classification reasons stay pure.** The criterion is rendered at
   the envelope level (ClassifiedChain, PrintChain output). It is never
   grafted onto individual StepClassification.Reason strings that were
   produced by v1 heuristics.

---

## What this enables (future milestones, not planned here)

- **Criterion-aware classification**: Layer 3 — a comparison function
  that uses Preserve/Ignore to judge steps differently from v1.
- **Multi-criterion comparison**: showing the same chain under two
  different criteria and comparing the readings.
- **Criterion as trace**: recording the analyst's criterion choice as
  a trace in the mesh (reflexive criterion-tracing), following the
  `IdentifyGraph`/`GraphRef` pattern.
- **Shadow-criterion interaction**: when the criterion is operationalised,
  aspects in the Ignore list should produce shadow-like entries so the
  analyst can inspect what the criterion made invisible.
- **Per-step criteria**: allowing different criteria for different
  segments of a chain, where translation boundaries cross regime changes.
- **DeclaredBy / DeclaredAt metadata**: making the criterion self-situated
  (who declared it, when) for full Principle 8 compliance.

---

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Criterion becomes decorative (carried but never used) | Low | Acknowledged — this is intentional for this milestone. Layer 3 is a separate step. |
| Over-engineering the struct before real usage | Medium | Keep struct minimal (4 fields). Only add fields when real criteria demand them. |
| Confusion between "criterion carried" and "criterion applied" | Medium | Explicit "(criterion carried — classification uses v1 heuristics)" in output. Design rule C1/5. |
| Name hardens into ontology of reading modes | Medium | Name is optional, not required. Declaration is the grounding layer. No registry, no uniqueness constraint. |

---

## Estimated scope

- Phase 1: small (new file, ~70 lines + tests)
- Phase 2: medium (modify 2 files, ~80 lines changes + tests)
- Phase 3: small-medium (CLI file loading + validation + tests)
- Phase 4: docs only

Total: moderate scope, conceptually significant.
