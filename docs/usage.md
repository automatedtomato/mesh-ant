# MeshAnt — Usage Guide

This guide covers installation, the interactive web UI, the full CLI, the LLM-assisted
ingestion pipeline, and the reference datasets. For the theoretical background see
[principles.md](./principles.md) and [glossary.md](./glossary.md).

---

## Installation

**From source (recommended):**

```bash
git clone https://github.com/automatedtomato/mesh-ant
cd mesh-ant/meshant
go build -o meshant ./cmd/meshant
```

**Via `go install`:**

```bash
go install github.com/automatedtomato/mesh-ant/meshant/cmd/meshant@latest
```

Go 1.22 or later required.

---

## Quick start — Web UI

The fastest way to explore a dataset is the interactive graph server.

```bash
meshant serve data/examples/software_incident.json
```

Open `http://localhost:8080`. The observer gate is the only thing shown on load —
every graph is a positioned reading, and the UI enforces this structurally.

**Available observer chips** appear below the input automatically, fetched from
`GET /observers`. Click a chip to fill the input, then press **Load graph**.

### What the UI shows

| Panel | Description |
|-------|-------------|
| **Graph** (centre) | Cytoscape force-directed graph of the observer-positioned cut. Nodes = elements, edges = traces. Node label shows appearance count. |
| **Shadow** (right, amber) | Elements that exist in the full substrate but are invisible from this observer position. A non-empty shadow panel means the world looks different from another position. |
| **Element detail** (right, grey) | Click any node or shadow item to load its trace cards: `what_changed`, timestamp, observer, source/target, mediation, tags. Session-promoted traces show a Provenance block. |
| **Cut header** (top bar) | Observer, time window, tag filter, trace count, shadow count — permanently visible. Every representation names its cut. |

### Export

- **Export JSON** — downloads the full articulate envelope (graph + cut metadata) as JSON.
- **Export DOT** — downloads a Graphviz DOT file mirroring `meshant articulate --format dot`.

### Changing port

```bash
meshant serve --port 9090 data/examples/software_incident.json
```

### Using a Neo4j database

```bash
meshant serve --db bolt://localhost:7687
# or via env var:
MESHANT_DB_URL=bolt://localhost:7687 meshant serve
```

---

## Reference datasets

Three hand-authored datasets ship with MeshAnt. Each covers a different domain and
demonstrates how the same substrate produces different graphs from different observer positions.

### D.1 — Software incident (`data/examples/software_incident.json`)

A payment service degradation during a flash sale. 32 traces, 4 observers.

```bash
meshant serve data/examples/software_incident.json
```

| Observer | What they see |
|----------|---------------|
| `on-call-engineer` | Connection pool exhaustion, retry-buffer amplification, circuit-breaker trigger, rollback |
| `product-manager` | Status-page updates, executive notification, incident timeline |
| `customer-support-lead` | Ticket surge, duplicate-order reports, escalation policy |
| `dataset-analyst` | LLM extraction/critique sessions (reflexive meta-traces) |

Key ANT demonstration: `retry-buffer` is a mediator for the engineer (it amplifies the
cascade) but is in shadow from the product-manager's position entirely.

### D.2 — Multi-agent pipeline (`data/examples/multi_agent_pipeline.json`)

An AI compliance pipeline processing SEC 10-K filings. 28 traces, 3 observers.
Eight pipeline agents appear as non-human actants in Source/Target — never as Observer.

```bash
meshant serve data/examples/multi_agent_pipeline.json
```

| Observer | What they see |
|----------|---------------|
| `pipeline-auditor` | Compliance scores, validation failures, routing decisions |
| `ml-engineer` | Model parameters, prompt templates, confidence thresholds, extraction truncation |
| `dataset-analyst` | LLM extraction/critique sessions |

Key ANT demonstration: inscription conflict — `compliance-taxonomy-2026q1` and the
classifier's embedded prompt template were written at different times with contradictory
category definitions. The auditor sees a validation failure; the engineer sees a version
mismatch. Neither sees the full picture.

### D.3 — Policy/procurement (`data/examples/policy_procurement.json`)

Metro City public-sector IT procurement for a $2.4M citizen-services-platform. 27 traces,
5 observers. 17 institutional actants (policies, regulations, checklists, enforcement systems)
vs 6 human actants — apparatus-thick, humans-thin.

```bash
meshant serve data/examples/policy_procurement.json
```

| Observer | What they see |
|----------|---------------|
| `procurement-officer` | Full procurement chain: requisition → RFP → evaluation → award |
| `budget-approver` | Only the budget escalation and council approval phase |
| `vendor-alpha` | Only the public comment channel and formal objection they filed |
| `compliance-auditor` | Only the audit and conflict-of-interest disclosure phase |
| `dataset-curator` | LLM extraction/critique sessions |

Key ANT demonstration: Principle 3 (mediation before intention) — policies, regulations,
and enforcement systems act without human deliberation throughout. `budget-ceiling-policy`
escalates the request automatically; `deadline-enforcement-system` rejects a vendor
without human adjudication; `procurement-appeals-regulation` suspends the award process
the moment an objection is filed.

