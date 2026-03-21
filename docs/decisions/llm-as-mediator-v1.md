# Decision Record: LLM as Mediator Convention v1

**Date:** 2026-03-21
**Status:** Active
**Thread:** Thread F — LLM-Internal Boundary (v2.0.0)
**Packages:** convention; governs `meshant/llm`, `meshant/schema`, `data/prompts`
**Related:** `docs/decisions/tracedraft-v2.md`, `docs/decisions/interactive-review-v1.md`, `tasks/plan_thread_f.md`

---

## What was decided

Seven conventions govern how the LLM participates in the MeshAnt ingestion pipeline. The LLM
is a mediator — it transforms source material into candidate drafts. Its transformations are
visible, provenance-bearing, and contestable. No LLM-produced draft enters the mesh without
a named analytical position (`ExtractedBy`, `ExtractionStage`), a session link (`SessionRef`),
and a framework-imposed uncertainty signal (`UncertaintyNote`). These conventions bind all
Thread F code; subsequent issues (F.2–F.6) cite specific decisions by number.

---

## Context

In v1.x, the LLM is external. The user runs an LLM separately and feeds its JSON output to
`meshant draft`. The framework never touches the LLM; it receives a file and assigns provenance.
The boundary is explicit: the user is responsible for the LLM's output.

In v2.0.0, the LLM boundary moves inside the CLI. Three new subcommands — `extract`, `assist`,
`critique` — call the LLM directly. This changes the provenance problem: the framework is now
responsible for ensuring that every LLM call produces a fully-provenanced record. The risk is
that LLM output is treated as extracted fact rather than as a candidate reading from one
instrument at one moment under one set of conditions.

This record defines the conventions before the code exists. The conventions are not derived from
implementation experience — they are commitments made before F.2 is written, so that F.2
through F.6 can implement to a shared standard. A different set of conventions would produce
a different codebase. The choices made here reflect four ANT commitments from the Thread F plan
(FM1–FM4): provenance on every draft, trace-first vocabulary, system instructions that enforce
the vocabulary, and a mandatory session record on every code path.

---

## Decision 1: LLM is a mediator, not an extractor

The LLM's output is a candidate draft, not a trace. The LLM transforms source material; the
transformation is visible and contestable. This is the governing commitment of the entire thread.

In ANT terms, a mediator transforms what passes through it — its output is not a faithful relay
of its input. An intermediary relays faithfully; its output carries its input unchanged. The LLM
is never an intermediary. Even when the LLM's reading appears to match the source text closely,
it has selected, compressed, and classified: it has made analytical choices that a different
instrument or a different prompt would make differently. Calling the LLM an "extractor" and its
output "extracted data" suppresses this transformation and presents the LLM's reading as a
property of the source document rather than as a property of the encounter between instrument
and material.

The function names in Thread F use "extraction" in the pipeline sense — `RunExtraction` names a
pipeline step, not an epistemological claim. The vocabulary constraint applies to what the
functions assert about their outputs, not to the step names themselves. `RunExtraction` produces
candidate drafts; it does not extract facts.

---

## Decision 2: ExtractedBy uses model ID strings

`ExtractedBy` on LLM-produced drafts must be the model ID string — `"claude-sonnet-4-6"`, never
the generic label `"llm"`. Two runs with different models produce drafts with different
`ExtractedBy` values.

`ExtractedBy` is the analyst-position cut axis for the ingestion layer, parallel to `Observer`
for the graph layer. It names an analytical position, not a person or system identity (see
`tracedraft.go` doc comment). A generic `"llm"` label collapses all models into a single
undifferentiated position, erasing the distinction between instruments that is the point of
tracking `ExtractedBy`. The model ID is the most specific available identifier for the
instrument's position.

This decision carries a tension named below (T3): the model ID names a system version, not a
full analytical position. The same model with different system instructions occupies a different
position, but `ExtractedBy` alone does not distinguish them. `SessionRef` mitigates this — the
session record carries the full prompt and conditions — but the field itself carries the tension.

---

## Decision 3: UncertaintyNote is set by framework code

The framework appends `"LLM-extracted; unverified by human review"` to the `UncertaintyNote`
field on every LLM-produced draft. The LLM may suggest uncertainty content in response to the
prompt, but the framework always appends its own note. The LLM cannot override this; the
framework code is the final writer.

