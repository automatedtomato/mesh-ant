# Decision Record: Multi-Analyst Ingestion Comparison v1

**Date:** 2026-03-20
**Status:** Active
**Milestone:** Thread C — Multi-Analyst Ingestion Comparison
**Packages:** `meshant/loader` (analyst.go, extractiongap.go, classdiff.go), `meshant/cmd/meshant` (extraction-gap, chain-diff)
**Related:** `docs/decisions/tracedraft-v2.md`, `docs/decisions/translation-chain-v2.md`, `docs/decisions/interactive-review-v1.md`, `tasks/plan_thread_c.md`

---

## What was decided

1. **`ExtractedBy` is the analyst-position cut axis** — no new field or wrapper type was introduced; `ExtractedBy` is sufficient to partition analyst positions.
2. **`AnalystSet` wrapper was rejected** — wrapping a draft set in a named type adds indirection without adding information the field already carries.
3. **Slice fields use set-based comparison** — `Source`, `Target`, `Tags`, and `IntentionallyBlank` are compared as sorted string sets, not as ordered sequences.
4. **Chain-length mismatches are surfaced, not padded** — `CompareChainClassifications` compares up to `min(len(chainA), len(chainB))` steps and reports the asymmetry separately.
5. **`extraction-gap` and `chain-diff` are separate subcommands** — they answer different analytical questions and require different inputs.
6. **`intentionally_blank` over empty string** — an empty `mediation` field signals missing data; `intentionally_blank: ["mediation"]` signals deliberate absence.
7. **`observer` is a network actor, not the analyst** — the analyst is named in `extracted_by`; the `observer` field names who, within the traced network, observed the event.

---

## Context

Thread C opens a second analytical axis in MeshAnt: instead of asking "what did one observer position see in the network?", it asks "where did two analyst positions diverge in their extraction from the same source?" This is a different kind of cut — a cut on the extraction process itself, not on the traced network.

C.1–C.4 implemented four capabilities:
- **C.1**: Partition a TraceDraft dataset by analyst position (`GroupByAnalyst`)
- **C.2**: Compare which spans each position extracted and how they filled each field (`CompareExtractions`, `PrintExtractionGap`)
- **C.3**: Compare how two positions classified the derivation steps within the same span (`CompareChainClassifications`, `PrintClassificationDiffs`, `chain-diff` CLI)
- **C.4**: A multi-analyst example dataset that exercises both C.2 and C.3 against a concrete incident scenario

The decisions made during implementation are not recoverable from the code alone; this record captures what was tried, what was rejected, and what ANT tensions were encountered.

---

## Decision 1: `ExtractedBy` is the analyst-position cut axis

`ExtractedBy` is a required field on `TraceDraft`. It names the extraction position that produced the draft — a human analyst, an LLM pass, a review stage. Before C.1, it was used to track provenance in a single-pipeline context. Thread C reuses it as the axis for partitioning multi-analyst sets.

`GroupByAnalyst` uses `ExtractedBy` as its partition key. This is the entire implementation:

```go
func GroupByAnalyst(drafts []schema.TraceDraft) map[string][]schema.TraceDraft {
    result := map[string][]schema.TraceDraft{}
    for _, d := range drafts {
        result[d.ExtractedBy] = append(result[d.ExtractedBy], d)
    }
    return result
}
```

No new schema field was introduced. A new field ("analyst-position", "extraction-label", or similar) was considered but rejected: it would duplicate what `ExtractedBy` already records and force callers to populate two fields that encode the same fact from different angles.

The existing doc comment on `ExtractedBy` was updated to make the analyst-position interpretation explicit: "ExtractedBy names the analyst position or pipeline stage that produced this draft." This is the right place for the clarification — in the type itself, not in a wrapper.

---

## Decision 2: `AnalystSet` wrapper was rejected

During design, an `AnalystSet` struct was proposed:

```go
// Proposed — rejected
type AnalystSet struct {
    Label  string
    Drafts []schema.TraceDraft
}
```

This was rejected for two reasons:

**The field already carries the label.** Every draft in a set already knows its analyst position via `ExtractedBy`. A wrapper that adds a `Label` field outside the drafts creates a redundancy: the label is now in two places, and the invariant that they match is not enforced.

**The function signatures would become opaque.** `CompareExtractions(analystA string, setA []schema.TraceDraft, analystB string, setB []schema.TraceDraft)` is explicit about what it receives. `CompareExtractions(setA AnalystSet, setB AnalystSet)` hides the same structure behind indirection and gives future readers no advantage over the explicit form.

