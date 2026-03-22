# M6 Plan — Minimal Demo + Docker (release v0.2.0)

The demo is a **cut** of the current state of the framework, not a determination of final form.
It shows what form has emerged from following the traces so far.

**Known gap (named explicitly):** The demo runs articulations but does not record those acts
as traces. Principle 8 remains partially open — the framework observes but does not yet observe
itself observing. This is tracked as M7-B.

**Deforestation dataset:** retained as development data and a future demo variation. Nothing
is deleted. The demo binary accepts a path argument, so
`data/examples/deforestation_longitudinal.json` can be passed directly to the Docker image.

---

## The Scenario: Coastal Evacuation Order

A category-3 hurricane approaches a coastal region over 72 hours. Multiple actors — human
and non-human — must decide when an advisory becomes a mandatory evacuation order.

The network is dense with translations, threshold crossings, frictions, and asymmetries.
Storm track models, tide gauges, shelter capacity databases, and road infrastructure are
*genuinely* agentic in the ANT sense: they do not merely transmit, they transform what the
next actor can do. The moment "forecast" becomes "order" is not a single decision but a chain
of translations across heterogeneous entities that most participants never see.

This scenario is Latourian in the late sense: *Facing Gaia*, *Down to Earth* — climate,
infrastructure, institutions, and bodies that refuse to stay in their assigned categories.

---

## Actants (human and non-human)

| Actant | Type |
|---|---|
| storm-track-model-nhc | non-human (forecast output) |
| tide-gauge-sensor-network | non-human (instrument ensemble) |
| surge-inundation-model | non-human (simulation output) |
| road-capacity-model | non-human (logistics model) |
| shelter-database | non-human (resource registry) |
| national-meteorological-service | institution |
| federal-emergency-management-agency | institution |
| local-mayor | human / political |
| emergency-management-director | human / operational |
| meteorological-analyst | human / epistemic |
| coastal-resident-association | collective |
| hospital-administrator | human / operational |
| media-broadcast-network | institution |
| utility-grid-operator | institution |

---

## Observer Positions (6)

| Position | Sees |
|---|---|
| `meteorological-analyst` | storm models, sensor data, uncertainty ranges, advisory chain |
| `emergency-management-director` | logistics, resource allocation, inter-agency coordination |
| `local-mayor` | political pressure, liability, public communications, the order |
| `coastal-resident` | immediate environment, social trust/distrust, mobility constraints |
| `hospital-administrator` | patient evacuation, medical convoy, critical infrastructure |
| `media-correspondent` | official statements, public information flow |

---

## Temporal Structure (28 traces, 3 days)

| Day | Label | Traces | Key events |
|---|---|---|---|
| 2026-04-14 | T-72h | 10 | Storm advisory issued; models disagree on track; tide sensors first elevated; media broadcasts initial advisory |
| 2026-04-15 | T-48h | 9 | Storm upgraded; surge model crosses 3m threshold; voluntary evacuation recommended; hospital begins patient triage |
| 2026-04-16 | T-24h | 9 | Mandatory evacuation order issued; road capacity model activates; shelter database hits 80%; resident-association fractures; utility grid pre-emptive shutdown |

All 6 tag types across all 3 days. Non-human actants appear in `source` and `target` throughout.
At least one graph-ref trace. `mediation` field used on ≥40% of traces.

---

## The Two Demo Cuts

**Cut A — `meteorological-analyst`, 2026-04-14 only (T-72h)**
- Sees: storm-track-model, tide gauges, surge-inundation-model, advisory chain
- Shadow: everything political, social, logistical — the entire "order" apparatus is invisible
- Dominated by: `translation`, `threshold`, `mediation`

**Cut B — `local-mayor`, 2026-04-16 only (T-24h)**
- Sees: the mandatory order, media broadcast, resident friction, road capacity, shelter overflow
- Shadow: all scientific data, sensor readings, the model chain that triggered the order
- Dominated by: `friction`, `blockage`, `translation`

**Why this pair:** maximal epistemic asymmetry. The analyst sees the causal origin of the
order but not its political execution. The mayor sees the order as a political-legal act but
not the non-human chain that made it necessary. The diff makes both blindnesses visible
simultaneously — that is the ANT demonstration.

---

## File structure

```
data/
  examples/
    evacuation_order.json           — 28-trace dataset
meshant/
  cmd/
    demo/
      main.go                       — run(io.Writer, string) error + main()
      main_test.go                  — 7 tests, package demo_test
  loader/
    evacuation_test.go              — validation tests for new dataset
Dockerfile                          — multi-stage build
.dockerignore                       — exclude .git, test files, dev artifacts
```

---

## Pipeline in run()

