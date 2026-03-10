# MeshAnt — Task Tracking

## Language and form factor

- **Go** — primary implementation language (trace schema, loader, any CLI or pipeline work)
- **Python** — LLM API integration and reference code only (when/if needed)
- **JSON/YAML** — trace data format (language-agnostic, inspectable)
- **Form factor** — deliberately left open. Do not force a shape (CLI / web app / agent) before the project has followed enough traces to warrant it. Let the form emerge from the work.

---

## Milestone 1: Trace Schema and Minimal Loader

The goal of this milestone is to define what a trace is in MeshAnt and demonstrate that traces can be loaded and inspected, without defining actors or roles in advance.

### Tasks

- [ ] **M1.1 — Define the trace schema**
  - Create `meshant/schema/trace.go`
  - A trace record must capture at minimum:
    - `id`: unique identifier
    - `timestamp`: when the trace was recorded
    - `what_changed`: a short description of what difference was made
    - `source`: what produced the trace (left intentionally loose — could be a human, a rule, a threshold, a queue)
    - `target`: what was affected (also loose)
    - `mediation`: optional — what transformed, redirected, or relayed the action
    - `tags`: optional list of descriptors (e.g. `delay`, `threshold`, `blockage`, `amplification`)
    - `observer`: who or what recorded this trace, and from what position
  - Use Go structs with JSON tags for serialization
  - Do not pre-specify what `source` or `target` must be — keep them as strings or open structures
  - Record the design rationale briefly (what cut was made and why) in a comment or `docs/decisions/`

- [ ] **M1.2 — Write a small example trace dataset**
  - Create `data/examples/traces.json`
  - Write 8–12 hand-crafted traces representing a simple scenario
  - Suggested scenario: a message passing through a bureaucratic queue (submitted → routed → delayed → re-routed → approved/rejected)
  - Ensure the traces include: at least one delay, one threshold crossing, one redirection, one non-human mediator (e.g. a rule, a form, a rate-limiter)
  - Do not assign fixed actor roles — let source/target be descriptive but open

- [ ] **M1.3 — Write a minimal trace loader**
  - Create `meshant/loader/loader.go`
  - Load the trace dataset from a JSON file
  - Output a simple provisional mesh summary:
    - list of all sources and targets that appear
    - frequency of each element across traces
    - list of observed mediations
    - list of traces with delays or thresholds tagged
  - Print as a readable plain-text report to stdout
  - No LLM, no persona generation, no simulation engine

- [ ] **M1.4 — Record the schema cut**
  - Create `docs/decisions/trace-schema-v1.md`
  - Explain briefly:
    - what was included in the schema and why
    - what was deliberately excluded
    - what assumptions the schema makes about what counts as a trace
  - This is a first-class MeshAnt design principle: the designer is inside the mesh

---

## Notes

- Do not begin simulation, persona generation, or LLM integration before M1 is complete.
- Do not copy the Miro Fish schema or pipeline. Use it only as a reference for structural patterns.
- The trace schema should feel provisional and revisable — not like a finished ontology.
- Do not lock in a form factor (CLI / web app / agent framework). Let it emerge.
