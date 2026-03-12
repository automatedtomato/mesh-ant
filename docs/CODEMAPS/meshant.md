# MeshAnt — Codemap

**Last Updated:** 2026-03-12
**Module:** `github.com/automatedtomato/mesh-ant/meshant`
**Go Version:** 1.25
**Root Directory:** `/meshant`

## Package Overview

| Package | Purpose |
|---------|---------|
| `schema` | Core trace types, graph-reference predicates, and validators. |
| `loader` | Load traces from JSON, summarize datasets, print summaries. |
| `graph` | Articulate graphs, compute diffs, identify graphs as actors, reflexive tracing. |
| `cmd/demo` | Minimal demonstration: two observer-position cuts on evacuation dataset. |

## Package: schema

### Files

| File | Contains |
|------|----------|
| `trace.go` | `Trace` struct; `TagValue` constants; `Validate()` method. |
| `graphref.go` | Graph-reference string predicates (`IsGraphRef`, `GraphRefKind`, `GraphRefID`). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `Trace` | `ID` (uuid), `Timestamp` (time), `WhatChanged` (string), `Source` ([]string), `Target` ([]string), `Mediation` (string), `Tags` ([]string), `Observer` (string, required) | Fundamental unit of record: a moment where something made a difference in a network. |
| `TagValue` | (string constant type) | Vocabulary for trace descriptors: `TagDelay`, `TagThreshold`, `TagBlockage`, `TagAmplification`, `TagRedirection`, `TagTranslation`, `TagValueArticulation`. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Trace.Validate()` | `(t Trace) Validate() error` | Validate required fields (ID, Timestamp, WhatChanged, Observer). Returns all violations joined. |
| `IsGraphRef` | `func IsGraphRef(s string) bool` | Check if string is a graph-reference (prefix "meshgraph:" or "meshdiff:"). |
| `GraphRefKind` | `func GraphRefKind(s string) string` | Return kind prefix ("meshgraph", "meshdiff", or empty). |
| `GraphRefID` | `func GraphRefID(s string) string` | Extract UUID portion after prefix. |

## Package: loader

### Files

| File | Contains |
|------|----------|
| `loader.go` | `Load`, `Summarise`, `PrintSummary`; `MeshSummary`, `FlaggedTrace` types. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `MeshSummary` | `Elements` (map[string]int), `Mediations` ([]string), `MediatedTraceCount` (int), `FlaggedTraces` ([]FlaggedTrace), `GraphRefs` ([]string) | Provisional first-pass reading of a trace dataset. |
| `FlaggedTrace` | `ID` (string), `WhatChanged` (string), `Tags` ([]string) | Minimal projection of traces tagged delay or threshold. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Load` | `func Load(path string) ([]schema.Trace, error)` | Load JSON file, decode traces, validate via schema.Validate(); max 50 MB. |
| `Summarise` | `func Summarise(traces []schema.Trace) MeshSummary` | Build MeshSummary from validated traces: count elements, deduplicate mediations, flag delay/threshold, extract graph-refs. |
| `PrintSummary` | `func PrintSummary(w io.Writer, s MeshSummary) error` | Write formatted summary to io.Writer. Elements sorted by descending frequency, mediations in encounter order. |

## Package: graph

### Files

