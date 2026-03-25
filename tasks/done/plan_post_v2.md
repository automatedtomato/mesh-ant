# Post-v2.0.0 Plan — ANT-like Knowledge Graph

**Date:** 2026-03-22
**Status:** Planning
**Follows:** v2.0.0 (LLM-assisted ingestion, tagged 2026-03-22)

---

## Direction

The next major form is an **ANT-like Knowledge Graph**: persistent, queryable, interactive.
The current tool is stateless — every run loads JSON files from scratch. The KG gives
traces a persistent home and gives the user an interactive surface for exploration.

"Actors act" simulation (v3.0.0) comes after this layer exists, not before. The KG is
the substrate simulation would eventually run on. None of this work is wasted.

---

## Design Decisions (confirmed 2026-03-22)

**Storage backend:** GraphDB (Neo4j-compatible) for scalability and native graph traversal.

**Query model (Choice B hybrid):** The Go analytical engine (`Articulate`, `Diff`, `Shadow`,
`Gaps`, `Follow`, `Bottleneck`) stays entirely intact. The GraphDB replaces JSON files as
the source of traces. The DB handles storage, indexing, and efficient pre-filtering
(by observer, time window, tags). The Go layer applies cut logic on the filtered set.

```
CLI flags
    │
    ▼
DB adapter layer  ← narrow, swappable; returns trace subgraph
    │
    ▼
Go analytical engine  ← unchanged; all ANT logic lives here
    │
    ▼
output (text / JSON / DOT / Mermaid / Web UI)
```

Rationale: the ANT logic — shadow calculation, cut semantics, diff, reason-tracking — is
carefully aligned with the project's commitments. Re-expressing it in a graph query
language (Choice A) would introduce drift. The DB is used for what it is genuinely
good at; the framework owns the analytical discipline.

**Interactive surface:** CLI commands remain the primary interface. `meshant serve`
starts a localhost HTTP server; the browser renders the current cut as an interactive
graph. D3.js or Cytoscape.js for the frontend. The CLI and Web UI are additive — the
CLI works independently.

**Layer 3 discipline:** every rendered graph carries cut metadata (observer position,
time window, shadow count). The UI never shows "the graph" — it shows "this cut from
this position". Shadow is always named. Provenance is never stripped.

**ANT principle:** the graph itself is always already a cut. MeshAnt and its outputs
are mediators — we cannot control how users receive them, but we can make the positioned
character of every output undeniable. This is not a guardrail; it is the evidence trail.

---

## Sequencing

### Phase 1 — Deferred items and ingestion gaps (immediate)

Small, self-contained. Close known gaps before building the KG layer.

**1.1 — `meshant split`**
LLM-assisted span splitting. The user currently must pre-split source material before
calling `meshant assist`. A `split` command removes the biggest friction in the
ingestion pipeline. It runs the LLM with a span-boundary prompt and produces a spans
file directly. Requires its own `SessionRecord` (a split is an LLM mediating act with
provenance).

**1.2 — Session records → Traces**
A `SessionRecord` is an observation act: the LLM operated at a specific time, under
specific conditions, on specific source material. Not recording this as a `Trace` is
a Principle 8 gap — the framework observes but doesn't observe itself observing.
Promote-able session records close that loop.

**1.3 — Multi-document ingestion**
`meshant extract` currently takes one source document per session. Multi-document
ingestion lets the user feed several documents in one session, with the LLM producing
traces that may reference material across them. Provenance question: `SourceDocRef`
needs to carry the specific document for each draft, not just a session-level reference.

**1.4 — Non-text source adapters**
PDF, HTML, structured logs → text → existing LLM pipeline. Mostly an adapter problem;
the analytical core does not change. Two distinct sub-problems:
- **Unstructured** (PDF, HTML): extract text, then hand off to `extract`/`assist`
- **Structured** (logs, JSON events): may produce traces more directly, bypassing
  the LLM or using it only for classification, not extraction

### Phase 2 — Form 3 scoping document

Before any Layer 1 or 3 code is written, produce a scoping document that answers:
- Schema: how are `Trace` records stored as graph nodes/edges in Neo4j? What are the
  node labels and relationship types? How is the cut structure represented?
- Adapter contract: what interface does the Go engine call? What does the DB adapter
  implement?
