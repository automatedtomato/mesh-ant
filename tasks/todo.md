# MeshAnt — Task Tracking

## Language and form factor

- **Go** — primary implementation language (trace schema, loader, any CLI or pipeline work)
- **Python** — LLM API integration and reference code only (when/if needed)
- **JSON/YAML** — trace data format (language-agnostic, inspectable)
- **Form factor** — deliberately left open. Do not force a shape (CLI / web app / agent) before the project has followed enough traces to warrant it. Let the form emerge from the work.

## Branch strategy

- `main` — stable
- `develop` — primary development branch (cut from main)
- `feat/*` — feature branches cut from **develop**

---

## Milestone 1: Trace Schema and Minimal Loader

The goal of this milestone is to define what a trace is in MeshAnt and demonstrate that traces can be loaded and inspected, without defining actors or roles in advance.

### Tasks

- [x] **M1.1 — Define the trace schema**
  - `meshant/schema/trace.go` — Trace struct, TagValue constants, Validate()
  - `meshant/schema/trace_test.go` — 27 tests, all passing
  - `docs/decisions/trace-schema-v1.md` — design decision record
  - Key decisions: source/target as []string, observer required, tags open vocabulary

- [ ] **M1.2 — Write a small example trace dataset**
  - Branch: `feat/m1-trace-dataset` (cut from develop) — **IN PROGRESS**
  - Plan: `tasks/plan_m1_2.md`
  - Create `data/examples/traces.json`
  - Scenario: vendor registration application through a government procurement office
    (submitted → validated → rate-limited → classified → threshold-escalated → compliance review → approved)
  - 10 traces covering: delay ×2, threshold ×3, redirection ×2, blockage ×1, translation ×2
  - Non-human mediators: form-validator, queue-policy, classification-ruleset,
    approval-threshold-rule, routing-matrix, background-check-webhook, approval-checklist
  - Multi-source traces: #1 (vendor+form), #4 (rate-limiter+policy), #10 (reviewer+checklist)
  - Absent-source traces: #3 (automated resubmission), #9 (webhook with no system id)
  - All traces must pass meshant/schema/trace.go Validate()

- [ ] **M1.3 — Write a minimal trace loader**
  - Create `meshant/loader/loader.go`
  - Load dataset from JSON file
  - Output provisional mesh summary to stdout:
    - all sources and targets that appear
    - frequency of each element
    - observed mediations
    - traces tagged with delay or threshold
  - No LLM, no simulation

- [x] **M1.4 — Record the schema cut**
  - Done: `docs/decisions/trace-schema-v1.md` (completed alongside M1.1)

---

## Notes

- Do not begin simulation, persona generation, or LLM integration before M1 is complete.
- Do not copy the Miro Fish schema or pipeline. Use it only as a reference for structural patterns.
- The trace schema should feel provisional and revisable — not like a finished ontology.
- Do not lock in a form factor (CLI / web app / agent framework). Let it emerge.
