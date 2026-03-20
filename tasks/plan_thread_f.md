# Thread F — LLM-Internal Boundary (v2.0.0)

## Overview

Thread F moves the LLM boundary inside the MeshAnt CLI. In v1.x the LLM is external:
users run an LLM separately and feed its JSON output to `meshant draft`. In v2.0.0 the
CLI calls the LLM directly through three new subcommands — `extract`, `assist`, and
`critique` — while preserving every ANT commitment the project has made so far.

The governing principle is that the LLM is a **mediator**, never a neutral extractor.
Its output is a candidate draft, not a trace. Every LLM-produced draft carries
provenance (`ExtractedBy` = model ID string, `ExtractionStage` = `"weak-draft"`),
uncertainty (`UncertaintyNote` set by the framework, never by the LLM), and a session
link (`SessionRef` pointing to a `SessionRecord` that preserves the full extraction
conditions). The LLM's transformations are visible, inspectable, and contestable. No
draft enters the mesh without a named analytical position and no session completes
without a written record — even on error.

**ANT commitments governing this thread:**

- FM1 (Provenance): `ExtractionConditions` + `SessionRef` on every LLM-produced draft;
  `UncertaintyNote` set by framework code, never delegated to the LLM.
- FM2 (Vocabulary): Function names use "extraction", "suggestion", "critique" — never
  "analysis", "scoring", or "accuracy".
- FM3 (System instructions): Prompt templates enforce trace-first vocabulary;
  `IntentionallyBlank` required for empty fields in LLM output.
- FM4 (Session record): `SessionRecord` is a mandatory return value; per-draft
  disposition (accepted/edited/skipped); written even when output goes to stdout.

---

## Planner's Decisions on Open Questions

### 1. Span splitting for `assist`

**Decision: User-supplied spans only in v2.0.0; LLM-assisted splitting deferred.**

The `assist` command reads a source document that has already been split into spans
(one JSON array of `{"source_span": "..."}` objects, or a plain-text file where each
span is separated by a blank line). Heuristic or LLM-assisted span splitting is a
meaningful problem but it is not the core of Thread F. Adding it now would complicate
the session loop and create a second LLM call whose provenance must also be tracked.

Deferred to a follow-up issue: `meshant split` command that produces span-separated
output, with its own `SessionRecord`. This keeps `assist` focused on the
suggest-confirm loop.

### 2. `assist` session: reuse `review` package or reimplement?

**Decision: Reuse `review` for rendering and ambiguity detection; new session loop in
`llm` package.**

The `review` package's `RunReviewSession` is tightly coupled to the weak-draft filter
and the accept/edit/skip/quit action set without LLM interaction. The `assist` session
has a different flow: present a span, call the LLM, show the suggestion, then
accept/edit/skip. Rather than force `RunReviewSession` to handle both paths, the `llm`
package will implement `RunAssistSession` with its own loop, but it will import:

- `review.RenderDraft` — for displaying the LLM suggestion to the user
- `review.DetectAmbiguities` — for surfacing structural ambiguity in the suggestion
- `review.RenderChain` — for showing the derivation chain as it grows

This avoids reimplementing rendering while keeping the session logic clean.

### 3. `main.go` file split

**Decision: Split before adding Thread F subcommands.**

`main.go` is already ~2010 lines (tracked in Deferred Items since Thread C). Thread F
adds three subcommands (`extract`, `assist`, `critique`), each with flag parsing,
validation, and output logic.

The split happens as a prerequisite step (Phase 0, before F.0). Each existing
subcommand handler moves to its own file within `cmd/meshant/`:

```
cmd/meshant/
  main.go               — main(), run(), usage(), shared helpers
  cmd_summarize.go      — cmdSummarize
  cmd_validate.go       — cmdValidate
  cmd_articulate.go     — cmdArticulate
  cmd_diff.go           — cmdDiff
  cmd_follow.go         — cmdFollow
  cmd_draft.go          — cmdDraft
  cmd_promote.go        — cmdPromote
  cmd_rearticulate.go   — cmdRearticulate
  cmd_lineage.go        — cmdLineage
  cmd_shadow.go         — cmdShadow
  cmd_gaps.go           — cmdGaps
  cmd_bottleneck.go     — cmdBottleneck
  cmd_extraction_gap.go — cmdExtractionGap
  cmd_chain_diff.go     — cmdChainDiff
  cmd_review.go         — cmdReview
```

