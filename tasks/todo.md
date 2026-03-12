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

## Milestone 4: Graph Diff — COMPLETE (merged to develop)

Situated comparison of two articulations. A diff is not a neutral changelog; it records
both positions it compares and what became visible or invisible between them.

**Full plan:** `tasks/plan_m4.md`

- [x] **M4.1** — `Diff()` + `GraphDiff`, `PersistedNode`, `ShadowShift`, `ShadowShiftKind` types; 47 unit tests (groups 10–15)
- [x] **M4.2** — `PrintDiff()` + output tests
- [x] **M4.3** — E2E tests against longitudinal dataset (group 16, 8 tests)
- [x] **M4.4** — `docs/decisions/graph-diff-v1.md`; `docs/potential-forms.md`

125 tests total; 99% graph coverage, 100% loader + schema.

---

## Milestone 5: Graph-as-Actor — COMPLETE (merged to develop)

The observation apparatus enters the mesh it observes. A produced `MeshGraph` or
`GraphDiff` can be assigned a stable identifier and appear as a `Source` or `Target`
in subsequent traces. Reflexivity: the framework can observe its own action in the network.

**Full plan:** `tasks/plan_m5.md`

- [x] **M5.1 — Schema additions**
  - `IsGraphRef`, `GraphRefKind`, `GraphRefID` in `meshant/schema/graphref.go`
  - `parseGraphRef` private helper using `strings.Cut`; 14 tests, 100% coverage
  - Branch: `feat/m5-schema` (merged to develop)

- [x] **M5.2 — Graph actor additions**
  - `ID string` on `MeshGraph` and `GraphDiff` (zero = not an actor)
  - `meshant/graph/actor.go`: `IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`, `newUUID4`
  - 15 tests (groups 17–21); branch: `feat/m5-actor` (merged to develop)

- [x] **M5.3 — Loader addition and new dataset**
  - `GraphRefs []string` on `MeshSummary`; `Summarise` populates from source/target
  - `data/examples/graph_ref_traces.json` — 6 traces with 3 distinct graph-refs
  - 14 unit tests (groups 5–6) + 5 E2E tests (group 7), loader 100% coverage
  - Branch: `feat/m5-loader` (merged to develop)

- [x] **M5.4 — Decision record**
  - `docs/decisions/graph-as-actor-v1.md`
  - 10 decisions; ideological grounding (ANT/Strathern/Haraway/Principle 8)

---

## Milestone 6: Minimal Demo + Docker — release 0.2.0

A minimal demo is a cut of the current state of the framework, not a determination of the
final product. It shows what form has emerged from following the traces so far. The form
factor is not forced — it arises from the work.

The demo will itself become an actant: once it runs, it shapes what gets built next.
That is not a reason to avoid it; it is a reason to make the cut deliberately.

**Known gap:** the demo will run articulations but will not record those acts as traces.
Principle 8 remains partially open — the framework observes but does not yet observe itself
observing. This is tracked as M7-B.

**Deforestation dataset:** retained as development data and future demo variation.
The demo binary accepts a path argument; the Docker image supports volume mount.

**Full plan:** `tasks/plan_m6.md`

### Tasks

- [x] **M6.1 — Design the demo cut and re-plan M6** *(complete)*
  - Scenario: coastal evacuation order (category-3 hurricane, 72h window)
  - Cut A: `meteorological-analyst`, 2026-04-14 only (T-72h)
  - Cut B: `local-mayor`, 2026-04-16 only (T-24h)
  - Chosen for maximal epistemic asymmetry; non-human actants central; structural blindness made visible by diff

- [x] **M6.2 — Write the evacuation order dataset**
  - Branch: `feat/m6-dataset`
  - `data/examples/evacuation_order.json` — 28 traces, 3 days (2026-04-14/15/16), 6 observer positions
  - 14 actants including 5 non-human; all 6 tag types; mediation on ≥40% of traces; ≥1 graph-ref trace
  - `meshant/loader/evacuation_test.go` — validation tests

- [x] **M6.3 — Write the demo entry point**
  - Branch: `feat/m6-demo`
  - `meshant/cmd/demo/main.go` — `run(io.Writer, string) error` + thin `main()`
  - `meshant/cmd/demo/main_test.go` — 7 tests (black-box, package `demo_test`)
  - Pipeline: Load → PrintSummary → Articulate A → PrintArticulation → Articulate B → PrintArticulation → Diff → PrintDiff → closing note naming the shadow
  - Stdlib only: no new dependencies

- [x] **M6.4 — Docker environment + tag release v0.2.0**
  - Branch: `feat/m6-docker`
  - `Dockerfile` — multi-stage build (golang:1.25-alpine builder, alpine:latest runtime)
  - `.dockerignore` — exclude .git, test files, dev artifacts
  - `docker build -t mesh-ant-demo . && docker run --rm mesh-ant-demo` produces full demo output
  - Volume mount supports deforestation dataset as variation
  - Merge chain: feat/m6-dataset → feat/m6-demo → feat/m6-docker → develop → main
  - Release notes: scenario, two cuts, diff, Docker usage, shadow named (M7-A/B)
  - `git tag v0.2.0 -a -m "v0.2.0: minimal demo — coastal evacuation order, two observer cuts, diff, named shadow"`

---

## Milestone 7: Graph Serialisation + Reflexive Tracing — release v0.3.0

Closes the two gaps named at M6. M7-A makes graphs durable (immutable mobiles that travel
without deforming). M7-B closes the Principle 8 loop: the act of articulation becomes a trace;
the resulting graph enters the mesh as an actant. M7-B depends on M7-A.

