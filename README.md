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
- [Glossary](./docs/glossary.md)

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

## v2.0.0 — LLM-Assisted Ingestion

v2.0.0 moves the LLM boundary inside the CLI. Three new subcommands call the LLM directly:

- **`meshant extract`** — takes a source document, calls the LLM once, produces `weak-draft`
  TraceDraft records with full provenance (`extracted_by`, `session_ref`, `uncertainty_note`).
- **`meshant assist`** — interactive session: presents each source span to the user, calls the
  LLM for a candidate draft, asks the user to accept / edit / skip. Skipped and accepted drafts
  are both preserved — shadow is not absence.
- **`meshant critique`** — takes existing TraceDraft records, calls the LLM to produce
  `critiqued` derived drafts with `derived_from` links to the originals.

The LLM is a **mediator**, not an extractor. Every LLM-produced draft carries the model ID
(`extracted_by`), a session link (`session_ref` → `SessionRecord`), and a framework-appended
uncertainty note. No LLM draft enters the mesh without a named analytical position.

A `SessionRecord` is written on every code path — success, partial, and error. It records the
extraction conditions (model ID, prompt template, source document reference, system
instructions, timestamp) alongside every draft ID produced.

**API key setup:**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**Example pipeline:**

```bash
# Step 1: LLM extracts candidate drafts from a source document
meshant extract \
  --source-doc data/examples/llm_assisted_extraction/source_document.md \
  --output raw_drafts.json \
  --session-output session.json

# Step 2: Interactive human review — accept, edit, or skip each draft
meshant assist \
  --spans-file spans.txt \
  --output reviewed_drafts.json

# Step 3: LLM critiques existing drafts — produces derived re-articulations
meshant critique \
  --input reviewed_drafts.json \
  --output critiqued_drafts.json

# Step 4: Promote reviewed drafts to canonical traces
meshant promote --output traces.json reviewed_drafts.json

# Step 5: Articulate from a named observer position
meshant articulate --observer registry-security-team --format json traces.json
```

See `data/examples/llm_assisted_extraction/` for a complete worked example with provenance
analysis and documented analytical divergences between the LLM's reading and a human reviewer's.

See `docs/decisions/llm-as-mediator-v1.md` and `docs/decisions/llm-boundary-v2.md` for
the design decisions governing this boundary.

---

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

**Trace analysis**

```
meshant summarize   traces.json
meshant validate    traces.json
meshant articulate  --observer <pos> [--tag <t>] [--from RFC3339] [--to RFC3339] [--format text|json|dot|mermaid] [--output <file>] traces.json
meshant diff        --observer-a <pos> --observer-b <pos> [per-side --tag/--from/--to] [--format text|json|dot|mermaid] [--output <file>] traces.json
meshant follow      --observer <pos> --element <name> [--direction forward|backward] [--depth N] [--criterion-file <path>] [--format text|json] traces.json
```

**Shadow and gap analysis**

```
meshant shadow      --observer <pos> [--tag <t>] [--from RFC3339] [--to RFC3339] [--output <file>] traces.json
meshant gaps        --observer-a <pos> --observer-b <pos> [per-side --tag/--from/--to] [--output <file>] traces.json
```

**Ingestion pipeline**

```
meshant draft       [--source-doc <ref>] [--extracted-by <label>] [--stage <stage>] [--output <file>] extraction.json
meshant promote     [--output <file>] drafts.json
meshant rearticulate [--id <id>] [--criterion-file <path>] [--output <file>] drafts.json
meshant lineage     [--id <id>] [--format text|json] drafts.json
```

**LLM-assisted ingestion (v2.0.0)**

Requires `ANTHROPIC_API_KEY` set in the environment.

```
meshant extract     --source-doc <path> [--source-doc-ref <ref>] [--prompt-template <path>] [--model <id>] [--criterion-file <path>] [--output <file>] [--session-output <file>]
meshant assist      --spans-file <path> [--prompt-template <path>] [--model <id>] [--source-doc-ref <ref>] [--criterion-file <path>] [--output <file>] [--session-output <file>]
meshant critique    --input <path> [--prompt-template <path>] [--model <id>] [--source-doc-ref <ref>] [--criterion-file <path>] [--id <id>] [--output <file>] [--session-output <file>]
```

**Interactive review**

```
meshant review      [--output <file>] drafts.json
meshant bottleneck  [--observer <pos>] [--tag <t>] [--from RFC3339] [--to RFC3339] [--output <file>] traces.json
```

**Multi-analyst comparison**

```
meshant extraction-gap  --analyst-a <label> --analyst-b <label> [--output <file>] drafts.json
meshant chain-diff      --analyst-a <label> --analyst-b <label> --span <source_span> [--output <file>] drafts.json
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

# Show what is shadowed from the meteorological analyst's position
meshant shadow data/examples/evacuation_order.json \
  --observer meteorological-analyst

# Compare what each observer can and cannot see — neither is authoritative
meshant gaps data/examples/evacuation_order.json \
  --observer-a meteorological-analyst \
  --observer-b local-mayor

# Export as Graphviz DOT
meshant articulate data/examples/evacuation_order.json \
  --observer meteorological-analyst --format dot > graph.dot

# Follow a translation chain through the graph
meshant follow data/examples/evacuation_order.json \
  --observer meteorological-analyst --element evacuation-order
```

