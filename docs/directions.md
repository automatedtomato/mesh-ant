# MeshAnt — Possible Directions

This note records a design discussion about where MeshAnt could go.
It is not a roadmap or a commitment. It is a provisional articulation — a cut of the
current thinking, from a particular moment. Treat it accordingly.

---

## The core inversion

Most agent frameworks work like this:

> Define the agents first. Give them roles, personas, context. Then run them.

MeshAnt flips that:

> Traces come first. Actors are not defined in advance — they emerge from what the traces
> show made a difference. The agent, if there is one, is assembled from relational history,
> not prescribed from outside.

This is not just a technical choice. It is the methodological commitment that makes MeshAnt
distinct. Any future direction should preserve it.

---

## Three possible forms (non-exclusive)

### 1. Library + CLI

MeshAnt as a Go package and command-line tool. Static analysis: load traces, articulate,
diff, export to JSON / DOT / Mermaid. The analyst is outside the mesh, using the tool.

**Current target: v1.0.0**

The foundation. Everything else builds on this.

---

### 2. Interactive analysis tool

MeshAnt with an LLM-assisted session interface — closer to Claude Code or Codex in feel.
The analyst converses with the tool. The LLM helps with the hardest part of using MeshAnt:
deciding what counts as a trace, identifying observer positions, naming mediations.

Trace authoring becomes dialogue rather than manual instrumentation. The tool is still
primarily analytical — it helps you understand a network, not run one.

**LLM boundary — layered:**
- **Framework / core layer (v1.x, now):** LLM external. The `draft` command consumes
  LLM-produced extraction JSON as input; it does not make live LLM calls. The LLM is
  a mediator whose output is a named, inspectable file — not a hidden step inside the CLI.
- **Interactive CLI layer (v2.0.0):** LLM internal. The session interface calls the LLM
  directly. The LLM's transformations are still visible in the mesh (it appears as a
  mediator node, not a neutral extractor), but the boundary moves inside the tool.

**Tentative: v2.0.0**

Requires the static analysis foundation to be complete and polished first:
visualization path, shadow analysis operations, re-articulation, ingestion entrypoint
(M11), anti-ontology critique pass (M11.5/M12). Do not rush here. An interactive layer
on an incomplete foundation makes the gaps harder to fix.

---

### 3. Knowledge-graph-aware layered system

A potential architectural form emerging from the graph integration discussion
(`docs/reviews/graph_integration_note_14-mar-26.md`). Not a replacement for the core
inversion — an extension of it into a three-layer stack:

**Layer 1 — Trace/episode substrate (Graphiti-like)**
Stores traces, source episodes, relation candidates, provenance, temporal updates.
Resists premature actorization. Pre-articulation material only.
Possible technology: Graphiti-like temporal graph memory, or a graph store with
provenance tracking.

**Layer 2 — MeshAnt articulation engine (current)**
Applies cuts, criteria, preserve/ignore declarations, shadow logic, actor-like emergence,
diff across articulations. This is MeshAnt's core responsibility and must not be
delegated to the substrate or the visualization layer.

**Layer 3 — Visualization / exploration surface (Neo4j-like)**
Displays articulated renderings as navigable graphs. Comparison across cuts. Provenance
inspection. Shadow browsing. NOT a neutral truth display — always framed as a current
articulation, a situated reading, a rendered stabilization.

The critical discipline: the visualization layer shows *a cut*, not *the world*. Any
graph rendering must carry cut metadata (observer-position, criterion, shadow list)
explicitly, or the rendering silently lies. MeshAnt's current JSON exports already
carry this metadata; the work is in the adapter layer and in making the UI refuse to
strip provenance.

**Current status**: future-compatible boundary only. The existing `--format json`
exports from `articulate` and `diff` already produce cut-aware output that could feed
a future adapter. No implementation planned until the analytical core and ingestion
entrypoint are more mature.

**Tentative: post-v2.0.0, or as a parallel track alongside v2.0.0.**

