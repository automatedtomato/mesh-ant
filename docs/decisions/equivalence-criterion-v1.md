# Decision Record: Equivalence Criterion v1

**Date:** 2026-03-14
**Status:** Active
**Packages:** `meshant/graph`, `meshant/cmd/meshant`
**Branch merged:** `33-equivalence-criterion-docs` (part of M10.5+ Equivalence Criterion)

---

## What was decided

1. **`EquivalenceCriterion` is an interpretive declaration, not a computational rule**
2. **Three-layer design: Declaration (Layer 1) → Preserve/Ignore (Layer 2) → comparison function (Layer 3, deferred)**
3. **Layer ordering is strict and enforced: Layer 2 fields require Layer 1 to be non-empty**
4. **`ClassifiedChain.Criterion` carries the criterion as envelope metadata; v1 step heuristics unchanged**
5. **Zero criterion = v1 behaviour: all existing code paths unaffected**
6. **The criterion governs the function; the function never defines the criterion**
7. **Name is a handle, not identity; declaration-only criterion is the primary mode**
8. **`Ignore` is a second-order shadow: aspects declared irrelevant, not absent**
9. **`--criterion-file` loads criterion from a JSON file; individual flags deferred**
10. **`DisallowUnknownFields()` enforced for criterion files: precision over tolerance**
11. **Zero-value criterion file is a hard error: "at least a declaration (or a name as a handle)"**
12. **Name-only output signals analytical weakness: "(handle only — no declaration grounds this reading)"**

---

## Context

M10.5 introduced translation chains and step classification. The v1 classification heuristics
outsource mediation judgment to the trace author: if the author wrote a `Mediation` field, we
classify the step as mediator-like. This is honest, but it adds no analytical value beyond reading
a field.

