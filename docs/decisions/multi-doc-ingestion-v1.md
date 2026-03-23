# Decision: Multi-Document Ingestion (Issue #139)

**Status:** Accepted
**Date:** 2026-03-23
**Issue:** #139 — Multi-document ingestion

---

## Context

Before #139, `meshant extract` accepted exactly one `--source-doc` flag and
one `--source-doc-ref` flag. Analysts working with several related source
documents had to run multiple extract sessions, producing multiple session
files and requiring manual reconciliation of the resulting draft sets.

The goal is to allow a single extract session to span multiple source
documents, preserving the provenance invariant that all drafts from a session
share one session ID and one session record.

---

## Options considered

**Option A — Repeatable flags, single session envelope**
`--source-doc` and `--source-doc-ref` become repeatable flags. One LLM call
is made per document. All resulting drafts share one `session_id`. The
`SessionRecord` carries `input_paths []string` and
`conditions.source_doc_refs []string`. Per-draft `SourceDocRef` is stamped
from the specific document that produced the draft.

**Option B — Multiple session files, merge tool**
Keep single-doc extraction; add a `meshant merge-sessions` command. Analysts
run extract N times and then merge.

Option A was chosen. Option B defers the complexity to a secondary tool that
would need to invent a merge provenance convention. Option A keeps the session
envelope as the natural unit of one analytical act (one analyst, one prompt,
one model, one sitting), regardless of how many documents are processed.

---

## Schema changes

### `ExtractionConditions`

| Old field | New field | Notes |
|-----------|-----------|-------|
| `source_doc_ref string` | `source_doc_refs []string` | Plural; single-doc sessions use a one-element slice |
| — | `source_doc_ref string` | Retained as legacy read-only field for backward compat |

### `SessionRecord`

| Old field | New field | Notes |
|-----------|-----------|-------|
| `input_path string` | `input_paths []string` | Plural; single-doc sessions use a one-element slice |
| — | `input_path string` | Retained as legacy read-only field for backward compat |

### `ExtractionOptions`

| Old field | New field |
|-----------|-----------|
| `InputPath string` | `InputPaths []string` |
| `SourceDocRef string` | `SourceDocRefs []string` |

`AssistOptions`, `CritiqueOptions`, and `SplitOptions` keep their single-string
`InputPath` and `SourceDocRef` fields — those operations are single-document
by design.

---

## Backward compatibility stance

Session files written before #139 carry `input_path` (singular) and
`source_doc_ref` (singular). The Go JSON decoder silently ignores unknown
fields in session files (documented in `cmd_promote_session.go`), and the
new plural fields carry `omitempty`. Old session files remain readable.

`PromoteSession` checks `SourceDocRefs` first; if empty, falls back to
`SourceDocRef`. Old session records promote correctly without modification.

---

## Fail-fast error strategy

A session ID is allocated first so that every error path — including
validation failures — returns a `SessionRecord` with a non-empty ID. The
caller writes this record to disk even on failure, making the error
inspectable. Structural validation (mismatched lengths, empty `InputPaths`,
cap exceeded) then runs before any LLM call. If document N fails (LLM error,
refusal, parse error), the entire session fails with an error and
`SessionRecord.ErrorNote` is set. Draft output is not written, but the
session record is always persisted with partial `DraftCount`/`DraftIDs` so
the provenance record accurately reflects what was produced. This matches the
existing single-doc behaviour and
avoids silent provenance gaps where some documents extracted and others did not.

---

## Per-draft provenance

Each draft carries `SourceDocRef` set to the specific document that produced
it (not the full list). The session-level `conditions.source_doc_refs` records
all documents processed. Analysts can recover which draft came from which
document by examining `SourceDocRef` on each draft.

---

## Standing tensions

- **SourceSpan overlap**: If two documents contain the same passage, two drafts
  with identical `SourceSpan` values may be produced in one session. The
  framework does not deduplicate; that is an analytical question for the analyst.
- **Order sensitivity**: Documents are processed in flag order. LLM output may
  vary if order is changed (same documents, different order = different session).
  This is consistent with MeshAnt's situated-reading stance.
- **Session size**: A session is capped at `maxDocsPerSession = 20` documents
  as an API quota safeguard. Exceeding the cap returns an error before any LLM
  call. Large document sets within the cap will produce proportionally more LLM
  calls. The existing `maxSourceBytes` per-document cap still applies.
- **Sequential processing**: Documents are processed sequentially in loop
  order. This preserves the "processed in flag order" guarantee and keeps error
  handling simple (fail-fast on the first failed document). Parallel LLM calls
  would reduce wall-clock time for large batches but would complicate partial
  failure semantics. Concurrency is a possible future optimisation if analysts
  regularly hit the wall-clock ceiling on 20-document sessions.
- **Multi-target promotion**: A promoted trace from a multi-document session
  will have multiple entries in `Target` (one per source document ref).
  Downstream consumers that assumed `len(Target) == 1` should not — the
  schema treats `Target` as a slice throughout. This is analytically correct:
  the session acted on multiple documents, so the trace should reflect that.