---

### 4. Simulation — actors act

The most speculative direction. Not near-term.

In the current framework, actors emerge from traces and then wait — they are names in a
graph, crystallized from history, static. This direction asks: what if emerged actors
could act? Generate new traces. Respond to conditions. Propagate effects through the
network.

An actor's character would be derived entirely from its trace history — what it mediated,
what it blocked, what it amplified, from which position it observed. That character
initializes an agent. The agent generates new traces. Those traces are re-articulated.
New structure emerges.

This transforms MeshAnt from a retrospective analysis tool into a generative simulation
and problem-solving tool. "What happens to this network if this threshold changes?" becomes
answerable not by inspection but by running the network forward.

The hard design problem: what constrains an actor's action? Actors must be shaped by their
relational history, not free to act arbitrarily. Otherwise the simulation drifts from the
network it emerged from. Getting that constraint right is the core methodological challenge
of this direction — not the engineering.

**Tentative: v3.0.0 or later. Do not plan this in detail now.**

---

## What is not a direction

To stay sharp, it helps to name what MeshAnt is not trying to become:

- A replacement for existing agent frameworks (CrewAI, AutoGen, etc.)
- A tool where roles are defined first and traces fill in later
- A god's-eye dashboard that claims a neutral total view
- A prediction engine or optimization tool

The value is in the inversion — trace-first, emergence-first, observer-explicit.
If a direction requires abandoning that, it is probably the wrong direction.

---

## Note on sequencing

The gap between v1.0.0 (CLI) and v2.0.0 (interactive) is not trivial.
Between them: visualization (how does a user actually see a graph?), shadow analysis
operations, re-articulation, a `validate` command, a trace authoring guide.
These are not features added for completeness — they are the foundation the interactive
tool needs to stand on. Do that work before adding LLM.

---

## Post-v1.0.0: what the review surfaced (2026-03-13)

The v1.0.0 review (`docs/reviews/release_v1_review_13-mar-26.md`) identified v1.0.0 as a
strong Layer 2 — the analytical kernel — surrounded by two thin layers that need thickening:

**Layer 1 — Ingestion / authoring support.**
How do raw materials become usable traces? The current input model is conceptually elegant
but practically demanding. It assumes a user who can already think in MeshAnt's terms.
The central proposal: an intermediate representation (working name: candidate trace / draft
trace) that sits between unstructured material and canonical `Trace`. It would carry
uncertainty, evidence spans, confidence flags, provenance — making the authoring cut itself
inspectable. This is deeply aligned with the method.

**Layer 3 — Interpretation / rendering support.**
How do MeshAnt outputs become actionable results? The current outputs are methodologically
rich but demand specialised MeshAnt literacy. Shadow summaries, bottleneck reports,
observer-gap analysis, re-articulation suggestions — these bridge the gap between the
analytical object and a practical result.

The review's key constraint for LLM integration:

> Do not hide the cut in the name of usability.

The interactive layer (v2.0.0) should begin as a **trace-authoring companion** — not an
autonomous analyst. It should suggest candidate traces, surface ambiguity, show provenance,
and let the user confirm or reject. Assisted authoring with visible uncertainty, not
automated truth.

### Deferred technical work (kernel deepening)

The review also names work that deepens Layer 2 itself, deferred across multiple milestones:

- Tag-filter cut axis (deferred since M3)
- GraphDiff DOT / Mermaid export (deferred since M8)
- Shadow analysis operations (named in sequencing note above)
- Re-articulation operations

These are not opposed to the Layer 1/3 work. Deepening and extending proceed together.

---

*Last updated: 2026-03-15. This is a note, not a spec. Supersedes `docs/potential-forms.md` (removed).*
*2026-03-15 additions: Knowledge-graph-aware layered form (Form 3); LLM boundary layering in Form 2; based on `docs/reviews/graph_integration_note_14-mar-26.md` and `docs/reviews/llm_limit_14-mar-26.md`.*
