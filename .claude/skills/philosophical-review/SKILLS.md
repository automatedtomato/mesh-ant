---
name: philosophical-review
description: >
  Audit the codebase against MeshAnt's core ideological commitments — trace-first,
  articulation-first, no pre-defined actors — then refactor any violations. This is not
  a conventional code review. It asks: does the code embody the philosophy, or does it
  quietly contradict it?
origin: mesh-ant
---

# Philosophical Review

## The single most important principle

**Actors are never pre-defined or given. They emerge temporarily as a result of the network.**

This is the founding commitment of Actor-Network Theory (ANT). In ANT, there are no actors
before the network — only actants that become actors through the relations they enter and the
mediations they perform. The network does not connect pre-existing actors; it produces them.

This applies to everything: human participants, non-human mediators, software systems,
and the observation apparatus itself (including produced graphs and diffs). None of these
are actors before the traces say so. A coder, a graph, a webhook — none of them exist as
actors until and unless the network of traces makes them one.

The framework's job is to follow traces first, articulate what the traces show, and only then
— provisionally — name what seems to be acting. That naming is a cut, not a discovery.

---

## Reference documents

- `docs/principles.md` — MeshAnt's 8 design principles
- `docs/manifesto.md` — project framing
- `docs/ant-notes.md` — ANT theoretical grounding
- `docs/decisions/` — all decision records

---

## What to check

### 1. Trace-first: no pre-assigned actors

The code must not encode actor identity before the trace layer has established it.

**Violations:**
- Types or fields named `Actor`, `Role`, `AgentType`, or similar that classify elements
  before traces are read
- Any enumeration of "who can be a source" or "who can be a target" — sources and targets
  are open `[]string` by design
- Any function that accepts a named actor and queries traces about it, rather than accepting
  traces and surfacing what acts
- Any default, fallback, or assumed identity for an element that has not appeared in traces

---

### 2. Articulation-first: every output is a cut, not a view

An articulation is a provisional reading from a specific position. It is not a map of
what is there. The cut must name its observer position, its time window, and its shadow
(what it excludes). No output should imply completeness, objectivity, or finality.

**Violations:**
- Labels or strings that imply neutrality: `"all"`, `"no filter"`, `"complete"`,
  `"objective"`, `"total"`, `"global"` — must be qualified or renamed
- The full-cut position (no observer filter) must be named as a deliberate choice, not
  an absence. Canonical label: `"(all — full cut)"`
- Shadow sections that are optional or skippable — they are mandatory
- Summary or print functions that use language like "report", "analysis", "result" without
  a qualifier — prefer "provisional", "cut", "situated", "first look"
- `Cut` values stored by reference rather than deep-copied — a mutable cut retroactively
  re-situates the graph (immutability is a methodological requirement, not just a style choice)

---

### 3. Emergence is temporary: no permanent actor records

An element becomes an actor by appearing in traces. That status is tied to a specific cut.
Change the cut (different observer, different time window) and the actor may vanish or fragment.
The code must not canonise actors across cuts.

**Violations:**
- Any registry, map, or singleton that accumulates actor identities across multiple
  articulations without recording which cut each identity came from
- Any function that merges or unions nodes across cuts without explicitly re-articulating
- `IdentifyGraph`/`IdentifyDiff` assigning IDs automatically (they must be explicit opt-in —
  the caller decides when a graph is intended to act, not the framework)

---

### 4. Generalised symmetry: no privileged actants

Human and non-human actants are treated identically. A graph that influenced an outcome
is an actant like any other. A webhook, a policy document, a threshold rule, a produced
diff — all are candidates for actancy if the traces say so.

**Violations:**
- Any field, type, or branch that handles human elements differently from non-human ones
- Graph-reference strings (`meshgraph:`, `meshdiff:`) handled differently from plain strings
  in source/target loops — they are elements and must be counted as such
- Any new field added to `Trace` that gives special status to a category of actant

---

### 5. Friction is real: provisional cuts must name what they exclude

`FlaggedTraces` selects `delay` and `threshold` as a proxy for quantifiable friction.
This is a cut. All 6 tag types — `delay`, `threshold`, `blockage`, `redirection`,
`amplification`, `translation` — are ANT-significant. Any subset selection must name
itself as provisional and name what it leaves out.

**Violations:**
- Tag-filtering code that silently drops tag types without documentation
- The `FlaggedTraces` doc comment omitting the other 4 tag types
- Any "flagged" or "notable" section that implies the selected tags are the only meaningful ones

---

### 6. The observer is inside the mesh

The framework is not outside what it observes. The `Observer` field on every trace is
mandatory — it records that every observation is made from somewhere. The graph-as-actor
pattern extends this: the observation apparatus can itself appear in traces.

**Violations:**
- `Observer` made optional or given a default value
- Any new articulation-producing function that does not capture the observer position in the `Cut`
- Any graph or diff produced without a path to re-entering it into the mesh as an actant
  (i.e., without `IdentifyGraph`/`IdentifyDiff` being available to the caller)

---

## Process

### Step 1 — Identify scope

For a milestone review, read all files in `meshant/graph/`, `meshant/schema/`, `meshant/loader/`.
For a targeted review, read only the files changed since the last review.

### Step 2 — Launch three agents in parallel

**Agent A — Trace-first and emergence (checks 1, 3)**
Look for pre-defined actors, permanent registries, automatic identity assignment.

**Agent B — Articulation and cuts (checks 2, 5)**
Look for god's-eye language, mutable cuts, missing shadow, provisional language failures.

**Agent C — Symmetry and reflexivity (checks 4, 6)**
Look for privileged actant handling, missing observer fields, blocked re-entry into mesh.

### Step 3 — Fix violations, note tensions

Fix each genuine violation (edit comment, rename label, add doc note, fix deep copy).
Do not add new functionality. Philosophical refactor only.

A tension (where the code holds two commitments in productive friction) is not a violation —
name it and leave it.

### Step 4 — Verdict

**PHILOSOPHICALLY ALIGNED** — no violations. Architecture and language embody the commitments.

**ALIGNED WITH TENSIONS** — no violations, but named tensions worth tracking in future work.

**VIOLATION FOUND — REFACTORED** — list each:
- What the violation was
- Which principle it violated
- What change was made

---

## Violations found and fixed in M1–M5 (reference)

| Violation | Principle | Fix |
|---|---|---|
| `"(all — no filter)"` implied the full cut is neutral | Articulation-first | Changed to `"(all — full cut)"` |
| `ShadowCount` comment was purely structural, no methodological meaning | Articulation-first | Added "partial connection" framing |
| `FlaggedTraces` selected delay/threshold without naming other tag types | Friction is real | Added doc note naming all 6 types as provisional cut |
| `GraphRef`/`DiffRef` comments omitted positional information warning | Articulation-first | Added note: carries no observer position, time window, or shadow |
| `GraphRefs` double-listed in `PrintSummary` without explanation | Generalised symmetry | Updated header: "also counted in Elements above" |
