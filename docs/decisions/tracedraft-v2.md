# Decision Record: TraceDraft v1

**Date:** 2026-03-15
**Status:** Active
**Packages:** `meshant/schema`, `meshant/loader`, `meshant/cmd/meshant`
**Branches:** `39-cve-dataset`, `40-tracedraft-type`, `41-draft-loader`, `42-cmd-draft`

---

## What was decided

1. **`TraceDraft` is a legitimate analytical object, not a trace-in-waiting**
2. **`SourceSpan` is the only required field — the ground truth anchor for all extraction**
3. **Empty is structurally enforced: no path in the ingestion pipeline creates pressure to fill content fields**
4. **The LLM boundary is an explicit file on disk, not a hidden API call**
5. **The extraction chain is followable from day one via `DerivedFrom`**
6. **`Promote()` is a deliberate analyst act; promoted traces carry `TagValueDraft` as provenance**
7. **`ExtractionStage` names positions in a pipeline, not stages of progress**
8. **`ExtractedBy` is a free-form string — no structural distinction between human and non-human producers**
9. **Over-actorized drafts are structurally indistinguishable by design; critique belongs to a later pass**
10. **Live LLM calls are outside the CLI boundary for v1.x**

---

## Context

MeshAnt's input model at v1.0.0 assumed a user who already thinks in MeshAnt terms — who
can name observer positions, identify mediations, and populate every required field. This
blocked the most important use case: a user who has raw material (logs, documents,
transcripts) and wants to begin tracing.

M11 introduces the first ingestion entrypoint: a pathway from unstructured material to
canonical traces that preserves uncertainty, keeps the extraction process visible, and
resists premature actorization. The central challenge is that any extraction process —
especially an LLM-assisted one — tends to pull material back into familiar vocabularies
(actor, subject, intention, root cause). The framework must make that pull visible and
resistible rather than hiding it inside an automated pipeline.

---

## Decision 1: TraceDraft is a legitimate analytical object

`TraceDraft` is not a `Trace` that has not yet passed validation. It is a distinct type
that may be incomplete, unresolved, or explicitly uncertain — and may remain so
indefinitely. A draft that resists further articulation is analytically meaningful: the
resistance names something about the source material.

The name was chosen over `CandidateTrace` deliberately. "Candidate" implies a record
awaiting a verdict — on its way to becoming real. "Draft" implies provisionality without
implying inevitability of completion.

---

## Decision 2: SourceSpan as the ground truth anchor

`SourceSpan` is the only field required by `Validate()`. Everything else — `WhatChanged`,
`Source`, `Target`, `Observer`, `Mediation` — is optional at the draft stage.

This is a structural embodiment of the trace-first commitment: the verbatim text that
provoked the extraction is the ground truth. All interpretation is layered on top of it.
If a field cannot be filled confidently from the source span, it should be left empty.

A `TraceDraft` with only a `SourceSpan` is a valid record. It preserves the material
that warranted attention without forcing premature resolution.

---

## Decision 3: Empty is structurally enforced

The empty-over-fabricated principle is not only documented — it is made the path of
least resistance by design:

- `Validate()` requires only `SourceSpan`; all content fields have a meaningful zero
  value
- `IsPromotable()` requires `WhatChanged` and `Observer`, but never `Source` or `Target`
- `Promote()` transfers content fields as-is, fabricating nothing; nil slices stay nil
- `LoadDrafts()` assigns only framework fields (ID, Timestamp) — never content fields
- `cmdDraft` flag overrides apply only to provenance metadata (`--source-doc`,
  `--extracted-by`, `--stage`), never to content fields

This means an extraction can be promoted to a canonical `Trace` without naming any actors
at all. The framework asks *what changed?* and *from what position?* — but not *who did
it?* — as the minimum conditions for canonical status.

---

## Decision 4: The LLM boundary is a named file on disk

`meshant draft` reads an extraction JSON file. It does not make API calls. The pipeline is:

```
raw document → external LLM tool → extraction JSON → meshant draft → TraceDraft records
```

The boundary between the LLM's transformation and MeshAnt's ingestion is a file on disk
— inspectable, version-controllable, and replayable. This makes the LLM's transformation
visible as a discrete step rather than hiding it inside the CLI.

This is consistent with treating the LLM as a mediator, not an intermediary. A mediator
transforms what passes through it (compresses, selects, stabilizes, over-articulates). An
intermediary passes input through unchanged. The extraction JSON is the LLM's output as
artifact — it can be read, critiqued, and revised before MeshAnt processes it.

The LLM boundary moves internal only at the interactive CLI layer (v2.0.0). Until then,
the file-as-boundary preserves inspectability at the cost of a manual step.

---

## Decision 5: The extraction chain is followable from day one

`DerivedFrom` links a revision draft to its parent by ID. The extraction pipeline:

