# Actors Act — Design Note

**Date:** 2026-03-25
**Status:** Pre-design discussion — not a spec, not a plan
**Context:** Post-v3.1.0 direction discussion; MiroFish examined as reference

---

## The question

"What makes actors actors?" What gives them the capacity to act in a forward simulation?

This is the central design problem for v5.0.0 (Form 4 — actors act). It cannot be
answered by copying an existing approach. It has to be answered from MeshAnt's own
commitments.

---

## What MiroFish does (and why MeshAnt can't copy it)

MiroFish's pipeline:

1. Ingest text → build a knowledge graph (Zep graph memory)
2. For each graph entity, call an LLM to generate an **agent profile**: bio, MBTI type,
   gender, age, country, profession, interested_topics, follower_count, karma
3. Inject each profile into an OASIS social media simulation agent
4. Run simulation rounds: agents post/like/retweet on Twitter/Reddit-like platforms

**The persona is the answer to "what makes actors act."** The graph is mined for
character attributes, then discarded. What drives action is an LLM performing as a
character — "you are a professor who believes X and has MBTI type INTJ."

This is role-first. It prescribes identity before following traces. The graph becomes
a source of character descriptions, not a constraint on action. MeshAnt explicitly
rejects this — it is the inversion MeshAnt exists to perform.

---

## The ANT-native answer

In ANT, an actor is not a stable substance with properties that cause action.
An actor is what other actors make it. Agency is constituted by **enrollment** —
being named in translations, delegated to, made to count in a network.

So "what makes actors act" reframes: **what does the network do to this actor,
and what does that obligate?**

An actor acts when it is enrolled — when another trace names it as source, target, or
mediation. Being named is being called on. The network initiates; the actor responds.

---

## What MeshAnt's actor is

The trace record *is* the actor — not a description that gets translated into a persona,
but the actual substance of what the actor is. Three things the trace record provides:

### 1. Mediation repertoire
Which (source, target) pairs this actor has appeared between. Which transformations it
has performed. This is what it *can* do — its capability is relational, not intrinsic.

### 2. Enrollment triggers
What conditions in the network have historically caused this actor to appear. A threshold
actor appears when thresholds are crossed. A delay actor appears when timelines slip.
Those patterns are in the traces, not in a character description.

### 3. Relational neighborhood
Which other actors it has co-appeared with, and in what roles. An actor can only act
with respect to actors it has a trace relation to. It cannot introduce entities it has
never been trace-adjacent to.

---

## The contrast with MiroFish

| Dimension | MiroFish | MeshAnt v5 |
|-----------|----------|------------|
| Actor identity | LLM-generated persona (MBTI, bio, profession) | Trace history as relational record |
| What drives action | Prescribed character | Enrollment by another actor (being named in a trace) |
| Action generation | "What would this persona post?" | "What trace would this relational position produce in response to this enrollment?" |
| Action form | Social media post (text) | Trace — (source, target, mediation, what_changed, observer) |
| Network constraint | None (persona acts freely) | Relational history constrains action space |
| Observer position | None (implicit god's-eye run) | Required — simulation is always a cut |
| New artifacts | Social media content, not traces | TraceDraft records, re-articulable into the mesh |

The key distinction: MiroFish uses the graph as a source of character descriptions.
MeshAnt should use the graph as the actual substance of action. The traces don't
describe the actor — they *are* the actor.

---

## The LLM's role in MeshAnt simulation

The LLM is not performing a persona. It is a constrained generator. Given:
- The full trace history of the actor being called on
- The trace that just enrolled this actor (the trigger)
- The constraint: only entities within the actor's historical neighborhood; only
  mediations within its historical repertoire

The LLM generates the next plausible trace. Not "what would this character do?" but
"what trace would this relational position produce in response to this enrollment?"

This is the inversion applied to generation: trace-first, not role-first. Even when
generating forward, the constraint comes from the mesh, not from the actor's character.

---

## Open design questions (must be answered before v5 planning)

1. **Unit of action**: What is one actor action? A single trace? A batch of traces?
   A temporal step covering all actors simultaneously?

2. **Conflict resolution**: What happens when two actors claim the same mediation in
   opposite directions? (This is the most interesting case — it is where the network
   resists the simulation.) How does MeshAnt surface conflict rather than resolving it?

3. **Simulation loop host**: Does the LLM generate one trace per actor per step (round-
   based, like MiroFish), or does propagation work differently — one trace triggering
   enrollments which trigger traces, etc. (cascade-based)? The cascade model is more
   ANT-native but harder to bound.

4. **What does "forward" mean in ANT terms?**: ANT is retrospective by method. Forward
   simulation requires a claim about what actors *would* do, which ANT is normally
   suspicious of. The simulation must be marked as a **hypothesis**, not a prediction.
   New traces are tagged `generated` and carry the simulation run as provenance.
   The simulation is a cut, not a truth claim.

---

## Why this comes after MCP and interactive CLI

- **MCP server** forces every analytical command to be clean, schema-declared, and
  programmatically callable — prerequisite for a simulation loop that calls articulate,
  diff, and shadow at each step.

- **Interactive CLI** builds understanding of what multi-turn articulation state looks
  like: how the analyst threads context across turns, what "interesting forward projection"
  means in practice. You need to have used the tool across real networks before you know
  what a simulation should illuminate.

The simulation is only interesting if it surfaces something the retrospective analysis
couldn't. What that is — what the forward projection shows that the backward reading
missed — is a question that has to be answered from experience before it can be designed.

---

## The standing tension

ANT traces are records of what happened. Simulation traces are projections of what
might happen. These are not the same epistemic category. MeshAnt must name this
distinction rather than collapsing it. The distinction is not a defect of the design —
it is the most interesting thing about it.

A simulation run in MeshAnt is always already a reading: situated, positioned,
generated under declared conditions. It is the mesh reading itself forward, not the
world unfolding. That framing, kept explicit, is what makes this different from
a prediction engine.

---

*This note records a design discussion, not a design decision.*
*It should be revisited when v4.x (interactive CLI) is complete.*
*See also: `docs/directions.md`, `tasks/plan_post_v3.md`, `reference/` (MiroFish).*
