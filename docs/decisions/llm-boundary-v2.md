# Decision Record: LLM-Internal Boundary v2

**Date:** 2026-03-22
**Status:** Active
**Thread:** Thread F — LLM-Internal Boundary (v2.0.0)
**Packages:** `meshant/llm`, `meshant/cmd/meshant`, `meshant/schema`, `meshant/review`
**Related:** `docs/decisions/llm-as-mediator-v1.md`, `tasks/plan_thread_f.md`, `data/examples/llm_assisted_extraction/`

---

## What was decided

Thread F moves the LLM boundary inside the MeshAnt CLI. Nine implementation decisions govern
how that boundary was drawn: package structure, interface design, session record contracts,
stage semantics, span handling, API key management, error handling, the CLI file split, and
the decision not to introduce a separate ExtractionCut type.

---

## Context

In v1.x, the LLM is external. Users run an LLM separately and feed its JSON output to
`meshant draft`. The framework receives a file; the LLM boundary is the user's responsibility.

In v2.0.0, three new subcommands call the LLM directly:

- `meshant extract` — takes a source document, calls the LLM, produces `weak-draft` records
- `meshant assist` — interactive: presents each source span, calls the LLM, asks the user to
  accept/edit/skip the candidate draft
- `meshant critique` — takes existing drafts, calls the LLM to produce `critiqued` derived drafts

The governing convention for how the LLM participates was established before the code in
`docs/decisions/llm-as-mediator-v1.md` (F.1). This record documents the nine structural
decisions made during F.2–F.5 implementation.

---

## Decision 1: `llm` package boundary — imports only `schema` and `loader`

The `meshant/llm` package imports `meshant/schema` and `meshant/loader`. It does not import
`meshant/graph`, `meshant/review`, `meshant/persist`, or `meshant/cmd/meshant`.

This enforces a one-way dependency: `cmd/meshant` → `llm` → `loader` → `schema`. The LLM layer
is above the schema layer (it produces drafts) and below the command layer (commands orchestrate
calls and I/O). Breaking this would create cycles or force the `llm` package to carry graph or
CLI concerns.

The exception: `assist.go` imports `meshant/review` for rendering (`review.RenderDraft`,
`review.DetectAmbiguities`). This is a narrow, justified import — the rendering functions are
in `review` because they were written there for `meshant review`, and duplicating them in `llm`
would create divergence. The import is for display logic only; no session state from `review`
enters `llm`.

---

## Decision 2: `LLMClient` interface — single `Complete` method

```go
type LLMClient interface {
    Complete(ctx context.Context, system, prompt string) (string, error)
}
```

`Complete` receives a system prompt and a user prompt and returns a string. The interface is
minimal by design. A wider interface (streaming, structured output, tool-use) would lock the
framework to API surface that is not needed for v2.0.0 and would make mock implementations
for tests harder to write.

The single-method interface means tests can implement the interface with an inline struct or a
function-pointer mock. The production `AnthropicClient` implements the interface by calling the
Anthropic Messages API with a fixed `max_tokens` ceiling and a response size cap.

The tension here is that `Complete` hides the LLM's internal mediation — see T2 below.

---

## Decision 3: `SessionRecord` as mandatory return value; per-draft disposition

`RunExtraction`, `RunAssistSession`, and `RunCritique` return a non-nil `SessionRecord` on every
code path: success, partial completion, and error. The session record is written to disk even
when output goes to stdout without `--session-output` being set.

The `Dispositions` field tracks the human reviewer's decision about each draft in `assist`
sessions: `"accepted"`, `"edited"`, `"skipped"`, or `"abandoned"`. This makes the curatorial
act visible as data, not as hidden metadata in a log. The `extract` command uses no dispositions
(all drafts from `extract` are implicitly in the output); the `critique` command uses no
dispositions (no interactive curation step).

The practical consequence: a user can lose the session record only by running without
`--session-output` and without capturing stdout. This is named as a known user-agency gap in T4.

