# Plan: M11 — TraceDraft + Provenance-First Ingestion

**Date:** 2026-03-15
**Status:** Confirmed — design discussion 2026-03-15
**Source:** Discussion notes in `docs/temps/review_response_graph_llm_15-mar-26.md`,
`docs/reviews/llm_limit_14-mar-26.md`, `docs/reviews/graph_integration_note_14-mar-26.md`

---

## Problem

MeshAnt's current input model is a hard barrier for new users. Authoring canonical `Trace`
records requires a user who already thinks in MeshAnt terms — who can name observer
positions, identify mediations, and populate every required field. This blocks the most
important use case: a user who has raw material (logs, documents, transcripts, notes) and
wants to begin tracing.

The framework needs an **ingestion entrypoint** that:
1. Accepts material that is not yet a trace
2. Preserves uncertainty rather than forcing resolution
3. Keeps the extraction process itself visible and inspectable
4. Lays groundwork for the extraction chain to eventually appear *in* the mesh

---

## Core design decisions

### LLM boundary: external for M11 core

The `draft` subcommand consumes LLM-produced output as input — it does **not** make live
LLM calls. The pipeline is:

```
raw document → external LLM tool → extraction JSON → meshant draft → TraceDraft records
```

This keeps the CLI stdlib-only, makes the LLM's transformation visible as a discrete step,
and is consistent with treating the LLM as a mediator (not a hidden intermediary). The LLM
boundary moves internal only at the interactive CLI layer (v2.0.0).

### Type name: TraceDraft, not CandidateTrace

`TraceDraft` is the preferred name. `CandidateTrace` implies a record already on the way to
promotion; `TraceDraft` implies something weaker — incomplete, provenance-heavy, possibly
never promoted. A draft is a legitimate analytical object in its own right, not merely a
trace that has not yet passed validation.

The CLI verb `draft` is independent of the type name. `meshant draft` producing `TraceDraft`
records is coherent.

### The extraction chain must be structurally followable

Even though M11 does not implement the full critique pipeline, `TraceDraft` must carry the
fields needed for that pipeline to be visible later:

- `ExtractionStage` — where in the pipeline this draft was produced
- `ExtractedBy` — who or what produced it (human, llm-pass1, reviewer, etc.)
- `DerivedFrom` — ID of parent draft (links revisions into a chain)

This means the ingestion pipeline itself — raw span → LLM draft → critique → human revision
→ accepted trace — is a sequence of `TraceDraft` records linked by `DerivedFrom`. When M11.5
(critique pass) arrives, that chain already exists in the data. MeshAnt can eventually follow
it as a translation chain, classifying each step: did the LLM act as an intermediary (passed
span unchanged) or a mediator (transformed it)?

The LLM appearing as a node in the mesh is not a corruption of the framework. It is one of
the most faithful ways to represent that extraction is itself a mediated process.

### Ingestion contract: TraceDraft minus framework-assigned fields

The external LLM tool targets a minimal JSON format (the **ingestion contract**) that maps
directly onto `TraceDraft`. MeshAnt's `draft` command reads that JSON, assigns IDs, stamps
provenance fields (`ExtractionStage`, `ExtractedBy`, `Timestamp`), and writes canonical
`TraceDraft` records.

The contract requires only `source_span`. Everything else is optional — the LLM is
encouraged to leave fields empty rather than fabricate confident assignments.

---

## Non-goals

- Live LLM calls from the CLI (deferred to v2.0.0)
- Anti-ontology critique pass (deferred to M11.5 / M12)
- Interactive trace review (deferred to M11.5 / M12)
- Per-step criteria for the ingestion chain (deferred to M12+)
- Graphiti / Neo4j adapter (deferred — future-compatible boundary only)

---

## Example domain: CVE vulnerability response

The dataset for M11 is a CVE/dependency vulnerability response scenario. This domain was
chosen because:
- Every engineer knows the workflow (CVE alert → triage → fix → deploy)
- The raw document form is natural: a CVE advisory is unstructured prose an LLM would
  confidently mis-actorize ("attacker" as a stable entity, CVSS score as an agent)
- Strong regime crossings make the promoted traces analytically interesting with `follow`:
  security advisory → risk assessment → engineering decision → deployment gate
