# MeshAnt — Codemap

**Last Updated:** 2026-03-17 (M12 Re-articulation Pass)
**Module:** `github.com/automatedtomato/mesh-ant/meshant`
**Go Version:** 1.25
**Root Directory:** `/meshant`

## Package Overview

| Package | Purpose |
|---------|---------|
| `schema` | Core trace types, graph-reference predicates, and validators. |
| `loader` | Load traces from JSON, summarize datasets, print summaries. |
| `graph` | Articulate graphs, compute diffs, identify graphs as actors, reflexive tracing, follow translation chains, classify chains, export to JSON/DOT/Mermaid. |
| `persist` | Read and write graphs to JSON files. |
| `cmd/demo` | Minimal demonstration: two observer-position cuts on evacuation dataset. |
| `cmd/meshant` | CLI entry point: `summarize`, `validate`, `articulate`, `diff`, `follow`, `draft`, `promote`, `rearticulate`, `lineage` subcommands. |

## Package: schema

### Files

| File | Contains |
|------|----------|
| `trace.go` | `Trace` struct; `TagValue` constants; `Validate()` method. |
| `tracedraft.go` | `TraceDraft` struct; `TagValueDraft` constant; `Validate()`, `IsPromotable()`, `Promote()` methods (M11). |
| `graphref.go` | Graph-reference string predicates (`IsGraphRef`, `GraphRefKind`, `GraphRefID`). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `Trace` | `ID` (uuid), `Timestamp` (time), `WhatChanged` (string), `Source` ([]string), `Target` ([]string), `Mediation` (string), `Tags` ([]string), `Observer` (string, required) | Fundamental unit of record: a moment where something made a difference in a network. |
| `TagValue` | (string constant type) | Vocabulary for trace descriptors: `TagDelay`, `TagThreshold`, `TagBlockage`, `TagAmplification`, `TagRedirection`, `TagTranslation`, `TagValueArticulation`, `TagValueDraft` (M11). |
| `TraceDraft` | `ID` (uuid, optional), `Timestamp` (time), `SourceSpan` (string, required), `SourceDocRef` (string), `WhatChanged` (string), `Source` ([]string), `Target` ([]string), `Mediation` (string), `Observer` (string), `Tags` ([]string), `UncertaintyNote` (string), `ExtractionStage` (string), `ExtractedBy` (string), `DerivedFrom` (string) | Provisional, provenance-bearing record from ingestion pipeline. Minimal requirement: `SourceSpan`. May be promoted to canonical `Trace` when sufficiently complete (M11). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Trace.Validate()` | `(t Trace) Validate() error` | Validate required fields (ID, Timestamp, WhatChanged, Observer). Returns all violations joined. |
| `TraceDraft.Validate()` | `(d TraceDraft) Validate() error` | Validate required field (SourceSpan). Returns error if absent. |
| `TraceDraft.IsPromotable()` | `(d TraceDraft) IsPromotable() bool` | Check if draft has sufficient fields (valid UUID ID, non-empty WhatChanged, non-empty Observer) to promote to canonical Trace. |
| `TraceDraft.Promote()` | `(d TraceDraft) Promote() (Trace, error)` | Convert draft to canonical Trace; appends `TagValueDraft` as provenance signal. Errors if not promotable; names all violations. |
| `IsGraphRef` | `func IsGraphRef(s string) bool` | Check if string is a graph-reference (prefix "meshgraph:" or "meshdiff:"). |
| `GraphRefKind` | `func GraphRefKind(s string) string` | Return kind prefix ("meshgraph", "meshdiff", or empty). |
| `GraphRefID` | `func GraphRefID(s string) string` | Extract UUID portion after prefix. |

## Package: loader

### Files

| File | Contains |
|------|----------|
| `loader.go` | `Load`, `Summarise`, `PrintSummary`; `MeshSummary`, `FlaggedTrace` types. |
| `draftloader.go` | `LoadDrafts`, `SummariseDrafts`, `PrintDraftSummary`; `DraftSummary` type; UUID generation (M11). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `MeshSummary` | `Elements` (map[string]int), `Mediations` ([]string), `MediatedTraceCount` (int), `FlaggedTraces` ([]FlaggedTrace), `GraphRefs` ([]string) | Provisional first-pass reading of a trace dataset. |
| `FlaggedTrace` | `ID` (string), `WhatChanged` (string), `Tags` ([]string) | Minimal projection of traces tagged delay or threshold. |
| `DraftSummary` | `Total` (int), `Promotable` (int), `ByStage` (map[string]int), `ByExtractedBy` (map[string]int), `FieldFillRate` (map[string]int) | Provenance-aware reading of a TraceDraft dataset. Reveals ingestion pipeline breakdown and field fill rates (M11). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Load` | `func Load(path string) ([]schema.Trace, error)` | Load JSON file, decode traces, validate via schema.Validate(); max 50 MB. |
| `Summarise` | `func Summarise(traces []schema.Trace) MeshSummary` | Build MeshSummary from validated traces: count elements, deduplicate mediations, flag delay/threshold, extract graph-refs. |
| `PrintSummary` | `func PrintSummary(w io.Writer, s MeshSummary) error` | Write formatted summary to io.Writer. Elements sorted by descending frequency, mediations in encounter order. |
| `LoadDrafts` | `func LoadDrafts(path string) ([]schema.TraceDraft, error)` | Load JSON file of TraceDraft records; assign UUIDs and timestamps to missing fields; validate each via `TraceDraft.Validate()`; max 50 MB (M11). |
| `SummariseDrafts` | `func SummariseDrafts(drafts []schema.TraceDraft) DraftSummary` | Build DraftSummary from TraceDraft slice: count by stage/extracted-by, count promotable records, compute per-field fill rates (M11). |
| `PrintDraftSummary` | `func PrintDraftSummary(w io.Writer, s DraftSummary) error` | Write provenance summary to io.Writer. Shows total/promotable, breakdown by extraction stage and extracted_by, per-field fill rates (M11). |