This prevents a specific failure mode: the LLM claiming certainty it cannot have. An LLM that
produces a draft with an empty `UncertaintyNote` is not expressing certainty — it is following
its training to produce confident-sounding output. The framework's note makes the epistemic
status explicit regardless of what the LLM signals about its own confidence.

The framework note and any LLM-suggested uncertainty content are both present: if the LLM
produced a non-empty `UncertaintyNote`, the framework appends to it, separated by a space.
The LLM's signal is preserved; the framework's signal is always added. See T1 below for the
tension this creates.

---

## Decision 4: ExtractionStage known values

Four values are recognised in Thread F. `ExtractionStage` names a position in the pipeline,
not a quality level. Stages are not ordered; no stage is ranked above another.

- `"span-harvest"` — raw spans, no candidate fields populated. The draft holds only a
  `SourceSpan`; content fields are intentionally absent because no extraction has occurred.
  This is valid at the `Validate()` level (only `SourceSpan` is required).

- `"weak-draft"` — candidate fields have been populated, by a human or by an LLM. The fields
  are candidates: they represent one reading from one position. The draft is reviewable.

- `"critiqued"` — an LLM re-articulation of an existing draft (new in Thread F). Produced by
  `meshant critique`. The LLM was given the original draft as context and asked to produce
  an alternative reading of the same `SourceSpan`. `"critiqued"` is a mediating act — an
  LLM suggestion, not a human decision. See Decision 5.

- `"reviewed"` — a human decision. The reviewer accepted or edited the draft in a `meshant
  review` session. `ExtractedBy` is `"meshant-review"` for both accept and edit (see
  `docs/decisions/interactive-review-v1.md`, Decision 2).

The progression `span-harvest → weak-draft → critiqued → reviewed` describes a typical pipeline
order but is not a quality ordering. A `"critiqued"` draft is not better than a `"weak-draft"`;
a `"reviewed"` draft is not more valid than a `"critiqued"` one. Each stage names where in
the pipeline the draft was produced. Two drafts at different stages with the same `SourceSpan`
have equal standing as analytical objects and can be compared via `ClassifyDraftChain`.

---

## Decision 5: "critiqued" vs "reviewed" — distinct epistemic positions

`"critiqued"` and `"reviewed"` are distinct `ExtractionStage` values because they name
distinct epistemic positions, not different steps in a quality ladder.

A `"critiqued"` draft is produced by an LLM acting as a mediating apparatus. The LLM was
given a draft as context and asked to produce an alternative reading. Its output reflects the
LLM's instrument position — its training data, its system instructions, and the specific
prompt it received. It is a suggestion.

A `"reviewed"` draft is produced by a human reviewer making a curatorial decision. The
reviewer saw the draft, its derivation chain, and its ambiguity warnings, and chose to
accept or modify it. The `meshant review` session is the apparatus; the human is part of
the assemblage. The reviewed draft carries `ExtractedBy: "meshant-review"`.

Both are derivation steps. Both produce new drafts via `DerivedFrom`. Both are
first-class analytical objects. Neither is closer to truth. The difference is not epistemic
quality but epistemic kind: a suggestion from an instrument vs. a decision from an
assemblage that includes a human.

This distinction is also why `filterReviewable` in `review/session.go` will include
`"critiqued"` drafts in the review queue (F.4): a `"critiqued"` draft is precisely the
kind of draft that benefits from human review. The LLM's suggestion is an input to the
human's decision, not a substitute for it.

---

## Decision 6: SessionRecord is mandatory

Every LLM interaction — `meshant extract`, `meshant assist`, `meshant critique` — must
produce a `SessionRecord`, even on error. A session that errors with no drafts produced
returns a `SessionRecord` with `DraftCount: 0` and `ErrorNote` set. The caller writes the
`SessionRecord` to disk.

The `SessionRecord` carries `ExtractionConditions`: model ID, prompt template, system
instructions, source document reference, and timestamp. Its `ID` is the `SessionRef` set on
every draft produced in the session. This makes the conditions under which a draft was
produced recoverable from the draft itself — not only from file co-location.