- Multiple non-human mediators: CVSS scorer, dependency scanner, CI pipeline, deployment gate
- Observer asymmetry: scanner sees a score; engineer sees blast radius; pipeline sees go/no-go

### Dataset files

**`data/examples/cve_response_raw.md`** (or `.txt`) — raw source material:
- A CVE advisory excerpt (CVSS score, affected versions, description)
- A Dependabot alert body
- Brief engineer triage notes
- Security review sign-off
- Deployment approval note
Approximately 1 page. This is the document a user would bring to MeshAnt.

**`data/examples/cve_response_extraction.json`** — pre-made extraction JSON:
- ~12–15 span extractions simulating LLM pass-1 output
- Follows the ingestion contract: `source_span` required, fields incomplete, uncertainty
  notes where the source is ambiguous, `extracted_by: "llm-pass1"`, `extraction_stage: "weak-draft"`
- Intentionally includes 1–2 over-actorized drafts (seeds the future critique pass demo)

**`data/examples/cve_response_drafts.json`** — TraceDraft output:
- What `meshant draft cve_response_extraction.json` produces
- Provided as a fixture for `promote` testing and documentation

---

## Phases

### Phase 0: CVE vulnerability response dataset

**Files:**
- `data/examples/cve_response_raw.md` — raw source document
- `data/examples/cve_response_extraction.json` — pre-made LLM extraction fixture
- `data/examples/cve_response_drafts.json` — expected TraceDraft output (for tests)

The extraction JSON must follow the ingestion contract: `source_span` required, all
other fields optional, uncertainty notes used where the source is ambiguous.

### Phase 1: Define `TraceDraft` type

**File:** `meshant/schema/tracedraft.go` (new)