---

## Decision 4: `"critiqued"` stage semantics

`ExtractionStage: "critiqued"` marks a draft produced by `meshant critique`: an LLM
re-articulation of an existing draft, linked to the original via `DerivedFrom`. A `critiqued`
draft is not ranked above or below a `weak-draft`. Both are LLM-produced candidates from
different positions: `weak-draft` from the source document, `critiqued` from an existing draft.

`filterReviewable` in `review/session.go` includes `"critiqued"` drafts, so they appear in the
`meshant review` queue. The intent: after `meshant critique`, the user can run `meshant review`
on the critiqued output the same way they review a first-pass extraction. The stage names the
provenance, not the quality.

The concern that `"critiqued"` introduces an implicit hierarchy between stages is named as T5.
The field comment in `tracedraft.go` explicitly states: "stages name positions, not quality
levels."

---

## Decision 5: Span splitting deferred

`meshant assist` requires the user to supply a pre-split spans file. Heuristic or LLM-assisted
span splitting is not implemented in v2.0.0.

The `ParseSpans` function in `llm/assist.go` accepts three formats:
- JSON array of `{"source_span": "..."}` objects
- JSON array of strings
- Plain text with blank-line separators

Anything further (window-based splitting, LLM-assisted boundary detection) would add a second
LLM call with its own provenance requirements, complicating the session loop without resolving
the core design question of what a span boundary means analytically. Deferred to a follow-up:
`meshant split` as a standalone command with its own `SessionRecord`.

---

## Decision 6: API key from environment only

The `LLMClient` reads the API key from `MESHANT_LLM_API_KEY` (primary) or `ANTHROPIC_API_KEY`
(fallback). No config file, no keyring, no interactive prompt. The constructor validates at
construction time that the key is present and non-empty.

The `ExtractionConditions` struct never holds the API key. Two runs with different keys but the
same model, prompt, and source document are analytically indistinguishable — the key is an
operational credential, not an analytical parameter.

---

## Decision 7: No automatic retry

LLM call failures — network errors, malformed output, refusals — are not retried automatically.
The calling command writes a `SessionRecord` with `ErrorNote` set and returns an error. The user
decides whether to re-run.

Retry logic raises questions of idempotency (is a partial session record from the failed attempt
still valid?), cost visibility (how many retries is the user implicitly accepting?), and session
record semantics (does a retry produce a new session ID or mutate the existing one?). None of
these are resolved in v2.0.0. The `assist` command has per-span error recovery (skip/quit
prompt), which is the practical workaround for transient failures in interactive use.

---

## Decision 8: `main.go` file split

Before Thread F added subcommands, `cmd/meshant/main.go` had grown to ~2010 lines. The file
split (Phase 0, PR #123) moved each subcommand handler to its own file (`cmd_summarize.go`,
`cmd_validate.go`, etc.) within `package main`. The shared helpers (`outputWriter`,
`confirmOutput`, `parseTimeFlag`, `parseTimeWindow`, `stringSliceFlag`, `loadCriterionFile`)
remained in `main.go` alongside `main()`, `run()`, and `usage()`.

The split is purely structural: no behavioral change. All tests passed before and after the move
without modification. The motivation was to keep Thread F subcommand additions readable —
`cmd_extract.go`, `cmd_assist.go`, `cmd_critique.go` are each ~150–200 lines; adding them to a
2010-line file would make navigation difficult and diffs unreadable.

---

## Decision 9: No ExtractionCut type — ExtractionConditions + SessionRef as the discipline

An early design question (see `plan_thread_f.md`) was whether to introduce an `ExtractionCut`
type analogous to `Cut` in the graph layer: a named, first-class record of the analytical
position under which extraction occurred.

The decision was to not introduce `ExtractionCut`. `ExtractionConditions` (model ID, prompt
template, system instructions, source document reference, timestamp) combined with `SessionRef`
on every draft carries the same information without a new type. The discipline is enforced
structurally: every LLM-produced draft has a non-empty `SessionRef` pointing to a
`SessionRecord`, which contains the full `ExtractionConditions`.

