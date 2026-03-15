# Response to graph_integration_note and llm_limit reviews

**Date:** 2026-03-15
**Context:** Discussion before M11 planning, following docs/reviews/graph_integration_note_14-mar-26.md and docs/reviews/llm_limit_14-mar-26.md

---

## 1. The Knowledge Graph question

The graph integration note draws a clean three-layer architecture:

- **Substrate** (Graphiti-like): stores traces, episodes, relation candidates, provenance — pre-actor, pre-articulation
- **MeshAnt engine**: applies cuts, criteria, shadow logic — renders actor-like stabilizations
- **Visualization** (Neo4j-like): displays articulated renderings as navigable graphs — not neutral truth

This is philosophically coherent and technically sound. The critical discipline is that Neo4j shows *a cut*, not *the world*. The danger is that graph UIs feel authoritative — nodes look like facts. MeshAnt would need to visibly carry cut metadata into any graph rendering (observer-position, criterion, shadow list) or the rendering silently lies.

The current CLI already produces the right substrate for this: `--format json` on `articulate` and `diff` exports structured, cut-aware graphs that a future adapter could push into Neo4j. The work is in the adapter layer and in making the UI refuse to strip provenance.

## 2. The LLM entrypoint question — and where it connects to M11

This is the more immediately relevant one. The `llm_limit` doc is essentially a design spec for M11 under a different framing. What it describes as `TraceDraft` maps almost exactly onto what M11 calls `CandidateTrace` — a provisional, unvalidated, provenance-bearing record that is not yet a canonical trace.

But the doc adds something M11's current plan doesn't fully address: the **pipeline shape**. M11 plans `CandidateTrace` as a type and a `draft` CLI command. The doc argues for a multi-pass process:

1. Raw span extraction (no actorization)
2. Weak trace drafting (candidate descriptions, unresolved references preserved)
3. Anti-ontology critique (second-pass: where did the LLM smuggle in an actor?)
4. Human review / refinement
5. Articulation

M11 as planned currently covers steps 2 and 4 (type + interactive CLI). It doesn't yet have a story for step 3 (the critique pass) or step 1 (document-level span harvesting).

**The most important design question for M11 is**: does the `CandidateTrace` type need to carry the anti-ontology critique as a field, or is the critique a separate pipeline stage that produces a human-facing report?

The doc suggests the latter: critique is a pass, not just a field on the struct. That has implications — the `draft` CLI may need a `--critique` mode or a separate `critique` subcommand that reads existing drafts and produces warnings about premature actorization.

---

## The entrypoint question

The first real user entrypoint isn't the CLI's `follow` command (that's analytical, not ingestion). It's the moment a user has a document — a log, a report, a transcript — and wants to get it into MeshAnt. Today that requires manually authoring JSON traces, which is a hard barrier.

The LLM-assisted ingestion pipeline is that entrypoint. But based on the doc, it should land as:

> `meshant draft <document>` — produces `TraceDraft` candidates with spans, uncertainty notes, and unresolved references preserved. Not final traces. The user reviews, refines, and promotes.

Rather than:

> `meshant ingest <document>` — produces canonical traces automatically.

---

## Open questions for M11 planning discussion

1. **Critique pass placement**: Does the anti-ontology critique belong in the same milestone as the draft type, or as a separate milestone? The doc treats it as a distinct pass; folding it into M11 may make the milestone too large.

2. **LLM integration scope**: Is the LLM integration intended as a first-class feature of the CLI (requiring API key config, external call), or as a separate tool/service that feeds into MeshAnt's trace format? The latter preserves the CLI as stdlib-only and keeps the boundary clean.

3. **TraceDraft vs CandidateTrace naming**: The doc uses `TraceDraft`; the M11 plan uses `CandidateTrace`. These may or may not be the same concept — `CandidateTrace` implies a trace that is a candidate for promotion; `TraceDraft` implies a trace that is not yet fully formed. Worth resolving before implementation.

4. **Provenance fields**: The doc lists extraction_stage, extracted_by (human / LLM / reviewer pass), source span, and source document reference as fields. The current M11 plan does not specify these. If the entrypoint is LLM-assisted ingestion, these fields are load-bearing — without them the extraction is invisible.

5. **LLM as node in the mesh**: The doc's strongest idea is that the LLM should appear as a mediator in the articulated graph — raw span → LLM extraction → critique → human revision → accepted trace as a visible chain. This is philosophically correct and technically achievable, but it implies that the ingestion pipeline itself becomes a translation chain that MeshAnt can follow and classify. That is a significant design commitment worth explicit discussion.