## Package: graph

### Files

| File | Contains |
|------|----------|
| `graph.go` | `MeshGraph`, `Node`, `Edge`, `Cut`, `ShadowElement`, `ShadowReason`, `TimeWindow`, `ArticulationOptions`. `Articulate`, `PrintArticulation` functions. Filter logic, edge/node/shadow builders. |
| `diff.go` | `GraphDiff`, `ShadowShift`, `PersistedNode`, `ShadowShiftKind`. `Diff`, `PrintDiff` functions. Diff computation helpers. |
| `actor.go` | Graph-as-actor identity: `IdentifyGraph`, `IdentifyDiff`, `GraphRef`, `DiffRef`, `newUUID4`. |
| `serial.go` | Custom JSON codec for `TimeWindow`: `MarshalJSON`, `UnmarshalJSON`. Null encoding for unbounded bounds. |
| `reflexive.go` | Reflexive tracing: `ArticulationTrace`, `DiffTrace`. Functions that record articulation and diffing as traces. |
| `chain.go` | Translation chain traversal: `TranslationChain`, `ChainStep`, `ChainBreak`, `Direction`, `FollowOptions`. `FollowTranslation()` function and unexported helpers. |
| `criterion.go` | Equivalence criterion: `EquivalenceCriterion` type. `IsZero()`, `Validate()` methods. Interpretive declaration for classification readings. |
| `classify.go` | Chain classification: `ClassifiedChain`, `StepClassification`, `StepKind`, `ClassifyOptions`. `ClassifyChain()` function. Heuristic classification (intermediary, mediator, translation). Carries criterion as envelope metadata. |
| `chain_print.go` | Chain output formatting: `PrintChain`, `PrintChainJSON`. Text and JSON rendering of classified chains, including criterion block when present. |
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
| `Direction` | (string constant: `DirectionForward`, `DirectionBackward`) | Direction of chain traversal: follow edges as targets (forward) or sources (backward). |
| `ChainStep` | `Step` (int, 0-indexed), `Edge` (Edge), `TargetNode` (string) | One edge in a translation chain; the element we arrived at. |
| `ChainBreak` | `Step` (ChainStep), `Kind` (ChainBreakKind) | Alternative edge not followed, cycle detected, or depth exceeded. |
| `ChainBreakKind` | (string constant: `BranchNotTaken`, `DepthExceeded`, `CycleDetected`) | Reason why the chain stopped at this point. |
| `TranslationChain` | `Element` (string), `Direction` (Direction), `Steps` ([]ChainStep), `Breaks` ([]ChainBreak), `Observer` (string), `GraphID` (string) | Path through a graph from starting element to terminal node, with branches and breaks. |
| `FollowOptions` | `Direction` (Direction), `DepthLimit` (int, 0=unlimited) | Parameters for translation chain traversal. |
| `EquivalenceCriterion` | `Name` (string), `Declaration` (string), `Preserve` ([]string), `Ignore` ([]string) | Interpretive declaration for a chain reading. Carries the conditions under which a chain is classified. Governs future comparison functions (Layer 3, deferred). `Ignore` is a second-order shadow of aspects, not elements. |
| `StepKind` | (string constant: `StepIntermediary`, `StepMediator`, `StepTranslation`) | Classification of a chain step based on mediation presence and tags. |
| `StepClassification` | `StepIndex` (int), `Kind` (StepKind), `Reason` (string) | Classification and justification for a single chain step. Reason strings are purely edge-driven (v1 heuristics). |
| `ClassifiedChain` | `Chain` (TranslationChain), `Classifications` ([]StepClassification), `Criterion` (EquivalenceCriterion) | Translation chain with step-by-step classifications and optional criterion metadata. Criterion is envelope-only — does not alter v1 heuristics. |
| `ClassifyOptions` | `Criterion` (EquivalenceCriterion) | Parameters for chain classification. Zero value = v1 heuristics (backwards-compatible). Criterion is carried into ClassifiedChain as provenance; does not alter step logic yet. |

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
| `FollowTranslation` | `func FollowTranslation(g MeshGraph, element string, opts FollowOptions) TranslationChain` | Traverse graph from starting element via first-match branching; record alternatives and cycles as breaks. |
| `ClassifyChain` | `func ClassifyChain(chain TranslationChain, opts ClassifyOptions) ClassifiedChain` | Apply heuristic classification (intermediary, mediator, translation) to each step in a chain. Carries criterion as envelope metadata if provided; does not alter v1 step heuristics. |
| `EquivalenceCriterion.IsZero` | `(c EquivalenceCriterion) IsZero() bool` | True when all fields are empty (nil and empty slice treated equally). Zero = v1 implicit criterion in effect. |
| `EquivalenceCriterion.Validate` | `(c EquivalenceCriterion) Validate() error` | Error if Preserve or Ignore non-empty but Declaration empty (layer ordering: Layer 2 requires Layer 1 grounds). |
| `PrintChain` | `func PrintChain(w io.Writer, cc ClassifiedChain) error` | Write human-readable classified chain to io.Writer. Includes steps with classifications, breaks with reasons, and criterion block when non-zero. |
| `PrintChainJSON` | `func PrintChainJSON(w io.Writer, cc ClassifiedChain) error` | Export `ClassifiedChain` as JSON to io.Writer. Criterion key omitted entirely when zero (pointer + omitempty). |

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
| `main.go` | CLI entry point: subcommand dispatcher, helper types and functions. Includes `cmdDraft`, `cmdPromote` (M11), `cmdRearticulate`, `cmdLineage` (M12) handlers. |
| `main_test.go` | Tests: all subcommands, flag parsing, file output, error handling, criterion file loading, draft/promote pipeline (M11). |

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
| `run` | `func run(w io.Writer, args []string) error` | Command dispatcher. Parses args to identify subcommand and flags; routes to `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, `cmdDiff()`, `cmdFollow()`, `cmdDraft()`, `cmdPromote()`, `cmdRearticulate()`, or `cmdLineage()`. |
| `cmdSummarize` | `func cmdSummarize(w io.Writer, args []string) error` | Subcommand: Load traces, compute mesh summary, print via `loader.PrintSummary()`. Usage: `meshant summarize <file>`. |
| `cmdValidate` | `func cmdValidate(w io.Writer, args []string) error` | Subcommand: Load and validate traces. Reports success message or errors. Usage: `meshant validate <file>`. |
| `cmdArticulate` | `func cmdArticulate(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut with `--observer` (repeatable), `--tag` (repeatable, any-match), `--from`, `--to` (RFC3339), `--format text\|json\|dot\|mermaid`, `--output <file>`. |
| `cmdDiff` | `func cmdDiff(w io.Writer, args []string) error` | Subcommand: Load traces, articulate two cuts (`--observer-a/b`, `--tag-a/b`, per-side time windows), compute diff via `graph.Diff()`. `--format text\|json\|dot\|mermaid`, `--output <file>`. |
| `cmdFollow` | `func cmdFollow(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut, follow translation chain from --element with `--direction forward\|backward`, `--depth N`, `--observer`, `--tag`, `--from`, `--to`. Optional `--criterion-file <path>` loads an EquivalenceCriterion before trace I/O. Classify and print via `PrintChain()`. `--format text\|json`, `--output <file>`. |
| `cmdDraft` | `func cmdDraft(w io.Writer, args []string) error` | Subcommand: Load extraction JSON, assign UUIDs/timestamps, apply optional `--source-doc`, `--extracted-by`, `--stage` overrides, write TraceDraft JSON via `loader.LoadDrafts()`, print provenance summary via `PrintDraftSummary()`. `--output <file>` (M11). |
| `cmdPromote` | `func cmdPromote(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON via `loader.LoadDrafts()`, call `IsPromotable()` on each, promote qualifying drafts to canonical Traces (each tagged with `TagValueDraft`), write promoted Trace JSON, report promotion summary naming failures (M11). `--output <file>`. |
| `cmdRearticulate` | `func cmdRearticulate(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON, produce skeleton JSON array — for each draft: `source_span` copied verbatim, `derived_from` set to original ID, all content fields blank, `extraction_stage:"reviewed"`. Flags: `--id <id>` (single draft only), `--output <file>` (M12). |
| `cmdLineage` | `func cmdLineage(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON, walk DerivedFrom links, render positional reading sequences as indented trees. Anchors (drafts with no DerivedFrom) are sequence roots. Cycle detection required (DFS grey-set). Flags: `--id <id>` (single chain), `--format text\|json` (M12). |
| `loadCriterionFile` | `func loadCriterionFile(path string) (graph.EquivalenceCriterion, error)` | Load, decode, and validate an EquivalenceCriterion from a JSON file. Uses `DisallowUnknownFields()` for precision. Zero-value criterion is a hard error. Returns validated criterion or descriptive error. |
| `outputWriter` | `func outputWriter(w io.Writer, outputPath string) (io.Writer, error)` | Return file writer if `--output` is set, otherwise stdout. |
| `confirmOutput` | `func confirmOutput(w io.Writer, outputPath string) error` | Print "wrote <path>" confirmation to stdout when file output is used. |
| `usage` | `func usage()` | Print CLI usage message listing all subcommands and flags. |

### Key Design Notes

- **Stdlib only**: No external dependencies; uses only Go standard library (`flag`, `time`, `io`, `fmt`, `errors`, etc.)
- **Testable structure**: Core logic in `run()`, `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, `cmdDiff()`; `main()` is thin wrapper that wires os.Stdout/os.Args and exits non-zero on error
- **Flag parsing**: Uses stdlib `flag.FlagSet` for subcommand isolation; `stringSliceFlag` enables repeatable `--observer` flags without comma-separation
- **Time handling**: RFC3339 timestamps throughout; `parseTimeFlag()` and `parseTimeWindow()` provide clear error messages with formatting hints
- **Format options**: `articulate` and `diff` both support text/json/dot/mermaid; `follow` supports text/json
- **File output**: `--output <file>` writes to file instead of stdout (with deferred close for safety)
- **Ingestion pipeline** (M11): `draft` command ingests LLM extraction JSON and produces TraceDraft records; `promote` command converts promotable TraceDraft records to canonical Traces (tagged with `draft` provenance signal)
- **Re-articulation pipeline** (M12): `rearticulate` command produces critique skeletons (SourceSpan + DerivedFrom set, all content fields blank); `lineage` command walks DerivedFrom links and renders positional reading sequences as CLI output
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
| Follow translation chain | `graph/chain.go` → `FollowTranslation()` |
| Classify chain steps | `graph/classify.go` → `ClassifyChain()` |
| Declare equivalence criterion | `graph/criterion.go` → `EquivalenceCriterion` |
| Load criterion from file | `cmd/meshant/main.go` → `loadCriterionFile()` |
| Print chain output | `graph/chain_print.go` → `PrintChain()` |
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
| Load TraceDraft records | `loader/draftloader.go` → `LoadDrafts()` |
| Summarise draft dataset | `loader/draftloader.go` → `SummariseDrafts()` |
| Check if draft is promotable | `schema/tracedraft.go` → `TraceDraft.IsPromotable()` |
| Promote draft to canonical Trace | `schema/tracedraft.go` → `TraceDraft.Promote()` |
| Produce critique skeleton from draft | `cmd/meshant/main.go` → `cmdRearticulate()` |
| Walk DerivedFrom lineage chain | `cmd/meshant/main.go` → `cmdLineage()` |
| Read critique prompt contract | `data/prompts/critique_pass.md` |
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

### Translation Chains (M10.5)
- `FollowTranslation()` reads *through* a graph via first-match branching (Layer 4 operation)
- When multiple edges leave a node, first is followed (by dataset order); others recorded as `branch-not-taken` breaks
- Breaks record alternatives (branch-not-taken), cycles (cycle-detected), and depth limits (depth-exceeded)
- Cycle detection includes the closing step in `Steps` before adding a break (asymmetry documented)
- Classification is observer-position dependent; same chain under different cuts may differ
- `ClassifyChain()` applies heuristic classification (intermediary/mediator/translation) based on `Mediation` field presence and tags
- Classification outsources judgment to trace author: if author wrote mediation, we acknowledge it (v1 implicit criterion)
- `ClassifyOptions.Criterion` carries an `EquivalenceCriterion`; zero value = v1 heuristics (backwards-compatible)
- `PrintChain()` always shows breaks (named absence); consistent with shadow philosophy

### Equivalence Criterion (M10.5+)
- `EquivalenceCriterion` is an interpretive declaration, not a computational rule
- Three-layer design: Declaration (Layer 1, grounds) → Preserve/Ignore (Layer 2, aspects) → comparison function (Layer 3, deferred)
- Layer ordering enforced at `Validate()`: Preserve/Ignore require Declaration
- Criterion is carried as `ClassifiedChain` envelope metadata; does not alter v1 step heuristics
- `Ignore` is a second-order shadow: aspects declared irrelevant under this criterion, not absent
- `--criterion-file` loads criterion from JSON file before trace I/O; criterion governs function, not reverse
- `DisallowUnknownFields()` enforced for criterion files: precision over forward-compatibility tolerance
- Zero criterion → v1 behaviour; all existing code paths unaffected

### Re-articulation Pipeline (M12)
- **Re-articulation is a cut, not a correction**: a critique draft is a parallel reading of the same SourceSpan, not a verdict on the original; both have equal standing in the DerivedFrom chain
- **SourceSpan as invariant**: `cmdRearticulate` copies `source_span` and `source_doc_ref` verbatim; all interpretation fields are blank (honest abstentions, not missing data)
- **Blank scaffold output is correct**: `cmdRearticulate` intentionally does not call `Validate()` on output (skeleton has no ID/Timestamp yet); critiquing agent fills content fields, then submits via `meshant draft`
- **DerivedFrom is positional vocabulary**: `subsequent`/`anchors` naming (not `children`/`roots`) avoids genealogical framing; chain order names production sequence, not hierarchy
- **Cycle detection via DFS grey-set**: `cmdLineage` detects circular DerivedFrom references (A→B→A) and returns a named error rather than looping
- **cmdLineage is a chain reader, not a diff tool**: renders structure (which reading followed which, at what stage, by whom); comparing readings in a chain is the analyst's job
- **Critique prompt template as methodological constraint**: `data/prompts/critique_pass.md` is the extraction contract that makes re-articulation ANT-faithful; the CLI enforces structural constraints, the template enforces interpretive constraints

### Ingestion Pipeline (M11)
- **TraceDraft** is a first-class analytical object, not a halfway house to Trace
- **SourceSpan** is the only required field; minimal record carrying verbatim source text without forcing resolution
- **Provenance chain**: `DerivedFrom` links draft revisions into a followable extraction history (span → LLM → critique → human revision → promoted)
- **Promotion criterion** (not equivalence): A draft is promotable when `IsPromotable() == true` (valid UUID ID, non-empty WhatChanged, non-empty Observer)
- **Three-stage naming** for extraction provenance: `ExtractionStage` ("span-harvest", "weak-draft", "reviewed") and `ExtractedBy` (e.g., "human", "llm-pass1", "reviewer")
- **Field fill rates**: `DraftSummary` measures honest abstentions (empty fields) vs. populated assignments; reveals what ingestion pipeline is confident in
- **Promotion signal**: Promoted Traces carry `TagValueDraft` ("draft") tag; makes provenance visible in downstream analysis
- **UncertaintyNote** is a first-class field, not an exception: records where source span does not support confident assignment (anti-fabrication principle)

## Example Datasets

### Trace Datasets

| Dataset | Location | Size | Observers | Actants | Notes |
|---------|----------|------|-----------|---------|-------|
| Deforestation (M2) | `data/examples/deforestation.json` | 20 traces | 8 | — | 3 threads, 2026-03-11, development reference |
| Deforestation Longitudinal (M3) | `data/examples/deforestation_longitudinal.json` | 40 traces | 8 | — | 3 days (03-11/14/18), time-window testing |
| Evacuation Order (M6) | `data/examples/evacuation_order.json` | 28 traces | 6 | 5 | 3 days (04-14/15/16), 1 graph-ref trace, demo dataset |
| Graph Ref (M5) | `data/examples/graph_ref_traces.json` | — | — | — | Graph-reference examples for M5 actor testing |
| Incident Response (M8) | `data/examples/incident_response.json` | 22 traces | 5 | 8 | 2 days (05-10/11), postmortem scenario, export testing |

### Ingestion Pipeline Datasets (M11)

| Dataset | Location | Stage | Purpose |
|---------|----------|-------|---------|
| CVE Response (Raw) | `data/examples/cve_response_raw.md` | Input | Verbatim source document (incident response narrative) |
| CVE Response (Extraction) | `data/examples/cve_response_extraction.json` | Intermediate | LLM-produced extraction JSON (source_span required, other fields optional) |
| CVE Response (Drafts) | `data/examples/cve_response_drafts.json` | Output | TraceDraft records after `meshant draft` processing (UUIDs assigned, validation applied) |

### Re-articulation Datasets (M12)

| Dataset | Location | Stage | Purpose |
|---------|----------|-------|---------|
| CVE Critique Skeleton | `data/examples/cve_critique_skeleton.json` | Scaffold | Skeleton output from `meshant rearticulate`: SourceSpan + DerivedFrom set, content fields blank, one record per CVE draft |
| CVE Critique Drafts | `data/examples/cve_critique_drafts.json` | Reviewed | Filled critique drafts for E3 (resists "attacker" as stable actor) and E14 (reframes CVE as document, not agent); methodological demonstration material |

### Extraction Prompt Templates

| File | Purpose |
|------|---------|
| `data/prompts/critique_pass.md` | Extraction contract for the critique step: what to preserve (SourceSpan verbatim), what to question (stable actor attributions, imputed intentions), what honest abstention looks like, DerivedFrom semantics, worked E3 example |

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
- `docs/decisions/translation-chain-v1.md` — Translation chain traversal, classification heuristics, first-match branching (M10.5)
- `docs/decisions/equivalence-criterion-v1.md` — Equivalence criterion design, three-layer model, v1 implicit criterion, second-order shadow (M10.5+)
- `docs/decisions/tracedraft-v1.md` — TraceDraft design, ingestion pipeline as analytical object, source span as ground truth, promotion criterion, provenance chain (M11)
- `docs/decisions/rearticulation-v1.md` — Re-articulation as cut not correction, SourceSpan invariant, blank scaffold as correct output, DerivedFrom positional vocabulary, cmdLineage as first-class CLI output, E3/E14 as demonstration material (M12)
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
- `graph/chain_test.go` — unit tests for translation chain traversal (first-match, cycle detection, direction reversal, depth limit)
- `graph/criterion_test.go` — 18 tests: zero detection, Validate layer ordering, structural stability
- `graph/classify_test.go` — unit tests for chain classification heuristics; criterion carried through, step reasons unchanged, two criteria = same result
- `graph/chain_print_test.go` — tests for chain text and JSON output formatting; criterion block, name-only handle signal
- `graph/chain_e2e_test.go` — E2E tests using deforestation, evacuation_order, and incident_response datasets
- `graph/incident_e2e_test.go` — E2E tests using incident response dataset
- `persist/persist_test.go` — tests for file I/O functions
- `cmd/demo/main_test.go` — E2E test
- `cmd/meshant/main_test.go` — tests covering all CLI subcommands including follow, draft, promote (M11), rearticulate, lineage (M12), flag parsing, file output, error handling; 659 total tests, 88.2% cmd/meshant coverage
