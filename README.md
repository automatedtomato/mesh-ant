# MeshAnt

**MeshAnt** is an experimental framework for building AI agent systems from **traces** rather than predefined roles.

Contemporary AI agents are often modeled as miniature workers: planner, reviewer, coder, manager.  
MeshAnt takes a different path.

Drawing inspiration from **Bruno Latour** and **Actor-Network Theory (ANT)**, MeshAnt begins with a simple methodological shift:

**before asking what the actors are, follow the traces of what makes a difference.**

Forms, delays, thresholds, interfaces, metrics, notifications, sensors, rules, queues, and price displays can all redirect action.  
Agency is not treated here as an exclusively human property, but as something that emerges through mediation, articulation, and relation.

MeshAnt is a **trace-first, articulation-first** experiment for simulations and distributed systems where actors are not assumed in advance, but provisionally assembled from traces, frictions, and mediations.

## Core principles

MeshAnt follows a simple methodological shift inspired by **Bruno Latour** and **Actor-Network Theory (ANT)**:

**before defining actors, follow the traces of what makes a difference.**

- **Trace before actor**  
  Do not begin by deciding what the actors are. Begin by following traces.

- **Articulation before ontology**  
  Record how distinctions are drawn before deciding what things essentially are.

- **Mediation before intention**  
  Do not explain every transformation through a subject with goals; follow the mediations.

- **Difference before role**  
  Roles are only one way of stabilizing action. Start from what produces differences.

- **Plural observers before god’s-eye view**  
  Preserve multiple observation positions instead of collapsing the mesh into one total perspective.

- **Re-articulation before essence**  
  Treat actors and boundaries as revisable effects, not fixed substances.

- **Friction and asymmetry matter**  
  Delays, thresholds, opacity, and uneven access are not noise around the system; they help compose it.

- **The designer is inside the mesh**  
  Every schema is already an intervention. Record the cut instead of hiding it.

## Documents

- [Manifesto](./docs/manifesto.md)
- [Principles](./docs/principles.md)
- [ANT Notes](./docs/ant-notes.md)

## What MeshAnt is trying to do

MeshAnt does not start by defining a set of fixed human-like agents and assigning them roles.

Instead, it asks:

- what leaves a trace?
- what makes a difference?
- what mediates action?
- where do thresholds, delays, and frictions appear?
- under what conditions does something become actor-like?

This is not a rejection of agents.
It is an attempt to loosen the assumption that agency must always begin in person-like roles.

## Demo

The demo constructs two observer-position cuts on a coastal evacuation order dataset and diffs them to make the epistemic asymmetry between the two positions visible.

**Cut A — meteorological-analyst, 2026-04-14 (T-72h):**
sees the sensor and model chain that triggers the alert. The political and logistical network is in shadow.

**Cut B — local-mayor, 2026-04-16 (T-24h):**
sees the mandatory order, media broadcast, resident friction, shelter overflow, road constraints. The sensor and model chain is in shadow.

The diff names both absences simultaneously — a provisional reading, not a god's-eye account.

### Run with Docker

```bash
docker build -t mesh-ant-demo .
docker run --rm mesh-ant-demo
```

### Run with alternate dataset (volume mount)

```bash
docker run --rm \
  -v /path/to/your/dataset.json:/data/dataset.json \
  mesh-ant-demo /data/dataset.json
```

### Run from source

```bash
cd meshant
go run ./cmd/demo
```

A path argument overrides the default dataset:

```bash
go run ./cmd/demo /path/to/dataset.json
```

### Known gap — Principle 8

The demo records observer positions (`meteorological-analyst`, `local-mayor`) but does not record its own position: the choice of these two cuts, these parameters, this rendering. Tracked as M7-B.