All files remain in `package main`. Tests stay in `main_test.go` (they test through
`run()` and do not import subcommand functions directly). This is a refactor-only
change — every existing test must pass unchanged.

### 4. API key management

**Decision: Environment variable only for v2.0.0.**

The `LLMClient` reads the API key from `MESHANT_LLM_API_KEY` (or a provider-specific
fallback like `ANTHROPIC_API_KEY`). No config file, no keyring, no interactive prompt.
The `llm` package validates at construction time that the key is present and non-empty;
a missing key produces a clear error naming the expected environment variable.

### 5. LLM refusal and malformed output

**Decision: Structured error types; retry is the caller's responsibility; SessionRecord
always written.**

The `llm` package defines two sentinel error types:

- `ErrLLMRefusal` — the LLM explicitly declined to produce output. Carries the refusal text.
- `ErrMalformedOutput` — the LLM returned text that does not parse as valid TraceDraft
  JSON. Carries the raw response for debugging.

On either error, the calling command writes the `SessionRecord` with zero `DraftCount`
and an error note in `SessionRecord.ErrorNote`. No automatic retry in v2.0.0.

---

## Schema Change: SessionRef on TraceDraft (F.0)

### Should this be standalone or bundled into F.1?

**Decision: Standalone issue, numbered F.0.**

`SessionRef` is a schema change that must be merged before any other Thread F issue.
F.2 (`extract`) is the first command that produces drafts with `SessionRef` set, and
its tests will assert that the field is present. Bundling it into F.1 (a decision
record) would create a dependency tangle.

### Scope

