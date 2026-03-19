# Decision Record: Interactive Review CLI v1

**Date:** 2026-03-19
**Status:** Active
**Milestone:** Thread A — Interactive Review CLI
**Packages:** `meshant/review`, `meshant/cmd/meshant` (cmdReview)
**Related:** `docs/decisions/tracedraft-v2.md`, `docs/decisions/cli-v2.md`, `tasks/plan_thread_a.md`

---

## What was decided

1. **Render functions return string, not write to io.Writer** — `RenderDraft`, `RenderChain`, `RenderAmbiguities` return a composed string; the caller writes it where it needs to go.
2. **ExtractedBy is "meshant-review" for both accept and edit** — the session apparatus is the actor regardless of whether content changed; the distinction between relay and transformation is recoverable from the chain.
3. **Edit is one derivation step, not two** — edit-then-accept is a single cut producing one derived draft with no intermediate "edited-but-not-accepted" record.
4. **filterReviewable fallback presents all drafts when no stage metadata exists** — epistemic absence of stage annotations is treated as a signal to present all drafts, not to reject all.
5. **cmdReview has a different signature from all other cmd\* functions** — `cmdReview(w, in, args)` accepts an `io.Reader` for stdin; all other subcommands take only `(w, args)`.
6. **Interactive prompts go to stderr; data goes to stdout** — `RunReviewSession` writes prompts to `os.Stderr`; accepted/edited draft JSON and the summary go to `w` (stdout), keeping stdout pipeable.
7. **Provenance/content partition in deriveEdited** — SourceSpan, SourceDocRef, and IntentionallyBlank come from the parent (unchanged provenance); all content fields come from the reviewer's input.
8. **main.go file size debt acknowledged, deferred** — `main.go` reached ~1720 lines during Thread A, exceeding the 800-line ceiling. No split was made in Thread A; the refactor is tracked as separate technical debt.

---

## Context

The TraceDraft pipeline (Milestones 11–13, Thread B) produces `weak-draft` records from LLM extractions. These records carry provenance (`SourceSpan`, `ExtractedBy`, `ExtractionStage`) and uncertainty signals (`UncertaintyNote`, `IntentionallyBlank`), but they are not yet promotable to Traces usable for articulation. Between extraction and promotion there is a gap: a human pass that can refine, endorse, or skip records.

Thread A closes this gap with a terminal review session: `meshant review <file>`. The reviewer sees each draft with its derivation chain and ambiguity warnings, then acts — accept (endorse as-is), edit (modify one or more content fields), skip (defer), or quit. Every acceptance and every edit produces a new derived `TraceDraft`, leaving the original intact. The session is a cut, not a correction pass.

The review package is designed for composability. It imports `schema` and `loader`; it is itself imported only by `cmd/meshant`. No cycle. Session logic is independently testable with injected `io.Reader` / `io.Writer`. No external dependencies.

This record documents the decisions made during Thread A that are not recoverable from the code alone. A different cut on this thread — a different reviewer reading the same pull requests — would surface different decisions. The eight decisions and five ANT tensions named here are those that surfaced through implementation, code review, and ANT-theorist review.

---

## Decision 1: Render functions return string, not write to io.Writer

The thread plan sketched render functions with `io.Writer` parameters — each function would write directly to a terminal writer. In implementation, `RenderDraft`, `RenderChain`, and `RenderAmbiguities` were built as string-returning functions instead.

The reason is composability. `RunReviewSession` calls all three in sequence for each draft, then writes to the session's `out io.Writer`. If each render function held its own writer, the session would need to thread the writer through three separate calls. Instead, it calls `fmt.Fprint(out, RenderChain(...))` and `fmt.Fprint(out, RenderDraft(...))` — the session owns the output, the render functions produce values.

String-returning functions are also easier to test: the test inspects a return value rather than capturing a writer. And they are easier to compose: a future caller that wants to log render output, display it in a TUI, or write it to a file can do so without changing the render functions.

This also preserves a separation between articulation (what to render) and commitment (where to send it). The render functions produce readings; the session decides their destination. This mirrors MeshAnt's broader principle that the apparatus should not foreclose how its outputs are used.

The divergence from the plan was noted in the A.2 code review and accepted by the architect.

---

## Decision 2: ExtractedBy is "meshant-review" for both accept and edit

