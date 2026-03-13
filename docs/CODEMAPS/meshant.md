# MeshAnt — Codemap

**Last Updated:** 2026-03-13
**Module:** `github.com/automatedtomato/mesh-ant/meshant`
**Go Version:** 1.25
**Root Directory:** `/meshant`

## Package Overview

| Package | Purpose |
|---------|---------|
| `schema` | Core trace types, graph-reference predicates, and validators. |
| `loader` | Load traces from JSON, summarize datasets, print summaries. |
| `graph` | Articulate graphs, compute diffs, identify graphs as actors, reflexive tracing, export to JSON/DOT/Mermaid. |
| `persist` | Read and write graphs to JSON files. |
| `cmd/demo` | Minimal demonstration: two observer-position cuts on evacuation dataset. |
| `cmd/meshant` | CLI entry point: `summarize`, `validate`, `articulate`, `diff` subcommands. |

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
| `export.go` | Export functions: `PrintGraphJSON`, `PrintDiffJSON`, `PrintGraphDOT`, `PrintGraphMermaid`, `PrintDiffDOT`, `PrintDiffMermaid`. Internal helpers for DOT/Mermaid formatting. `stripNewlines()` security helper prevents injection from crafted trace values. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `TimeWindow` | `Start` (time.Time), `End` (time.Time) | Inclusive temporal range; zero bounds mean unbounded. |
| `ShadowReason` | (string constant: `ShadowReasonObserver`, `ShadowReasonTagFilter`, `ShadowReasonTimeWindow`) | Why an element is in the shadow (three reasons, sorted alphabetically). |
| `ArticulationOptions` | `ObserverPositions` ([]string), `TimeWindow` (TimeWindow), `Tags` ([]string) | Parameters for Articulate: three cut axes. Empty = full cut on that axis. |
| `MeshGraph` | `ID` (string, actor identity), `Nodes` (map[string]Node), `Edges` ([]Edge), `Cut` (Cut) | Articulated graph from trace dataset with observer position and shadow. |
| `Node` | `Name` (string), `AppearanceCount` (int), `ShadowCount` (int) | Element and its visibility across included vs. shadow traces. |
| `Edge` | `TraceID`, `WhatChanged`, `Mediation`, `Observer`, `Sources`, `Targets`, `Tags` (all []string) | One trace in the graph, preserving source context. |
| `Cut` | `ObserverPositions`, `TimeWindow`, `Tags`, `TracesIncluded`, `TracesTotal`, `DistinctObserversTotal`, `ShadowElements`, `ExcludedObserverPositions` | Metadata naming the articulation position and shadow (three cut axes). |
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
| `PrintGraphJSON` | `func PrintGraphJSON(w io.Writer, g MeshGraph) error` | Export `MeshGraph` as JSON to io.Writer. |
| `PrintDiffJSON` | `func PrintDiffJSON(w io.Writer, d GraphDiff) error` | Export `GraphDiff` as JSON to io.Writer. |
| `PrintGraphDOT` | `func PrintGraphDOT(w io.Writer, g MeshGraph) error` | Export `MeshGraph` as Graphviz DOT format. Includes node labels and edge cardinality. |
| `PrintGraphMermaid` | `func PrintGraphMermaid(w io.Writer, g MeshGraph) error` | Export `MeshGraph` as Mermaid flowchart syntax. Sanitized IDs and truncated labels for readability. |
| `PrintDiffDOT` | `func PrintDiffDOT(w io.Writer, d GraphDiff) error` | Export `GraphDiff` as Graphviz DOT format. Added=green/bold, removed=red/dashed, persisted with count labels, shadow shifts in cluster subgraph. |
| `PrintDiffMermaid` | `func PrintDiffMermaid(w io.Writer, d GraphDiff) error` | Export `GraphDiff` as Mermaid flowchart. Same visual conventions as DOT diff; style directives for color coding. |

## Package: persist

### Files

| File | Contains |
|------|----------|
| `persist.go` | `WriteJSON`, `ReadGraphJSON`, `ReadDiffJSON`. File I/O for graphs and diffs. |

### Types

None (persist carries no domain types; wraps graph types).

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `WriteJSON` | `func WriteJSON(path string, v any) error` | Marshal value to JSON and write to file with 0644 permissions. |
| `ReadGraphJSON` | `func ReadGraphJSON(path string) (graph.MeshGraph, error)` | Read and unmarshal JSON file as `MeshGraph`. |
| `ReadDiffJSON` | `func ReadDiffJSON(path string) (graph.GraphDiff, error)` | Read and unmarshal JSON file as `GraphDiff`. |

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

