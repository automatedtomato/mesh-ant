# Thread A — Interactive Review CLI

**Goal:** human-in-the-loop refinement between draft extraction and promotion.
The review session is a cut, not a correction. Every acceptance is a new derived draft;
the original is never overwritten.

**Depends on:** Thread B complete (develop, 2026-03-18)
**Targets:** v1.x → v2.0.0 prereq
**Branch base:** `develop`
**Rough plan source:** `tasks/plan_v2_roadmap.md` §Thread A
**Reviewed by:** architect agent + ant-theorist agent (2026-03-18)

---

## Design principles for Thread A

1. **Immutability** — accept creates a new derived `TraceDraft` (new UUID, `DerivedFrom` set,
   `ExtractionStage: "reviewed"`). The original draft is never modified.
2. **The session is a cut** — not a correction. The user is a mediator in the ingestion
   pipeline, not an oracle correcting errors.
3. **Language discipline** — ambiguity warnings are invitations to attend, not demands to fix.
   No completeness pressure. No god's-eye prompts.
4. **Composability** — the `review` package depends on `schema` and `loader`; it does not
   import `cmd/meshant`. Session logic is independently testable.
5. **No external dependencies** — stdlib only; no `$EDITOR` in v1.
6. **Testability** — injectable `io.Reader` / `io.Writer` throughout; `run()` dispatcher
   signature unchanged for now (deferred migration to Thread F).

---

## ANT pre-fix: `classifyDraftStep` heuristic (issue A.0)