```go
// TraceDraft is a provisional, provenance-bearing record produced during
// ingestion. It is not a Trace — it may be incomplete, unresolved, or
// explicitly uncertain. A TraceDraft is a legitimate analytical object in
// its own right. It may be promoted to a canonical Trace when its fields
// are sufficient, or it may remain a draft indefinitely.
//
// The extraction pipeline (span → LLM draft → critique → human revision →
// canonical trace) is represented as a chain of TraceDraft records linked
// by DerivedFrom. This makes the ingestion process itself followable and
// inspectable — the LLM is a mediator in the chain, not a hidden extractor.
type TraceDraft struct {
    // Framework-assigned fields
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`

    // Source material fields
    SourceSpan   string `json:"source_span"`             // verbatim text from source (required)
    SourceDocRef string `json:"source_doc_ref,omitempty"` // document identifier / path / URL

    // Candidate trace fields (all optional at draft stage)
    WhatChanged string   `json:"what_changed,omitempty"`
    Source      []string `json:"source,omitempty"`
    Target      []string `json:"target,omitempty"`
    Mediation   string   `json:"mediation,omitempty"`
    Observer    string   `json:"observer,omitempty"`
    Tags        []string `json:"tags,omitempty"`

    // Uncertainty and provenance fields
    UncertaintyNote string `json:"uncertainty_note,omitempty"` // where the source doesn't support confident assignment
    ExtractionStage string `json:"extraction_stage,omitempty"` // e.g. "span-harvest", "weak-draft", "reviewed"
    ExtractedBy     string `json:"extracted_by,omitempty"`     // e.g. "human", "llm-pass1", "reviewer"
    DerivedFrom     string `json:"derived_from,omitempty"`     // ID of parent draft (links revisions into a chain)
}
```

Methods:
- `Validate() error` — only `SourceSpan` and `ID` required; all other fields optional;
  error if `SourceSpan` is empty
- `IsPromotable() bool` — returns true when fields sufficient for canonical `Trace`:
  `WhatChanged` non-empty, `Observer` non-empty, `ID` parseable as UUID
- `Promote() (schema.Trace, error)` — converts to canonical `Trace`; sets `Tags` to
  include `TagValueDraft` (new constant); errors if `IsPromotable()` returns false

New tag constant in `meshant/schema/trace.go`:
- `TagValueDraft TagValue = "draft"` — marks a trace that was promoted from a TraceDraft;
  carries provenance signal that this trace passed through the ingestion pipeline

**Tests:** `meshant/schema/tracedraft_test.go`
- Zero value detection
- Validate: empty SourceSpan → error
- Validate: SourceSpan only → ok
- Validate: full fields → ok
- IsPromotable: missing WhatChanged → false
- IsPromotable: missing Observer → false
- IsPromotable: all required present → true
- Promote: success path → valid Trace, passes Trace.Validate(), has "draft" tag
- Promote: not promotable → error
- DerivedFrom chain: two drafts linked by ID (structural test)

---

### Phase 2: Draft loader

**File:** `meshant/loader/draftloader.go` (new)

Functions:
- `LoadDrafts(path string) ([]schema.TraceDraft, error)` — reads JSON array of TraceDraft
  records; assigns UUIDs to any records missing ID; max 50 MB (consistent with Load)
- `SummariseDrafts(drafts []schema.TraceDraft) DraftSummary` — counts records by
  ExtractionStage, ExtractedBy; counts promotable vs not; counts fields filled vs empty
- `PrintDraftSummary(w io.Writer, s DraftSummary) error` — renders summary to writer

New type:
```go
type DraftSummary struct {
    Total          int
    Promotable     int
    ByStage        map[string]int  // ExtractionStage → count
    ByExtractedBy  map[string]int  // ExtractedBy → count
    FieldFillRate  map[string]int  // field name → count with non-empty value
}
```

**Tests:** `meshant/loader/draftloader_test.go`
- LoadDrafts: valid file → correct count
- LoadDrafts: ID auto-assigned when missing
- LoadDrafts: empty SourceSpan → validation error
- SummariseDrafts: counts by stage, by extracted_by, promotable count
- PrintDraftSummary: output contains expected fields

---

### Phase 3: Ingestion contract + `draft` CLI subcommand

**Ingestion contract** — the format the external LLM tool must produce. Defined as a JSON
array of objects matching this minimal schema (a subset of TraceDraft):

```json
[
  {
    "source_span": "The routing matrix redirected the form to a secondary queue.",
    "what_changed": "routing-matrix redirected form-submission-3847",
    "source": ["form-submission-3847"],
    "target": ["secondary-queue"],
    "mediation": "secondary-queue routing rule applied",
    "observer": "queue-monitor",
    "uncertainty_note": "observer identity inferred, not explicit in source",
    "extraction_stage": "weak-draft",
    "extracted_by": "llm-pass1"
  }
]
```

Rules:
- `source_span` is required; everything else is optional
- Leave fields empty rather than fabricating confident assignments
- Use `uncertainty_note` to name where the source does not justify confident assignment
- `extraction_stage` should be one of: `span-harvest`, `weak-draft`, `reviewed`
- `extracted_by` should identify the extraction pass: `human`, `llm-pass1`, `llm-pass2`,
  `reviewer`, etc.

**File:** `meshant/cmd/meshant/main.go` (add `cmdDraft`)

Add to `follow` subcommand pattern:

```
meshant draft [--source-doc <ref>] [--extracted-by <label>] [--stage <stage>]
              [--output <file>] <extraction.json>