What is missing is the **equivalence criterion** — an explicit declaration of what conditions the
analyst is reading under. Without it, classification is mechanical: it applies heuristics without
knowing what question the analyst is asking. Two analysts could read the same chain under
different interpretive commitments ("preserve operational meaning" vs. "preserve documentary
form") and produce different classifications — but neither would be visible in the output.

M10.5+ adds the criterion as an interpretive object: something the analyst declares before they
encounter the data. This milestone covers Layers 1–2 of a three-layer design. Layer 3 (a
comparison function that uses the criterion to judge steps differently from v1) is deferred.

The criterion is not a configuration knob — it is a methodological statement. Loading it from a
file (not individual flags) makes it reusable, version-controllable, and inspectable. The
declaration inscribes the analyst's position before the analysis begins.

---

## The v1 implicit criterion (C2)

Before M10.5+, every chain classification ran under an unnamed, undeclared criterion: **trust the
trace author's mediation annotation**. If the author wrote `Mediation: "...something..."`, the step
was mediator-like. If they wrote nothing, the step was intermediary-like.

This criterion is not absent — it is implicit. M10.5+ names it in `IsZero()`:

> "trust the trace author's mediation annotation"

When no `--criterion-file` is provided, this unnamed criterion remains in effect. Zero criterion
means v1 heuristics apply — not that no interpretive commitment exists. The most dangerous
assumptions are the ones that present themselves as not-assumptions.

---

## Decisions

### 1. `EquivalenceCriterion` is an interpretive declaration, not a computational rule

```go
type EquivalenceCriterion struct {
    Name        string   `json:"name,omitempty"`
    Declaration string   `json:"declaration,omitempty"`
    Preserve    []string `json:"preserve,omitempty"`
    Ignore      []string `json:"ignore,omitempty"`
}
```

The criterion declares *what conditions* a reading is conducted under. It does not compute
classifications. A future comparison function (Layer 3) will use Preserve/Ignore to judge steps,
but the criterion itself only states the analyst's interpretive stance.

**Why an interpretive declaration and not a rule?** Because the criterion governs the function;
the function must not define the criterion. If a future comparison function disagrees with the
declared criterion, the criterion wins. The declaration is prior.

This is an anti-positivist commitment: the analyst's position is named before they encounter the
data. The criterion is not discovered through analysis — it is brought to analysis.

### 2. Three-layer design: Declaration → Preserve/Ignore → comparison function

- **Layer 1 (Declaration)**: Natural-language statement of the interpretive conditions.
  The grounding layer. Without it, nothing else has meaning.
- **Layer 2 (Preserve/Ignore)**: Lists of aspects that are continuity-bearing or irrelevant
  under this criterion. Free-text human vocabulary — not schema field names. Requires Layer 1.
- **Layer 3 (comparison function)**: A function that uses the criterion to judge steps
  differently from the v1 heuristics. **Deferred.** Not part of this milestone.

This milestone implements Layers 1–2 only. The criterion is *carried* as provenance metadata.
It does not yet alter step classifications. The output makes this gap visible:

```
(criterion carried — classification uses v1 heuristics)
```

### 3. Layer ordering is strict: Validate() enforces it at the type boundary

```go
func (c EquivalenceCriterion) Validate() error {
    if c.Declaration == "" && (len(c.Preserve) > 0 || len(c.Ignore) > 0) {
        return errors.New("equivalence criterion: Preserve and Ignore require Declaration " +
            "(layer ordering: Layer 2 has no meaning without Layer 1 grounds)")
    }
    return nil
}
```

Preserve and Ignore have no meaning without a declaration to ground them. The error message
names the violation in terms of the layer model, not just the fields.

**What `Validate()` does NOT reject**: a zero-value criterion and a name-only criterion are both
structurally valid. The CLI (`loadCriterionFile`) additionally rejects zero-value criteria —
silent fallback to v1 is not acceptable when the analyst explicitly provided a criterion file.

### 4. `ClassifiedChain.Criterion` carries the criterion as envelope metadata only

```go
type ClassifiedChain struct {
    Chain           TranslationChain
    Classifications []StepClassification
    Criterion       EquivalenceCriterion   // envelope metadata only; does not alter v1 heuristics
}
```

The criterion is stored on `ClassifiedChain` as provenance metadata. It records *what the
analyst intended to look for* even if the apparatus cannot yet look for it. Step `Reason`
strings are purely edge-driven — the v1 heuristics produce them. The criterion is never
grafted onto individual step reasons (design rule C1).

This means two different criteria applied to the same chain produce the same classifications
and the same reasons. Only the envelope metadata differs. This is correct: the criterion is
carried, not applied. Layer 3 will change this.

### 5. Zero criterion = v1 behaviour: existing code paths unaffected

`ClassifyOptions.Criterion` defaults to zero. When `Criterion.IsZero()`:

- The step heuristics are unchanged (v1 logic)
- `PrintChain` emits no criterion block
- `PrintChainJSON` emits no `criterion` key (pointer `*EquivalenceCriterion` with `omitempty`)

This ensures full backwards compatibility. All existing tests and callers see identical behaviour.

### 6. The criterion governs the function; the function never defines the criterion

In `cmdFollow`, the criterion is loaded and validated *before* the trace file is opened, before
articulation, before chain traversal, and before classification:

```
load criterion → open traces → articulate → follow → classify
```

The criterion is established as an interpretive stance prior to encountering the data. This is
the deepest anti-positivist commitment in the implementation: the analyst declares their reading
conditions, then reads.

### 7. Name is a handle, not identity; declaration-only is the primary mode

The `Name` field is a convenience for repeated use. It is not an identity:

- Two criteria with different names may declare the same grounds
- Two criteria with the same name may drift in their declarations over time
- A declaration-only criterion (no name) is valid and *encouraged* as the primary mode

A name-only criterion (Declaration empty) is structurally valid — `Validate()` does not reject it.
But it is analytically weak: there are no grounds for the reading. The CLI accepts name-only
criteria but the output signals the weakness:

```
Criterion: handle-x
(handle only — no declaration grounds this reading)
(criterion carried — classification uses v1 heuristics)
```

This makes the distinction between handle and grounds visible, not hidden.

### 8. `Ignore` is a second-order shadow: aspects declared irrelevant, not absent (C3)

```go
// Ignore is an optional list of aspects treated as irrelevant to equivalence under this
// criterion. These aspects are not absent — they are declared irrelevant under this
// criterion. A different criterion might treat them as decisive. This is a second-order
// shadow: what the reading conditions exclude from relevance.
Ignore []string
```

The existing shadow apparatus (articulation shadow, diff shadow) names elements that a cut
cannot see. `Ignore` operates at a different level: it names aspects that the *criterion*
excludes from relevance. A different criterion might treat those aspects as decisive.

This is a second-order shadow: shadow of aspects, not elements. It is tracked here for the
future milestone that will extend the shadow apparatus to criterion-excluded aspects.

### 9. `--criterion-file` is intentional inscription, not a deferral of convenience

```
meshant follow <file> --element NAME --criterion-file <path> [...]
```

Loading the criterion from a file (rather than individual `--criterion-name`,
`--criterion-declaration` flags) is a deliberate design choice:

- **Reusability**: a criterion file can be shared, committed, and versioned
- **Coherence**: the criterion is a unified declaration, not a bag of flags that can
  be provided partially or inconsistently
- **Inscription**: materialising the criterion as a file encourages treating it as an
  analytical artefact with its own lifecycle, not an ephemeral command-line option

Individual flags are *not* added here. If real usage shows that file-based input is
too cumbersome for quick one-liners, flags can be added in a future milestone.

### 10. `DisallowUnknownFields()` enforced: precision over tolerance (T3)

```go
dec := json.NewDecoder(limited)
dec.DisallowUnknownFields()
```

An analyst who misspells `"declaration"` as `"declarations"` must receive an error, not a
silent zero-value criterion that falls back to v1 without warning. Precision matters more than
forward-compatibility tolerance for an interpretive declaration.

This aligns with the view that the criterion is not a data payload where unknown fields can be
dropped safely. It is a methodological statement. Silent tolerance would be epistemologically
dishonest.

### 11. Zero-value criterion file is a hard error naming the Declaration/Name asymmetry (T1)

```go
if c.IsZero() {
    return graph.EquivalenceCriterion{}, fmt.Errorf(
        "criterion-file: %q decoded to a zero-value criterion — " +
        "file must contain at least a declaration (or a name as a handle)", path)
}
```

An empty JSON object (`{}`) decodes to a zero-value criterion. This is not silent fallback
to v1 — the analyst explicitly provided a file and it was empty. The error message names the
Declaration/Name asymmetry: declaration is the grounding layer; name is a handle. Both are
accepted at the CLI; neither producing a zero-value criterion is not.

### 12. Preserve/Ignore are free-text human vocabulary (P3)

Values in `Preserve` and `Ignore` are free-text strings in the analyst's vocabulary, not schema
field names. For example:

```json
{
  "declaration": "Preserve operational meaning; ignore representational variation",
  "preserve": ["target", "obligation_level"],
  "ignore": ["display_format", "wording"]
}
```

These are not validated against any schema. This is deliberate: the criterion is an interpretive
object, and forcing analysts into a controlled vocabulary before the vocabulary is established
would prematurely close the design.

Future milestone: when Layer 3 (comparison function) is implemented, Preserve/Ignore values
may be mapped to trace fields, tags, or other structured attributes. For now, they are
human-readable signals about what to count as continuity-bearing or irrelevant.

---

## Uniform criterion across chain: a known simplification (P1)

A single `EquivalenceCriterion` is applied uniformly across all steps in a chain. This is a
deliberate limitation, not the final conceptual model:

- Translation steps may cross regime boundaries (scientific → juridical, etc.)
- A criterion that spans such a boundary may not be coherent
- Different steps in a chain may warrant different reading conditions

This is the **pre-translation-aware** design. A future milestone may introduce per-step or
per-regime criteria. The current design records one criterion at the `ClassifiedChain` level.

---

## Criterion as traceable object (future, P2)

The criterion is designed to follow the `IdentifyGraph`/`GraphRef` pattern. A future milestone
may assign the criterion an ID and record it as a trace (reflexive criterion-tracing), so the
analyst's reading conditions become part of the mesh and can be traced like any other actant.

For now, the criterion is an in-memory and file-resident object only.

---

## Types and signatures

### In meshant/graph/criterion.go (new)

```go
type EquivalenceCriterion struct {
    Name        string   `json:"name,omitempty"`
    Declaration string   `json:"declaration,omitempty"`
    Preserve    []string `json:"preserve,omitempty"`
    Ignore      []string `json:"ignore,omitempty"`
}

func (c EquivalenceCriterion) IsZero() bool
func (c EquivalenceCriterion) Validate() error
```

### In meshant/graph/classify.go (modified)

```go
type ClassifyOptions struct {
    Criterion EquivalenceCriterion   // zero = v1 heuristics (no criterion declared)
}

type ClassifiedChain struct {
    Chain           TranslationChain
    Classifications []StepClassification
    Criterion       EquivalenceCriterion   // envelope metadata only
}
```

### In meshant/graph/chain_print.go (modified)

```go
// chainJSONEnvelope now includes:
// Criterion *EquivalenceCriterion `json:"criterion,omitempty"` (pointer for correct omitempty)
```

### In meshant/cmd/meshant/main.go (modified)

```go
const maxCriterionBytes = 1 * 1024 * 1024

func loadCriterionFile(path string) (graph.EquivalenceCriterion, error)
// --criterion-file <path> flag added to cmdFollow
```

---

## Files added or modified

- `meshant/graph/criterion.go` — `EquivalenceCriterion` type; `IsZero()`, `Validate()` methods
- `meshant/graph/criterion_test.go` — 18 tests: zero detection, Validate layer ordering, structural stability
- `meshant/graph/classify.go` — `Criterion` field added to `ClassifyOptions` and `ClassifiedChain`; defensive copy in `ClassifyChain()`
- `meshant/graph/classify_test.go` — tests for criterion carried through, step reasons unchanged, two criteria = same result
- `meshant/graph/chain_print.go` — `printChainCriterion` helper; `*EquivalenceCriterion` in `chainJSONEnvelope`; name-only handle signal (T2)
- `meshant/graph/chain_print_test.go` — criterion block rendering tests; name-only, declaration-only, full block
- `meshant/cmd/meshant/main.go` — `loadCriterionFile()`; `--criterion-file` flag; criterion loaded before traces (T3, T1, T2)
- `meshant/cmd/meshant/main_test.go` — Group 11: 13 CLI integration tests for criterion file loading

---

## What M10.5+ explicitly defers

- **Layer 3 (comparison function)**: a function that uses Preserve/Ignore to classify steps
  differently from v1 heuristics. Separate future milestone.
- **Multi-criterion comparison**: showing the same chain under two different criteria and
  comparing the readings.
- **Reflexive criterion-tracing**: recording the criterion choice as a trace in the mesh
  (following `IdentifyGraph`/`GraphRef` pattern). See (P2) above.
- **Shadow-criterion interaction**: when the criterion is operationalised, aspects in the
  Ignore list should produce shadow-like entries so the analyst can inspect what the criterion
  made invisible.
- **Per-step criteria**: different criteria for different chain segments, where translation
  boundaries cross regime changes.
- **DeclaredBy / DeclaredAt metadata**: self-situated criteria (who declared it, when) for
  full Principle 8 compliance.
- **Individual CLI flags**: `--criterion-name`, `--criterion-declaration`, `--criterion-preserve`,
  `--criterion-ignore` are deferred unless file-based input proves too cumbersome.

---

## Relation to earlier decisions

- **Translation chain (M10.5)**: `ClassifyOptions` was an empty struct designed as an extension
  point for the criterion. M10.5+ is the realisation of that extension point.
- **Shadow (M2)**: `Ignore` follows the shadow philosophy — named absence is methodologically
  significant. It is a second-order shadow: what the reading conditions exclude.
- **Graph-as-actor (M5)**: the criterion is designed to follow the `IdentifyGraph`/`GraphRef`
  traceable-object pattern in a future milestone.
- **Reflexive tracing (M7)**: future criterion-tracing will use the same `ArticulationTrace`
  pattern — recording the analytical act as a trace in the mesh.
- **CLI (M9/M10)**: `--criterion-file` follows the same file-as-artefact pattern as `--output`,
  preferring materialised, inspectable objects over ephemeral flag combinations.

---

## Design rationale: why the criterion matters

The v1 heuristics classify a step by asking: "did the trace author write a mediation field?"
This is honest — it acknowledges that we are reading the author's annotation. But it provides
no analytical leverage: the classification is purely a reflection of the annotation.

The equivalence criterion shifts the question to: "under what conditions would we say these
two trace moments are equivalent?" This is the question that makes classification *productive*
rather than merely *descriptive*. It makes the analyst's interpretive commitment explicit and
inspectable.

By declaring the criterion before encountering the data, the analyst names their position.
By carrying the criterion on the output, the classification becomes conditional and contestable:
"under this criterion, this step is mediator-like." Another analyst under a different criterion
might reach a different conclusion — and the criterion makes that disagreement discussable.

The current milestone only carries the criterion. Layer 3 will make it governing. But even
carrying it is a methodological advance: it records what the analyst was looking for, even
before the apparatus knows how to look for it.