The analyst-position label is passed as a string to the comparison and print functions. This is intentional: the label is an argument to the comparison, not a property of the data. Two callers can compare the same drafts under different labels (for different analytical purposes) without modifying any draft.

---

## Decision 3: Slice fields use set-based comparison

`Source`, `Target`, `Tags`, and `IntentionallyBlank` are `[]string` fields. When comparing two drafts from different analyst positions, the question is whether the two positions recorded the same elements — not whether they recorded them in the same order.

`FieldDisagreement` is emitted only when the element sets differ. If analyst-a records `"source": ["connection-pool-monitor"]` and analyst-b records `"source": ["connection-pool-monitor"]` in a different encounter order, no disagreement is reported.

Comparison is performed by sorting both slices and comparing element-by-element. The implementation:

```go
func sliceFieldEqual(a, b []string) bool {
    if len(a) != len(b) {
        return false
    }
    sa, sb := sorted(a), sorted(b)
    for i := range sa {
        if sa[i] != sb[i] {
            return false
        }
    }
    return true
}
```

The alternative — sequence comparison — was rejected because it would treat `["A", "B"]` and `["B", "A"]` as disagreements. In TraceDraft, `Source` and `Target` are unordered (the schema treats them as sets of participants); reporting ordering differences as analytical disagreements would overcount.

This creates a known limitation: if two positions are being compared on ordering conventions rather than element membership, set-based comparison will miss the disagreement. This limitation is documented in the `CompareExtractions` doc comment.

---

## Decision 4: Chain-length mismatches are surfaced, not padded

`CompareChainClassifications` compares two chains by position: `chainA[0]` is compared to `chainB[0]`, `chainA[1]` to `chainB[1]`, and so on, up to `min(len(chainA), len(chainB))`. Steps beyond the shorter chain are not compared.

Padding was considered: extend the shorter chain with synthetic "no classification" entries so all steps are compared. This was rejected.

**Padding invents data.** If analyst-a has a two-step chain and analyst-b has a one-step chain, the second step of analyst-a's chain has no counterpart in analyst-b's chain. Inventing a counterpart (a null classification, a "no opinion" step) imports an assumption: that analyst-b's position, if extended, would have produced a step at that position. That assumption is not in the data.

**The asymmetry is itself analytical information.** A longer chain means a position traced more derivation depth within the same span. `PrintClassificationDiffs` surfaces the length mismatch with both chain lengths named:

```
Position A has 2 steps; Position B has 1 step.
Steps beyond position 1 are not visible from this comparison.
```

This makes the asymmetry legible to the analyst without claiming to know what the shorter position would have said.

`StepIndex` in `ClassificationDiff` is derived from the loop counter (`i+1`), not from `a.StepIndex` or `b.StepIndex`. Using the loop position makes the index unambiguous and correct even if a caller passes chains with non-sequential or misaligned StepIndex values.

---

## Decision 5: `extraction-gap` and `chain-diff` are separate subcommands

Two CLI designs were considered:

1. **Integrated**: `chain-diff` extends `extraction-gap` with a `--classify` flag; a single subcommand runs both analyses.
2. **Separate**: `extraction-gap` and `chain-diff` are independent subcommands.

Separate subcommands were chosen.

**The analytical questions are different.** `extraction-gap` asks: "Which spans did each position extract, and how did they fill each field?" `chain-diff` asks: "For this specific span, how did each position classify the derivation steps?" These are not variants of the same question — they operate on different data structures and require different interpretation.

**The required inputs differ.** `extraction-gap` takes two analyst labels and a dataset; it compares across all shared spans. `chain-diff` takes two analyst labels, a dataset, and a required `--span` flag; it operates on a single span at a time. Combining them into one subcommand would require optional-required flag semantics or a mode flag — both of which increase cognitive overhead.

**Composability.** An analyst can run `extraction-gap` first to find where positions diverge, then run `chain-diff` for specific spans of interest. The two subcommands compose naturally as separate steps without forcing a combined output the analyst may not need.

---

## Decision 6: `intentionally_blank` over empty string

When a TraceDraft field is empty, two interpretations are possible: (a) the analyst did not find relevant content and left the field blank without reflection, or (b) the analyst considered the field and made a deliberate choice to leave it empty.

