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

## Milestone 3: Longitudinal Dataset and Time-Window Cut Axis

The goal of this milestone is to introduce temporal depth: a dataset that spans multiple
days and a second cut axis (time-window) that lets an articulation ask not just "what did
this observer see?" but "what did this observer see within this window?"

**Full plan:** `tasks/plan_m3.md`

### Tasks

- [x] **M3.1 — Write the longitudinal dataset**
  - `data/examples/deforestation_longitudinal.json` — 40 traces, 3 days
  - Extends the deforestation scenario: 2026-03-11 (20 existing) + 2026-03-14 (10) + 2026-03-18 (10)
  - All 8 observer positions; all 6 tag types per day; cross-day element persistence
  - `meshant/loader/longitudinal_test.go` — 19 tests
  - Branch: `feat/m3-dataset`

- [x] **M3.2 — Extend the graph articulation package with time-window axis**
  - New types: `TimeWindow`, `ShadowReason`
  - `ShadowElement` gains `Reasons []ShadowReason`
  - `ArticulationOptions` and `Cut` gain `TimeWindow`
  - `Articulate`: AND semantics, inclusive bounds, reason tracking per shadow element
  - `PrintArticulation`: time-window line + reason annotations in shadow section
  - `meshant/graph/graph_test.go` — new groups 6–9 (~26 tests)
  - `meshant/graph/e2e_test.go` — 9 new longitudinal e2e tests
  - Branch: `feat/m3-time-window`

- [x] **M3.3 — Record the time-window cut**
  - `docs/decisions/time-window-v1.md`
  - 8 decisions: TimeWindow in graph package, zero=full-cut, AND semantics, inclusive bounds,
    ShadowReason per element, Cut.TimeWindow stored verbatim, deferred items, relation to
    articulation-v1.md Decision 6

---

## Milestone 4: Graph Diff

Situated comparison of two articulations — recording what became visible or invisible between
two cuts. A diff is not a neutral changelog; it names both positions it compares.

**Full plan:** `tasks/plan_m4.md`

### Tasks

- [ ] **M4.1 — `Diff()` function and unit tests (groups 10–14, ~25 tests)**
  - New types: `ShadowShiftKind`, `ShadowShift`, `PersistedNode`, `GraphDiff`
  - `Diff(g1, g2 MeshGraph) GraphDiff` in `meshant/graph/graph.go`
  - `validTraceWithElements` helper in `meshant/graph/testhelpers_test.go`
  - Branch: `feat/m4-diff`

- [ ] **M4.2 — `PrintDiff()` and output tests (group 15, ~14 tests)**
  - `PrintDiff(w io.Writer, d GraphDiff) error` in `meshant/graph/graph.go`
  - Branch: `feat/m4-diff`

- [ ] **M4.3 — E2E tests (group 16, ~8 tests)**
  - `meshant/graph/e2e_test.go` — day-1 vs day-3 diffs from longitudinal dataset
  - Branch: `feat/m4-diff`

- [ ] **M4.4 — Decision record**
  - `docs/decisions/graph-diff-v1.md`
  - 10 decisions covering: directionality, edge/node identity, ShadowShiftKind, From/To verbatim
    copy, sort discipline, unconditional PrintDiff sections, deferred items

---

## Provisional: M5 — Graph-as-Actor

Not scoped. Core requirement: a produced `MeshGraph` or `GraphDiff` can be assigned a stable
identifier and appear as a `Source` or `Target` in subsequent traces. Once a graph is itself
traceable, MeshAnt can record its own observation apparatus as part of the mesh it observes.

Key open questions documented in `tasks/plan_m4.md` (Provisional M5 Note):
- Schema convention for graph-reference IDs (URI scheme vs structural prefix vs new convention)
- ID assignment for MeshGraph and GraphDiff (deterministic vs random UUID)
- schema.Validate() treatment for graph-reference strings
- loader.Load / MeshSummary treatment

---

## Notes

- Do not begin simulation, persona generation, or LLM integration before M1 is complete.
- Do not copy the Miro Fish schema or pipeline. Use it only as a reference for structural patterns.
- The trace schema should feel provisional and revisable — not like a finished ontology.
- Do not lock in a form factor (CLI / web app / agent framework). Let it emerge.
- Tag-filter axis deferred to M4+ (not implemented in M3 or M4).
- Graph-as-actor deferred to M5 (after graph-diff lands in M4).