---

## CLI reference

### Trace analysis

```
meshant summarize   traces.json
meshant validate    traces.json
meshant articulate  --observer <pos> [--tag <t>] [--from RFC3339] [--to RFC3339]
                    [--format text|json|dot|mermaid] [--output <file>] traces.json
meshant diff        --observer-a <pos> --observer-b <pos>
                    [--from-a RFC3339] [--to-a RFC3339] [--tag-a <t>]
                    [--from-b RFC3339] [--to-b RFC3339] [--tag-b <t>]
                    [--format text|json|dot|mermaid] [--output <file>] traces.json
meshant follow      --observer <pos> --element <name>
                    [--direction forward|backward] [--depth N]
                    [--criterion-file <path>] [--format text|json] traces.json
```

### Shadow and gap analysis

```
meshant shadow      --observer <pos> [--tag <t>] [--from RFC3339] [--to RFC3339]
                    [--output <file>] traces.json
meshant gaps        --observer-a <pos> --observer-b <pos>
                    [per-side --tag/--from/--to] [--output <file>] traces.json
meshant bottleneck  [--observer <pos>] [--tag <t>] [--from RFC3339] [--to RFC3339]
                    [--output <file>] traces.json
```

### Ingestion pipeline

```
meshant draft        [--source-doc <ref>] [--extracted-by <label>]
                     [--stage <stage>] [--output <file>] extraction.json
meshant promote      [--output <file>] drafts.json
meshant rearticulate [--id <id>] [--criterion-file <path>] [--output <file>] drafts.json
meshant lineage      [--id <id>] [--format text|json] drafts.json
meshant review       [--output <file>] drafts.json
```

### LLM-assisted ingestion

Requires `MESHANT_LLM_API_KEY` (or `ANTHROPIC_API_KEY` as fallback).

```
meshant extract     --source-doc <path> [--source-doc-ref <ref>]
                    [--prompt-template <path>] [--model <id>]
                    [--criterion-file <path>] [--output <file>] [--session-output <file>]

meshant assist      --spans-file <path> [--prompt-template <path>] [--model <id>]
                    [--source-doc-ref <ref>] [--criterion-file <path>]
                    [--output <file>] [--session-output <file>]

meshant critique    --input <path> [--prompt-template <path>] [--model <id>]
                    [--source-doc-ref <ref>] [--criterion-file <path>]
                    [--id <id>] [--output <file>] [--session-output <file>]

meshant split       --input <path> [--output <file>]

meshant promote-session  --session <path> [--output <file>] traces.json
```

`--source-doc` is repeatable — pass it multiple times to extract from several documents
in a single session. If `--source-doc-ref` is also given, its count must match.

### Multi-analyst comparison

```
meshant extraction-gap  --analyst-a <label> --analyst-b <label>
                        [--output <file>] drafts.json
meshant chain-diff      --analyst-a <label> --analyst-b <label>
                        --span <source_span> [--output <file>] drafts.json
```

### Store management

```
meshant store       [--db <bolt-url>] [--output <file>] traces.json
```

Writes a JSON trace file into a persistent store (JSON file or Neo4j). All analytical
commands accept `--db <bolt-url>` to query from Neo4j instead of a local file.

### Convert (non-text adapters)

```
meshant convert     --adapter pdf|html|jsonlog [--output <file>] input-file
```

Converts PDF, HTML, or structured log files to plain text suitable for `meshant extract`.

---

## CLI walkthrough — software incident dataset

```bash
# What is in the dataset?
meshant summarize data/examples/software_incident.json

# What did the on-call engineer see?
meshant articulate data/examples/software_incident.json \
  --observer on-call-engineer

# What did the product manager see?
meshant articulate data/examples/software_incident.json \
  --observer product-manager

# Compare the two positions — makes structural blindness explicit
meshant diff data/examples/software_incident.json \
  --observer-a on-call-engineer \
  --observer-b product-manager

# What is in shadow from the engineer's position?
meshant shadow data/examples/software_incident.json \
  --observer on-call-engineer

# Follow the retry-buffer translation chain
meshant follow data/examples/software_incident.json \
  --observer on-call-engineer --element retry-buffer

# Export as Graphviz DOT, render with graphviz
meshant articulate data/examples/software_incident.json \
  --observer on-call-engineer --format dot | dot -Tpng -o engineer-view.png
```

---

## End-to-end walkthrough: from real documents to the web UI

This walkthrough takes three real-world source documents — a PDF meeting memo, an HTML news
page, and a Markdown IR draft — and turns them into an interactive graph in the browser.

### Step 1 — Convert non-text sources to plain text

`meshant extract` reads plain text. PDF and HTML need converting first; Markdown is already
plain text and can be passed directly.

```bash
meshant convert --adapter pdf   meeting_memo.pdf  --output memo.txt
meshant convert --adapter html  news_page.html    --output news.txt
# ir_draft.md needs no conversion
```

### Step 2 — Extract trace drafts from all three documents in one session