| File | Contains |
|------|----------|
| `graph.go` | `MeshGraph`, `Node`, `Edge`, `Cut`, `ShadowElement`, `ShadowReason`, `TimeWindow`, `ArticulationOptions`. `Articulate`, `PrintArticulation` functions. Filter logic, edge/node/shadow builders. |
| `diff.go` | `GraphDiff`, `ShadowShift`, `PersistedNode`, `ShadowShiftKind`. `Diff`, `PrintDiff` functions. Diff computation helpers. |
| `actor.go` | Graph-as-actor identity: `IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`, `newUUID4`. |
| `serial.go` | Custom JSON codec for `TimeWindow`: `MarshalJSON`, `UnmarshalJSON`. Null encoding for unbounded bounds. |
| `reflexive.go` | Reflexive tracing: `ArticulationTrace`, `DiffTrace`. Functions that record articulation and diffing as traces. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `TimeWindow` | `Start` (time.Time), `End` (time.Time) | Inclusive temporal range; zero bounds mean unbounded. |
| `ShadowReason` | (string constant: `ShadowReasonObserver`, `ShadowReasonTimeWindow`) | Why an element is in the shadow. |
| `ArticulationOptions` | `ObserverPositions` ([]string), `TimeWindow` (TimeWindow) | Parameters for Articulate: empty ObserverPositions = full cut. |
| `MeshGraph` | `ID` (string, actor identity), `Nodes` (map[string]Node), `Edges` ([]Edge), `Cut` (Cut) | Articulated graph from trace dataset with observer position and shadow. |
| `Node` | `Name` (string), `AppearanceCount` (int), `ShadowCount` (int) | Element and its visibility across included vs. shadow traces. |
| `Edge` | `TraceID`, `WhatChanged`, `Mediation`, `Observer`, `Sources`, `Targets`, `Tags` (all []string) | One trace in the graph, preserving source context. |
| `Cut` | `ObserverPositions`, `TimeWindow`, `TracesIncluded`, `TracesTotal`, `DistinctObserversTotal`, `ShadowElements`, `ExcludedObserverPositions` | Metadata naming the articulation position and shadow. |
| `ShadowElement` | `Name` (string), `SeenFrom` ([]string), `Reasons` ([]ShadowReason) | Element in shadow: visible from excluded observer positions, excluded for named reasons. |
| `GraphDiff` | `ID` (string), `NodesAdded`, `NodesRemoved` ([]string), `NodesPersisted` ([]PersistedNode), `EdgesAdded`, `EdgesRemoved` ([]Edge), `ShadowShifts` ([]ShadowShift), `From`, `To` (Cut) | Comparison of two MeshGraph articulations. |
| `ShadowShift` | `Name`, `Kind` (ShadowShiftKind), `FromReasons`, `ToReasons` ([]ShadowReason) | Element movement across shadow boundary between two graphs (emerged, submerged, reason-changed). |
| `ShadowShiftKind` | (string constant: `ShadowShiftEmerged`, `ShadowShiftSubmerged`, `ShadowShiftReasonChanged`) | Direction of element movement. |
| `PersistedNode` | `Name`, `CountFrom`, `CountTo` (int) | Node present in both graphs with appearance count from each. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `TimeWindow.IsZero` | `(tw TimeWindow) IsZero() bool` | Check if both Start and End are zero (no time filter). |
| `TimeWindow.Validate` | `(tw TimeWindow) Validate() error` | Validate that Start ≤ End (if both non-zero). |
| `TimeWindow.MarshalJSON` | `(tw TimeWindow) MarshalJSON() ([]byte, error)` | Encode zero bounds as JSON null, non-zero as RFC3339. |
| `TimeWindow.UnmarshalJSON` | `(tw *TimeWindow) UnmarshalJSON(data []byte) error` | Decode JSON null as zero time.Time, strings as RFC3339. |
| `Articulate` | `func Articulate(traces []schema.Trace, opts ArticulationOptions) MeshGraph` | Build MeshGraph from traces and cut parameters. Splits traces into included/excluded, computes nodes/edges/shadow. ID field is empty (not identified as actor). |
| `PrintArticulation` | `func PrintArticulation(w io.Writer, g MeshGraph) error` | Write human-readable articulation to io.Writer. Includes observer positions, nodes, edges, shadow elements with reasons. |
| `Diff` | `func Diff(g1, g2 MeshGraph) GraphDiff` | Compare two MeshGraph articulations. Computes nodes added/removed/persisted, edges added/removed, shadow shifts. ID field is empty. |
| `PrintDiff` | `func PrintDiff(w io.Writer, d GraphDiff) error` | Write human-readable diff comparison to io.Writer. Includes From/To cuts, node and edge changes, shadow shifts. |
| `IdentifyGraph` | `func IdentifyGraph(g MeshGraph) MeshGraph` | Assign fresh UUID to g.ID; return updated graph (immutable pattern). |
| `IdentifyDiff` | `func IdentifyDiff(d GraphDiff) GraphDiff` | Assign fresh UUID to d.ID; return updated diff. |
| `GraphRef` | `func GraphRef(g MeshGraph) (string, error)` | Return "meshgraph:<g.ID>" graph-reference string. Error if g.ID empty. |
| `DiffRef` | `func DiffRef(d GraphDiff) (string, error)` | Return "meshdiff:<d.ID>" graph-reference string. Error if d.ID empty. |
| `ArticulationTrace` | `func ArticulationTrace(g MeshGraph, observer string, source []string) (schema.Trace, error)` | Produce Trace recording the act of articulation (reflexive tracing). g must be identified; observer required. Target set to GraphRef(g). Always passes schema.Validate. |
| `DiffTrace` | `func DiffTrace(d GraphDiff, g1, g2 MeshGraph, observer string) (schema.Trace, error)` | Produce Trace recording the act of diffing. All three graphs must be identified; observer required. Source = [GraphRef(g1), GraphRef(g2)], Target = [DiffRef(d)]. |