**Full plan:** `tasks/plan_m7.md`

### Tasks

- [x] **M7.1 — JSON codec**
  - `meshant/graph/serial.go` — json tags on all graph types; custom TimeWindow codec (null for zero bounds)
  - `meshant/graph/serial_test.go` — round-trip MeshGraph + GraphDiff; JSON snapshot; null/zero TimeWindow
  - Branch: `feat/m7-codec`

- [x] **M7.2 — Reflexive tracing**
  - `meshant/schema/trace.go` — add `TagValueArticulation = "articulation"`
  - `meshant/graph/reflexive.go` — `ArticulationTrace(g, observer, source)`, `DiffTrace(d, g1, g2, observer)`
  - `meshant/graph/reflexive_test.go` — produced traces pass Validate(); error cases
  - Branch: `feat/m7-reflexive`

- [x] **M7.3 — Decision record**
  - `docs/decisions/m7-serialisation-reflexivity-v1.md`
  - 8 decisions: codec scope, TimeWindow null, observer-as-param, absent source for articulation,
    derived source for diff, new tag, mediation=function name, what M7 does not close

---

## Milestone 8: Structured Export + Second Domain — release v0.3.1

Closes the two gaps named post-M7: structured output (graphs leave stdout) and second demo
domain (validates generality beyond the evacuation scenario).

**Full plan:** `tasks/plan_m8.md` (planned in session, not written to file)

### Tasks

- [x] **M8.1 — JSON export**
  - `meshant/graph/export.go` — `PrintGraphJSON`, `PrintDiffJSON`; 100% coverage
  - Branch: `feat/m8-json-export`

- [x] **M8.2 — DOT + Mermaid export**
  - `meshant/graph/export.go` — `PrintGraphDOT`, `PrintGraphMermaid`; shadow in cluster/subgraph; Cartesian product for multi-source edges; 100% coverage
  - Branch: `feat/m8-dot-mermaid`

- [x] **M8.3 — File persistence package**
  - `meshant/persist/persist.go` — `WriteJSON`, `ReadGraphJSON`, `ReadDiffJSON`; 100% coverage
  - Branch: `feat/m8-persist`

- [x] **M8.4 — Second demo domain**
  - `data/examples/incident_response.json` — 22 traces, e-commerce API outage, 2 days, 5 observers, 8 non-human actants
  - `meshant/loader/incident_test.go` + `meshant/graph/incident_e2e_test.go`
  - Branch: `feat/m8-incident-dataset`

- [x] **M8.5 — Decision record + codemap**
  - `docs/decisions/structured-export-v1.md` — 6 decisions
  - `docs/CODEMAPS/meshant.md` — updated for M8

---

## Notes

- Do not begin simulation, persona generation, or LLM integration until the framework is stable.
- Do not copy the Miro Fish schema or pipeline. Use it only as a reference for structural patterns.
- The trace schema should feel provisional and revisable — not like a finished ontology.
- Do not lock in a form factor (CLI / web app / agent framework). Let it emerge.
- Tag-filter cut axis deferred to M5+ (not implemented in M3, M4, or M5).
- Graph-as-actor fulfilled in M5; graph-diff fulfilled in M4.
- Form factor (CLI) emerged in M9; now suitable for users who code in other languages or prefer CLI use.

---

## Milestone 9: CLI + Docs + Release v1.0.0

Library + CLI form. The framework can be used without writing Go.

**Full plan:** `tasks/plan_m9.md`

### Tasks

- [x] **M9.1 — CLI core: `summarize` and `validate`**
  - `meshant/cmd/meshant/main.go` — `run()`, `cmdSummarize()`, `cmdValidate()`, `usage()`
  - `meshant/cmd/meshant/main_test.go` — 10 tests; Branch: `feat/m9-cli-core`

- [x] **M9.2 — CLI `articulate` subcommand**
  - `--observer` (repeatable), `--from`, `--to`, `--format text|json|dot|mermaid`
  - `stringSliceFlag`; `parseTimeFlag`; 20 tests; Branch: `feat/m9-cli-articulate`

- [x] **M9.3 — CLI `diff` subcommand**
  - `--observer-a/b`, per-side time windows, `--format text|json`
  - `parseTimeWindow` helper; 30 tests; Branch: `feat/m9-cli-diff`

- [x] **M9.4 — Trace authoring guide**
  - `docs/authoring-traces.md` — 188 lines, 8 sections, worked example
  - Branch: `feat/m9-authoring-guide`

- [x] **M9.5 — README, decision record, Dockerfile**
  - README: "Who is this for?", CLI usage, removed stale Principle 8 gap note
  - `docs/decisions/cli-v1.md` — 6 decisions; Dockerfile: CLI at `/usr/local/bin/meshant`
  - Branch: `feat/m9-readme`

- [x] **M9.6 — Refactor and clean pass (whole codebase)**
  - Stale milestone comments removed; `go vet ./...` clean; Branch: `feat/m9-refactor`

- [x] **M9.7 — Philosophical review**
  - Two violations fixed: `"no time filter"` → `"full temporal cut"` (articulation-first, B1+B2)
  - `docs/reviews/review_philosophical_m9.md`; Verdict: VIOLATION FOUND — REFACTORED
  - Branch: `feat/m9-philosophical-review`

- [x] **M9.8 — Codemap + release v1.0.0**
  - `docs/CODEMAPS/meshant.md` updated with `cmd/meshant` package and new docs
  - Merged to main; tagged v1.0.0

37 CLI tests, 92.9% `cmd/meshant` coverage; `go vet` clean across all packages.
