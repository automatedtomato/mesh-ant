# MeshAnt

**MeshAnt** is an experimental framework for building AI agent systems from **traces** rather than predefined roles.

Contemporary AI agents are often modeled as miniature workers: planner, reviewer, coder, manager.  
MeshAnt takes a different path.

Drawing inspiration from **Bruno Latour** and **Actor-Network Theory (ANT)**, MeshAnt begins with a simple methodological shift:

**before asking what the actors are, follow the traces of what makes a difference.**

Forms, delays, thresholds, interfaces, metrics, notifications, sensors, rules, queues, and price displays can all redirect action.  
Agency is not treated here as an exclusively human property, but as something that emerges through mediation, articulation, and relation.

MeshAnt is a **trace-first, articulation-first** experiment for simulations and distributed systems where actors are not assumed in advance, but provisionally assembled from traces, frictions, and mediations.

## Who is this for?

MeshAnt is a trace-first framework for making sense of messy distributed or
socio-technical systems — especially when behavior emerges from interactions between
services, policies, interfaces, delays, and human actions rather than from a single
explicit actor.

If you are debugging a multi-agent pipeline, auditing a procurement process, or mapping
how a decision propagated through a network of tools and people, MeshAnt gives you a
way to articulate what each observer position could and could not see — without
claiming a god's-eye view.

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

The `meshant` CLI binary is also available — see the [CLI](#cli) section below for
`articulate`, `diff`, and other commands.

## CLI

Install the binary:

```bash
go install github.com/automatedtomato/mesh-ant/meshant/cmd/meshant@latest
```

Or build from source:

```bash
git clone https://github.com/automatedtomato/mesh-ant
cd mesh-ant
go build -o meshant ./meshant/cmd/meshant
```

### Commands

```
meshant summarize   traces.json
meshant validate    traces.json
meshant articulate  traces.json --observer <pos> [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid]
meshant diff        traces.json --observer-a <pos> --observer-b <pos> [--from-a ...] [--to-a ...] [--from-b ...] [--to-b ...] [--format text|json]
```

### Example

```bash
# Summarise what actors and mediations are in the dataset
meshant summarize data/examples/evacuation_order.json

# Articulate what the meteorological analyst saw on day 1
meshant articulate data/examples/evacuation_order.json \
  --observer meteorological-analyst \
  --from 2026-04-14T00:00:00Z --to 2026-04-14T23:59:59Z

# Compare two observer positions — makes structural blindness visible
meshant diff data/examples/evacuation_order.json \
  --observer-a meteorological-analyst --from-a 2026-04-14T00:00:00Z --to-a 2026-04-14T23:59:59Z \
  --observer-b local-mayor           --from-b 2026-04-16T00:00:00Z --to-b 2026-04-16T23:59:59Z

# Export as Graphviz DOT
meshant articulate data/examples/evacuation_order.json \
  --observer meteorological-analyst --format dot > graph.dot
```