```

Flags:
- `--source-doc <ref>` — document identifier stamped on all drafts (SourceDocRef)
- `--extracted-by <label>` — override the ExtractedBy field for all loaded drafts
- `--stage <stage>` — override the ExtractionStage field for all loaded drafts
- `--output <file>` — write TraceDraft JSON to file (default: stdout)

Behaviour:
1. Read extraction JSON (LLM-produced, matches ingestion contract)
2. Assign UUIDs to records missing ID; stamp Timestamp
3. Apply `--source-doc`, `--extracted-by`, `--stage` overrides if provided
4. Validate each draft (SourceSpan required)
5. Write TraceDraft JSON array + print summary to stdout

**Tests:** `meshant/cmd/meshant/main_test.go` (Group 12)
- Valid extraction file → draft output with correct count
- Missing source_span → error naming the record
- --source-doc stamps SourceDocRef on all drafts
- --extracted-by overrides ExtractedBy on all drafts
- --output writes to file
- Empty extraction file → error
- Malformed JSON → error

---

### Phase 4: `promote` CLI subcommand

**File:** `meshant/cmd/meshant/main.go` (add `cmdPromote`)

```
meshant promote [--output <file>] <drafts.json>
```

Behaviour:
1. Load drafts from JSON
2. For each draft: call `IsPromotable()`; if true, call `Promote()`
3. Collect promoted traces + failed drafts
4. Write promoted traces to `--output` (or stdout)
5. Print summary: N promoted, M not promotable (with field-missing reasons)

**Tests:** `meshant/cmd/meshant/main_test.go` (Group 13)
- All promotable drafts → trace output, trace count matches
- Mixed promotable/not → partial output, summary names failures
- None promotable → no output, error report
- --output writes promoted traces to file

---

### Phase 5: Review, clean, and document

**Step 1 — Refactor-cleaner pass**
Run refactor-cleaner agent across all M11 files:
- `meshant/schema/tracedraft.go` + tests
- `meshant/loader/draftloader.go` + tests
- `meshant/cmd/meshant/main.go` (draft + promote additions) + tests

Fix any actionable findings before proceeding to the philosophical review.

**Step 2 — Philosophical (ANT) review**
Run ant-theorist agent across all M11 code and design. Key checks:
- LLM-as-mediator commitment visible in code and output (not hidden extractor)
- `DerivedFrom` chain preserves the extraction pipeline as followable
- `SourceSpan` as ground truth is structurally enforced
- Empty-over-fabricated principle reflected in `Validate()` and ingestion contract
- `TagValueDraft` does not reify ingestion provenance as ontology
- No naming, typing, or ordering decisions that misrepresent ANT commitments

**Step 3 — Decision record + codemap**
Only after both reviews pass:

**Files:**
- `docs/decisions/tracedraft-v2.md`
  - Name the LLM-as-mediator commitment (not hidden extractor)
  - Document the ingestion contract
  - Acknowledge ExtractionStage/ExtractedBy/DerivedFrom as load-bearing for M11.5
  - Note `TagValueDraft` as provenance signal on promoted traces
  - Acknowledge what M11 does NOT do: critique pass, interactive review, live LLM calls
- `docs/CODEMAPS/meshant.md` — updated for M11

---

## Key design rules

1. **SourceSpan is the ground truth.** Everything else is derived, uncertain, or optional.
   A TraceDraft with only a source span is valid. It preserves the text that provoked the
   extraction without forcing premature resolution.

2. **Empty is better than fabricated.** The ingestion contract explicitly encourages leaving
   fields empty. A blank `Observer` is more honest than a confidently wrong one. The
   `UncertaintyNote` is the field for explaining why.

3. **The extraction chain is structurally followable from day one.** `DerivedFrom` links
   draft revisions into a chain. When the critique milestone arrives, that chain already
   exists. MeshAnt can follow it without retrofitting.

4. **Promotion is a deliberate act.** `Promote()` is not automatic. The analyst calls it
   when a draft is ready. The promoted Trace carries `"draft"` tag as a provenance signal.

5. **LLM boundary is explicit.** The `draft` command reads a file. It does not call an API.
   The boundary between the LLM's transformation and MeshAnt's ingestion is a named file
   on disk — inspectable, version-controllable, and replayable.

---

## What this enables (M11.5 / M12, not planned here)

- **Anti-ontology critique pass**: a second-pass `meshant critique <drafts.json>` that reads
  existing drafts and produces critique records (DerivedFrom linking critique to draft)
- **Interactive review**: human-in-the-loop refinement of drafts before promotion
- **Ingestion chain as translation chain**: follow the DerivedFrom chain of a draft from
  span-harvest to canonical trace; classify each step (did the LLM act as intermediary or
  mediator?). The LLM's transformation becomes visible as a classified step in the mesh.
- **LLM as node in the mesh**: when reflexive tracing is applied to the ingestion pipeline,
  the LLM's extraction act appears as a trace; the ingestion chain becomes articulable.

---

## Estimated scope

- Phase 1: small (new file, ~80 lines + tests)
- Phase 2: small-medium (new loader file, ~60 lines + tests)
- Phase 3: medium (CLI subcommand + ingestion contract + tests)
- Phase 4: small (CLI subcommand + tests)
- Phase 5: docs only

Total: moderate scope. Conceptually significant as the first ingestion entrypoint.
