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

**Tentative: v2.0.0**

Requires the static analysis foundation to be complete and polished first:
visualization path, shadow analysis operations, re-articulation, validation tooling.
Do not rush here. An interactive layer on an incomplete foundation makes the gaps
harder to fix.

---

### 3. Simulation — actors act

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

*Last updated: 2026-03-12. This is a note, not a spec.*