Pass each document with a human-readable reference label. One LLM call is made per document;
all drafts are aggregated into a single output file and share one session record.

```bash
export MESHANT_LLM_API_KEY=sk-ant-...

meshant extract \
  --source-doc memo.txt \
  --source-doc news.txt \
  --source-doc ir_draft.md \
  --source-doc-ref "meeting-memo" \
  --source-doc-ref "news-page" \
  --source-doc-ref "ir-draft" \
  --output raw_drafts.json \
  --session-output session.json
```

Each draft in `raw_drafts.json` carries its provenance: which source doc, which model, which
session. The LLM is a mediator here — every draft is a candidate, not a fact.

### Step 3 — Interactive review

Accept, edit, or skip each draft one at a time. This is where you name observer positions,
correct mediations, and remove noise.

```bash
meshant review raw_drafts.json --output reviewed_drafts.json
```

### Step 4 — Optional: LLM critique pass

Ask the LLM to check the reviewed drafts for consistency and ANT alignment. Review any
flagged items before promoting.

```bash
meshant critique \
  --input reviewed_drafts.json \
  --output critiqued_drafts.json
```

### Step 5 — Promote to canonical traces

```bash
meshant promote --output traces.json reviewed_drafts.json
```

Each accepted draft becomes a `Trace`. Session-promoted traces carry a Provenance block
visible in the web UI element detail panel.

### Step 6 — Serve and open the web UI

```bash
meshant serve traces.json
# → open http://localhost:8080
```

The observer gate appears. The chips below the input (fetched from `GET /observers`) show
every observer position in the substrate — click a chip and press **Load graph** to
see the graph, shadow panel, and element detail for that position.

---

## LLM-assisted ingestion pipeline

The full pipeline for extracting traces from a single source document:

```bash
export MESHANT_LLM_API_KEY=sk-ant-...

# 1. LLM extracts candidate trace drafts
meshant extract \
  --source-doc my-incident-report.md \
  --output raw_drafts.json \
  --session-output session.json

# 2. Interactive review — accept / edit / skip each draft
meshant review raw_drafts.json --output reviewed_drafts.json

# 3. LLM critiques the reviewed drafts
meshant critique \
  --input reviewed_drafts.json \
  --output critiqued_drafts.json

# 4. Promote to canonical traces
meshant promote --output traces.json reviewed_drafts.json

# 5. Analyse from a named position
meshant articulate --observer analyst --format json traces.json
```

Multi-document extraction (one session, multiple source docs):

```bash
meshant extract \
  --source-doc report-part-1.md \
  --source-doc report-part-2.md \
  --source-doc-ref "part-1" \
  --source-doc-ref "part-2" \
  --output drafts.json
```

See `data/examples/llm_assisted_extraction/` for a complete worked example.

---

## HTTP API

`meshant serve` exposes a JSON API on the same port as the web UI.
All endpoints return `{ "cut": CutMeta, "data": ... }`.

| Endpoint | Parameters | Returns |
|----------|------------|---------|
| `GET /observers` | — | `[]string` — sorted list of observer names in the substrate |
| `GET /articulate` | `observer` (required), `from`, `to`, `tags` | `MeshGraph` |
| `GET /diff` | `observer-a`, `observer-b` (both required), `from`, `to`, `tags` | `GraphDiff` |
| `GET /shadow` | `observer` (required), `from`, `to`, `tags` | `[]ShadowElement` |
| `GET /traces` | `observer` (required), `from`, `to`, `tags`, `limit` | `[]Trace` |
| `GET /element/{name}` | `observer` (required), `from`, `to`, `tags` | `[]Trace` |

`observer` is required on all endpoints except `/observers` — every reading is positioned.

---

## Trace schema

A trace is the minimal unit of record. All fields except `id`, `timestamp`, `what_changed`,
and `observer` are optional.

```json
{
  "id":           "a1b2c3d4-...",
  "timestamp":    "2026-06-10T14:00:00Z",
  "what_changed": "retry-buffer amplified connection-pool failures by a factor of 3",
  "source":       ["connection-pool"],
  "target":       ["retry-buffer"],
  "mediation":    "circuit-breaker-policy",
  "tags":         ["amplification", "threshold"],
  "observer":     "on-call-engineer"
}
```

**`mediation`** — name the actant that transforms what passes between source and target.
Leave empty when the passage is a faithful relay. The same actant can be a mediator
from one observer position and an intermediary from another.

**`tags`** — open vocabulary. Canonical values: `delay`, `threshold`, `blockage`,
`amplification`, `redirection`, `translation`, `articulation`, `session`.

See [authoring-traces.md](./authoring-traces.md) for a full authoring guide.

---

## Further reading

- [Manifesto](./manifesto.md) — why this project exists
- [Principles](./principles.md) — the 8 design principles in detail
- [Glossary](./glossary.md) — mediator, intermediary, cut, shadow, articulation, inscription
- [ANT Notes](./ant-notes.md) — Actor-Network Theory grounding
- [Authoring traces](./authoring-traces.md) — how to write good traces
- [Decisions](./decisions/) — one decision record per milestone