The ant-theorist review identified a latent violation in `loader/draftchain.go`:
`classifyDraftStep` currently treats "stage-changes-but-content-unchanged" as
`DraftIntermediary`. This is wrong — advancing `ExtractionStage` is a mediating act
(transformation of the draft's epistemic standing), even when no content fields change.

**Fix:** Add a fourth case to `classifyDraftStep`:

```go
// existing
case content && stage: DraftTranslation
case content:          DraftMediator
// ADD
case stage:            DraftMediator (reason: "extraction_stage advanced — endorsement
                        transformed standing, not content")
// default
default:               DraftIntermediary
```

This must land before Thread A work begins; it makes the classification apparatus
consistent with the plan's framing: "the session is a cut, not a correction."

---

## Package and file layout

### New package: `meshant/review/`

| File | Purpose |
|------|---------|
| `review/ambiguity.go` | `DetectAmbiguities`, `AmbiguityWarning` — pure functions |
| `review/ambiguity_test.go` | table-driven tests, no I/O |
| `review/render.go` | `RenderDraft`, `RenderAmbiguities`, `RenderChain` |
| `review/render_test.go` | golden-string output tests |
| `review/review.go` | `SessionConfig`, `SessionResult`, `Action`, `RunSession`, `readAction`, `deriveAccepted`, `filterUnreviewed`, `runEditFlow`, `deriveEdited` |
| `review/review_test.go` | session tests with scripted `io.Reader` input |

### Modified files

| File | Change |
|------|--------|
| `meshant/loader/draftchain.go` | Fix `classifyDraftStep` heuristic (issue A.0) |
| `meshant/loader/draftloader.go` | Export `NewUUID` (rename `newUUID`) |
| `meshant/cmd/meshant/main.go` | Add `cmdReview(w io.Writer, r io.Reader, args []string) error`; add `"review"` case to `run()` dispatcher (passes `os.Stdin`) |
| `meshant/cmd/meshant/main_test.go` | Group 20+ tests for `cmdReview` using `strings.NewReader` |
| `docs/decisions/interactive-review-v1.md` | Decision record |
| `docs/CODEMAPS/meshant.md` | Add `review` package |
| `tasks/todo.md` | Thread A tasks marked complete |

---

## Type definitions

### `review/review.go`

```go
// SessionConfig holds parameters for a review session.
// All fields set by cmdReview before the session begins.
type SessionConfig struct {
    Drafts        []schema.TraceDraft // full set of loaded drafts
    IDFilter      string              // restrict to one draft by ID; empty = all unreviewed
    CriterionName string              // from loaded EquivalenceCriterion; empty if none
    In            io.Reader           // os.Stdin in production, strings.NewReader in tests
    Out           io.Writer           // os.Stdout in production, bytes.Buffer in tests
}

// SessionResult holds the output of a completed review session.
type SessionResult struct {
    Accepted []schema.TraceDraft // new derived drafts from accept/edit actions
    Reviewed int                 // drafts the user saw (accepted + edited + skipped)
    Skipped  int                 // drafts the user skipped without action
}

// Action represents a user's response to a single draft.
type Action int

const (
    ActionAccept Action = iota
    ActionEdit
    ActionSkip
    ActionQuit
)
```

### `review/ambiguity.go`

```go
// AmbiguityWarning names a structural condition in a draft that warrants attention.
// It is not an error — the user may acknowledge and accept the draft as-is.
type AmbiguityWarning struct {
    Field   string // field name or concern area (e.g. "source", "uncertainty_note")
    Message string // human-readable invitation to attend; no completeness pressure
}
```

No new `schema` types are required. Derived drafts use existing `TraceDraft` fields:
- `ID`: fresh UUID from `loader.NewUUID()`
- `DerivedFrom`: reviewed draft's ID
- `ExtractionStage`: `"reviewed"`
- `ExtractedBy`: `"meshant-review"` (names the session instrument, not just "human")
- `Timestamp`: session time
- Content fields: copied from original (accept) or user-modified (edit)
- `CriterionRef`: session's criterion name if provided

**Why `ExtractedBy: "meshant-review"`?**
The review session is itself a mediating instrument — it selects, orders, and frames what
the user sees. Using `"human"` would hide the CLI's mediation. `"meshant-review"` names
both the instrument and the position (human-in-the-loop via the review session).

---

## Function signatures

### `review/ambiguity.go`

```go
// DetectAmbiguities checks a draft for structural conditions warranting attention:
//   - Empty candidate field not listed in IntentionallyBlank
//   - UncertaintyNote present (extractor flagged unresolved provenance)
//   - CriterionRef mismatch (draft uses different criterion than session)
//
// Returns nil if no ambiguities found. Does not block acceptance.
func DetectAmbiguities(d schema.TraceDraft, sessionCriterion string) []AmbiguityWarning
```

**Language guidance for warning messages:**
- Empty field: `"<field> is empty and not marked intentionally_blank — was this deliberate?"`
- UncertaintyNote: `"uncertainty_note present: \"<note>\" — the extractor flagged uncertainty here"`
- CriterionRef mismatch: `"criterion_ref on this draft (<a>) differs from session criterion (<b>) — produced under different reading conditions"`

### `review/render.go`

```go
// RenderDraft writes a formatted display of a single draft to w.
// Includes: source span, all candidate fields, metadata (stage, extractor, criterion).
func RenderDraft(w io.Writer, d schema.TraceDraft, index int, total int) error

// RenderAmbiguities writes ambiguity warnings for a draft.
// Each warning is an invitation to attend, not a demand to fix.
// Returns the count of warnings written.
func RenderAmbiguities(w io.Writer, warnings []AmbiguityWarning) (int, error)

// RenderChain writes a compact derivation chain with ClassifyDraftChain annotations.
// Shows the full DerivedFrom lineage up to draftID with step kinds and reasons inline.
// Reuses loader.FollowDraftChain and loader.ClassifyDraftChain.
func RenderChain(w io.Writer, drafts []schema.TraceDraft, draftID string) error
```

### `review/review.go`

```go
// RunSession walks the user through unreviewed drafts interactively.
// Filters to ExtractionStage != "reviewed" (unless IDFilter is set).
// Returns SessionResult with all newly derived drafts.
// Caller (cmdReview) merges them with originals and writes output JSON.
func RunSession(cfg SessionConfig) (SessionResult, error)

// filterUnreviewed returns drafts eligible for review:
// ExtractionStage is not "reviewed".
func filterUnreviewed(drafts []schema.TraceDraft) []schema.TraceDraft

// readAction reads a single action from the input stream.
// Accepts: "a"/"accept", "e"/"edit", "s"/"skip", "q"/"quit".
func readAction(scanner *bufio.Scanner) (Action, error)

// deriveAccepted creates a new TraceDraft derived from orig:
// new UUID, DerivedFrom = orig.ID, ExtractionStage = "reviewed",
// ExtractedBy = "meshant-review", CriterionRef = criterionName if non-empty.
// Content fields copied unchanged from orig.
func deriveAccepted(orig schema.TraceDraft, criterionName string, now time.Time) (schema.TraceDraft, error)

// deriveEdited creates a new TraceDraft derived from orig with user-supplied
// edits applied. Edit-then-accept is one derivation step, not two.
// edits maps field name to new string value; absent keys keep orig value.
func deriveEdited(orig schema.TraceDraft, edits map[string]string, criterionName string, now time.Time) (schema.TraceDraft, error)

// runEditFlow prompts the user for each editable field, showing the current value.
// Empty input (Enter) keeps the current value.
// Editable fields: what_changed, source, target, mediation, observer, tags,
// uncertainty_note (comma-separated where slices).
// Returns map of field names to new values (only changed fields).
func runEditFlow(scanner *bufio.Scanner, w io.Writer, d schema.TraceDraft) (map[string]string, error)
```

### `cmd/meshant/main.go` additions

```go
// cmdReview implements the "review" subcommand.
// Takes io.Reader for interactive input (os.Stdin in production,
// strings.NewReader in tests).
func cmdReview(w io.Writer, r io.Reader, args []string) error
```

Dispatcher addition:
```go
case "review":
    return cmdReview(w, os.Stdin, args[1:])
```

---

## Session flow

```
For each unreviewed draft (filtered by IDFilter or all):
  1. Print separator: "--- Draft N/M ---"
  2. Detect + render ambiguity warnings (invitations, not demands)
  3. Render draft fields (source span, candidates, metadata)
  4. Render compact derivation chain with classification annotations
  5. Prompt: "[a]ccept  [e]dit  [s]kip  [q]uit > "
  6. Read action:
     - accept: deriveAccepted → append to accepted list
     - edit:   runEditFlow → deriveEdited → append to accepted list
     - skip:   increment skip counter, continue
     - quit:   break loop
     - unrecognized: "unknown action — enter a, e, s, or q" → reprompt

After loop:
  7. Print session summary: N accepted, M skipped, K remaining
  8. Return SessionResult to cmdReview

cmdReview:
  9. Merge: output = original drafts + result.Accepted
  10. Write JSON to --output file or stdout
  11. Print confirmation
```

---

## Inline editing (v1)

Fields editable inline (no $EDITOR dependency):
`what_changed`, `source` (comma-separated), `target` (comma-separated), `mediation`,
`observer`, `tags` (comma-separated), `uncertainty_note`

Format: `  source [current value]: ` → user types new value or presses Enter to keep.

Edit-then-accept is **one derivation step**. A single derived draft carries both the
edits and the stage advancement. Creating two derived drafts (one for edit, one for
acceptance) would introduce a phantom intermediary step that did not actually occur.

**Deferred to v2:** `$EDITOR` support via temp file for long source spans.

---

## Test strategy

All interactive functions take explicit `io.Reader`/`io.Writer`. Tests use `strings.NewReader`
for scripted input and `bytes.Buffer` for captured output.

### `review/ambiguity_test.go` (~8 tests, pure, table-driven)
```
TestDetectAmbiguities_EmptyFieldNotBlank      — warns when field empty, not in IntentionallyBlank
TestDetectAmbiguities_IntentionallyBlankOK    — no warning when field empty + in IntentionallyBlank
TestDetectAmbiguities_UncertaintyNoteWarns    — warns when UncertaintyNote set
TestDetectAmbiguities_CriterionMismatch       — warns when CriterionRef != sessionCriterion
TestDetectAmbiguities_NoMismatchWhenNoCrit    — no warning when sessionCriterion empty
TestDetectAmbiguities_NilOnClean             — nil return when no ambiguities
TestDetectAmbiguities_WarningLanguage         — messages are invitations, not demands
TestDetectAmbiguities_Multiple               — multiple warnings returned together
```

### `review/render_test.go` (~10 tests)
```
TestRenderDraft_ShowsSourceSpan
TestRenderDraft_ShowsCandidateFields
TestRenderDraft_ShowsIndex
TestRenderAmbiguities_ShowsWarnings
TestRenderAmbiguities_ZeroCountOnNone
TestRenderChain_ShowsStepKinds
TestRenderChain_ShowsClassification
TestRenderChain_EmptyChain
TestRenderChain_MultiStep
TestRenderChain_CycleDetected     — does not panic; shows cycle notice
```

### `review/review_test.go` (~15 tests)
```
TestRunSession_AcceptThenQuit           — "a\nq\n" → 1 accepted, correct DerivedFrom
TestRunSession_SkipThenQuit             — "s\nq\n" → 0 accepted, 1 skipped
TestRunSession_EditThenAccept           — one derived draft, not two
TestRunSession_QuitImmediately          — empty result
TestRunSession_UnknownInputReprompts    — "x\na\nq\n" → still accepts on retry
TestRunSession_AllAlreadyReviewed       — empty input, session exits gracefully
TestRunSession_IDFilterSingleDraft      — only the specified draft reviewed
TestRunSession_DerivedFromSet           — derived draft has DerivedFrom = orig.ID
TestRunSession_ExtractionStageReviewed  — derived draft has ExtractionStage "reviewed"
TestRunSession_ExtractedBySession       — derived draft has ExtractedBy "meshant-review"
TestRunSession_CriterionRefSet          — criterion name propagated when provided
TestRunSession_OriginalUnmodified       — original draft unchanged after accept
TestRunSession_OutputMergesOriginals    — result.Accepted + originals = full output set
TestRunSession_EditFlowKeepsUnchanged   — Enter keeps current value
TestRunSession_EditFlowChangesField     — typed value replaces current
```

### `main_test.go` additions (~8 tests, Group 20)
```
TestCmdReview_HappyPath               — full run via cmdReview(w, r, args), temp file
TestCmdReview_IDFilter                — --id restricts to one draft
TestCmdReview_CriterionFile           — --criterion-file sets CriterionRef on derived
TestCmdReview_OutputFlag              — --output writes to file
TestCmdReview_NoUnreviewed            — session exits gracefully, no output mutation
TestCmdReview_MissingFile             — error on missing input file
TestCmdReview_InvalidJSON             — error on malformed JSON
TestCmdReview_QuitWritesPartial       — quit mid-session writes accepted-so-far
```

---

## Key design decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Acceptance creates new derived draft | Immutability; review is a cut, not a correction |
| D2 | `ExtractedBy: "meshant-review"` | Names the session instrument; hides neither CLI nor human |
| D3 | No `$EDITOR` in v1 | Testability; platform independence; v2 can add it |
| D4 | Edit-then-accept = one derivation step | Reflects actual rhythm of user engagement; no phantom intermediary |
| D5 | Output = originals + derived drafts | Self-contained for downstream `meshant promote` and `meshant lineage` |
| D6 | Filter by stage, not by action | Re-running session on same file skips already-reviewed drafts |
| D7 | `run()` signature unchanged | Avoid touching 12 existing cmdXxx functions; migration deferred to Thread F |
| D8 | Separate `review/` package | `main.go` already large; session logic independently testable |
| D9 | No `"rejected"` stage value | ANT concern: skipped ≠ rejected; absence of a derived draft is the record |
| D10 | Session's own cut untraced in v1 | Named shadow; Thread F (`meshant assist`) is where session reflexivity belongs |

---

## Issue decomposition

### Parent: Thread A — Interactive Review CLI
Top-level tracking issue. All child issues reference this.

### A.0 — Fix `classifyDraftStep` heuristic
**Files:** `meshant/loader/draftchain.go`, `meshant/loader/draftchain_test.go`
**Scope:** Add `case !content && stage → DraftMediator` to heuristic; add 1-2 tests.
**Why first:** Blocking correctness issue; Thread A acceptance will expose this classification.

### A.1 — `review` package scaffold: ambiguity detection + draft rendering
**Files:** `review/ambiguity.go`, `review/ambiguity_test.go`, `review/render.go` (RenderDraft + RenderAmbiguities), `review/render_test.go`; also export `loader.NewUUID`
**Scope:** Pure functions, no interactive I/O. ~8 ambiguity tests + ~6 render tests.

### A.2 — Chain rendering in review session
**Files:** `review/render.go` (RenderChain), `review/render_test.go`
**Scope:** `RenderChain` using `loader.FollowDraftChain` + `loader.ClassifyDraftChain`.
~4-5 tests.

### A.3 — Review session core: accept + skip + quit
**Files:** `review/review.go` (SessionConfig, SessionResult, Action, RunSession, readAction, deriveAccepted, filterUnreviewed), `review/review_test.go`
**Scope:** Interactive loop; scripted-input tests (accept, skip, quit). ~10 tests.

### A.4 — Edit flow
**Files:** `review/review.go` (runEditFlow, deriveEdited), `review/review_test.go`
**Scope:** Inline field editing; edit-then-accept as one step. ~5 tests.

### A.5 — CLI wiring: `cmdReview`
**Files:** `meshant/cmd/meshant/main.go` (cmdReview, dispatcher), `meshant/cmd/meshant/main_test.go` (Group 20)
**Scope:** Flag parsing, draft loading, session invocation, output merging. ~8 CLI tests.

### A.6 — Decision record + codemap
**Files:** `docs/decisions/interactive-review-v1.md`, `docs/CODEMAPS/meshant.md`, `tasks/todo.md`
**Scope:** Documentation only. All design decisions above recorded.

---

## Invariants

- No god's-eye language in session output or warning text
- Original draft never modified; every acceptance = new derived draft with `DerivedFrom`
- `ExtractedBy: "meshant-review"` on all session-produced derived drafts
- `go test ./...` and `go vet ./...` green before each PR merge
- TDD: tests first, then implement

---

## File summary

| File | Status | What changes |
|------|--------|--------------|
| `meshant/loader/draftchain.go` | modified | stage-only change → DraftMediator heuristic fix |
| `meshant/loader/draftloader.go` | modified | export `NewUUID` |
| `meshant/review/ambiguity.go` | new | `AmbiguityWarning`, `DetectAmbiguities` |
| `meshant/review/ambiguity_test.go` | new | ~8 tests |
| `meshant/review/render.go` | new | `RenderDraft`, `RenderAmbiguities`, `RenderChain` |
| `meshant/review/render_test.go` | new | ~10 tests |
| `meshant/review/review.go` | new | `SessionConfig`, `SessionResult`, `Action`, `RunSession`, all helpers |
| `meshant/review/review_test.go` | new | ~15 tests |
| `meshant/cmd/meshant/main.go` | modified | `cmdReview`, `"review"` dispatcher case |
| `meshant/cmd/meshant/main_test.go` | modified | Group 20, ~8 CLI tests |
| `docs/decisions/interactive-review-v1.md` | new | design decisions A.0–A.6 |
| `docs/CODEMAPS/meshant.md` | modified | `review` package added |
| `tasks/todo.md` | modified | Thread A tasks marked complete |