An empty string conflates both interpretations. `IntentionallyBlank` was introduced in M12.5 to distinguish them. In the multi-analyst example dataset, `span-flash-sale-load-pattern` (analyst-b) has `intentionally_blank: ["mediation"]` — the absence of mediation is a deliberate analytical claim, not missing data.

An empty `"mediation": ""` in the same record would have been ambiguous: did analyst-b decide there was no mediator, or simply not fill in the field? The `intentionally_blank` marker makes the deliberate absence visible to both `extraction-gap` (which counts it as a field-level difference when one position has it and the other does not) and to readers inspecting the dataset directly.

---

## Decision 7: `observer` is a network actor, not the analyst

An early version of the example dataset set `observer` to the analyst label (`"analyst-a"`, `"analyst-b"`). This was incorrect and fixed before merge.

The `observer` field names who, within the traced network, observed the event being traced — not who extracted the trace. In TraceDraft, the analyst is named in `extracted_by`. The distinction matters:

- `extracted_by`: who produced this draft (the extraction position)
- `observer`: who in the network was positioned to observe this event (a network actor)

Conflating them would corrupt the analytical value of the observer field. A reader querying "what did on-call-engineer observe?" would find analyst names, not network events. The gap between analyst and observer is itself an ANT commitment: the analyst is not inside the network being traced; they observe it from outside and produce a draft that names someone inside the network as the observer.

---

## ANT tensions surfaced during Thread C

### The extractor is not a network actor

`GroupByAnalyst` partitions by `ExtractedBy` — the name of the extraction position. This creates an analytical layer that sits above the network: the analyst who extracted the draft is not (typically) an actor in the traced network. Comparing two analysts is a meta-level operation: two observers of the observers.

MeshAnt does not resolve this tension in Thread C. The analyst-position is treated as a label, not as a network actor. A future thread may address whether and how the extraction process should itself be traced.

### `SourceSpan` as a shared key

`CompareExtractions` uses `SourceSpan` as the key for matching drafts across analyst positions. Two drafts with the same `SourceSpan` string are assumed to be attempts to trace the same source material. This is a necessary working assumption but not a guarantee: two analysts may use the same span string to label different readings of an ambiguous source.

The assumption is documented in the `CompareExtractions` doc comment: "SourceSpan is used as the matching key; identical strings do not guarantee the same source material was read." The tension is named; it is not resolved.

### Classification comparison is positional, not identity-based

`CompareChainClassifications` compares chains by position. Two analysts may have produced chains of the same length from the same span with the same source drafts — but derived in different orders, or from different root candidates. The step at position 1 in analyst-a's chain may not be the same derivation moment as the step at position 1 in analyst-b's chain.

`PrintClassificationDiffs` notes this explicitly in its footer: "Step indices reflect positional depth in each chain, not shared derivation identity." Positional comparison is the most tractable approach given the available data; a deeper identity-based comparison would require derivation graphs to be aligned across analyst positions, which Thread C does not attempt.

### Multiple-draft sentinel as a proxy for complexity

When a `SourceSpan` has more than one draft from the same analyst position, `CompareExtractions` records a `(multiple-drafts)` sentinel rather than comparing all field combinations. This avoids combinatorial comparison explosion but loses information: the analyst produced multiple readings, and those readings may differ from each other in ways that matter.

The sentinel is a known limitation. A future version may replace it with a per-chain comparison or a count of drafts per position.

---

## What Thread C does NOT do

- **Resolve analytical disagreements** — Thread C makes disagreements visible; it does not adjudicate between positions.
- **Trace the extraction process** — extractors are labels, not network actors. The extraction process is not itself traced.
- **Compare chains across analyst positions by derivation identity** — comparison is positional.
- **Handle multiple roots per span gracefully** — `cmdChainDiff` reports ambiguous roots as an error; it does not attempt to pick one.
- **Automate interpretation** — `PrintExtractionGap` and `PrintClassificationDiffs` produce positioned reports; no automated conclusion is offered.

---

## Related

- `docs/decisions/tracedraft-v2.md` — `TraceDraft` schema, `ExtractedBy` provenance, `IntentionallyBlank`
- `docs/decisions/translation-chain-v2.md` — `FollowDraftChain`, `ClassifyDraftChain`, `DraftStepKind`, `DraftStepClassification`
- `docs/decisions/interactive-review-v1.md` — `ExtractedBy` as pipeline stage vs analyst-position distinction
- `docs/decisions/interpretive-outputs-v1.md` — Thread B: Layer 3 output design patterns
- `docs/glossary.md` — mediator, intermediary, cut, shadow, articulation