- Session handling: how are `SessionRecord` and `ExtractionConditions` stored?
- Query pre-filtering: what DB queries replace the JSON `Load()` call?
- Web UI contract: what JSON does `meshant serve` emit per endpoint?
- Provenance enforcement: what does the server refuse to return without cut metadata?

### Phase 3 — Layer 1: Trace substrate

**3.1 — DB adapter interface**
Define the storage interface the Go engine will call. The interface is narrow: store
a trace, retrieve traces by observer/time/tag, retrieve a trace by ID. The Go engine
calls this interface; the JSON file loader and the GraphDB adapter both implement it.
This keeps the existing CLI working during the transition.

**3.2 — Neo4j adapter**
Implement the interface against a Neo4j-compatible backend. Schema:
- Node: `Trace` (all trace fields as properties)
- Node: `Observer`, `Element`, `SessionRecord`
- Relationship: `OBSERVED_BY`, `INVOLVES`, `DERIVED_FROM`, `IN_SESSION`
Every relationship carries the `trace_id` so the full trace is always recoverable.

**3.3 — `meshant store` subcommand**
Load traces from JSON and write them to the connected DB.
`meshant store --db bolt://localhost:7687 traces.json`

**3.4 — DB-backed analytical commands**
`--db` flag on `articulate`, `diff`, `shadow`, `gaps`, `follow`, `bottleneck`.
When provided, the command queries the DB instead of loading a JSON file.
The Go analytical engine is unchanged.

### Phase 4 — Layer 3: Interactive graph output

**4.1 — `meshant serve`**
Starts a localhost HTTP server backed by a DB connection (or a JSON file for
compatibility). Endpoints:
- `GET /articulate?observer=X&from=T&to=T` → cut JSON (same schema as `--format json`)
- `GET /diff?observer-a=X&observer-b=Y` → diff JSON
- `GET /shadow?observer=X` → shadow JSON
- `GET /traces?observer=X` → trace list (observer required — the raw list is still a positioned cut)

Every endpoint response includes cut metadata. No endpoint returns a "raw graph"
without an observer position.

**4.2 — Web UI**
Single-page app served by `meshant serve`. Graph rendering with D3.js or Cytoscape.js.
Key UI requirements:
- Observer position selector (required before any graph is shown)
- Cut metadata always visible (observer, time window, trace count, shadow count)
- Shadow panel: named elements in shadow, with reasons
- Node click: show full trace detail including provenance chain
- Export: download current cut as JSON / DOT

**4.3 — Provenance panel**
For LLM-produced traces: show `session_ref → SessionRecord → ExtractionConditions`.
For reviewed traces: show `derived_from` chain back to the original. The provenance
chain is always one click away from any node.

### Phase 5 — Thread D datasets

With the full stack in place (ingestion pipeline, KG storage, interactive UI), produce
three datasets that exercise different domains and showcase the complete pipeline.

- **D.1 — Software incident** — a complex multi-service outage with competing observer
  positions (on-call engineer, product manager, customer). Exercises the full LLM
  ingestion pipeline and multi-analyst comparison.
- **D.2 — Multi-agent pipeline** — an AI workflow where agents are actants (not roles).
  Validates MeshAnt's claim to be useful for AI systems analysis, not just human processes.
- **D.3 — Policy/procurement** — a regulatory or procurement process with institutional
  mediators, delays, thresholds, and formal objections. Different from all existing
  datasets in its emphasis on institutional non-human actants.

---

## What comes after (v3.0.0 — not planned in detail)

Once the KG substrate is established and traces have a persistent home, the "actors act"
direction becomes tractable. An actor's character would be derived from its trace history.
The actor generates new traces. Those traces enter the persistent graph. The mesh evolves.

The hard design problem is constraint: actors must be shaped by their relational history,
not free to act arbitrarily. That constraint design is the methodological challenge.
Do not begin this until Form 3 is stable.

---

## Constraints that carry forward

- **Do not hide the cut in the name of usability.** The Web UI must name the observer
  position before rendering anything.
- **The graph itself is always already a cut.** This is a living design principle, not
  a configuration option. Every output, every API response, every rendered graph is a
  positioned reading.
- **LLM integration enters as assisted authoring with visible uncertainty.** The LLM
  is a mediator in the chain; it never becomes an authoritative extractor.
- **The designer is inside the mesh.** `meshant serve` is a participant in the mesh it
  serves. The act of running the server is not recorded as a trace yet — that gap should
  be named when it becomes relevant.
