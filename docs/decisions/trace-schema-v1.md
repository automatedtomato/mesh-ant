# Trace Schema — Decision Record v1

**Date:** 2026-03-10
**Status:** Provisional
**Branch:** feat/m1-trace-schema
**Observer position:** tool-assisted design session (Claude + human author)

---

## What cut was made here

This document records the decisions made in defining the first version of the
MeshAnt `Trace` schema. Per Principle 8 — "the designer is inside the mesh" —
this record is itself a trace: a record of what was included, what was excluded,
and from what position the schema was drawn.

The schema lives at: `meshant/schema/trace.go`

---

## What was included and why

### `id` (string, required)

A UUID string that uniquely identifies the trace record. UUIDs are universally
generatable without coordination. The field is `string` rather than a typed UUID
to keep the schema dependency-free (stdlib only). The UUID format is validated
by `Validate()` but is intentionally lenient on version bits (v4 pattern matched
by prefix shape, not strict version nibble) to avoid rejecting traces from systems
that produce other UUID variants.

### `timestamp` (time.Time, required)

Records when the trace was *captured*, not when the underlying event "really"
occurred. This distinction is deliberate: observation is always situated in time.
A zero `time.Time` is rejected by `Validate()` because a trace without a temporal
anchor cannot be followed or ordered. Stored as `time.Time` for nanosecond
precision and idiomatic Go JSON serialization (RFC3339Nano).

### `what_changed` (string, required)

The primary content of the trace. Required because a trace without a description
of the difference it records is not a trace — it is an empty marker. No length
constraint is imposed; the schema does not police how much or how little is said.

### `source` (string, optional)

Names what produced this trace. Left as a plain `string` — not a typed `Actor`
struct — because the schema must not decide in advance what counts as an actor.
A source could be a human, a rule, a queue, a sensor, a threshold, a form, or
anything else that redirects, amplifies, blocks, or transforms action. Optional
because attribution is sometimes genuinely unknown or unattributable.

### `target` (string, optional)

Names what was affected. Same openness as `source`. Optional because effects are
sometimes diffuse, deferred, or not yet observable at the moment of trace capture.

### `mediation` (string, optional)

Names what transformed, redirected, or displaced the action between source and
target. A mediator is not a neutral conduit — it changes what passes through it.
This field holds the central ANT concept: mediation is not a secondary
detail but a first-class element of the trace. Its *absence* is meaningful —
it means no mediator was observed, not that none could exist.

### `tags` ([]string, optional)

Descriptors characterizing the kind of difference the trace records. The field
type is `[]string` rather than `[]TagValue` so the vocabulary stays open and
revisable — callers may supply novel descriptors without breaking the schema.
The `TagValue` constants (`delay`, `threshold`, `blockage`, `amplification`,
`redirection`, `translation`) are a starting vocabulary, not a closed enum.

### `observer` (string, required)

Records who or what captured this trace, and from what position. Required.
A trace without a stated observer silently erases the position from which the
observation was made — which contradicts Principle 5 (plural observers before
god's-eye view) and Principle 8 (the designer is inside the mesh). Making
`observer` required in `Validate()` makes the situated nature of observation
non-deniable in the schema itself.

---

## What was deliberately excluded

### Actor typing

No `Actor` struct. No `ActorID` type. No registry of known actors.

Rationale: actors in MeshAnt are provisional effects of linked traces, not
unquestioned first principles. Introducing a typed `Actor` at schema definition
time would install role-thinking at the foundation — exactly what MeshAnt
intends to resist. Actors can emerge later, from patterns in traces.

### Trace relationships and causation links

No `caused_by`, `parent_id`, or `links []string` field.

Rationale: trace chains and causal graphs can be inferred from source/target
co-occurrence and timestamp ordering. Embedding causation directly into the
schema would presuppose a causal structure before it has been followed.
Relation is an outcome of analysis, not a field to be filled in.

### Severity, priority, and confidence fields

No `severity`, `priority`, `confidence`, or `weight` fields.

Rationale: evaluative metadata belongs to the observer and the analysis context,
not to the trace itself. A trace records what happened; it does not rank it.

### Schema version field in the record

No `schema_version` field embedded in every `Trace` instance.

Rationale: the module path (`github.com/automatedtomato/mesh-ant/meshant`) and
this decision document serve as the versioning record. Embedding a version in
every trace record is premature and adds noise before the schema has stabilized.

### Closed tag enumeration

`tags` is `[]string`, not `[]TagValue`. `TagValue` constants are not enforced
by the type system.

Rationale: closing the tag vocabulary would foreclose the ability to follow
traces into territory the schema did not anticipate. Open vocabulary is the
correct default for a system that aims to follow traces before stabilizing
categories.

---

## What assumptions this schema makes

1. Every trace is attributed to at least one observer position (`observer` required).
2. `what_changed` is required: a trace without content is not a trace.
3. `source` and `target` may be unknown — attribution is not always possible or appropriate.
4. Tags are descriptive, not prescriptive. The schema does not validate tag values.
5. Timestamps record observation time, not event time.
6. UUID format is sufficient for trace identity; no central registry is assumed.

---

## What this schema does not settle

- Whether traces should be immutable after creation.
- How traces relate to each other (chains, graphs, temporal sequences).
- What counts as a "valid" source or target string.
- Whether `observer` should itself become a structured type.
- Whether `source` and `target` should become `[]string` to accommodate
  heterogeneous assemblages as producers or targets of a trace.
- What happens when the same event is traced from multiple observer positions.

---

## Why this is v1 and not final

Following Principle 6 — "re-articulation before essence" — this schema is
expected to change as the project follows more traces and learns what
distinctions are worth stabilizing. The version label is a reminder that the
schema is a provisional cut, not a settled ontology.

---

## References

- `docs/principles.md` — Principles 5, 6, 8 most directly relevant
- `docs/ant-notes.md` — On mediation, articulation, re-articulation, observation
- `tasks/todo.md` — M1.1, M1.4
- `tasks/plan_m1_1.md` — Implementation plan for this milestone
