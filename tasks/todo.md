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

- [x] **M1.2 — Write a small example trace dataset**
  - `data/examples/traces.json` — 10 traces, all passing Validate()
  - Scenario: vendor registration through a government procurement office
  - Covers: delay ×2, threshold ×3, redirection ×2, blockage ×1, translation ×2
  - Non-human mediators: form-validator, queue-policy, classification-ruleset,
    approval-threshold-rule, routing-matrix, background-check-webhook, approval-checklist
  - Absent-source traces: #3 (automated resubmission), #9 (webhook with no system id)

- [x] **M1.3 — Write a minimal trace loader**
  - `meshant/loader/loader.go` — Load(), Summarise(), PrintSummary(io.Writer) error
  - `meshant/loader/loader_test.go` + `e2e_test.go` — 56 tests, 100% coverage
  - Followed by: e2e test, code review, security review, architecture review (A+)
  - All HIGH/MEDIUM findings resolved before merge

- [x] **M1.4 — Record the schema cut**
  - Done: `docs/decisions/trace-schema-v1.md` (completed alongside M1.1)

---

## Milestone 2: Deforestation Dataset and Graph Articulation

The goal of this milestone is to introduce a richer, multi-threaded dataset and a first
articulation layer: a way to render a provisional graph from traces taken from a
particular observer position. A graph is a cut — not a god's-eye view. Every cut names
its shadow.

**Full plan:** `tasks/plan_m2.md`

### Tasks

- [x] **M2.1 — Write the deforestation example dataset**
  - `data/examples/deforestation.json` — 20 traces, 3 crossing threads
  - Branch: `feat/m2-dataset` (merged to develop)
  - 8 observer positions; absent-source ×3; multi-source ×4; multi-target ×1
  - All 6 tag types; 19 validation tests, 100% coverage

- [x] **M2.2 — Write the graph articulation package**
  - `meshant/graph/graph.go` — Articulate(), PrintArticulation(), MeshGraph with shadow
  - `meshant/graph/graph_test.go` + `e2e_test.go` — 42 tests, 100% coverage
  - Branch: `feat/m2-graph`
  - Observer position as primary cut axis; shadow mandatory; full cut named
  - Code + security reviews passed; all HIGH/MEDIUM findings resolved

- [x] **M2.3 — Record the articulation cut**
  - `docs/decisions/articulation-v1.md`
  - 6 decisions: observer axis, shadow mandatory, empty=full-cut, ExcludedObserverPositions,
    graph-as-actor noted, time/tag axes deferred

---

## Notes

- Do not begin simulation, persona generation, or LLM integration before M1 is complete.
- Do not copy the Miro Fish schema or pipeline. Use it only as a reference for structural patterns.
- The trace schema should feel provisional and revisable — not like a finished ontology.
- Do not lock in a form factor (CLI / web app / agent framework). Let it emerge.
- M2 introduces observer-positioned articulation. Do not implement time-window or
  tag-filter cut axes in M2 — defer to M3+.
- Graph-as-actor (the produced graph entering the mesh as a force) is noted
  architecturally but deferred to a later milestone.
