# Plan: v5.0.0 — Actors Act (rough notes)

**Date:** 2026-03-25
**Parent issue:** #173
**Status:** Rough notes only — not yet decomposed into issues
**Follows:** v4.0.0 (MCP) + v4.x (interactive CLI)

---

## Status

Do not start implementation planning until:
1. v4.0.0 (MCP server) is complete
2. v4.x (interactive CLI) is complete
3. The four open design questions below have provisional answers

See also: `docs/reviews/actors_act_design_note.md`

---

## Direction

Emerged actors generate new traces. An actor's character is derived entirely from its
relational trace history — what it mediated, what it blocked, what it amplified, from
which observer positions it was visible.

The key: the trace record *is* the actor. Not a description that gets translated into a
persona — the actual substance of what the actor is.

---

## The ANT-native answer to "what makes actors act?"

An actor acts when it is enrolled — when another trace names it as source, target, or
mediation. Being named is being called on. The network initiates; the actor responds.

Three things the trace record provides:

1. **Mediation repertoire** — which (source, target) pairs this actor has appeared between;
   which transformations it has performed. Capability is relational, not intrinsic.

2. **Enrollment triggers** — what conditions in the network have historically caused this
   actor to appear. Those patterns are in the traces, not in a character description.

3. **Relational neighborhood** — which other actors it has co-appeared with, and in what
   roles. An actor cannot introduce entities it has never been trace-adjacent to.

---

## What MeshAnt rejects (the MiroFish inversion)

MiroFish: graph → LLM-generated persona (MBTI, bio, profession) → agent → action

MeshAnt: trace history → relational constraints → LLM as constrained generator → trace

The difference: MiroFish uses the graph as a source of character descriptions. MeshAnt uses
the graph as the actual substance of action. The traces don't describe the actor — they *are*
the actor.

---

## Candidate constraint design (rough, not committed)

- An actor's action space = the set of (source, target, mediation) triples it has previously
  appeared in, weighted by recency and frequency
- An actor cannot introduce new elements it has never been trace-adjacent to
- Observer position of the simulation run must be declared (simulation is always a cut)
- New traces tagged `generated`, carrying actor trace lineage as provenance
- Re-articulation after simulation shows what *would* change under these conditions, not what
  did change

---

## The LLM's role

The LLM is not performing a persona. It is a constrained generator. Given:
- The full trace history of the actor being called on
- The trace that just enrolled this actor (the trigger)
- The constraint: only entities within the actor's historical neighborhood; only mediations
  within its historical repertoire

The LLM generates the next plausible trace. Not "what would this character do?" but "what
trace would this relational position produce in response to this enrollment?"

---

## Open design questions (must have provisional answers before child issues open)

**1. Unit of action**
What is one actor action? A single trace? A batch of traces? A temporal step covering
all enrolled actors simultaneously?

**2. Conflict resolution**
What happens when two actors claim the same mediation in opposite directions? This is the
most interesting case — it is where the network resists the simulation. How does MeshAnt
surface conflict rather than resolving it?

**3. Simulation loop host**
Does the LLM generate one trace per actor per step (round-based, like MiroFish), or does
propagation work differently — one trace triggering enrollments which trigger traces
(cascade-based)? The cascade model is more ANT-native but harder to bound.

**4. What does "forward" mean in ANT terms?**
ANT is retrospective by method. Forward simulation requires a claim about what actors
*would* do, which ANT is normally suspicious of. That tension must be named explicitly,
not resolved away. The simulation must be marked as a hypothesis, not a prediction.
New traces are tagged `generated` and carry the simulation run as provenance.

---

## Prerequisite: TraceOrigin field on schema.Trace

Before v5 planning, add a `TraceOrigin` field to `schema.Trace`:

```go
// TraceOrigin distinguishes how a trace came to exist.
// "observed"  — recorded from observation of a real event
// "generated" — produced by the actors-act simulation
// "promoted"  — promoted from a TraceDraft or AnalysisSession
type TraceOrigin string

const (
    TraceOriginObserved  TraceOrigin = "observed"
    TraceOriginGenerated TraceOrigin = "generated"
    TraceOriginPromoted  TraceOrigin = "promoted"
)
```

Without structural distinguishability, generated traces cannot be identified as hypotheses
at the schema level. This is a v4.x cleanup issue (could be a standalone issue opened
before v5 planning begins).

---

## Standing tension

ANT traces are records of what happened. Simulation traces are projections of what might
happen. These are not the same epistemic category. MeshAnt must name this distinction
rather than collapsing it.

A simulation run in MeshAnt is always already a reading: situated, positioned, generated
under declared conditions. It is the mesh reading itself forward, not the world unfolding.
That framing, kept explicit, is what makes this different from a prediction engine.

---

## Versioning sketch

| Version | Scope |
|---------|-------|
| v5.0.0 | Actor enrollment detection; single-actor response generation; `generated` tag + provenance |
| v5.x | Multi-actor simulation rounds; conflict surfacing (not resolution) |
| v5.x | Re-articulation after simulation run; hypothesis vs. observation display in Web UI |

---

*This is rough notes only — a provisional articulation.*
*Revisit after v4.x (interactive CLI) is complete.*
*Do not open child issues until the four design questions above have provisional answers.*
*See also: `docs/reviews/actors_act_design_note.md`, `tasks/plan_post_v3.md`.*