```
span-harvest → LLM weak-draft → critique → human revision → promoted trace
```

is represented as a chain of `TraceDraft` records linked by `DerivedFrom`. When the
critique pass (M11.5/M12) arrives, that chain already exists in the data. MeshAnt can
follow it as a translation chain, classifying each step: did the LLM act as intermediary
(passed the span unchanged) or mediator (transformed it)?

`DerivedFrom` is named as a positional link, not a genealogical one. "Derived from" says
"this came after that in a sequence" — not "this descends from" or "this is a version of."
The chain is followable without implying that the root is the most authentic or that the
leaf is the most refined.

---

## Decision 6: Promotion is a deliberate act

`Promote()` is not automatic. The analyst calls it when a draft is ready. This deliberateness
is a methodological commitment: promotion is a decision, not a threshold crossing.

Every promoted `Trace` carries `TagValueDraft = "draft"` as a provenance signal. This tag
marks where the trace came from — it does not mark what the trace is. A promoted trace is
a canonical trace; the "draft" tag records that it passed through the ingestion pipeline.
The tag should be read as "draft-origin," not "still a draft."

---

## Decision 7: ExtractionStage names positions, not progress

The known `ExtractionStage` values are `"span-harvest"`, `"weak-draft"`, and `"reviewed"`.
These names carry a risk: they suggest a desirable direction of travel, as if a record
should move from harvest toward review.

In ANT terms, a record that remains at `"weak-draft"` indefinitely is not stalled — it
may be resisting articulation in a way that is itself analytically meaningful. The
resistance is a fact about the source material, not a failure of the extraction process.

`ExtractionStage` should be read as a positional marker (where in the pipeline was this
record produced?) rather than a progress marker (how far along is this record?). The
framework does not enforce movement between stages.

---

## Decision 8: ExtractedBy is a free-form string

`ExtractedBy` takes values like `"human"`, `"llm-pass1"`, `"reviewer"`. These are
free-form strings, not an enum. There is no `ExtractorType`, no `IsHuman` boolean, no
structural distinction between human and non-human producers.

This preserves generalised symmetry: the framework treats `"human"` and `"llm-pass1"` as
the same kind of value in the same kind of field. The producer of a draft is identified
for provenance purposes, not categorized ontologically.

---

## Decision 9: Over-actorized drafts are structurally indistinguishable by design

The CVE dataset (`cve_response_extraction.json`) contains two intentionally over-actorized
records: E3 (treating `"attacker"` as a stable actor) and E14 (treating `"cve-2026-44228"`
as an actor with agency). Both lack `uncertainty_note`. Both are structurally
indistinguishable from well-drafted records.

This is the correct design. Building an `is_over_actorized` boolean or a structural flag
would itself be an act of premature ontologizing — the framework would be classifying
actorization quality before the analyst has read the draft. The differentiation belongs
to the critique pass (M11.5/M12), where it emerges as a reading, not as a property of
the data.

The current framework cannot distinguish the seed from the soil. That is acknowledged
and intentional. The critique pass will produce `DerivedFrom`-linked critique drafts that
name the over-actorization — making the critique itself a part of the chain rather than
a property of the original record.

---

## Decision 10: Live LLM calls deferred to v2.0.0

`meshant draft` reads a file. It does not call an LLM API. This is not a limitation — it
is a boundary that makes the LLM's role explicit and inspectable.

The LLM's transformation appears in the data (via `ExtractedBy`, `DerivedFrom`, and the
content of the extraction JSON itself) but not in the code. When the interactive CLI
layer (v2.0.0) adds live LLM calls, the LLM's outputs will still flow through the same
`TraceDraft` ingestion pipeline — the boundary will move inside the tool, but the data
model will remain the same.

---

## What M11 does NOT do

- ~~**Anti-ontology critique pass**~~ — resolved in M12: `meshant rearticulate` builds
  `DerivedFrom`-linked critique skeletons from seeded drafts; `meshant lineage` walks the
  derivation chain. See `docs/decisions/rearticulation-v2.md`.
- **Interactive trace review** — deferred to v2.0.0
- **Per-step criteria for the ingestion chain** — deferred to M13+
- **Live LLM calls from the CLI** — deferred to v2.0.0
- **Graphiti / Neo4j adapter** — future-compatible boundary only

---

## Related

- `docs/reviews/llm_limit_14-mar-26.md` — LLM-as-mediator framing, what the LLM should
  and should not be asked to do
- `tasks/plan_m11.md` — full M11 plan including phase breakdown and design rules
- `docs/decisions/trace-schema-v2.md` — canonical Trace type that TraceDraft promotes into
- `docs/decisions/equivalence-criterion-v1.md` — interpretive declaration pattern (parallel
  to TraceDraft's role as provisional analytical object)