## Package: cmd/demo

### Files

| File | Contains |
|------|----------|
| `main.go` | Demo entry point and pipeline: `main()` (flag parsing), `run()` (full pipeline), `printClosingNote()` (methodological coda). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `main` | `func main()` | Entry point. Accepts dataset path argument or uses default. Logs errors. |
| `run` | `func run(w io.Writer, datasetPath string) error` | Full pipeline: Load → Summary → Articulate (Cut A: meteorological-analyst, 2026-04-14) → Articulate (Cut B: local-mayor, 2026-04-16) → Diff → Closing note. |
| `printClosingNote` | `func printClosingNote(w io.Writer) error` | Write methodological coda naming observer positions, shadows, and Principle 8 gap. |

## Cross-Package Relationships

```
schema/
  ├─→ (used by) loader
  ├─→ (used by) graph
  └─→ (imported by) graph/reflexive

loader/
  ├─→ imports: schema
  └─→ (used by) cmd/demo

graph/
  ├─→ imports: schema
  ├─→ (modules within graph: graph.go, diff.go, actor.go, serial.go, reflexive.go)
  └─→ (used by) cmd/demo

cmd/demo/
  ├─→ imports: graph, loader
  └─→ (uses patterns from) graph, schema
```

### Import Cycle Prevention

- `schema` has no internal dependencies → safe to import everywhere
- `graph` imports `schema` only
- `graph/reflexive` imports `schema` to construct `Trace` records
- `graph.actor` defines graph-reference prefixes; `schema.graphref` carries matching unexported copies to avoid import cycle

## Key Data Flows

### Trace Loading Pipeline
1. `loader.Load(path)` → reads JSON, validates each trace via `schema.Validate()`
2. `loader.Summarise(traces)` → counts elements, deduplicates mediations, extracts graph-refs, flags delay/threshold
3. `loader.PrintSummary(w, summary)` → renders summary to writer

### Articulation Pipeline
1. `graph.Articulate(traces, opts)` → filters by ObserverPositions and TimeWindow
2. Splits traces into included/excluded
3. Builds Nodes (from included) and ShadowElements (from excluded-only)
4. Returns `MeshGraph` with empty ID (not an actor yet)
5. Optional: `graph.IdentifyGraph(g)` → assigns UUID, making it traceable
6. Optional: `graph.ArticulationTrace(g, observer, source)` → records articulation as reflexive Trace

### Diff Pipeline
1. `graph.Diff(g1, g2)` → compares two MeshGraphs
2. Computes NodesAdded/Removed/Persisted by name
3. Computes EdgesAdded/Removed by TraceID
4. Computes ShadowShifts (emerged, submerged, reason-changed)
5. Returns `GraphDiff` with empty ID (not an actor yet)
6. Optional: `graph.IdentifyDiff(d)` → assigns UUID
7. Optional: `graph.DiffTrace(d, g1, g2, observer)` → records diff as reflexive Trace

### Demo Pipeline (cmd/demo/main.go)
1. Load evacuation_order.json
2. Print mesh summary
3. Articulate Cut A (meteorological-analyst, 2026-04-14)
4. Print Cut A
5. Articulate Cut B (local-mayor, 2026-04-16)
6. Print Cut B
7. Diff Cut A → Cut B
8. Print diff
9. Print closing note (Principle 8 gap)

## Where to Find Things

