# MeshAnt

**MeshAnt** is an experimental framework for analysing socio-technical systems from **traces** rather than predefined roles.

Contemporary systems — incident response pipelines, procurement processes, multi-agent AI workflows — are often modelled as collections of actors with fixed roles. MeshAnt takes a different path.

Drawing inspiration from **Bruno Latour** and **Actor-Network Theory (ANT)**, MeshAnt begins with a simple methodological shift:

**before asking what the actors are, follow the traces of what makes a difference.**

Forms, delays, thresholds, interfaces, metrics, notifications, sensors, rules, queues, and policies can all redirect action. Agency is not treated here as an exclusively human property, but as something that emerges through mediation, articulation, and relation.

## Who is this for?

MeshAnt is useful when behaviour emerges from interactions between services, policies, interfaces, delays, and human actions rather than from a single explicit actor.

If you are debugging a multi-agent pipeline, auditing a procurement process, or mapping how a decision propagated through a network of tools and people, MeshAnt gives you a way to articulate what each observer position could and could not see — without claiming a god's-eye view.

## Core principles

- **Trace before actor** — do not begin by deciding what the actors are; begin by following traces.
- **Articulation before ontology** — record how distinctions are drawn before deciding what things essentially are.
- **Mediation before intention** — follow the mediations rather than imputing goals.
- **Difference before role** — start from what produces differences, not from assigned roles.
- **Plural observers** — preserve multiple observation positions; never collapse to one total perspective.
- **Re-articulation before essence** — treat actors and boundaries as revisable effects, not fixed substances.
- **Friction matters** — delays, thresholds, opacity, and uneven access help compose the system.
- **The designer is inside the mesh** — every schema is already an intervention; record the cut.

Full elaboration: [docs/principles.md](./docs/principles.md)

## Quick start

```bash
git clone https://github.com/automatedtomato/mesh-ant
cd mesh-ant/meshant
go build -o meshant ./cmd/meshant

# Serve a dataset in the interactive web UI
./meshant serve ../data/examples/software_incident.json
# → open http://localhost:8080
```

The observer gate is the first thing you see. Available observer positions are shown as
clickable chips — no guessing required. Select one and load the graph.

Three reference datasets ship with MeshAnt:

| Dataset | Domain | Traces | Command |
|---------|--------|--------|---------|
| `software_incident.json` | Payment service outage | 32 | `meshant serve data/examples/software_incident.json` |
| `multi_agent_pipeline.json` | AI compliance pipeline | 28 | `meshant serve data/examples/multi_agent_pipeline.json` |
| `policy_procurement.json` | Public-sector IT procurement | 27 | `meshant serve data/examples/policy_procurement.json` |

## CLI

```bash
# What is in the dataset?
meshant summarize data/examples/software_incident.json

# What did the on-call engineer see?
meshant articulate data/examples/software_incident.json --observer on-call-engineer

# Compare two positions — makes structural blindness explicit
meshant diff data/examples/software_incident.json \
  --observer-a on-call-engineer \
  --observer-b product-manager

# What is in shadow from one position?
meshant shadow data/examples/software_incident.json --observer on-call-engineer

# Follow a translation chain
meshant follow data/examples/software_incident.json \
  --observer on-call-engineer --element retry-buffer

# Export as Graphviz DOT
meshant articulate data/examples/software_incident.json \
  --observer on-call-engineer --format dot | dot -Tpng -o view.png
```

**→ [Full usage guide](./docs/usage.md)** — web UI, all CLI commands, LLM ingestion pipeline, HTTP API, trace schema.

## v2.0.0 — LLM-Assisted Ingestion

`meshant extract`, `meshant assist`, and `meshant critique` call the LLM directly to
produce TraceDraft records with full provenance. The LLM is a **mediator**, not an
extractor — every draft carries a model ID, session reference, and uncertainty note.

```bash
export MESHANT_LLM_API_KEY=sk-ant-...

meshant extract --source-doc report.md --output drafts.json
meshant review drafts.json --output reviewed.json
meshant promote --output traces.json reviewed.json
meshant articulate --observer analyst traces.json
```

See [docs/usage.md — LLM ingestion pipeline](./docs/usage.md#llm-assisted-ingestion-pipeline) for the full walkthrough.

## Documents

- [Usage guide](./docs/usage.md) — installation, web UI, CLI, API, trace schema
- [Manifesto](./docs/manifesto.md) — why this project exists
- [Principles](./docs/principles.md) — 8 design principles in detail
- [Glossary](./docs/glossary.md) — mediator, intermediary, cut, shadow, articulation
- [ANT Notes](./docs/ant-notes.md) — Actor-Network Theory grounding
- [Authoring traces](./docs/authoring-traces.md) — how to write good traces
- [Decision records](./docs/decisions/) — one per milestone