`deriveAccepted` and `deriveEdited` both set `ExtractedBy: "meshant-review"`. Accept and edit are structurally different operations — accept is closer to an endorsement (content passes through unchanged), edit is closer to a transformation (content is modified) — but both produce the same `ExtractedBy` value.

This reflects Principle 7 (generalised symmetry): the session apparatus is the actor, not the human at the keyboard. The human is part of the session assemblage, but the session is what performs the cut. `ExtractedBy` names the instrument, not the person.

The alternative — different `ExtractedBy` values for accept vs edit (e.g., `"meshant-review:accepted"` vs `"meshant-review:edited"`) — was considered. It was rejected as premature classification (C1). The distinction between accept-as-relay and edit-as-transformation is an analytical claim that requires an equivalence criterion to make formally. Pre-classifying it in the data would commit MeshAnt to a theory of what endorsement vs transformation means before the framework has followed enough traces to warrant that theory.

A downstream analyst can always recover the distinction: compare the derived draft's content fields against the parent's. If content changed, the session transformed; if content is identical, the session relayed. The information is present in the chain; it is not pre-classified.

This tension was named by the ANT-theorist review of A.4 (Tension T2).

---

## Decision 3: Edit is one derivation step, not two

When a reviewer chooses `[e]dit`, the session enters `runEditFlow` (collecting field edits), then calls `deriveEdited(parent, edited)`. One derived draft is produced. There is no intermediate "edited-but-not-accepted" draft that gets separately accepted.

The alternative would be a two-step derivation: edit produces a draft at stage `"edited"`, and a separate accept step produces a draft at stage `"reviewed"`. This was explicitly rejected. It would create a phantom intermediary in the derivation chain — a draft that records the reviewer's edits but not their endorsement, as if editing and endorsing were separable acts. In practice they are not: a reviewer who edits a draft and commits the edit has both transformed and endorsed it in a single pass.

The thread plan (D4) named this explicitly: "edit-then-accept is one derivation step, not two." The implementation honours that constraint.

---

## Decision 4: filterReviewable fallback presents all drafts when no stage metadata exists

`filterReviewable` filters to `ExtractionStage == "weak-draft"` records. But if no draft in the input set has any `ExtractionStage` set, the function presents all drafts — the full slice becomes the review queue.

This fallback handles legacy datasets produced before stage metadata was introduced. Without it, a legacy dataset would silently produce an empty review queue, and the reviewer would see "no drafts to review" for a file full of unreviewed material.

The fallback is itself a cut: it converts the epistemic absence of stage annotations into the operational decision to review everything. This is not a neutral act. An alternative would be to require stage metadata and return an error when none is present. That alternative was rejected as too strict for a tool that should accommodate real-world data in various states of completion.

The ANT-theorist review of A.3 named this as Tension T1: the fallback converts epistemic absence to operational presence without surfacing that conversion to the reviewer. A future version could name the fallback in the session output ("no stage metadata found — presenting all drafts") to make the cut visible.

---

## Decision 5: cmdReview has a different signature from all other cmd\* functions

Every other subcommand handler in `cmd/meshant` has signature `func cmdX(w io.Writer, args []string) error`. `cmdReview` has signature `func cmdReview(w io.Writer, in io.Reader, args []string) error`.

The extra `in io.Reader` parameter exists because `review` is the only interactive subcommand. `RunReviewSession` reads from a reader and writes prompts to a writer. The reader must be injectable so that tests can supply scripted input via `strings.NewReader` rather than requiring a real terminal.

The alternative — using `os.Stdin` directly inside `cmdReview` — would make the function untestable without a real terminal, or would require a more complex mock infrastructure. The extra parameter is a small, visible divergence that makes the testability rationale self-evident.

`run()` calls `cmdReview(w, os.Stdin, args[1:])`. The `run()` signature itself is unchanged (`run(w io.Writer, args []string) error`). The `os.Stdin` reference is confined to one call site. If a second interactive subcommand is added in a later thread, the correct next step is to either add `in io.Reader` to `run()` directly, or introduce a `cmdContext` struct — not to accumulate more signature exceptions.

---

## Decision 6: Interactive prompts go to stderr; data goes to stdout

`RunReviewSession` writes interactive prompts to its `out io.Writer` parameter. In `cmdReview`, this parameter is `os.Stderr`. Accepted/edited draft JSON and the post-session summary go to `w` (stdout or `--output` file).