| Task | Look In |
|------|----------|
| Define or validate a trace | `schema/trace.go` |
| Check if a string is a graph-reference | `schema/graphref.go` |
| Load traces from JSON | `loader/loader.go` → `Load()` |
| Build summary statistics | `loader/loader.go` → `Summarise()` |
| Print human-readable summary | `loader/loader.go` → `PrintSummary()` |
| Articulate a cut from traces | `graph/graph.go` → `Articulate()` |
| Understand observer position and shadow | `graph/graph.go` → `Cut`, `ShadowElement` types |
| Print articulated graph | `graph/graph.go` → `PrintArticulation()` |
| Compare two graphs | `graph/diff.go` → `Diff()` |
| Understand element movement | `graph/diff.go` → `ShadowShift` type |
| Print diff output | `graph/diff.go` → `PrintDiff()` |
| Identify graph as actor | `graph/actor.go` → `IdentifyGraph()` |
| Get graph-reference string | `graph/actor.go` → `GraphRef()` |
| Record articulation in traces | `graph/reflexive.go` → `ArticulationTrace()` |
| Record diff in traces | `graph/reflexive.go` → `DiffTrace()` |
| TimeWindow JSON encoding | `graph/serial.go` → `MarshalJSON()`, `UnmarshalJSON()` |
| Run minimal demo | `cmd/demo/main.go` → `run()` |

## Notable Design Patterns

### Immutability
- `Articulate()` returns `MeshGraph` with empty `ID` (not automatically identified as actor)
- `Diff()` returns `GraphDiff` with empty `ID`
- `IdentifyGraph()` and `IdentifyDiff()` accept by value and return new values; inputs never mutated
- All slice fields (`Tags`, `Sources`, `Targets`) in `Edge` are defensive copies

### Defensive Copying
- `Edge.Tags`, `Edge.Sources`, `Edge.Targets` are independent of source trace
- `Cut.ObserverPositions` is a copy in `MeshGraph`
- `ShadowElement.SeenFrom` and `Reasons` are copies in `MeshGraph`

### Graph-as-Actor (M5)
- Identified graphs appear in traces via `GraphRef(g)` → "meshgraph:<uuid>"
- Identified diffs appear via `DiffRef(d)` → "meshdiff:<uuid>"
- String format follows ANT generalised symmetry: no privileged field

### Reflexive Tracing (M7)
- `ArticulationTrace()` records when a graph is articulated
- `DiffTrace()` records when two graphs are compared
- Both produce traces that pass `schema.Validate()` unconditionally
- Not called automatically; explicit opt-in by caller

### Shadow as Mandatory Output
- Every `MeshGraph` includes `Cut.ShadowElements` even when empty
- Shadow names what the cut cannot see (elements in excluded traces only)
- `PrintArticulation()` and `PrintDiff()` always show shadow sections
- Encodes Principle 8: the observer is positioned, not omniscient

### JSON Serialization
- `TimeWindow` encodes zero bounds as `null`, non-zero as RFC3339 strings
- All graph types carry JSON struct tags; `TimeWindow` alone needs custom codec
- Design rationale: zero bound means "unbounded" (not a real timestamp)

## Related Decision Records

- `docs/decisions/trace-schema-v1.md` — core Trace type rationale
- `docs/decisions/articulation-v1.md` — observer position and shadow design
- `docs/decisions/time-window-v1.md` — temporal filtering
- `docs/decisions/graph-diff-v1.md` — diff computation and shadow shifts
- `docs/decisions/graph-as-actor-v1.md` — identified graphs as actants
- `docs/decisions/m7-serialisation-reflexivity-v1.md` — TimeWindow JSON codec and reflexive tracing

## Test Coverage

- `schema/trace_test.go` — 27 tests, 100%
- `schema/graphref_test.go` — 14 tests, 100%
- `loader/loader_test.go` — 56 tests, 100%
- `graph/graph_test.go` — 84 tests (including M3 time-window tests), 99.3%
- `graph/diff_test.go` — 41 tests, 100%
- `graph/actor_test.go` — 15 tests, 100%
- `graph/serial_test.go` — 19 tests, 100%
- `graph/reflexive_test.go` — 19 tests, 100%
- `cmd/demo/main_test.go` — E2E test
- `loader/evacuation_test.go` — 27 tests (M6 dataset), all green