## Package: cmd/meshant

### Files

| File | Contains |
|------|----------|
| `main.go` | CLI entry point: subcommand dispatcher, helper types and functions. |
| `main_test.go` | 53 tests, 92.5% coverage: all subcommands, flag parsing, file output, error handling. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `stringSliceFlag` | `[]string` (implements flag.Value) | Custom flag collector for repeatable flags (e.g. `--observer a --observer b`). String() and Set() methods; rejects empty values. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `stringSliceFlag.String` | `(f *stringSliceFlag) String() string` | Return comma-joined slice representation (flag.Value interface). |
| `stringSliceFlag.Set` | `(f *stringSliceFlag) Set(v string) error` | Append value to slice; reject empty or whitespace-only values. |
| `parseTimeFlag` | `func parseTimeFlag(name, value string) (time.Time, error)` | Parse RFC3339 string to time.Time with contextual error message naming the flag. |
| `parseTimeWindow` | `func parseTimeWindow(fromName, fromStr, toName, toStr string) (graph.TimeWindow, error)` | Parse two RFC3339 strings (one or both may be empty) into a TimeWindow. Validates only when both bounds are set. |
| `main` | `func main()` | Entry point. Calls `run(os.Stdout, os.Args[1:])` and exits non-zero on error. |
| `run` | `func run(w io.Writer, args []string) error` | Command dispatcher. Parses args to identify subcommand and flags; routes to `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, or `cmdDiff()`. |
| `cmdSummarize` | `func cmdSummarize(w io.Writer, args []string) error` | Subcommand: Load traces, compute mesh summary, print via `loader.PrintSummary()`. Usage: `meshant summarize <file>`. |
| `cmdValidate` | `func cmdValidate(w io.Writer, args []string) error` | Subcommand: Load and validate traces. Reports success message or errors. Usage: `meshant validate <file>`. |
| `cmdArticulate` | `func cmdArticulate(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut with `--observer` (repeatable), `--tag` (repeatable, any-match), `--from`, `--to` (RFC3339), `--format text\|json\|dot\|mermaid`, `--output <file>`. |
| `cmdDiff` | `func cmdDiff(w io.Writer, args []string) error` | Subcommand: Load traces, articulate two cuts (`--observer-a/b`, `--tag-a/b`, per-side time windows), compute diff via `graph.Diff()`. `--format text\|json\|dot\|mermaid`, `--output <file>`. |
| `outputWriter` | `func outputWriter(w io.Writer, outputPath string) (io.Writer, error)` | Return file writer if `--output` is set, otherwise stdout. |
| `confirmOutput` | `func confirmOutput(w io.Writer, outputPath string) error` | Print "wrote <path>" confirmation to stdout when file output is used. |
| `usage` | `func usage()` | Print CLI usage message listing all subcommands and flags. |

### Key Design Notes

- **Stdlib only**: No external dependencies; uses only Go standard library (`flag`, `time`, `io`, `fmt`, `errors`, etc.)
- **Testable structure**: Core logic in `run()`, `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, `cmdDiff()`; `main()` is thin wrapper that wires os.Stdout/os.Args and exits non-zero on error
- **Flag parsing**: Uses stdlib `flag.FlagSet` for subcommand isolation; `stringSliceFlag` enables repeatable `--observer` flags without comma-separation
- **Time handling**: RFC3339 timestamps throughout; `parseTimeFlag()` and `parseTimeWindow()` provide clear error messages with formatting hints
- **Format options**: `articulate` and `diff` both support text/json/dot/mermaid
- **File output**: `--output <file>` writes to file instead of stdout (with deferred close for safety)
- **Binary installation**: `go install ./cmd/meshant` produces `meshant` binary at $GOPATH/bin; used in Dockerfile at `/usr/local/bin/meshant`

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
  ├─→ (modules within graph: graph.go, diff.go, actor.go, serial.go, reflexive.go, export.go)
  └─→ (used by) cmd/demo, persist

persist/
  ├─→ imports: graph
  └─→ (used by) external tools/pipelines

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
| Export graph to JSON | `graph/export.go` → `PrintGraphJSON()` |
| Export diff to JSON | `graph/export.go` → `PrintDiffJSON()` |
| Export graph to Graphviz DOT | `graph/export.go` → `PrintGraphDOT()` |
| Export graph to Mermaid | `graph/export.go` → `PrintGraphMermaid()` |
| Export diff to Graphviz DOT | `graph/export.go` → `PrintDiffDOT()` |
| Export diff to Mermaid | `graph/export.go` → `PrintDiffMermaid()` |
| Write graph to file | `persist/persist.go` → `WriteJSON()` |
| Read graph from JSON file | `persist/persist.go` → `ReadGraphJSON()` |
| Read diff from JSON file | `persist/persist.go` → `ReadDiffJSON()` |
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

### Structured Export (M8)
- `PrintGraphJSON()` serializes `MeshGraph` to JSON; `PrintDiffJSON()` for `GraphDiff`
- `PrintGraphDOT()` generates Graphviz format with quoted labels and edge cardinality
- `PrintGraphMermaid()` produces Mermaid flowchart syntax with sanitized IDs and truncated labels
- All export functions accept `io.Writer` for flexibility (file, buffer, stdout)

## Example Datasets

| Dataset | Location | Size | Observers | Actants | Notes |
|---------|----------|------|-----------|---------|-------|
| Deforestation (M2) | `data/examples/deforestation.json` | 20 traces | 8 | — | 3 threads, 2026-03-11, development reference |
| Deforestation Longitudinal (M3) | `data/examples/deforestation_longitudinal.json` | 40 traces | 8 | — | 3 days (03-11/14/18), time-window testing |
| Evacuation Order (M6) | `data/examples/evacuation_order.json` | 28 traces | 6 | 5 | 3 days (04-14/15/16), 1 graph-ref trace, demo dataset |
| Graph Ref (M5) | `data/examples/graph_ref_traces.json` | — | — | — | Graph-reference examples for M5 actor testing |
| Incident Response (M8) | `data/examples/incident_response.json` | 22 traces | 5 | 8 | 2 days (05-10/11), postmortem scenario, export testing |

**Dataset M8 (Incident Response):**
- **Observers:** monitoring-service, on-call-engineer, incident-commander, product-manager, customer-support
- **Actants:** alerting-pipeline, auto-scaler, circuit-breaker, sla-timer, runbook-engine, dashboard-service, connection-pool-monitor, pagerduty-webhook
- **Trace Stats:** 22 traces, 86% mediated, all 6 tag types represented, 1 graph-ref, 1 absent-source
- **Use Case:** Incident lifecycle (detection to postmortem); demonstrates observer positioning across operational roles

## Related Decision Records and Guides

- `docs/decisions/trace-schema-v1.md` — core Trace type rationale
- `docs/decisions/articulation-v1.md` — observer position and shadow design
- `docs/decisions/time-window-v1.md` — temporal filtering
- `docs/decisions/graph-diff-v1.md` — diff computation and shadow shifts
- `docs/decisions/graph-as-actor-v1.md` — identified graphs as actants
- `docs/decisions/m7-serialisation-reflexivity-v1.md` — TimeWindow JSON codec and reflexive tracing
- `docs/decisions/structured-export-v1.md` — graph export to JSON, DOT, Mermaid formats
- `docs/decisions/cli-v1.md` — CLI design decisions (M9)
- `docs/decisions/m10-tag-filter-diff-export-cli-v1.md` — Tag-filter axis, diff visual export, CLI integration (M10)
- `docs/authoring-traces.md` — Trace authoring guide with worked example (M9)
- `docs/reviews/review_philosophical_m9.md` — Philosophical review, M9 violations and fixes

## Test Coverage

- `schema/trace_test.go` — 27 tests, 100%
- `schema/graphref_test.go` — 14 tests, 100%
- `loader/loader_test.go` — 56 tests, 100%
- `loader/evacuation_test.go` — 27 tests (M6 dataset), all green
- `loader/incident_test.go` — tests for M8 incident response dataset
- `graph/graph_test.go` — 84 tests (including M3 time-window tests), 99.3%
- `graph/diff_test.go` — 41 tests, 100%
- `graph/actor_test.go` — 15 tests, 100%
- `graph/serial_test.go` — 19 tests, 100%
- `graph/reflexive_test.go` — 19 tests, 100%
- `graph/export_test.go` — tests for JSON, DOT, Mermaid export functions
- `graph/incident_e2e_test.go` — E2E tests using incident response dataset
- `persist/persist_test.go` — tests for file I/O functions
- `cmd/demo/main_test.go` — E2E test
- `cmd/meshant/main_test.go` — 37 tests, 92.9% coverage (M9 CLI subcommands, flag parsing, error handling)
