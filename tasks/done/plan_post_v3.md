# Post-v3 Plan — MCP → Interactive CLI → Actors Act

**Date:** 2026-03-25
**Status:** Rough plan — not yet decomposed into issues
**Follows:** v3.1.0 (deferred items batch, tagged 2026-03-25)

---

## Where we are

All three forms from `docs/directions.md` are complete:

- Form 1 (Library + CLI) — v1.0.0
- Form 2 (Interactive LLM-assisted ingestion) — v2.0.0
- Form 3 (Knowledge Graph substrate + Web UI) — v3.0.0

v3.1.0 closes the deferred items batch. The substrate is stable. The analytical engine
is intact. The sequencing constraint on "actors act" (Form 4) is now satisfied.

The question is what to build on top of it.

---

## Sequence

### v4.0.0 — MCP server

**What:** Expose MeshAnt's analytical commands as MCP tools. Any LLM client (Claude Code,
Cursor, Cline, etc.) can call `articulate`, `diff`, `shadow`, `gaps`, `follow`,
`bottleneck`, `store`, and `validate` without the analyst writing shell commands.

**Why first:** Highest leverage per line of code. The analytical engine already exists;
the work is the adapter layer. An MCP server makes MeshAnt accessible in the contexts
where analysts already work — inside editors, inside conversations — without requiring
them to switch to a terminal. It also stress-tests the observer-position discipline
in a new surface before we build more on top.

**Core design questions:**

- **Observer-position enforcement:** The HTTP serve endpoints require observer (400 on
  missing). MCP tools must enforce the same. An LLM calling `articulate` without an
  observer position is not performing an articulation — it is hiding the cut. The tool
  must refuse or require it.

- **Provenance in tool output:** MCP tool results should carry cut metadata (observer,
  time window, shadow count) — not just the graph data. The same discipline as the Web UI.
  A tool result that strips provenance silently lies.

- **`meshant mcp` subcommand:** Starts an MCP server (stdio transport for editor
  integration; HTTP transport for network-accessible use). Schema-declares tools;
  returns structured JSON. The `llm` package's `LLMClient` interface and the `serve`
  package's HTTP layer provide the reference shapes.

- **Tool set (initial):** `articulate`, `diff`, `shadow`, `gaps`, `follow`, `bottleneck`,
  `store`, `validate`, `summarize`. Read-heavy first. Write tools (`extract`, `assist`,
  `critique`) deferred — the interactive-CLI phase is the right context for those.

- **Session records for MCP calls:** Each MCP tool invocation could produce a
  `SessionRecord`-like provenance trace — the MCP server as an observable mediator.
  This connects to the Principle 8 reflexivity pattern already established in v2.0.0.

**Risk:** MCP becomes a black box if observer-position discipline is not enforced at the
tool schema level (required parameter, not optional). Must be hard, not soft.

---

### v4.x — Fully Interactive CLI

**What:** A conversational analysis mode where the analyst refines cuts in a loop rather
than issuing one-shot batch commands. The LLM helps suggest observer positions, identify
candidate mediators, notice shadows, and propose re-articulations. State persists across
turns.

**Why after MCP:** The MCP server forces us to make every analytical command clean,
schema-declared, and provenance-carrying. That discipline is prerequisite for building
a stateful session on top — a session is a sequence of those clean operations, with
context threading between them.

**Core design questions:**

- **Session state:** What persists across turns? The current cut (observer + criterion +
  time window), the articulation result, the diff history, the shadow list. Probably a
  `AnalysisSession` struct serialized to a session file — the same pattern as `SessionRecord`
  for ingestion sessions.

- **`meshant explore` command:** Entry point for the interactive mode. Loads a dataset
  (or connects to the DB), accepts an initial observer position, then enters a REPL-like
  loop. Commands: `cut <observer>`, `articulate`, `diff <observer-b>`, `shadow`, `gaps`,
  `suggest` (LLM-assisted), `re-articulate`, `save`, `quit`.

