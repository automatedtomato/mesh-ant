# Decision Record: Session Records → Traces (v1)

**Issue:** #138
**Branch:** `138-session-records-to-traces`
**Merged:** PR #154 → `develop` (2026-03-22)

---

## Problem

The MeshAnt pipeline uses `SessionRecord` to capture the conditions of an LLM ingestion
act: model ID, source document, prompt template, timestamp, draft IDs, errors. This record
is written to disk and carries provenance. But it is not a `Trace`.

This is a Principle 8 gap: the framework observes its domain (traces the mesh) but does
not observe itself observing. The act of LLM extraction — a significant mediating event
in the analyst's network — has no place in the mesh as a trace. The framework is
invisible to its own analysis.

---

## Decision: `PromoteSession` converts a `SessionRecord` to a canonical `Trace`

A new function `PromoteSession(rec SessionRecord, observer string) (schema.Trace, error)`
in `llm/session_promote.go` converts a session record into a canonical trace.

The promoted trace places the LLM session in the mesh as a mediation:

| Trace field | Value | Rationale |
|-------------|-------|-----------|
| `ID` | `rec.ID` | Same UUID — session and trace are the same act in different registers |
| `Timestamp` | `rec.Timestamp` | Time of the observation act |
| `WhatChanged` | Generated from Command + SourceDocRef(s) + ModelID | Names conditions of the act, not just that it happened |
| `Source` | `[rec.Conditions.ModelID]` | The model is the source of the extractive act |
| `Target` | `[rec.Conditions.SourceDocRefs...]` | The document(s) processed |
| `Mediation` | `rec.Command` (e.g. "extract", "critique") | The LLM session is a mediator — it transforms what passes through it |
| `Observer` | Caller-supplied (required) | No trace without an observer; no god's-eye view |
| `Tags` | `["session", "articulation"]` | Marks provenance: promoted from session, type of reflexive act |

`TagValueSession = "session"` is a new tag constant in `schema/trace.go` marking traces
promoted from `SessionRecord`s. This makes the provenance signal queryable.

---

## Key design decisions

### Observer is required, not inferred

`PromoteSession` requires the caller to provide the observer position. An empty observer
is a hard error: `"observe the session from a named position — no trace without an observer"`.

This is the most important constraint. The alternative — inferring the observer from the
session record (e.g. using `ExtractedBy` from one of the drafts) — would hide the
analyst's position behind a heuristic. The analyst promoting the session must name their
vantage point.

### Session ID reuse

The promoted trace uses the session's UUID as its own ID. This means:
- One session → exactly one promoted trace
- The session and the trace are the same act in different registers

This is a deliberate identity commitment: the session is not merely *described by* the
trace; it *is* the trace, looked at from an analytical position. A consequence: two analysts
cannot promote the same session under two different observer positions without collision
(same `Trace.ID`, different `Observer`). This constraint is documented in the standing
tensions below.

### `Mediation` names the command, not the model

`Mediation` is set to `rec.Command` (e.g. "extract"), not to the model ID. The command
names what the session *did* (extracted, assisted, critiqued) — the kind of transformation.
The model ID appears in `Source`, which names the actant that performed the act. This
preserves the conceptual distinction: `Source` is who/what acted; `Mediation` is how they
acted (the modality of transformation).

### Multi-document sessions (#139 integration)

`Target` is built from `SourceDocRefs []string` (added in #139) when present, falling back
to the legacy `SourceDocRef string` for backward compatibility. This means a multi-document
session produces a trace with multiple targets — the trace names all documents processed in
one act.

---

## `meshant promote-session` subcommand

A new CLI subcommand wraps `PromoteSession`:

```
meshant promote-session --session-file <path> --observer <position> [--output <file>]
```

The session file is read with `json.NewDecoder` without `DisallowUnknownFields` — session
files are written by the framework and may contain fields from newer versions. The analyst
supplies the observer position; the rest is derived from the session record.

Output is a `[]schema.Trace` JSON array (single element) — consistent with all other
subcommands that write trace output, enabling the standard `meshant promote` workflow
to follow.

---

## Standing tensions

### Session-trace identity coupling

`PromoteSession` reuses the session UUID as the promoted trace's ID. Two analysts cannot
promote the same session under two different observer positions — the second promotion
would produce a trace with the same ID, creating a validation conflict if both are loaded
into the same trace dataset. This is acceptable at v1 (one session = one promoted trace),
but it means the promoted trace is not re-analyzable from a different position without
special handling.

Future option: add an optional `--id` flag to `promote-session` that overrides the ID,
allowing two analysts to promote the same session under different positions with distinct
IDs. Not implemented at v1.

### Adapter not named in promoted trace's Mediation (#140 integration)

When `meshant extract --adapter pdf` is used, the adapter is a second mediator in the
extraction chain: PDF → text (adapter) → TraceDrafts (LLM). The promoted trace's
`Mediation` field currently names only the command ("extract"), not the adapter that
preceded it. The full chain is visible in the session record's `ExtractionConditions.AdapterName`,
but not at the trace level.

This is an opportunity for future enhancement: `Mediation` could include the adapter name
when present (e.g. "extract via pdf-extractor") to make the full mediation chain visible
in the trace without requiring access to the session file.