1. Resolve dataset path (argument or default `../../../data/examples/evacuation_order.json`)
2. `loader.Load(path)` — load and validate all 28 traces
3. `loader.Summarise` + `loader.PrintSummary` — full dataset header
4. `graph.Articulate(traces, optsA)` — Cut A (meteorological-analyst, T-72h)
5. `graph.PrintArticulation(w, gA)`
6. `graph.Articulate(traces, optsB)` — Cut B (local-mayor, T-24h)
7. `graph.PrintArticulation(w, gB)`
8. `graph.Diff(gA, gB)`
9. `graph.PrintDiff(w, d)`
10. Closing note naming the shadow: the act of articulation is not recorded as a trace (M7-B)

`main()` calls `run(os.Stdout, path)` and `log.Fatalf` on error. Thin wrapper only.

---

## Time windows

```go
day1Start = time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
day1End   = time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC)
day3Start = time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
day3End   = time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)

optsA = graph.ArticulationOptions{
    ObserverPositions: []string{"meteorological-analyst"},
    TimeWindow:        graph.TimeWindow{Start: day1Start, End: day1End},
}
optsB = graph.ArticulationOptions{
    ObserverPositions: []string{"local-mayor"},
    TimeWindow:        graph.TimeWindow{Start: day3Start, End: day3End},
}
```

---

## Tests (main_test.go)

Package: `demo_test` (black-box, consistent with codebase convention)

| Test | Assertion |
|---|---|
| `TestDemo_Run_NoError` | `run(buf, path)` returns nil |
| `TestDemo_Run_OutputContainsMeshSummary` | output contains `"=== Mesh Summary"` |
| `TestDemo_Run_OutputContainsBothArticulations` | output contains two `"=== Mesh Articulation"` blocks |
| `TestDemo_Run_OutputContainsDiff` | output contains `"=== Mesh Diff"` |
| `TestDemo_Run_OutputNamesObservers` | output contains `"meteorological-analyst"` and `"local-mayor"` |
| `TestDemo_Run_OutputContainsShadow` | output contains `"Shadow"` |
| `TestDemo_Run_InvalidPath_ReturnsError` | `run(buf, "/nonexistent.json")` returns non-nil error |

Coverage target: `run()` 100%.

---

## Docker environment (M6.3)

**Multi-stage build** — compile in `golang:1.25-alpine`, run in `alpine:latest`.
Final image contains only the binary and the evacuation dataset. No source code at runtime.

```
Stage 1 (builder):  golang:1.25-alpine
  WORKDIR /src/meshant
  COPY meshant/ .
  RUN go build -o /demo ./cmd/demo/

Stage 2 (runtime):  alpine:latest
  COPY --from=builder /demo /demo
  COPY data/examples/evacuation_order.json /data/examples/evacuation_order.json
  ENTRYPOINT ["/demo", "/data/examples/evacuation_order.json"]
```

**Standard usage:**
```bash
docker build -t mesh-ant-demo .
docker run --rm mesh-ant-demo
```

**Deforestation variation (volume mount):**
```bash
docker run --rm -v $(pwd)/data:/data mesh-ant-demo /data/examples/deforestation_longitudinal.json
```

---

## Path note

From `meshant/cmd/demo/`, the dataset resolves as:
`../../../data/examples/evacuation_order.json`
(up 3 levels: demo/ → cmd/ → meshant/ → repo root, then into data/examples/)

Inside the Docker image the path is `/data/examples/evacuation_order.json` (passed as ENTRYPOINT arg).

---

## Release checklist

**Before merging feat/m6-demo → develop:**
- [ ] `go build ./cmd/demo/` from `meshant/` succeeds
- [ ] `go test ./...` from `meshant/` passes (all existing + 7 new)
- [ ] `go vet ./...` clean
- [ ] `go run ./cmd/demo/` from `meshant/` produces readable output
- [ ] Manual: output contains both articulations and diff
- [ ] Security: no hardcoded secrets; path passed directly to `os.Open` (safe for trusted local files)
- [ ] File sizes: `main.go` < 200 lines, `main_test.go` < 150 lines
- [ ] Code-reviewer agent review

**Before merging feat/m6-docker → develop:**
- [ ] `docker build -t mesh-ant-demo .` succeeds from repo root
- [ ] `docker run --rm mesh-ant-demo` produces readable output (both articulations + diff)
- [ ] Volume mount with deforestation dataset works

**Before tagging v0.2.0 on main:**
- [ ] feat/m6-dataset, feat/m6-demo, feat/m6-docker → develop merged and green
- [ ] develop → main merged and green
- [ ] Release notes: dataset, two observer cuts, what the diff shows, Docker usage, shadow named (M7-A/B)
- [ ] `git tag v0.2.0 -a -m "v0.2.0: minimal demo — coastal evacuation order, two observer cuts, diff, named shadow"`
- [ ] `tasks/todo.md` M6.1–M6.4 marked complete