The separation keeps stdout machine-readable. A reviewer can run `meshant review drafts.json | jq .` and receive clean JSON on stdout while seeing prompts on stderr. Or they can use `--output reviewed.json` to route JSON to a file while the session runs interactively on stdout/stderr.

If prompts went to `w`, stdout would contain a mixture of human-readable prompts and machine-readable JSON, making piping and redirection unreliable. The stderr separation is a standard Unix convention and the right choice for any interactive command that also produces structured output.

This means tests for `cmdReview` cannot capture prompt text without redirecting `os.Stderr`. That is acceptable: the prompt rendering is already tested in the `review` package's own unit tests, where `out` is an injected `bytes.Buffer`.

---

## Decision 7: Provenance/content partition in deriveEdited

`deriveEdited(parent, edited TraceDraft)` splits fields into two sources:

- **Content fields** (from `edited`): `WhatChanged`, `Source`, `Target`, `Mediation`, `Observer`, `Tags`, `UncertaintyNote`, `CriterionRef` — the reviewer's articulation
- **Provenance fields** (from `parent`): `SourceSpan`, `SourceDocRef`, `IntentionallyBlank` — the extraction provenance, unchanged by the review

`SourceSpan` records where in the source document the trace came from. `SourceDocRef` records the document itself. `IntentionallyBlank` records which content fields were deliberately left empty during extraction. None of these can be changed by the reviewer in the edit flow: the reviewer refines the reading, not the location in the source text.

A reviewer who edits `WhatChanged` is re-articulating what the trace says. A reviewer who changes `SourceSpan` would be claiming the trace came from a different place in the document — a different kind of operation that implies re-extraction, not review. Keeping provenance fields fixed from the parent makes the partition explicit: the review session produces new content readings from fixed source positions.

This tension was named by the ANT-theorist review of A.4 (Tension T1): the provenance/content boundary is itself a cut, and a future use case may want to contest it.

---

## Decision 8: main.go file size debt acknowledged, deferred

`main.go` in `cmd/meshant` reached approximately 1720 lines by the end of Thread A. The project's coding style ceiling is 800 lines per file. The file grew incrementally across milestones (M11 added `cmdDraft`/`cmdPromote`, M12 added `cmdRearticulate`/`cmdLineage`, M13 added `cmdShadow`/`cmdGaps`, B.1 added `cmdBottleneck`, A.5 added `cmdReview`).

No split was made in Thread A. Splitting `main.go` into per-command files (`cmd_review.go`, `cmd_articulate.go`, etc.) would be mechanical — a package-internal reorganisation with no API surface change — but it is a multi-file refactor that was out of scope for A.5's focused change. The architect review of A.5 flagged this explicitly as pre-existing debt, not introduced by A.5.

The refactor is tracked as a separate issue. It should be done before Thread F (if Thread F adds more subcommands) to keep the file navigable.

---

## ANT tensions named during Thread A

### T1: filterReviewable converts epistemic absence to operational presence

`filterReviewable` presents all drafts when no stage metadata exists. This converts the absence of a classification into a decision to include — without naming that conversion to the reviewer. The reviewer cannot tell, from the session, whether they are seeing all drafts because they are all weak-drafts or because no stage metadata was found.

A future version should surface this in the session output: "no stage metadata found — presenting all N drafts." This would name the cut rather than silently performing it.

### T2: Accept and edit share the same ExtractedBy

Accept (relay) and edit (transform) both set `ExtractedBy: "meshant-review"`. The distinction is recoverable by inspecting the chain but is not pre-classified in the data. A downstream analyst who wants to distinguish "the session endorsed" from "the session transformed" must compare draft fields to parent fields.

This is consistent with C1 (avoid premature classification), but it is a named limitation: the `ExtractedBy` field cannot distinguish acceptance from editing without additional analysis.

### T3: Summary denominator names "loaded" count, not "reviewable" count

The post-session summary reports "N accepted/edited out of M loaded." The denominator M is the count of all drafts in the input file, which may include drafts at stages other than `"weak-draft"`. A reviewer who sees only weak-draft records but receives a denominator of all loaded drafts is being given a count that includes elements the session never presented.

This is a C6 concern: the shadow of non-reviewable drafts is not named. The summary performs a god's-eye count (all loaded) while the session operated from a situated position (only reviewable). The non-reviewed drafts are not absent — they are in shadow from the session's position — but the summary does not name that shadow.