- **LLM role:** Suggestion, not decision. The LLM can propose: "based on the current
  shadow list, these elements are consistently absent across your cuts — they may warrant
  a dedicated observer position." The analyst accepts or rejects. Same pattern as
  `meshant assist` for ingestion: LLM proposes, human cuts.

- **Re-articulation as loop:** The analyst refines their cut, sees the diff, names the
  shift. Each re-articulation produces a new provenance record. The loop is the method.

- **Output:** The session produces a `AnalysisTrace` — a record of the articulation
  sequence, the reasoning at each step, and the final cut. Promotable to the mesh
  (Principle 8: the analysis act is an observation act).

**Risk:** Scope creep. Keep the first version narrow: one observer position at a time,
linear cut refinement, no branching. Branching (holding multiple live cuts and diffing
them against each other) is a v5 problem.

---

### v5.0.0 — Actors Act

**What:** Emerged actors generate new traces. An actor's character is derived entirely
from its relational trace history — what it mediated, what it blocked, what it amplified,
from which observer position it was visible. That history initializes a generative agent.
The agent produces new traces. Those traces are re-articulated. New structure emerges.

**Why last:** This requires:
1. A persistent graph substrate (Form 3 — done)
2. A clean analytical engine that can be called programmatically from a simulation loop
   (the MCP adapter surfaces this)
3. An understanding of how articulation state threads through multi-turn reasoning
   (the interactive CLI phase builds that muscle)

**The hard problem (must be solved before implementation begins):**

> What constrains an actor's action?

An actor assembled from trace history must be *shaped* by that history, not just
initialized from it. If the actor is free to act arbitrarily after initialization, the
simulation immediately drifts from the network it emerged from. The constraint has to
be relational: the actor can only do what its trace history shows it as capable of doing,
from the positions it has been observed from, with the mediations it has previously
performed.

This is the methodological bet. Getting it wrong produces another agent framework with
Latour vocabulary bolted on. Getting it right produces something genuinely different.

**Candidate constraint design (rough, not committed):**

- An actor's action space = the set of (source, target, mediation) triples it has
  previously appeared in, weighted by recency and frequency
- An actor cannot introduce new elements it has never been trace-adjacent to
- The observer position of the simulation run must be declared — the simulation is
  always already a cut, never a god's-eye run
- New traces produced by actors are tagged `generated` and carry the actor's trace
  lineage as provenance — they are hypotheses, not observations
- Re-articulation after a simulation run shows what *would* change under these
  conditions, not what did change

**Open questions (must be answered before v5 planning):**

1. What is the unit of actor action? A single trace? A batch? A temporal step?
2. How does conflict between actors get resolved? (Two actors claiming the same
   mediation in opposite directions — this is the most interesting case)
3. Does the simulation loop inside Go or outside it? (An LLM generating traces for
   each actor step vs. a rule-based generator derived from trace statistics)
4. What does "running the network forward" mean in ANT terms? ANT is retrospective.
   Forward simulation requires a claim about what actors *would* do, which ANT is
   normally suspicious of. That tension must be named explicitly, not resolved away.

**Risk:** This is speculative enough that premature implementation would produce
something incoherent. Do not start v5 planning until the four questions above have
provisional answers. The constraint design note above should become a proper design
document (like `docs/decisions/actors-act-v0.md`) before any code is written.

---

## What does not change

The core inversion holds across all three phases:

> Traces come first. Actors are not defined in advance. The agent, if there is one,
> is assembled from relational history, not prescribed from outside.

- MCP tools enforce observer position; they do not invent one
- The interactive CLI helps the analyst articulate; it does not articulate for them
- Simulation actors are constrained by their trace history; they do not act freely

Any direction that requires abandoning the inversion should be rejected.

---

## Versioning sketch

| Version | Direction |
|---------|-----------|
| v3.x | Deferred items, minor deepening of existing commands |
| v4.0.0 | `meshant mcp` — analytical commands as MCP tools |
| v4.x | `meshant explore` — interactive analysis session |
| v5.0.0 | Actors act — emerged actors generate new traces |

---

*This is a rough plan — a provisional articulation. Not a commitment, not a spec.*
*Before any phase begins: decompose into issues, confirm design, run per-issue pipeline.*