- Add `SessionRef string` to `TraceDraft` (json: `"session_ref,omitempty"`)
- Add `SessionRef` to `DraftSummary.FieldFillRate` tracking in `SummariseDrafts`
- Add `"session_ref"` to the ordered fields list in `PrintDraftSummary`
- Update `deriveAccepted` and `deriveEdited` in `review/session.go` to copy `SessionRef`
  from parent (a derived draft inherits its parent's session link)

### Files changed

- `meshant/schema/tracedraft.go` — new field + doc comment
- `meshant/loader/draftloader.go` — `SummariseDrafts` + `PrintDraftSummary`
- `meshant/review/session.go` — `deriveAccepted`, `deriveEdited`

### Tests (TDD)

- TraceDraft with `SessionRef` round-trips through JSON
- `SessionRef` is not transferred by `Promote()` — it is draft-layer provenance only
- `SummariseDrafts` counts `SessionRef` in `FieldFillRate`
- `deriveAccepted` preserves `SessionRef` from parent

### Design rules

- `SessionRef` is optional; empty is valid (backward-compatible with all existing data)
- `SessionRef` is not transferred by `Promote()` — provenance stays at the draft layer
- Existing tests must pass without modification (new field is `omitempty`)

### ANT constraints

- `SessionRef` links each draft to its extraction conditions, making the LLM's
  analytical position inspectable beyond what `ExtractedBy` alone provides (ant-theorist
  requirement: structural guarantee, not file-co-location convention)

---

## F.1 — LLM Mediator Convention

### Scope

Write `docs/decisions/llm-as-mediator-v1.md`. Define the convention before the code.
Also define the `"critiqued"` ExtractionStage value and write the extraction prompt
template.

### Key files

- `docs/decisions/llm-as-mediator-v1.md` — new decision record
- `meshant/schema/tracedraft.go` — update `ExtractionStage` doc comment to list
  `"critiqued"` as a known value
- `data/prompts/extraction_pass.md` — extraction prompt template (system instructions
  enforcing trace-first vocabulary, `IntentionallyBlank` requirement)

### Content of decision record (7 decisions)

1. **LLM is a mediator, not an extractor.** Its output is a candidate draft. It
   transforms source material; the transformation is visible and contestable.
2. **`ExtractedBy` uses model ID strings.** `"claude-sonnet-4-6"`, never generic
   `"llm"`. Two runs with different models produce drafts with different `ExtractedBy`.
3. **`UncertaintyNote` is set by framework code.** The LLM may suggest content, but the
   framework appends: `"LLM-extracted; unverified by human review"`. This prevents the
   LLM from claiming certainty it cannot have.
4. **`ExtractionStage` known values:**
   - `"span-harvest"` — raw spans, no candidate fields
   - `"weak-draft"` — candidate fields populated (human or LLM)
   - `"critiqued"` — LLM re-articulation of an existing draft (new in Thread F)
   - `"reviewed"` — human decision (accept/edit in review session)
5. **`"critiqued"` vs `"reviewed"`:** A critiqued draft is an LLM suggestion (mediating
   act), not a human decision (curatorial act). They are distinct epistemic positions.
   `"critiqued"` is not ranked above or below `"weak-draft"` — stages name positions,
   not quality levels.
6. **`SessionRecord` is mandatory.** Every LLM interaction produces a `SessionRecord`
   even on error. The session ID is the `SessionRef` on produced drafts.
7. **`IntentionallyBlank` required in LLM output.** System instructions require the LLM
   to set `intentionally_blank` on any content field it deliberately leaves empty.

### Test strategy

- No code tests (decision record + doc comment update)
- Prompt template is a Markdown file; validated by `go vet` on doc comment change

### Dependencies

- F.0 (SessionRef referenced in decision record)

---

## F.2 — `meshant extract`

### Scope

New `meshant extract` subcommand. Introduces the `meshant/llm` package with the
`LLMClient` interface, `ExtractionConditions`, `SessionRecord`, `DraftDisposition`
types, and `RunExtraction`.

### New package: `meshant/llm`

```
meshant/llm/
  client.go       — LLMClient interface, NewAnthropicClient constructor
  types.go        — ExtractionConditions, SessionRecord, DraftDisposition, ErrLLMRefusal,
                    ErrMalformedOutput
  extract.go      — RunExtraction
  prompt.go       — LoadPromptTemplate
```

**Dependency direction:**

```
cmd/meshant  -->  llm  -->  loader  -->  schema
                  llm  -->  schema
```

`llm` must NOT import `graph`, `review`, `persist`, or `cmd/meshant`.

### Key types

```go
type LLMClient interface {
    Complete(ctx context.Context, system, prompt string) (string, error)
}

type ExtractionConditions struct {
    ModelID            string    `json:"model_id"`
    PromptTemplate     string    `json:"prompt_template"`
    CriterionRef       string    `json:"criterion_ref,omitempty"`
    SystemInstructions string    `json:"system_instructions"`
    SourceDocRef       string    `json:"source_doc_ref"`
    Timestamp          time.Time `json:"timestamp"`
}

type DraftDisposition struct {
    DraftID string `json:"draft_id"`
    Action  string `json:"action"` // "accepted", "edited", "skipped"
}

type SessionRecord struct {
    ID           string              `json:"id"`
    Command      string              `json:"command"` // "extract", "assist", "critique"
    Conditions   ExtractionConditions `json:"conditions"`
    DraftIDs     []string            `json:"draft_ids"`
    Dispositions []DraftDisposition  `json:"dispositions,omitempty"`
    InputPath    string              `json:"input_path"`
    OutputPath   string              `json:"output_path"`
    DraftCount   int                 `json:"draft_count"`
    ErrorNote    string              `json:"error_note,omitempty"`
    Timestamp    time.Time           `json:"timestamp"`
}
```

### Function signature

```go
func RunExtraction(ctx context.Context, client LLMClient, opts ExtractionOptions) ([]schema.TraceDraft, SessionRecord, error)
```

### Data flow

1. Read source document from `opts.InputPath`
2. Load prompt template from `opts.PromptTemplatePath`
3. Assemble system instructions (trace-first vocabulary rules)
4. Call `client.Complete(ctx, system, prompt)`
5. Parse LLM response as JSON array of partial `TraceDraft`
6. For each parsed draft: assign UUID, set `ExtractedBy` = model ID,
   `ExtractionStage` = `"weak-draft"`, `SessionRef` = session ID,
   `UncertaintyNote` = framework-appended, `SourceDocRef` = opts value
7. Write drafts JSON to `--output` or stdout
8. Write `SessionRecord` to `.session.json` sibling file (always)

### CLI flags

```
--source-doc <path>          source document file path (required)
--prompt-template <path>     default: data/prompts/extraction_pass.md
--criterion-file <path>      optional EquivalenceCriterion
--model <id>                 default: claude-sonnet-4-6
--output <path>              output file (default: stdout)
--session-output <path>      session record file (default: <output>.session.json)
```

### Test strategy (TDD)

All tests use a mock `LLMClient` — no real API calls.

**`llm/extract_test.go`** (write first):
- `TestRunExtraction_HappyPath` — mock returns valid JSON; assert correct provenance fields
- `TestRunExtraction_EmptyResponse` — mock returns `"[]"`; zero drafts, non-nil SessionRecord
- `TestRunExtraction_MalformedResponse` — assert `ErrMalformedOutput`; SessionRecord returned with ErrorNote
- `TestRunExtraction_Refusal` — assert `ErrLLMRefusal`; SessionRecord returned
- `TestRunExtraction_UncertaintyNote_FrameworkAppended` — framework appends even if LLM sets the field
- `TestRunExtraction_SessionRef_OnEveryDraft` — every draft has non-empty SessionRef

**`llm/client_test.go`**:
- `TestNewAnthropicClient_MissingKey` — clear error naming env var
- `TestNewAnthropicClient_EmptyKey` — clear error

**`llm/prompt_test.go`**:
- `TestLoadPromptTemplate_Valid`
- `TestLoadPromptTemplate_Missing`

**`cmd/meshant/main_test.go`** (integration):
- `TestCmdExtract_BasicRun` — mock client injected; assert output JSON + session file
- `TestCmdExtract_MissingSourceDoc`
- `TestCmdExtract_MissingModel`

*Note: `cmdExtract` accepts an optional `LLMClient` parameter (nil = construct real
client from env). Tests inject mock. Same pattern as `cmdReview` with `io.Reader`.*

### Design rules

- `RunExtraction` returns a non-nil `SessionRecord` on every code path, including errors
- `UncertaintyNote` is appended by framework, never delegated to the LLM
- `ExtractionConditions` must never hold the API key
- Source document size is capped (define `maxSourceBytes` constant)
- `llm` package must not import `graph`

### Dependencies

- F.0 (SessionRef on TraceDraft)
- F.1 (conventions + extraction prompt template)

### ANT constraints

- FM1: ExtractionConditions + SessionRef on every draft
- FM2: `RunExtraction` not `RunAnalysis`; no accuracy-scoring functions exist
- FM3: system instructions enforce trace-first vocabulary
- FM4: SessionRecord always returned; written to file even on error

---

## F.3 — `meshant assist`

### Scope

New `meshant assist` subcommand. Interactive session: for each source span, the LLM
suggests candidate trace fields, the user accepts/edits/skips. Both LLM drafts and
derived human drafts are written to output. Skipped drafts are preserved.

### New file

```
meshant/llm/assist.go — RunAssistSession
```

`assist.go` imports `review` package for rendering (`review.RenderDraft`,
`review.DetectAmbiguities`, `review.RenderChain`). The session loop itself is in
`llm`, not in `review`.

### Function signature

```go
func RunAssistSession(ctx context.Context, client LLMClient, spans []string, opts AssistOptions, in io.Reader, out io.Writer) ([]schema.TraceDraft, SessionRecord, error)
```

### Session loop per span

1. Display source span to user (via `out`)
2. Call `client.Complete` with span + extraction prompt
3. Assign UUID to LLM draft immediately (required for `DerivedFrom` if user edits)
4. Set framework provenance on LLM draft (`ExtractedBy`, `ExtractionStage: "weak-draft"`,
   `SessionRef`, `UncertaintyNote`)
5. Display using `review.RenderDraft` + `review.DetectAmbiguities`
6. Prompt: `[a]ccept  [e]dit  [s]kip  [q]uit >`
7. **Accept:** disposition = `"accepted"`; add LLM draft to results
8. **Edit:** field-by-field edit; derive new draft with `DerivedFrom` = LLM draft ID,
   `ExtractionStage: "reviewed"`, `ExtractedBy: "meshant-assist"`;
   disposition = `"edited"`; add both LLM draft and derived draft to results
9. **Skip:** disposition = `"skipped"`; LLM draft written to output with `"weak-draft"`
   stage — **not discarded** (it is a record of what the LLM's position made visible)
10. **Quit:** return partial results + SessionRecord

*On LLM error for a span:* display error; offer `[s]kip span / [q]uit session >`;
record error disposition in SessionRecord.

### Span input format

- JSON array of `{"source_span": "..."}` objects (same shape as `meshant draft` input)
- JSON array of strings
- Plain text with blank-line separators (detected by absence of `[` at start)

### Test strategy (TDD)

**`llm/assist_test.go`** (write first):
- `TestRunAssistSession_AcceptAll` — all spans accepted; assert correct provenance
- `TestRunAssistSession_EditOne` — user edits one field; assert derived draft with
  `DerivedFrom` + `ExtractionStage: "reviewed"`
- `TestRunAssistSession_SkipWritesDraft` — skipped LLM draft appears in output with
  `"weak-draft"` stage
- `TestRunAssistSession_Quit` — partial results returned; SessionRecord has correct count
- `TestRunAssistSession_LLMRefusal` — one span fails; session continues to next
- `TestRunAssistSession_SessionRecord_AlwaysReturned` — on quit, error, completion
- `TestRunAssistSession_DispositionsTracked` — SessionRecord.Dispositions has entry per
  span with correct action

**`cmd/meshant/main_test.go`** (integration):
- `TestCmdAssist_BasicRun` — mock client + strings.Reader for stdin

### Design rules

- `RunAssistSession` returns non-nil `SessionRecord` on every code path
- Skipped drafts are written to output, never discarded
- The edit flow produces a new derived draft — never mutates the LLM suggestion
- Prompts go to `out` (testable), not `os.Stderr` directly
- `SessionRecord.Dispositions` records every draft's fate

### Dependencies

- F.0 (SessionRef)
- F.1 (conventions)
- F.2 (`llm` package, `LLMClient`, `ExtractionConditions`, `SessionRecord`, error types)

### ANT constraints

- Skipped drafts are preserved — discarding them would erase the LLM's position from
  the record (principle: shadow is not absence)
- The session record's per-draft dispositions make the human's curatorial decisions
  visible data, not hidden metadata (FM4)
- The edit produces a derivation chain: LLM suggestion → human revision; this chain
  is classifiable by `ClassifyDraftChain` exactly as any human-produced chain

---

## F.4 — `meshant critique`

### Scope

New `meshant critique` subcommand. Reads existing drafts, sends each to the LLM with
the critique prompt, produces derived drafts with `ExtractionStage: "critiqued"` and
`DerivedFrom` linking to the original.

Also updates `filterReviewable` in `review/session.go` to include `"critiqued"` drafts
(so they appear in the `review` session queue after critique).

### New file

```
meshant/llm/critique.go — RunCritique
```

### Function signature

```go
func RunCritique(ctx context.Context, client LLMClient, drafts []schema.TraceDraft, opts CritiqueOptions) ([]schema.TraceDraft, SessionRecord, error)
```

### Data flow per draft

1. Assemble critique prompt: critique template + draft fields as context + criterion (if any)
2. Call `client.Complete`
3. Parse response as single `TraceDraft`
4. Validate: `SourceSpan` must match original — hard check, reject on mismatch
5. Set provenance: `DerivedFrom` = original ID, `ExtractionStage: "critiqued"`,
   `ExtractedBy` = model ID, `SessionRef` = session ID, `UncertaintyNote` (framework)
6. Continue on per-draft errors — partial results are valid

### CLI flags

```
--prompt-template <path>     default: data/prompts/critique_pass.md
--criterion-file <path>
--model <id>
--output <path>
--session-output <path>
--id <id>                    critique a single draft by ID
```

### Test strategy (TDD)

**`llm/critique_test.go`** (write first):
- `TestRunCritique_HappyPath` — assert `DerivedFrom`, `ExtractionStage: "critiqued"`,
  `ExtractedBy` = model ID
- `TestRunCritique_SourceSpanMismatch` — draft rejected; session continues; ErrorNote in record
- `TestRunCritique_MalformedResponse` — per-draft error; session continues
- `TestRunCritique_SessionRecord_AlwaysReturned`
- `TestRunCritique_IntentionallyBlank` — LLM sets `IntentionallyBlank`; framework preserves it
- `TestRunCritique_SingleDraft_ViaID` — `--id` flag filters to one draft

**`review/session_test.go`** (update):
- `TestFilterReviewable_IncludesCritiqued` — a `"critiqued"` draft appears in queue

### Design rules

- `RunCritique` returns non-nil `SessionRecord` even if all drafts fail
- `RunCritique` processes all drafts even if some fail — partial results valid
- `SourceSpan` integrity is a hard check: LLM cannot change the span it was given
- `"critiqued"` is not ranked above `"weak-draft"` — it names a position, not quality

### Dependencies

- F.0 (SessionRef)
- F.1 (`"critiqued"` stage defined)
- F.2 (`llm` package infrastructure)

### ANT constraints

- A critique is a second cut, not a correction — the original and the critique have
  equal standing in the derivation chain
- `"critiqued"` explicitly names the LLM's epistemic status: suggestion, not decision

---

## F.5 — Real-World LLM-Assisted Extraction Example

### Scope

A complete end-to-end worked example demonstrating the v2.0.0 pipeline:
source document → `meshant extract` → `meshant review` → `meshant promote` →
`meshant articulate`. Documents where the LLM agreed with human review and where it
diverged.

### Key files

```
data/examples/llm_assisted_extraction/
  source_document.md        — real-world source document (new domain)
  extraction_session.json   — SessionRecord from meshant extract
  raw_drafts.json           — LLM-produced drafts (weak-draft stage)
  reviewed_drafts.json      — after human review (reviewed stage)
  promoted_traces.json      — canonical traces after promotion
  articulation_output.json  — articulated graph
  README.md                 — walkthrough: pipeline, LLM divergences, observations
```

### Source document choice

The source document should be a domain the existing example datasets do not cover.
Candidates: a publicly available postmortem, a policy change announcement, or a
multi-party coordination transcript. The document should be short enough to produce
5–10 spans but rich enough to generate at least 2 analytical divergences between the
LLM's reading and the human reviewer's reading.

### Requirements for README

- Name where the LLM diverged from human review and explain *why* the divergence is
  analytically meaningful (not "the LLM got it wrong")
- Include at least one span where the LLM's reading was more faithful to the source
  than the human's initial extraction (to avoid presenting the LLM as uniformly deficient)
- The README names its own cut: "this example uses one observer position; a different
  position would produce different extractions"
- SessionRecords are included — full provenance is part of the example

### Test strategy

Validation tests in `meshant/loader/`:
- Load each JSON file, assert valid
- Assert provenance chain integrity: every reviewed draft has `DerivedFrom` pointing
  to a raw draft; every `SessionRef` value appears in `extraction_session.json`

### Dependencies

- F.2 (extract command, real API call needed to generate)
- F.3 and F.4 optional for this example

---

## F.6 — Decision Record + Docs + v2.0.0 Release

### Scope

Final documentation, codemap update, and release.

### Key files

- `docs/decisions/llm-boundary-v2.md` — Thread F implementation decision record
- `docs/CODEMAPS/meshant.md` — updated with `llm` package, new subcommands, new types
- `README.md` — v2.0.0 section: new commands, API key setup, LLM integration note
- `tasks/todo.md` — Thread F marked complete
- Tag: `v2.0.0`

### Decision record content

1. `llm` package boundary (imports only `schema` and `loader`)
2. `LLMClient` interface design (single `Complete` method)
3. `SessionRecord` as mandatory return + per-draft disposition
4. `"critiqued"` stage semantics
5. Span splitting deferred
6. API key from environment only
7. No automatic retry
8. `main.go` file split rationale and method
9. Option B strengthened (no ExtractionCut; ExtractionConditions + SessionRef as discipline)

### Dependencies

- All of Phase 0, F.0 through F.5

---

## Phase 0: CLI File Split (Prerequisite)

Standalone GitHub Issue + PR before F.0. Refactor only — no behavioral changes.

Moves all `cmd*` functions from `main.go` into per-subcommand files within `package main`.
Shared helpers (`outputWriter`, `confirmOutput`, `parseTimeFlag`, `parseTimeWindow`,
`stringSliceFlag`, `loadCriterionFile`) remain in `main.go`.

**Test rule:** every existing test in `main_test.go` must pass unchanged.

---

## Implementation Order

```
Phase 0: CLI file split (standalone PR)
    |
    v
  F.0: SessionRef on TraceDraft
    |
    v
  F.1: LLM mediator convention — decision record + "critiqued" stage + extraction prompt
    |
    v
  F.2: meshant extract — llm package + all new types + RunExtraction + cmdExtract
    |
    +------------------------+
    v                        v
  F.3: meshant assist      F.4: meshant critique
    |                        |
    +------------------------+
    v
  F.5: Real-world example
    |
    v
  F.6: Decision record + docs + v2.0.0
```

---

## Deferred Items (create GitHub Issues before thread ships)

| Item | Reason deferred | Issue placeholder |
|---|---|---|
| `PromptHash` in `ExtractionConditions` | Content hash of prompt template for reproducibility; adds complexity without blocking v2.0.0 | "F.x: PromptHash in ExtractionConditions" |
| `meshant split` (LLM-assisted span splitting) | Standalone analytical operation; deferred to keep `assist` focused | "Post-F: meshant split — LLM-assisted span identification" |
| Retry logic for LLM calls | Idempotency + cost questions; no blocking need in v2.0.0 | "Post-F: LLM call retry strategy" |
| Session records promotable to Traces | Future extension; SessionRecord already carries enough information | "Post-F: promote SessionRecord to reflexive Trace" |

---

## ANT Tensions Thread F Names But Does Not Resolve

These belong in the F.6 decision record. Name them explicitly.

**T1: Framework-imposed UncertaintyNote suppresses LLM's own signal.**
The framework appends its own note to the LLM's `UncertaintyNote`. This prevents false
certainty, but it also subordinates the LLM's uncertainty signal to a blanket label.
A more nuanced approach would distinguish "LLM-expressed uncertainty" from
"framework-imposed epistemic status" as separate fields. Thread F does not resolve this.

**T2: `Complete` interface hides the LLM's internal mediation.**
The interface sees inputs and outputs, not the chain-of-thought inside. The LLM performs
mediations (compression, selection, vocabulary imposition) that are invisible through
this boundary. SessionRecord captures the session's external boundary; it cannot capture
the LLM's internal process. This is the same limit that applies to any mediator in the
mesh.

**T3: Model ID as system identifier vs. analytical position.**
`ExtractedBy: "claude-sonnet-4-6"` names a model version, not an analytical position.
The same model with different system instructions occupies a different position, but
`ExtractedBy` alone does not distinguish them. `SessionRef` mitigates this, but the
field itself carries the tension.

**T4: Session record storage is not structurally enforced.**
`SessionRecord` is mandatory as a return value, but its write location is governed by
`--session-output`. If the user pipes output to stdout without setting `--session-output`,
the record is lost. A stricter design would refuse to run without a session output path.
Thread F names this as a user-agency decision, not a bug.

**T5: `"critiqued"` introduces an implicit hierarchy between stages.**
Adding `"critiqued"` between `"weak-draft"` and `"reviewed"` creates pressure to read
the stages as a quality ranking. The field comment must explicitly state that stages
name positions, not rank quality. Implementation must not rely on any ordered comparison
between stage values.

---

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| LLM output format instability | Parse + validate every draft; `ErrMalformedOutput` on failure; raw response in SessionRecord for debugging |
| API key exposure in session records | `ExtractionConditions` never holds API key; constructor consumes it; verified in F.2 code review |
| CLI file split regressions | Mechanical move only; no logic changes; full test suite after each file move |
| Mock client diverges from real API | F.5 serves as integration test with real API key |
| Session record bloat from system instructions | Acceptable for v2.0.0; `PromptHash` (deferred) provides compact alternative if needed |

---

## Success Criteria

- [ ] `SessionRef` on TraceDraft; all existing tests pass unchanged
- [ ] `docs/decisions/llm-as-mediator-v1.md` written; `"critiqued"` stage documented
- [ ] `meshant/llm` package exists; imports only `schema` and `loader`
- [ ] `LLMClient` interface with mock implementation for all tests
- [ ] `meshant extract` produces TraceDraft JSON with correct provenance on every draft
- [ ] `meshant assist` interactive session works with mock client in tests
- [ ] `meshant critique` produces derived drafts with `"critiqued"` stage
- [ ] `SessionRecord` returned on every code path (success, error, quit)
- [ ] Per-draft disposition tracked in `SessionRecord` for `assist`
- [ ] Skipped drafts in `assist` are written to output, not discarded
- [ ] `filterReviewable` in `review/session.go` includes `"critiqued"` drafts
- [ ] Real-world example with full pipeline documented in `data/examples/llm_assisted_extraction/`
- [ ] `main.go` split into per-subcommand files
- [ ] `go test ./...` green; `go vet ./...` clean
- [ ] v2.0.0 tagged on `main`
