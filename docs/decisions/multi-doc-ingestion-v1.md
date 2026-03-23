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

Validation (mismatched lengths, empty `InputPaths`) runs before allocating a
session ID. If document N fails (LLM error, refusal, parse error), the entire
session fails with an error and `SessionRecord.ErrorNote` is set. Partial
results are not written. This matches the existing single-doc behaviour and
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
- **Session size**: There is no cap on the number of documents per session.
  Large document sets will produce proportionally more LLM calls. The existing
  `maxSourceBytes` per-document cap still applies.