The alternative would have made the extraction position more visible at the schema level but
would have required updating every draft-producing code path. The current approach is sufficient
for v2.0.0. If future analysis reveals that extraction positions need to be compared across
sessions the way observer positions are compared with `meshant diff`, an `ExtractionCut` can be
introduced then.

---

## ANT Tensions Named by Thread F

These tensions are not resolvable within the scope of v2.0.0. They are named here so that
future work can address them explicitly rather than silently inheriting them.

### T1: Framework-imposed UncertaintyNote suppresses the LLM's own uncertainty signal

The framework appends `"LLM-produced candidate; unverified by human review"` to every LLM
draft's `UncertaintyNote`, whether or not the LLM expressed its own uncertainty. A more nuanced
approach would preserve the LLM's expressed uncertainty separately from the framework's epistemic
status label. The current design prevents false certainty but subordinates the LLM's own signal
to a blanket annotation.

### T2: The `Complete` interface hides the LLM's internal mediation

`LLMClient.Complete` receives inputs and returns a string. The LLM's internal process —
chain-of-thought, compression, selection, vocabulary imposition — is invisible through this
boundary. `SessionRecord` captures the session's external conditions; it cannot capture the
LLM's internal mediation. This is the same limit that applies to any mediator in the mesh: the
mediator's internal process is not transparent through the interface that defines its boundary.

### T3: Model ID as system identifier vs. analytical position

`ExtractedBy: "claude-sonnet-4-6"` names a model version, not a full analytical position. The
same model with different system instructions occupies a different analytical position, but
`ExtractedBy` alone does not distinguish them. `SessionRef → SessionRecord.Conditions.SystemInstructions`
fully recovers the position, but the field itself carries the tension.

### T4: Session record storage is not structurally enforced

`SessionRecord` is mandatory as a return value, but its write location is governed by
`--session-output`. If the user runs `meshant extract` with output to stdout and does not set
`--session-output`, the session record is silently lost (printed to stdout interleaved with the
draft JSON when using `--session-output` is omitted). The design names this as a user-agency
decision: the framework writes the record to a side-channel file when possible, but does not
refuse to run without a session output path. A stricter design would require `--session-output`
when stdout is used. Thread F names this gap; it is not resolved.

### T5: `"critiqued"` introduces pressure toward a quality hierarchy in stage names

Adding `"critiqued"` between `"weak-draft"` and `"reviewed"` creates a visual sequence that
invites reading as a quality progression: draft → critique → review → canonical. The framework's
intention is that stages name analytical positions, not quality levels. The stage comment in
`tracedraft.go` states this explicitly. The naming choice still carries the tension: "critiqued"
sounds like a quality judgment. A name like `"llm-rearticulation"` would better resist the
quality reading.

---

## Deferred Items (to be filed as follow-up issues)

| Item | Reason deferred |
|---|---|
| `meshant split` — LLM-assisted span splitting | Standalone analytical operation; deferred to keep `assist` focused on the suggest-confirm loop. Own `SessionRecord` required. |
| `PromptHash` in `ExtractionConditions` | Content hash of prompt template for reproducibility; adds complexity without blocking v2.0.0. |
| Retry logic for LLM calls | Idempotency + cost questions; no blocking need in v2.0.0 (see Decision 7). |
| Session records promotable to Traces | `SessionRecord` already carries enough information; future extension. |
| `ExtractionCut` type | Deferred pending evidence that extraction positions need to be compared across sessions (see Decision 9). |
| CVE example directory migration | `data/examples/cve_response_*.json` pre-dates the subdirectory convention; low-priority cleanup. |
| Assist session record in example directory | `data/examples/llm_assisted_extraction/` includes `extraction_session.json` but not the assist session record; the assist session ref is hardcoded in tests. A complete example would include both. |