A future version could report both counts: "N accepted out of P reviewable (Q total in file)." Naming the reviewable count makes the session's position explicit in its own output.

---

### T4: Skip does not produce a trace — non-action is not recorded as action

When a reviewer skips a draft, no derived record is produced. The draft remains at `"weak-draft"` in the input file. There is no `"rejected"` stage and no skip record.

This is a commitment consistent with C1 (traces before actors): a trace records a moment where something made a difference. A skip is a non-event in the session. Recording it as a `"rejected"` stage would claim that the reviewer performed an analytical act of rejection, when in fact they performed no act at all. Absence is not the same as negative assertion.

The corollary is that the session produces no record of which drafts were presented and skipped. A future session on the same file will present them again. This is intentional — the review session is not a stateful workflow manager. It is a cut, and the next cut may produce different results.

### T6: Edit-without-change is indistinguishable from accept in the derivation chain

When a reviewer chooses `[e]dit` but makes no changes (pressing Enter on every field), `deriveEdited` produces a derived draft whose content fields are identical to the parent's — the same outcome as `deriveAccepted`. The session distinguished accept from edit as actions, but the derivation chain cannot. Both produce a `DerivedFrom` link with `ExtractionStage: "reviewed"` and identical content.

The three derivation classifications (`intermediary`, `mediator`, `translation`, from `classifyDraftStep`) are based on content comparison, not on how the reviewer interacted with the session. An edit that changes nothing classifies identically to an accept.

This is surfaced rather than fixed. The session's action vocabulary is richer than the trace vocabulary: actions are accept/edit/skip/quit, but derivation classifications are content-based. Recording the session's action in a separate metadata field (e.g., `ReviewAction: "accepted"` vs `"edited"`) would be one resolution, but it would also be a form of premature classification (C1) — a claim that "edited but changed nothing" is a meaningfully different kind of event from "accepted." Whether that distinction matters depends on what question the analyst is asking.

A future version could record session metadata in a sidecar record, leaving the derivation chain clean while preserving the action-level detail for analysts who want it. Named here for visibility; deferred to a later thread.

---

### T5: The review session's own cut is untraced in v1

The review session is a mediating apparatus — it selects, orders, and frames what the reviewer sees (ambiguity warnings, derivation chains, field prompts). It transforms the reviewer's experience of the draft in ways that are not recorded. But the session does not produce a reflexive trace of its own operation.

This is a named instance of C7 (the designer is inside the mesh): the review apparatus shapes what is seen without recording that shaping. The session knows which drafts were presented, which warnings fired, and which fields were shown in the edit flow — but none of this is written to the output. Only the derived drafts are written.

The plan document (`tasks/plan_thread_a.md`) named this shadow explicitly. It is deferred to a later thread, when the framework has enough reflexive infrastructure to record session metadata meaningfully. Naming it here ensures the deferral is visible rather than forgotten.

---

## What Thread A does NOT do

- **Automated promotion** — the review session produces `"reviewed"` stage drafts; it does not automatically call `Promote()`. The reviewer must run `meshant promote` separately.
- **LLM-assisted review** — the session presents drafts as extracted; it does not fetch context, suggest completions, or call any LLM API. All decisions are made by the human reviewer.
- **Multi-file sessions** — `meshant review` accepts a single input file. Reviewing multiple files in sequence requires multiple invocations.
- **Undo/rollback within a session** — once a draft is accepted or edited, the session advances. There is no back-navigation within a session. The original input file is unchanged, so the reviewer can restart with it.
- **Field clearing** — the edit flow has no mechanism to explicitly clear a field to empty (Enter keeps the current value). Clearing a field is a stronger analytical claim — it should be expressed by adding the field to `IntentionallyBlank` in a separate step.
- **No `--reviewer` flag** — `ExtractedBy` is always `"meshant-review"`. The session is the cut; the human is part of the apparatus, not a named actor in the provenance field.

---

## Related

- `docs/decisions/tracedraft-v2.md` — TraceDraft schema, ExtractionStage values, provenance fields
- `docs/decisions/cli-v2.md` — CLI subcommand pattern, outputWriter/confirmOutput helpers
- `docs/decisions/interpretive-outputs-v1.md` — Thread B decision record (Layer 3 outputs)
- `tasks/plan_thread_a.md` — detailed design rules, issue-by-issue plan for A.0–A.6
- `docs/glossary.md` — mediation, cut, shadow, articulation, intermediary vocabulary