This requirement enforces ANT commitment FM4 from the Thread F plan: the session apparatus
is always visible. A session that silently produces no record when it errors makes the LLM's
analytical position unrecoverable from the output. Mandatory `SessionRecord` means the LLM's
position is always named, even when the session fails.

The write location is governed by `--session-output`. See T4 in `tasks/plan_thread_f.md`:
if the user pipes output to stdout without `--session-output`, the record is not written.
Thread F names this as a user-agency decision, not a design flaw.

---

## Decision 7: IntentionallyBlank required in LLM output

System instructions for all Thread F prompts require the LLM to set `intentionally_blank`
on any content field it deliberately leaves empty. Content fields are: `what_changed`,
`source`, `target`, `mediation`, `observer`, `tags`.

This requirement mirrors the existing `IntentionallyBlank` semantics from the rearticulation
pipeline (see `docs/decisions/rearticulation-v1.md`): the difference between "never attempted"
and "deliberately left empty" is a named distinction in the data, not an absence. A field absent
from the draft and absent from `IntentionallyBlank` means the LLM did not address it. A field
absent but listed in `IntentionallyBlank` means the LLM considered it and chose not to fill it.

The framework validates that field names listed in `IntentionallyBlank` are known content fields;
unknown names are rejected. This prevents the LLM from using `IntentionallyBlank` to suppress
provenance fields (which are framework-assigned and not editable in LLM output).

---

## ANT tensions named but not resolved

### T1: Framework-imposed UncertaintyNote may suppress the LLM's own signal

The framework appends its own note to any LLM-suggested `UncertaintyNote`. This prevents false
certainty but it also subordinates the LLM's uncertainty signal to a blanket label. A reviewer
reading the draft sees both signals — the LLM's and the framework's — but the framework's note
is always present regardless of how carefully the LLM expressed its uncertainty.

A more nuanced approach would distinguish `"LLM-expressed uncertainty"` from
`"framework-imposed epistemic status"` as separate fields. Thread F does not resolve this
because introducing a second uncertainty field would complicate the schema and the review
session display without a clear benefit in the current pipeline. The blanket note is a
conservative choice that errs toward flagging rather than toward trusting the LLM's
self-assessment.

### T2: The Complete interface hides the LLM's internal mediation

The `LLMClient` interface sees inputs and outputs, not the LLM's internal process. The LLM
performs compressions, selections, vocabulary impositions, and chain-of-thought steps that are
invisible through this boundary. `SessionRecord` captures the session's external conditions
(model, prompt, timestamp); it cannot capture the LLM's internal transformations.

This is the same limit that applies to any black-box mediator in the mesh — the framework
cannot observe what happens inside any mediator, only what enters and exits. But it is worth
naming because the LLM's internal mediation is unusually extensive: the gap between "prompt
received" and "output produced" involves more transformation than most mediators in the
MeshAnt datasets. The `Complete` interface makes this gap structurally invisible.

### T3: Model ID as system identifier vs. analytical position

`ExtractedBy: "claude-sonnet-4-6"` names a model version, not a full analytical position.
The same model with different system instructions occupies a different analytical position —
a different reading frame, a different set of vocabulary constraints, a different weighting
of competing framings. `ExtractedBy` does not distinguish between these positions; only the
`SessionRef` → `SessionRecord` → `ExtractionConditions.SystemInstructions` chain makes the
full position recoverable.

A downstream analyst comparing two drafts with the same `ExtractedBy` but different
`SessionRef` values may be comparing different analytical positions that happen to share a
model version. This is a real ambiguity in the data. Thread F names it; it does not resolve it.

---

## References

- `tasks/plan_thread_f.md` — Thread F detailed plan; source of FM1–FM4 commitments
- `docs/decisions/tracedraft-v2.md` — TraceDraft schema; ingestion contract; DerivedFrom chain
- `docs/decisions/interactive-review-v1.md` — review session decisions; ExtractedBy convention
- `docs/decisions/rearticulation-v1.md` — IntentionallyBlank origin and semantics
- `docs/glossary.md` — mediator, intermediary, cut, articulation, shadow
- `data/prompts/critique_pass.md` — critique prompt template (vocabulary style reference)
- `data/prompts/extraction_pass.md` — extraction prompt template (Thread F)
