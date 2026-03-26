# MeshAnt — Codemap

**Last Updated:** 2026-03-26 (#176: MCP server for LLM integration — JSON-RPC 2.0 over stdio, observer-position discipline, reflexive trace recording, meshant_articulate tool)
**Module:** `github.com/automatedtomato/mesh-ant/meshant`
**Go Version:** 1.25
**Root Directory:** `/meshant`

## Package Overview

| Package | Purpose |
|---------|---------|
| `adapter` | Format-conversion adapters: PDF, HTML, and JSONL → plain text. `Adapter` interface, `ConvertResult` struct, `ForName()` registry. Used by `cmd/meshant` for `meshant convert` and `meshant extract --adapter`. |
| `schema` | Core trace types, graph-reference predicates, and validators. |
| `store` | Storage interface and JSON-file backend. `TraceStore` interface (Store, Query, Get, Close), `QueryOpts`, `JSONFileStore`. Narrow abstraction between the CLI/analytical engine and trace backends; the Neo4j adapter (#143) implements the same interface. |
| `loader` | Load traces from JSON, summarize datasets, print summaries. |
| `graph` | Articulate graphs, compute diffs, identify graphs as actors, reflexive tracing, follow translation chains, classify chains, shadow analysis, observer-gap analysis, bottleneck analysis, re-articulation suggestions, narrative drafts, export to JSON/DOT/Mermaid. |
| `persist` | Read and write graphs to JSON files. |
| `review` | Ambiguity detection, terminal rendering, and interactive accept/edit/skip/quit session for TraceDraft records (Thread A). Exports `DeriveAccepted`, `DeriveEdited`, `RunEditFlow` for reuse by `llm` (F.3). |
| `llm` | LLM-mediated extraction, assist, critique, and split pipelines: `LLMClient` interface, `AnthropicClient`, `RunExtraction`, `RunAssistSession`, `ParseSpans`, `RunCritique`, `RunSplit`, `PromoteSession`, `SessionRecord`, and supporting types. Shared helpers (`readSourceDoc`, `isRefusal`) in `shared.go`. Enforces F.1 conventions (D1–D7): mediator framing, model-ID provenance, framework UncertaintyNote append, IntentionallyBlank validation (F.2, F.3, F.4, #137). `PromoteSession` closes the Principle 8 reflexivity gap: converts a SessionRecord to a canonical Trace (#138). Imports `review` (one-directional: `llm → review`) for derivation helpers and rendering in the assist session. |
| `serve` | HTTP server for interactive trace graph querying. `Server` struct, `NewServer(ts store.TraceStore)` constructor. Five endpoints: `/articulate` (GET, returns Cut as Envelope), `/diff` (GET, diffs two cuts), `/shadow` (GET, names shadowed elements), `/traces` (GET, returns raw traces), `/element/{name}` (GET, traces mentioning named element — observer required). All endpoints require `observer` query param (returns 400 with ANT reasoning if absent). `Envelope` and `CutMeta` types (moved to graph package in #174 for reuse by non-HTTP layers) wrap response data + positioned cut. `filterByElement` filters traces by Source/Target membership. Response helpers: `writeJSON`, `writeError`, query parameter parsers. Static file server via `go:embed web` serves the SPA at `/`. Graceful shutdown. 93.4% coverage. #145, #146, Phase 4. Web assets: `web/index.html` (SPA shell: observer gate, cut header, graph + sidebar), `web/style.css`, `web/app.js` (init, API fetch, wiring), `web/render.js` (Cytoscape graph, shadow panel, detail panel), `web/export.js` (JSON + DOT download), `web/lib/cytoscape.min.js` (vendored Cytoscape.js 3.30.4). |
| `mcp` | Model Context Protocol server for LLM integration. `Server` struct backed by `store.TraceStore`. JSON-RPC 2.0 over stdio; single-threaded dispatch with 4 MiB buffer for large graphs. Tool registration via `ToolHandler` interface; batch 1 (#176) registers `meshant_articulate`. Observer discipline on all tools (ANT position required per D1 mcp-v1.md). Analyst identity injected at startup and stamped on all CutMeta responses. Reflexivity (Principle 8): every cut-producing tool writes invocation traces tagged "mcp-invocation" back to the store. Decision record: `docs/decisions/mcp-v1.md`. |
| `cmd/demo` | Minimal demonstration: two observer-position cuts on evacuation dataset. |
| `cmd/meshant` | CLI entry point: `summarize`, `validate`, `articulate`, `diff`, `follow`, `draft`, `promote`, `rearticulate`, `lineage`, `shadow`, `gaps`, `bottleneck`, `review`, `extract`, `assist`, `critique`, `split`, `promote-session`, `convert`, `store`, `serve`, `mcp` subcommands. `articulate`, `diff`, `shadow`, `gaps`, `follow`, `bottleneck` accept `--db bolt://...` to load traces from a Neo4j DB instead of a JSON file (mutually exclusive with the file arg). `store` writes JSON traces to the DB. `serve` starts a localhost HTTP server (#145); accepts `--db` and `--port` flags. `mcp` starts a Model Context Protocol server on stdio (#176); requires `--analyst` flag. `articulate` also supports `--narrative`; `gaps` supports `--suggest`. `review` and `assist` are interactive (read from stdin). `extract` calls an LLM for TraceDraft extraction (F.2, #139); supports `--adapter` (#140). `split` splits a source document into spans (#137). `promote-session` promotes a SessionRecord to a Trace (#138). `convert` converts non-text sources to plain text (#140). |

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
| `TagValue` | (string constant type) | Vocabulary for trace descriptors: `TagDelay`, `TagThreshold`, `TagBlockage`, `TagAmplification`, `TagRedirection`, `TagTranslation`, `TagValueArticulation`, `TagValueDraft` (M11), `TagValueSession` (#138 — marks traces promoted from SessionRecords). |
| `TraceDraft` | `ID` (uuid, optional), `Timestamp` (time), `SourceSpan` (string, required), `SourceDocRef` (string), `WhatChanged` (string), `Source` ([]string), `Target` ([]string), `Mediation` (string), `Observer` (string), `Tags` ([]string), `UncertaintyNote` (string), `ExtractionStage` (string), `ExtractedBy` (string), `DerivedFrom` (string), `CriterionRef` (string, M13), `SessionRef` (string, F.0), `IntentionallyBlank` ([]string, M12.5) | Provisional, provenance-bearing record from ingestion pipeline. Minimal requirement: `SourceSpan`. May be promoted to canonical `Trace` when sufficiently complete (M11). `CriterionRef` names the EquivalenceCriterion governing a critique skeleton (citation, not copy). `SessionRef` names the ingestion session that produced this draft — preserved through the review pipeline, not transferred by `Promote()`. `IntentionallyBlank` names content fields deliberately left empty (honest abstention, not missing data). |

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
| `draftloader.go` | `LoadDrafts`, `SummariseDrafts`, `PrintDraftSummary`; `DraftSummary` type; `NewUUID` (exported, Thread A.1). `WithIntentionallyBlank`, `WithCriterionRef`, and `WithSessionRef` counts added (M12.5, M13, F.0). |
| `draftchain.go` | `FollowDraftChain`, `ClassifyDraftChain`; `DraftStepKind`, `DraftStepClassification`, `ClassifyDraftChainOptions`, `ClassifiedDraftChain` types; `DraftSubKindEndorsement` constant (M13, #96, #95). |
| `analyst.go` | `GroupByAnalyst`; analyst-position partitioning for multi-analyst comparison (C.1). |
| `extractiongap.go` | `CompareExtractions`, `PrintExtractionGap`; `ExtractionGap`, `FieldDisagreement` types (C.2). |
| `classdiff.go` | `CompareChainClassifications`, `PrintClassificationDiffs`; `ClassificationDiff` type (C.3). Classification-diff analysis: compare how two analyst positions classified the same derivation chain. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `MeshSummary` | `Elements` (map[string]int), `Mediations` ([]string), `MediatedTraceCount` (int), `FlaggedTraces` ([]FlaggedTrace), `GraphRefs` ([]string) | Provisional first-pass reading of a trace dataset. |
| `FlaggedTrace` | `ID` (string), `WhatChanged` (string), `Tags` ([]string) | Minimal projection of traces tagged delay or threshold. |
| `DraftSummary` | `Total` (int), `Promotable` (int), `ByStage` (map[string]int), `ByExtractedBy` (map[string]int), `FieldFillRate` (map[string]int), `WithIntentionallyBlank` (int, M12.5), `WithCriterionRef` (int, M13), `WithSessionRef` (int, F.0) | Provenance-aware reading of a TraceDraft dataset. Reveals ingestion pipeline breakdown and field fill rates (M11). Counts critique skeletons, self-situated skeletons, and session-linked drafts. |
| `DraftStepKind` | (string constant: `DraftIntermediary`, `DraftMediator`, `DraftTranslation`) | Classification of a draft chain step; mirrors `StepKind` from graph package. Heuristics are v1 and provisional (M13). |
| `DraftStepClassification` | `StepIndex` (int), `Kind` (DraftStepKind), `SubKind` (string, omitempty), `Reason` (string) | Classification and justification for a single draft chain step. `SubKind` is set to `DraftSubKindEndorsement` ("endorsement") when a `DraftMediator` step advances the extraction stage but does not change content fields — the stage change is an act of endorsement, not transformation (M13, #96). |
| `ClassifyDraftChainOptions` | `Criterion` (graph.EquivalenceCriterion) | Options for `ClassifyDraftChain`. `Criterion` is recorded in the returned envelope as metadata; it is NOT applied to step heuristics (design rule C1: criterion governs equivalence, not chain traversal) (#95). |
| `ClassifiedDraftChain` | `Classifications` ([]DraftStepClassification), `Criterion` (graph.EquivalenceCriterion) | Envelope return type from `ClassifyDraftChain`. Carries the step classifications alongside the criterion that was active when the chain was classified — provenance for downstream audit (#95). |
| `ExtractionGap` | `AnalystA` (string), `AnalystB` (string), `OnlyInA` ([]string), `OnlyInB` ([]string), `InBoth` ([]string), `Disagreements` ([]FieldDisagreement) | Comparison of two named extraction positions: partitions drafts by SourceSpan into three visibility groups; records field-level disagreements across 9 content fields (C.2). |
| `FieldDisagreement` | `SourceSpan` (string), `Field` (string), `ValueA` (string), `ValueB` (string) | Mismatch in a single field for a draft visible from both extraction positions; field name and both values recorded (C.2). |
| `ClassificationDiff` | `StepIndex` (int), `KindA` (DraftStepKind), `KindB` (DraftStepKind), `ReasonA` (string), `ReasonB` (string) | Classification disagreement at a single step position between two analyst positions; neither value is authoritative (C.3). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Load` | `func Load(path string) ([]schema.Trace, error)` | Load JSON file, decode traces, validate via schema.Validate(); max 50 MB. |
| `Summarise` | `func Summarise(traces []schema.Trace) MeshSummary` | Build MeshSummary from validated traces: count elements, deduplicate mediations, flag delay/threshold, extract graph-refs. |
| `PrintSummary` | `func PrintSummary(w io.Writer, s MeshSummary) error` | Write formatted summary to io.Writer. Elements sorted by descending frequency, mediations in encounter order. |
| `LoadDrafts` | `func LoadDrafts(path string) ([]schema.TraceDraft, error)` | Load JSON file of TraceDraft records; assign UUIDs and timestamps to missing fields; validate each via `TraceDraft.Validate()`; max 50 MB (M11). |
| `SummariseDrafts` | `func SummariseDrafts(drafts []schema.TraceDraft) DraftSummary` | Build DraftSummary from TraceDraft slice: count by stage/extracted-by, count promotable records, compute per-field fill rates (M11). |
| `PrintDraftSummary` | `func PrintDraftSummary(w io.Writer, s DraftSummary) error` | Write provenance summary to io.Writer. Shows total/promotable, breakdown by extraction stage and extracted_by, per-field fill rates, critique skeleton counts (M11, M12.5, M13). |
| `FollowDraftChain` | `func FollowDraftChain(drafts []schema.TraceDraft, from string) []schema.TraceDraft` | Traverse DerivedFrom links from draft with id `from`; return chain in derivation order. Empty slice if `from` not found. Cycle detection via visited set (M13). |
| `ClassifyDraftChain` | `func ClassifyDraftChain(chain []schema.TraceDraft, opts ClassifyDraftChainOptions) ClassifiedDraftChain` | Apply v1 heuristic classification to consecutive draft pairs. Returns envelope with `len(chain)-1` classifications and the criterion from opts (C1: criterion is envelope metadata only — does not alter heuristics) (M13, #95). |
| `GroupByAnalyst` | `func GroupByAnalyst(drafts []schema.TraceDraft) map[string][]schema.TraceDraft` | Partition drafts by ExtractedBy field (analyst-position cut axis). Preserves encounter order within each group; drafts with empty ExtractedBy grouped under key ""; result map never nil; no aliasing (C.1). |
| `CompareExtractions` | `func CompareExtractions(analystA string, setA []schema.TraceDraft, analystB string, setB []schema.TraceDraft) ExtractionGap` | Partition two named draft sets by SourceSpan into OnlyInA/OnlyInB/InBoth; compare 9 content fields (WhatChanged, Source, Target, Mediation, Observer, Tags, UncertaintyNote, IntentionallyBlank, SourceDocRef) for drafts visible in both positions; use set-based slice comparison; mark drafts from same SourceSpan but different sets with multiple-drafts sentinel (C.2). |
| `PrintExtractionGap` | `func PrintExtractionGap(w io.Writer, gap ExtractionGap) error` | Write human-readable extraction gap report to io.Writer. Names both analyst positions, three-way partition with SourceSpan lists, field disagreement block, shadow note (neither position is authoritative), non-authoritative disclaimer (C.2). |
| `CompareChainClassifications` | `func CompareChainClassifications(chainA, chainB []DraftStepClassification) []ClassificationDiff` | Compare two classified chains by position (0-indexed step index). Returns classifications differing by Kind or Reason, up to min(len(chainA), lenB) steps. Returns non-nil empty slice when chains are identical (C.3). |
| `PrintClassificationDiffs` | `func PrintClassificationDiffs(w io.Writer, analystA, analystB string, lenA, lenB int, diffs []ClassificationDiff) error` | Write human-readable classification diff report to io.Writer. Names both analyst positions, overall chain length context (lenA/lenB steps), per-diff lines (step position, Kind/Reason for each analyst, position note), footer caveat (neither position is authoritative, data-dependent heuristics) (C.3). |

## Package: adapter

### Files

| File | Contains |
|------|----------|
| `adapter.go` | `Adapter` interface, `ConvertResult` struct, `ForName()` registry, `maxRawBytes` cap (10 MiB). |
| `pdf.go` | `PDFAdapter` — pure-Go PDF extraction via `github.com/ledongthuc/pdf`; page-by-page; `Metadata["page_count"]`; `AdapterName: "pdf-extractor"`. |
| `html.go` | `HTMLAdapter` — iterative HTML tokenizer via `golang.org/x/net/html`; skips script/style/noscript; block elements produce newlines; `AdapterName: "html-extractor"`. |
| `jsonlog.go` | `JSONLogAdapter` — `bufio.Scanner` line-by-line; JSON objects rendered as "message (key=val, ...)"; non-JSON lines verbatim; `Metadata["line_count"]`; `AdapterName: "jsonlog-parser"`. |
| `pdf_test.go`, `html_test.go`, `jsonlog_test.go` | Black-box tests (`package adapter_test`); testdata fixtures in `testdata/`; cover happy path, adapter name, metadata, size cap, file-not-found, format errors, `ForName` registry (#140). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `ConvertResult` | `Text` (string), `AdapterName` (string), `Metadata` (map[string]string) | Output of a format-conversion adapter. `AdapterName` is set by the adapter itself (self-identifying mediator) and travels to the session record for provenance. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `ForName` | `func ForName(name string) (Adapter, error)` | Resolve adapter by explicit name ("pdf", "html", "jsonlog"). Returns descriptive error naming all known adapters on mismatch. |
| `PDFAdapter.Convert` | `(a *PDFAdapter) Convert(path string) (ConvertResult, error)` | Stat → size cap → open → page-by-page text extraction → concatenate. Known limitation: complex multi-column or scanned PDFs may produce degraded text. |
| `HTMLAdapter.Convert` | `(a *HTMLAdapter) Convert(path string) (ConvertResult, error)` | Stat → size cap → open → tokenizer-based text extraction with whitespace normalisation. |
| `JSONLogAdapter.Convert` | `(a *JSONLogAdapter) Convert(path string) (ConvertResult, error)` | Stat → size cap → line-by-line scanner with JSON-aware formatting. |

### Key Design Notes

- **Named mediator**: Analyst explicitly names the adapter (`--adapter pdf`). No auto-detection by extension — the act of format translation is a visible, recorded decision.
- **Self-identifying**: `ConvertResult.AdapterName` is set by the adapter, not the caller. The mediator names itself.
- **Provenance chain**: `AdapterName` flows from `ConvertResult` → `ExtractionOptions.AdapterName` → `ExtractionConditions.AdapterName` (omitempty) → session record JSON.
- **No circular deps**: `adapter` depends on no MeshAnt-internal packages; `llm` receives only the adapter name as a string.

---

## Package: store

### Files

| File | Build tag | Contains |
|------|-----------|----------|
| `store.go` | (none) | `TraceStore` interface; `QueryOpts` struct. |
| `json_file_store.go` | (none) | `JSONFileStore` implementation; `NewJSONFileStore(path)`; `loadOrEmpty`, `matchesOpts`, `writeAtomic` helpers. |
| `neo4j_store.go` | `neo4j` | `Neo4jStore`; `Neo4jConfig`; `NewNeo4jStore(ctx, cfg)`; `traceFromRecord` helper. |
| `neo4j_cypher.go` | `neo4j` | Cypher builders: `storeCypher`, `queryCypher`, `getCypher`; type conversion: `recordToTrace`, `anyListToStrings`, `anySliceToStrings`, `asString`. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `TraceStore` | (interface) | Narrow, swappable storage boundary. Four methods: `Store`, `Query`, `Get`, `Close`. Both `JSONFileStore` and `Neo4jStore` implement this. The analytical engine receives `[]schema.Trace` from `Query` without knowing the backend. |
| `QueryOpts` | `Observer string`, `Window graph.TimeWindow`, `Tags []string`, `Limit int` | Pre-filtering criteria for `Query`. AND semantics across all fields. `Tags` requires ALL listed tags (distinct from `graph.Articulate`'s OR). Zero values mean no filter. `Window` uses `graph.TimeWindow` (temporary coupling; will move to `schema`). |
| `JSONFileStore` | `path string` | JSON-file-backed `TraceStore`. Wraps `loader.Load`; atomic write via temp-rename + fsync; idempotent upsert by ID; missing file → empty slice. Not safe for concurrent writes. |
| `Neo4jConfig` | `BoltURL`, `Username`, `Password`, `Database string` | Connection parameters for `Neo4jStore`. Credentials via env vars; never hardcode. Build tag: `neo4j`. |
| `Neo4jStore` | `driver neo4j.DriverWithContext`, `database string`, `closeOnce sync.Once` | Neo4j-backed `TraceStore`. Single transaction per `Store()`. Timestamp strings RFC3339Nano UTC. MERGE semantics for idempotent upsert; FOREACH for element relationships. Build tag: `neo4j`. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `NewJSONFileStore` | `func NewJSONFileStore(path string) *JSONFileStore` | Constructor; path fixed at construction. File need not exist; `Store` creates it on first write. |
| `JSONFileStore.Store` | `(s *JSONFileStore) Store(ctx, []schema.Trace) error` | Validates all traces, loads existing, upserts by ID, sorts by timestamp, writes atomically. |
| `JSONFileStore.Query` | `(s *JSONFileStore) Query(ctx, QueryOpts) ([]schema.Trace, error)` | Loads all, applies in-memory AND filters, applies Limit. Missing file → `([], nil)`. |
| `JSONFileStore.Get` | `(s *JSONFileStore) Get(ctx, id string) (Trace, bool, error)` | Linear scan by ID. Missing file → `(zero, false, nil)`. |
| `JSONFileStore.Close` | `(s *JSONFileStore) Close() error` | No-op; idempotent. |
| `NewNeo4jStore` | `func NewNeo4jStore(ctx, cfg Neo4jConfig) (*Neo4jStore, error)` | Creates driver, verifies connectivity (fail-fast), returns store. Build tag: `neo4j`. |
| `Neo4jStore.Store` | `(s *Neo4jStore) Store(ctx, []schema.Trace) error` | Validates all, runs single write transaction (MERGE per trace + element FOREACH). Build tag: `neo4j`. |
| `Neo4jStore.Query` | `(s *Neo4jStore) Query(ctx, QueryOpts) ([]schema.Trace, error)` | Dynamic Cypher WHERE from opts; two-stage OPTIONAL MATCH for sources/targets; ordered ASC. Build tag: `neo4j`. |
| `Neo4jStore.Get` | `(s *Neo4jStore) Get(ctx, id string) (Trace, bool, error)` | MATCH by id + source/target collection. Returns `(zero, false, nil)` when not found. Build tag: `neo4j`. |
| `Neo4jStore.Close` | `(s *Neo4jStore) Close() error` | Closes driver via `sync.Once`; idempotent. Build tag: `neo4j`. |

### Key Design Notes

- **Narrow interface**: `TraceStore` does not prescribe transaction scope, batch size, or connection pooling — those are backend-specific concerns. The interface is the same size for both JSON and Neo4j adapters.
- **Pre-filtering ≠ cut**: `QueryOpts` applies coarse pre-filtering. Cut logic (shadow assignment, element inclusion/exclusion) stays entirely in the analytical engine. Setting `Observer` on `QueryOpts` is a retrieval hint, not a cut commitment.
- **Atomic write (JSON)**: `writeAtomic` writes to a temp file in the same directory (same filesystem), calls `fsync`, then renames. Without `fsync`, `close(2)` does not guarantee dirty pages are flushed before rename.
- **FOREACH not UNWIND (Neo4j)**: Element relationships use `FOREACH` because `UNWIND []` produces zero rows and would drop the trace. `FOREACH []` iterates zero times and leaves the trace visible in the transaction.
- **TimeWindow coupling**: `QueryOpts.Window` imports `graph.TimeWindow` — documented temporary coupling. `TimeWindow` will eventually move to `schema`; this note is in both the code and `kg-scoping-v1.md` §2.1.
- **Build tag isolation**: `neo4j_store.go` and `neo4j_cypher.go` carry `//go:build neo4j`. `go test ./...` passes without Neo4j. Integration tests require `MESHANT_NEO4J_TEST_URL`. Run `go mod tidy -tags neo4j` to preserve the driver in go.mod.
- **Decision records**: `docs/decisions/kg-scoping-v1.md` §2 is the contract; `docs/decisions/neo4j-adapter-v1.md` documents the #143 implementation decisions.

---

## Package: serve

### Files

| File | Contains |
|------|----------|
| `response.go` | `ErrorBody`; `writeJSON`, `writeError`, `parseTimeParam`, `parseQueryTime`, `parseLimit` helpers. (`CutMeta` and `Envelope` moved to `graph/envelope.go` in #174.) |
| `server.go` | `Server` struct, `NewServer(ts store.TraceStore) *Server`, `ServeHTTP` method. HTTP multiplexer registration for five API endpoints + static file handler (`go:embed web`). |
| `handlers.go` | `handleArticulate`, `handleDiff`, `handleShadow`, `handleTraces`, `handleElement` endpoint handlers; shared `filterTraces`, `filterByElement` helpers; request parameter parsing and validation. |
| `handlers_test.go` | Black-box tests via `httptest.NewRecorder` and `httptest.NewRequest`; 93.4% coverage. Tests for all five endpoints, observer validation, time-window parsing, tag filtering, limit application, URL encoding, element filtering, static file serving, API route precedence. |
| `web/index.html` | SPA shell: observer gate (`<header id="observer-gate">`), cut header (`<div id="cut-header">`), main flex layout with Cytoscape canvas (`<div id="cy">`) and sidebar (shadow + detail panels). |
| `web/style.css` | Layout: observer gate (centered, prominent), cut header (dark background, sticky), main flex row, sidebar (280px). Shadow panel (amber `#fef9e7`, left border `#f39c12`), detail panel (light grey). Trace cards, provenance block, shadow reason chips. |
| `web/app.js` | Sections: observer gate form wiring, cut metadata rendering, `loadGraph()` (parallel articulate+shadow fetch), `handleNodeClick()` (element fetch), export button wiring, `apiFetch()` helper, error banner, DOMContentLoaded init. |
| `web/render.js` | `initGraph(graphData, onNodeClick)` — Cytoscape init, element mapping, style; `renderShadowPanel(shadowData, onShadowClick)`; `renderDetailPanel(name, traces, observer)` — trace cards with provenance block for `session`-tagged traces. |
| `web/export.js` | `exportJSON()`, `exportDOT()`, `setLastArticulateEnvelope(env)`; `buildDOT(cut, graphData)` — mirrors `PrintGraphDOT` conventions; `triggerDownload(filename, blob)`. |
| `web/lib/cytoscape.min.js` | Vendored Cytoscape.js 3.30.4 (~374KB). Served at `/lib/cytoscape.min.js` via the embedded file server. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `Server` | `store store.TraceStore` | HTTP server wrapping a `TraceStore`. Implements `http.Handler` via `ServeHTTP` method. No state beyond the store reference; stateless endpoint handlers. |
| `ErrorBody` | `Error string` | Simple error response body. `Error` is a human-friendly message. |
| `graph.Envelope` | `Cut graph.CutMeta`, `Data interface{}` | Response envelope for all HTTP responses (defined in `graph/envelope.go`). `Data` carries the endpoint's primary result (Cut, GraphDiff, ShadowSummary, or []Trace). `Cut` provides observer/time-window/tags metadata. Moved to graph package (#174) so non-HTTP analytical layers can build positioned responses. |
| `graph.CutMeta` | `Observer string`, `Analyst string` (omitempty), `From *string`, `To *string`, `Tags []string` (omitempty), `TraceCount int`, `ShadowCount int` | Metadata about the cut applied to generate a response (defined in `graph/envelope.go`). Echoed back in Envelope so client knows what cut was performed. `Analyst` is optional (set by caller, not by `CutMetaFromGraph`). `From` and `To` are nullable RFC3339 strings (nil when unbounded). Moved to graph package (#174). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `NewServer` | `func NewServer(ts store.TraceStore) *Server` | Constructor; wraps a TraceStore. Returns server ready to be passed to `http.ListenAndServe()`. |
| `Server.ServeHTTP` | `(s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request)` | Implements `http.Handler`. Parses URL path and routes to appropriate handler. Returns 404 for unknown routes. |
| `handleArticulate` | Serves `GET /articulate?observer=...&window=...&tags=...&limit=...` | Load traces from store via QueryOpts, articulate graph under observer cut, return Envelope with Cut and CutMeta. Returns 400 if observer is absent. |
| `handleDiff` | Serves `POST /diff` with JSON body `{observer_a: string, observer_b: string, window: TimeWindow, tags: []string}` | Load traces, compute two cuts (observer_a vs observer_b), diff them, return Envelope with GraphDiff and CutMeta from observer_a. Both observers must be present. |
| `handleShadow` | Serves `GET /shadow?observer=...&window=...&tags=...&limit=...` | Load traces, articulate, compute shadow, return Envelope with ShadowSummary and CutMeta. Observer required. |
| `handleTraces` | Serves `GET /traces?observer=...&window=...&tags=...&limit=...` | Load traces from store (QueryOpts filtering applied), validate each, return Envelope with []Trace and CutMeta. Observer required (passed to store as retrieval hint; layer 1 pre-filter, not cut). |
| `filterTraces` | `func filterTraces(traces []schema.Trace, observer string, tw graph.TimeWindow, tags []string) []schema.Trace` | In-memory filtering: observer exact match, time-window bounds (inclusive), tag OR logic. Returns slice in original order. Used by `handleTraces` and `handleElement`. |
| `filterByElement` | `func filterByElement(traces []schema.Trace, elementName string) []schema.Trace` | Returns traces where `elementName` appears in `tr.Source` or `tr.Target` (exact string equality). Used by `handleElement` after observer/time/tag filtering. |
| `handleElement` | `(s *Server) handleElement(w http.ResponseWriter, r *http.Request)` | Serves `GET /element/{name}?observer=...`. Requires observer; optional from/to/tags. Loads full substrate, applies `filterTraces` then `filterByElement`. Returns `Envelope{Cut, Data: []schema.Trace}`. Returns `[]` (not null) for unknown elements. |
| `writeJSON` | `func writeJSON(w http.ResponseWriter, status int, v interface{})` | Marshal v as indented JSON with given HTTP status code. Sets Content-Type: application/json. Used by all handlers for success and error responses. |
| `writeError` | `func writeError(w http.ResponseWriter, status int, msg string)` | Create ErrorBody, marshal and write JSON error response. Used by all handlers for error responses (observer missing, invalid time window, storage failure, etc.). |
| `parseTimeParam` | `func parseTimeParam(name, value string) (time.Time, error)` | Parse a named RFC3339 query parameter. Returns zero time and nil error when value is empty (parameter absent). |
| `parseQueryTime` | `func parseQueryTime(fromStr, toStr string) (graph.TimeWindow, error)` | Parse ?from= and ?to= query parameters into a graph.TimeWindow. Either or both may be absent (half-open or unbounded window). Validates that Start ≤ End when both are set. |
| `parseLimit` | `func parseLimit(limitStr string) (int, error)` | Parse the ?limit= query parameter. Returns 0 (no limit) when the parameter is absent. Returns an error for non-integer or negative values. |

### Key Design Notes

- **Observer on all endpoints**: All five endpoints require `observer` query param (400 with ErrorBody message explaining ANT principle if absent). This enforces Principle 3 (observer position is a cut). Store's QueryOpts.Observer is a retrieval hint (layer 1 pre-filter), not a cut assertion.
- **Envelope invariant**: Success response has `graph.Envelope` with `Data + Cut` metadata. Error response has `ErrorBody` only. Never both. Clients check Error presence to detect failure.
- **CutMeta echo**: Every successful response echoes back the cut parameters (observer, time window, tags) so client knows what was queried. Prevents silent mismatches. `TraceCount` and `ShadowCount` included for audit.
- **Types moved to graph package**: `CutMeta` and `Envelope` were moved from serve/response.go to graph/envelope.go (#174) so non-HTTP analytical layers can build positioned responses without importing the serve package. `CutMetaFromGraph` helper extracts metadata from an articulated MeshGraph.
- **No context enforcement**: Handlers accept request timeout via http.Request.Context but do not enforce it (defer to store implementations). Store.Query and Store.Get accept context parameter for cancellation.
- **Stateless handlers**: No global state, no session storage, no in-process caching. Each request is independent.
- **Time-window custom JSON**: TimeWindow encodes/decodes via custom codec (see `graph.serial.go`). HTTP endpoints parse RFC3339 strings and reconstruct TimeWindow structs.
- **Decision record (serve)**: `docs/decisions/serve-v1.md` — observer requirement, Envelope structure, graceful shutdown, Phase 4 (#145).
- **Decision record (web-ui)**: `docs/decisions/web-ui-v1.md` — Cytoscape.js choice, `go:embed`, no build step, observer gate as structural HTML, `/element/{name}` in-memory design, client-side DOT duplication, provenance deferred, Phase 4 (#146).
- **Static file server**: `go:embed web` in `server.go` embeds the entire `web/` directory. `fs.Sub(webFS, "web")` strips the prefix; `http.FileServerFS` serves at `GET /`. API routes are registered first and take precedence over the catch-all `/` handler.

---

## Package: mcp

### Files

| File | Contains |
|------|----------|
| `server.go` | `Server` struct, `NewServer(ts store.TraceStore, analyst string) *Server` constructor, `Run(ctx, in, out) error` method. JSON-RPC 2.0 message types (`rpcRequest`, `rpcResponse`, `rpcError`). Tool registry via `registerTool` and tool handler dispatch via `dispatch`. Handlers for `initialize`, `tools/list`, `tools/call` MCP methods. Single-threaded stdio loop with 4 MiB scan buffer for large graph payloads. Immutable analyst identity stamped on every CutMeta response. All 6 batch-1 tools registered in `NewServer`. (#176/#177) |
| `tools.go` | MCP tool handler functions and schemas — batch 1 complete (#176/#177). Tool input types: `articulateArgs`, `shadowArgs`, `followArgs`, `bottleneckArgs`, `summarizeArgs`, `validateArgs`. Result types: `validateResult`, `validateError`. Input validation constants: `maxObserverLen` (500), `maxTagLen` (200), `maxTagCount` (50), `maxFollowDepth` (1000). Helpers: `validateObserver`, `validateTags` (guard against crafted clients), `filterByTagsOR` (OR tag semantics for validate — distinct from store's AND semantics), `recordInvocation` (Principle 8 reflexive trace, soft-fail), `parseTimeWindow`, `newUUID4`. Batch 2 (#178 — diff, gaps) deferred. |
| (CLI) `cmd/meshant/cmd_mcp.go` | `cmdMcp(w io.Writer, args []string) error` subcommand handler. `--analyst` required (D2 mcp-v1.md — hard refusal if absent). `--db` flag for Neo4j backend; mutually exclusive with `<traces.json>` positional argument. Mirrors `serve` and `store` command design: uniform `TraceStore` interface regardless of backend. Signal context (SIGINT/SIGTERM) for graceful shutdown. Startup banner to stderr (stdout reserved for MCP framing). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `Server` | `ts store.TraceStore`, `analyst string`, `tools map[string]ToolHandler`, `schemas []toolSchema` | MCP server backed by a TraceStore. Analyst identity is immutable and stamped on every response (D2, mcp-v1.md). No state beyond store and tool registry; stateless per-message dispatch. |
| `rpcRequest` | `JSONRPC string`, `ID *json.RawMessage`, `Method string`, `Params json.RawMessage` | Inbound JSON-RPC 2.0 message. `ID` is nullable (nil for notifications); params are raw JSON to allow tool-specific decoding. |
| `rpcResponse` | `JSONRPC string`, `ID *json.RawMessage`, `Result interface{}`, `Error *rpcError` | Outbound JSON-RPC 2.0 message. Result and Error are mutually exclusive. ID echoes the request ID (null for parse errors). |
| `rpcError` | `Code int`, `Message string` | JSON-RPC error code (-32700 parse, -32601 method-not-found, -32602 invalid-params) and human message. |
| `ToolHandler` | Function type: `func(ctx context.Context, params json.RawMessage) (interface{}, error)` | Signature for MCP tool handlers. Receives raw JSON params and context; returns result or error. Non-nil errors are returned as tool-level errors (content isError=true), not protocol errors. |
| `toolSchema` | `Name string`, `Description string`, `InputSchema inputSchema` | MCP tool descriptor returned by tools/list. Minimal JSON Schema for input validation. One schema per registered tool. |
| `inputSchema` | `Type string`, `Properties map[string]property`, `Required []string` | JSON Schema object descriptor for tool inputs. Lists required parameters. |
| `property` | `Type string`, `Description string`, `Items *property` (omitempty) | JSON Schema property descriptor. `Items` is used for array properties (e.g., `Items: &property{Type: "string"}` for `[]string` tags fields). |
| `toolsCallParams` | `Name string`, `Arguments json.RawMessage` | Params shape for a tools/call request (MCP protocol). `Arguments` is raw JSON — each tool decodes according to its schema. |
| `toolContent` | `Type string`, `Text string` | Single item in a tool result content array (MCP protocol). Returned as `{type: "text", text: ...}` on success. |
| `articulateArgs` | `Observer string`, `From string`, `To string`, `Tags []string` | Input for `meshant_articulate`. Observer required; From/To RFC3339 time window; Tags OR-filtered. |
| `shadowArgs` | `Observer string`, `From string`, `To string`, `Tags []string` | Input for `meshant_shadow`. Same shape as articulateArgs. |
| `followArgs` | `Observer string`, `Element string`, `Direction string`, `MaxDepth int`, `From string`, `To string`, `Tags []string` | Input for `meshant_follow`. Direction: "forward" (default) or "backward". MaxDepth: 0 = unlimited, 1–1000 allowed. Element is the starting actant name. |
| `bottleneckArgs` | `Observer string`, `From string`, `To string`, `Tags []string` | Input for `meshant_bottleneck`. Same shape as articulateArgs. |
| `summarizeArgs` | `Observer string`, `From string`, `To string`, `Tags []string` | Input for `meshant_summarize`. Same shape as articulateArgs. |
| `validateArgs` | `Tags []string` | Input for `meshant_validate`. No observer (D5 exemption). Tags are OR-filtered in memory. |
| `validateResult` | `TotalTraces int`, `ValidCount int`, `InvalidCount int`, `Errors []validateError` | Result of `meshant_validate`. Errors lists per-trace failures. |
| `validateError` | `TraceID string`, `Error string` | A single trace validation failure from `meshant_validate`. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `NewServer` | `func NewServer(ts store.TraceStore, analyst string) *Server` | Constructor; wraps a TraceStore and analyst identity. Registers all 6 batch-1 tools. Returns server ready to pass to Run. |
| `Server.Run` | `(s *Server) Run(ctx context.Context, in io.Reader, out io.Writer) error` | Reads newline-delimited JSON-RPC 2.0 messages from in, dispatches to handlers, writes responses to out. Returns when in reaches EOF or context is cancelled. Single-threaded, synchronous per-message. Scanner buffer expanded to 4 MiB to handle large graph payloads. |
| `Server.registerTool` | `(s *Server) registerTool(schema toolSchema, handler ToolHandler)` | Adds a named handler and schema to the server. Called during construction; not safe for concurrent use after Run begins. |
| `Server.dispatch` | `(s *Server) dispatch(ctx context.Context, req rpcRequest) rpcResponse` | Routes a request to the appropriate handler (initialize, initialized, tools/list, tools/call) or returns method-not-found error. |
| `Server.handleInitialize` | `(s *Server) handleInitialize(req rpcRequest) rpcResponse` | Responds to MCP initialize handshake with protocolVersion, serverInfo, and empty capabilities. |
| `Server.handleToolsList` | `(s *Server) handleToolsList(req rpcRequest) rpcResponse` | Returns tools array (all registered schemas) in result.tools. |
| `Server.handleToolsCall` | `(s *Server) handleToolsCall(ctx context.Context, req rpcRequest) rpcResponse` | Dispatches tools/call to registered handler. Decodes `toolsCallParams` from request, calls handler, encodes result as content array. On error, returns tool-level error (isError=true, not protocol error). |
| `Server.registerArticulate` | `(s *Server) registerArticulate()` | Registers `meshant_articulate`. Schema: observer (required), from, to, tags. |
| `Server.handleArticulate` | `(s *Server) handleArticulate(ctx, params) (interface{}, error)` | Queries full substrate, calls `graph.Articulate`, wraps in Envelope, records invocation trace. |
| `Server.registerShadow` | `(s *Server) registerShadow()` | Registers `meshant_shadow`. Schema: observer (required), from, to, tags. |
| `Server.handleShadow` | `(s *Server) handleShadow(ctx, params) (interface{}, error)` | Articulates substrate for observer, calls `graph.SummariseShadow`, wraps in Envelope, records invocation trace. |
| `Server.registerFollow` | `(s *Server) registerFollow()` | Registers `meshant_follow`. Schema: observer, element (required), direction, max_depth, from, to, tags. |
| `Server.handleFollow` | `(s *Server) handleFollow(ctx, params) (interface{}, error)` | Validates direction and max_depth bounds (0–1000). Articulates, calls `graph.FollowTranslation` + `graph.ClassifyChain`, wraps in Envelope, records invocation trace. |
| `Server.registerBottleneck` | `(s *Server) registerBottleneck()` | Registers `meshant_bottleneck`. Schema: observer (required), from, to, tags. |
| `Server.handleBottleneck` | `(s *Server) handleBottleneck(ctx, params) (interface{}, error)` | Articulates, calls `graph.IdentifyBottlenecks`, wraps in Envelope, records invocation trace. |
| `Server.registerSummarize` | `(s *Server) registerSummarize()` | Registers `meshant_summarize`. Schema: observer (required), from, to, tags. |
| `Server.handleSummarize` | `(s *Server) handleSummarize(ctx, params) (interface{}, error)` | Articulates, calls `graph.DraftNarrative`, wraps in Envelope, records invocation trace. |
| `Server.registerValidate` | `(s *Server) registerValidate()` | Registers `meshant_validate`. Schema: tags only (no observer — D5 exemption). |
| `Server.handleValidate` | `(s *Server) handleValidate(ctx, params) (interface{}, error)` | Queries full substrate, applies `filterByTagsOR`, calls `schema.Trace.Validate()` on each. Returns `validateResult`. No observer, no invocation trace (D5 exemption). |
| `recordInvocation` | `(s *Server) recordInvocation(ctx, toolName, observer string)` | Reflexivity helper (Principle 8): writes an invocation trace tagged "mcp-invocation" to the store. Observer = tool's observer arg (T1 tension). Soft-fail: logs on error, never returns error to caller. |
| `filterByTagsOR` | `func filterByTagsOR(traces []schema.Trace, filter []string) []schema.Trace` | In-memory OR tag filter: returns traces carrying at least one of the requested tags. Used by handleValidate because store QueryOpts.Tags uses AND semantics but the tool schema promises OR semantics. |
| `validateObserver` | `func validateObserver(obs string) error` | Validates observer is non-empty and ≤500 chars. |
| `validateTags` | `func validateTags(tags []string) error` | Validates tag count ≤50 and each tag ≤200 chars. |
| `parseTimeWindow` | `func parseTimeWindow(from, to string) (graph.TimeWindow, error)` | Parse RFC3339 strings into a graph.TimeWindow. Returns unbounded bounds when empty. Validates Start ≤ End. |
| `newUUID4` | `func newUUID4() string` | Generate a random RFC4122 UUID v4. Mirrors `graph.newUUID4` without export (deferred N1 — extract to internal/uuid if third consumer appears). |

### Key Design Notes

- **Observer discipline (D1, mcp-v1.md)**: Every tool requires an `observer` parameter. The `--analyst` flag is required at server startup (D2) — an LLM client that does not declare an analyst position is hiding the cut. Analyst and Observer are kept distinct: Analyst is the declared reader (on CutMeta); Observer is the ANT position (in graph articulation).
- **Reflexivity (Principle 8, D5 mcp-v1.md)**: Every cut-producing tool writes a trace back to the store. The MCP server is a mediator — its actions must be visible in the mesh, not hidden behind a service facade. Invocation traces are tagged "mcp-invocation" and are filterable and nameable, not suppressed.
- **Single-threaded stdio transport (D7 mcp-v1.md)**: The server reads messages synchronously from stdin and writes responses to stdout. No concurrency — JSON-RPC dispatch is single-threaded by design. SSE (server-sent events) is deferred to a future batch; stdio simplicity is valued for the initial MCP integration.
- **Large graph buffer**: Scanner buffer is 4 MiB (default 64 KiB) to accommodate dense substrate graphs. A single `tools/call` response from a large dataset can easily exceed the default limit, causing `bufio.ErrTooLong`; the expanded buffer prevents mid-session exits.
- **Tool registration batches**: Batch 1 (#176/#177) registers all 6 tools: articulate, shadow, follow, bottleneck, summarize, validate. Batch 2 (#178) will add diff and gaps. Every cut-producing tool follows the same pattern: validate inputs → query full substrate → call graph function → build Envelope with CutMeta.Analyst set → record invocation trace. `meshant_validate` is exempt (D5): no observer, no invocation trace, in-memory OR tag filter via `filterByTagsOR`.
- **Input validation**: `validateObserver` and `validateTags` cap field lengths (500/200 chars) and tag counts (50) before use. `handleFollow` additionally caps `max_depth` (0–1000) to prevent memory amplification from crafted clients.
- **JSON-RPC 2.0 protocol**: No external SDK — the protocol surface is small enough to implement inline (server.go ~75 lines for core dispatch). Supports notifications (null id) as no-op; returns -32700 for parse errors, -32601 for unknown methods, -32602 for invalid params (protocol errors), plus tool-level errors.
- **Graceful shutdown**: Run returns when stdin reaches EOF (client closes) or context is cancelled (SIGINT/SIGTERM). The cmdMcp handler sets up signal context; server exits cleanly without dropping in-flight messages.
- **Decision record (mcp)**: `docs/decisions/mcp-v1.md` — observer discipline (D1), analyst requirement (D2), tool parameter validation (D3), graph substrate boundaries (D4), invocation trace design (D5), tool batching (D6), stdio vs SSE (D7), ANT tensions (T171.1–T171.5).

---

## Package: graph

### Files

| File | Contains |
|------|----------|
| `envelope.go` | `CutMeta`, `Envelope` types; `CutMetaFromGraph` function. Positioned-response metadata and envelope structure. Moved from serve/response.go for reuse by non-HTTP layers (#174). |
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
| `shadow.go` | Shadow analysis: `SummariseShadow`, `PrintShadowSummary`; `ShadowSummary` type (M13). |
| `gaps.go` | Observer-gap analysis: `AnalyseGaps`, `PrintObserverGap`; `ObserverGap` type (M13). |
| `bottleneck.go` | Bottleneck analysis: `IdentifyBottlenecks`, `PrintBottleneckNotes`; `BottleneckOptions`, `BottleneckNote` types (B.1). |
| `suggest.go` | Re-articulation suggestion: `SuggestRearticulations`, `PrintRearticSuggestions`; `SuggestionKind`, `RearticSuggestion` types (B.2). |
| `narrative.go` | Narrative drafts: `DraftNarrative`, `PrintNarrativeDraft`; `NarrativeDraft` type (B.3). Positioned reading of a graph; names cut position, top elements by frequency, observed mediations, shadow count, and methodological caveats. |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `CutMeta` | `Observer string`, `Analyst string` (omitempty), `From *string`, `To *string`, `Tags []string` (omitempty), `TraceCount int`, `ShadowCount int` | Metadata about a positioned reading. Observer is the ANT position; Analyst is the human or agent that requested the cut (optional, set by caller). From/To are nullable RFC3339 strings (nil when unbounded time window). TraceCount is included traces; ShadowCount is shadow elements (articulation) or non-observer traces (raw-trace endpoints). |
| `Envelope` | `Cut CutMeta`, `Data interface{}` | Response wrapper for positioned data. Every analytical output (HTTP or otherwise) pairs data with its cut metadata so the position is undeniable. |
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
| `ShadowSummary` | `TotalShadowed` (int), `ByReason` (map[string]int), `Elements` ([]ShadowElement), `SeenFromCounts` (map[string]int), `Cut` (Cut) | Summary of shadowed elements in an articulated graph. ByReason counts by ShadowReason; SeenFromCounts maps excluded observer position to the count of elements seen from it; Elements sorted by name (M13). |
| `ObserverGap` | `OnlyInA` ([]string), `OnlyInB` ([]string), `InBoth` ([]string), `CutA` (Cut), `CutB` (Cut) | Visibility asymmetry between two articulations. All three element lists sorted alphabetically. Both cuts retained for self-situated reporting (M13). |
| `BottleneckOptions` | (empty struct) | Configuration for `IdentifyBottlenecks`. Reserved as extension point for future thresholds or heuristic toggles (v1: intentionally empty, B.1). |
| `BottleneckNote` | `Element` (string), `AppearanceCount` (int), `MediationCount` (int), `ShadowCount` (int), `Reason` (string) | Provisional centrality reading for one element from a cut. Three independent measures (not combined). Reason hedges with "from this cut" to signal provisionality (B.1). |
| `SuggestionKind` | (string constant: `SuggestionObserverExpansion`, `SuggestionTimeExpansion`, `SuggestionTagRelaxation`) | Category of re-articulation change being suggested (B.2). |
| `RearticSuggestion` | `Kind` (SuggestionKind), `Side` (string: "A" or "B"), `Rationale` (string), `SuggestedParams` (string) | Heuristic provocation for narrowing a gap. Rationale always names what the suggestion cannot know. SuggestedParams is plain-language description of suggested change (B.2). |
| `NarrativeDraft` | `PositionStatement` (string), `Body` (string), `ShadowStatement` (string), `Caveats` ([]string) | Provisional, positioned narrative reading of a graph from one observer cut. Immutable once returned. Zero-value for empty graphs (no edges); all four fields populated for non-empty graphs. Names cut position, trace count, top-3 elements by frequency, observed mediations (up to 5 + count of remainder), shadow count with exclusion reasons, and methodological caveats (B.3). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `TimeWindow.IsZero` | `(tw TimeWindow) IsZero() bool` | Check if both Start and End are zero (no time filter). |
| `TimeWindow.Validate` | `(tw TimeWindow) Validate() error` | Validate that Start ≤ End (if both non-zero). |
| `TimeWindow.MarshalJSON` | `(tw TimeWindow) MarshalJSON() ([]byte, error)` | Encode zero bounds as JSON null, non-zero as RFC3339. |
| `TimeWindow.UnmarshalJSON` | `(tw *TimeWindow) UnmarshalJSON(data []byte) error` | Decode JSON null as zero time.Time, strings as RFC3339. |
| `CutMetaFromGraph` | `func CutMetaFromGraph(g MeshGraph) CutMeta` | Build CutMeta from an articulated MeshGraph. Observer is the first entry in g.Cut.ObserverPositions (empty when no observer filter). From and To are RFC3339 strings when corresponding TimeWindow bounds are non-zero; nil when unbounded. Analyst is always empty (callers set it). (#174) |
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
| `SummariseShadow` | `func SummariseShadow(g MeshGraph) ShadowSummary` | Read `g.Cut.ShadowElements`; count total shadowed, count by ShadowReason, count SeenFrom per excluded observer; return self-contained summary (M13). |
| `PrintShadowSummary` | `func PrintShadowSummary(w io.Writer, s ShadowSummary) error` | Write shadow report to io.Writer. Observer position, shadow count by reason, SeenFrom counts descending, element list. Includes "No shadow" path when no elements shadowed (M13). |
| `AnalyseGaps` | `func AnalyseGaps(g1, g2 MeshGraph) ObserverGap` | Compare node sets of two pre-articulated graphs; partition names into OnlyInA, OnlyInB, InBoth; retain both Cuts. Does not re-articulate (M13). |
| `PrintObserverGap` | `func PrintObserverGap(w io.Writer, gap ObserverGap) error` | Write observer-gap report to io.Writer. Names both positions, three-way partition with element lists, "No gap" message when identical; neither position treated as authoritative (M13). |
| `IdentifyBottlenecks` | `func IdentifyBottlenecks(g MeshGraph, _ BottleneckOptions) []BottleneckNote` | Apply v1 centrality heuristic: include if MediationCount > 0 OR AppearanceCount ≥ 2 OR ShadowCount > 0. Sort by MediationCount desc → AppearanceCount desc → name asc. Always returns non-nil slice (empty when no nodes qualify) (B.1). |
| `PrintBottleneckNotes` | `func PrintBottleneckNotes(w io.Writer, g MeshGraph, notes []BottleneckNote) error` | Write bottleneck analysis report to io.Writer. Header, cut context, per-note lines (element, counts, reason), footer caveat (B.1). |
| `SuggestRearticulations` | `func SuggestRearticulations(gap ObserverGap) []RearticSuggestion` | Generate heuristic re-articulation suggestions from an ObserverGap. Returns nil when no gap (both OnlyInA and OnlyInB empty); returns non-nil empty slice when gap exists but no heuristic fires (B.2). |
| `PrintRearticSuggestions` | `func PrintRearticSuggestions(w io.Writer, gap ObserverGap, suggestions []RearticSuggestion) error` | Write re-articulation suggestions to io.Writer. Returns nil immediately for nil input. Header, gap summary, per-suggestion blocks, footer caveat naming suggestion engine's own shadow (B.2). |
| `DraftNarrative` | `func DraftNarrative(g MeshGraph) NarrativeDraft` | Produce a provisional narrative reading of graph g. Returns zero-value for empty graphs. For non-empty graphs: positions the cut, counts included traces, names top-3 elements by appearance frequency, lists up to 5 observed mediations, counts shadow elements with reasons, and generates methodological caveats based on shadow ratio, time window, and tag filters (B.3). |
| `PrintNarrativeDraft` | `func PrintNarrativeDraft(w io.Writer, n NarrativeDraft) error` | Write formatted narrative draft to io.Writer. Renders header, position statement, body (reading from cut), shadow statement (what is in shadow from this position), caveats (methodological qualifications), and footer note. Language is provisional throughout; "missing" never used (B.3). |

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

## Package: review

### Files

| File | Contains |
|------|----------|
| `ambiguity.go` | `AmbiguityWarning` struct; `DetectAmbiguities` function. |
| `render.go` | `RenderDraft`, `RenderAmbiguities`, `RenderChain`; helpers `valueOrEmpty`, `sliceOrEmpty`, `truncateString`. |
| `session.go` | `RunReviewSession`, `deriveAccepted`, `deriveEdited`, `runEditFlow`, `parseCommaSeparated`, `filterReviewable`, `cloneStrings`; interactive session loop and derivation logic (Thread A.3/A.4). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `AmbiguityWarning` | `Field` (string), `Message` (string) | Positioned observation that a draft field is unregistered or in shadow from this position. Language is ANT-disciplined: no "missing", "error", or "incomplete". Invitations to inspect, not demands to correct (Thread A). |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `DetectAmbiguities` | `func DetectAmbiguities(d schema.TraceDraft) []AmbiguityWarning` | Check 6 candidate content fields (what_changed, source, target, mediation, observer, tags) for blank values, subject to `IntentionallyBlank` suppression. Also checks for criterion_ref mismatch (UncertaintyNote set but CriterionRef absent). Returns nil if no ambiguities detected (Thread A.1). |
| `RenderDraft` | `func RenderDraft(d schema.TraceDraft, index, total int) string` | Format a TraceDraft for terminal display in the review session. Shows all candidate and provenance fields; blank values rendered as "(empty)". `index` is 1-based queue position (Thread A.1). |
| `RenderAmbiguities` | `func RenderAmbiguities(warnings []AmbiguityWarning) string` | Format `[]AmbiguityWarning` for display below a rendered draft. Returns "(none)" when warnings is nil or empty (Thread A.1). |
| `RenderChain` | `func RenderChain(chain []schema.TraceDraft, classifications []loader.DraftStepClassification) string` | Format a derivation chain for display in the review session. Shows each draft with truncated ID (8 chars), extraction_stage, extracted_by, and truncated what_changed (60 chars). Interleaves DraftStepClassification lines (Kind + Reason) between drafts. Last draft marked `<-- current`. Returns "(no derivation chain)" for empty input (Thread A.2). |
| `RunReviewSession` | `func RunReviewSession(drafts []schema.TraceDraft, in io.Reader, out io.Writer) ([]schema.TraceDraft, error)` | Interactive accept/edit/skip/quit loop over reviewable draft records. Renders chain, draft, and ambiguities per draft; returns all newly derived (accepted or edited) drafts. Filters to ExtractionStage "weak-draft" or "critiqued" (F.4); falls back to all drafts when no stage metadata present. EOF treated as quit (Thread A.3/A.4). |
| `deriveAccepted` | `func deriveAccepted(parent schema.TraceDraft) (schema.TraceDraft, error)` | Creates a new TraceDraft derived from parent: copies all candidate fields (deep-copies slices), sets DerivedFrom=parent.ID, ExtractionStage="reviewed", ExtractedBy="meshant-review", new UUID and Timestamp (Thread A.3). |
| `runEditFlow` | `func runEditFlow(d schema.TraceDraft, scanner *bufio.Scanner, out io.Writer) (schema.TraceDraft, bool, error)` | Field-by-field editing loop over 8 editable content fields (what_changed, source, target, mediation, observer, tags, uncertainty_note, criterion_ref). Empty input keeps current value; slice fields accept comma-separated input. Returns (editedDraft, completedOK, error). EOF mid-flow returns (partial, false, nil) — caller must not append. Provenance fields are not editable (Thread A.4). |
| `deriveEdited` | `func deriveEdited(parent schema.TraceDraft, edited schema.TraceDraft) (schema.TraceDraft, error)` | Creates a new TraceDraft derived from parent using content fields from edited. Content (WhatChanged, Source, Target, Mediation, Observer, Tags, UncertaintyNote, CriterionRef) from edited; provenance (SourceSpan, SourceDocRef, IntentionallyBlank) from parent. DerivedFrom=parent.ID, ExtractionStage="reviewed", ExtractedBy="meshant-review", new UUID and Timestamp. Edit = one derivation step, not two (Thread A.4). |
| `parseCommaSeparated` | `func parseCommaSeparated(s string) []string` | Splits s on commas, trims whitespace per element, drops empty strings. Returns nil when no non-empty elements remain (Thread A.4). |

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

## Package: llm

### Files

| File | Contains |
|------|----------|
| `types.go` | Shared types: `ExtractionConditions`, `CritiqueConditions`, `DraftDisposition`, `SessionRecord`, `ExtractionOptions`, `AssistOptions`, `CritiqueOptions`, `SplitOptions`; error types `ErrLLMRefusal`, `ErrMalformedOutput`; constants `frameworkUncertaintyNote`, `maxResponseBytes`, `httpTimeout`; `knownContentFields` map. `DraftDisposition.Action` values: `"accepted"`, `"edited"`, `"skipped"`, `"abandoned"`. `SessionRecord.DraftIDs` is nil (not set) for split sessions — spans are not TraceDraft records; use `DraftCount`. (#150, #151) |
| `client.go` | `LLMClient` interface; `AnthropicClient` implementation (net/http, zero external deps); `NewAnthropicClient()` (env-var key lookup); private `httpClient` with 180 s timeout; response body capped at 8 MiB; auth error scrubbing (401/403 bodies withheld). |
| `prompt.go` | `LoadPromptTemplate(path)` — reads prompt template up to 1 MiB; empty file is valid (no error). `HashPromptTemplate(path)` — SHA-256 of file bytes, first 16 hex chars; empty path returns `"", nil` (#150). |
| `shared.go` | Package-internal shared helpers: `readSourceDoc(path)` — reads source document capped at `maxSourceBytes` (1 MiB); `isRefusal(response)` — conservative prefix-based refusal detection. Used by `extract.go` and `split.go` (#137). |
| `extract.go` | `RunExtraction(ctx, client, opts)` — LLM extraction pipeline; supports multi-document ingestion via `opts.InputPaths`/`opts.SourceDocRefs` (#139); always returns non-nil `SessionRecord` even on error; validates `IntentionallyBlank` field names; strips LLM preamble before JSON parse; stamps provenance fields (D2–D7). Private helpers: `extractSingleDoc` (per-document helper), `parseResponse`, `validateIntentionallyBlank`. (`readSourceDoc` and `isRefusal` in `shared.go`.) |
| `split.go` | `RunSplit(ctx, client, opts)` — LLM span-boundary detection pipeline; calls LLM to propose candidate observation spans from a source document; always returns non-nil `SessionRecord`; `DraftIDs` nil (spans are not TraceDraft records); `DraftCount` = number of non-blank spans; empty result is an error. Private helper: `parseSplitResponse` (preamble-tolerant JSON string-array parser with blank filtering) (#137). |
| `assist.go` | `RunAssistSession(ctx, client, spans, opts, in, out)` — interactive per-span LLM-assist session; skip preserves draft (shadow is not absence); edit appends LLM draft + derived draft; EOF-during-edit records disposition `"abandoned"`. `ParseSpans(data)` — parses spans file from JSON array, newline-separated text, or single line. `parseSingleDraft` — stamps provenance, zeros `DerivedFrom`, validates `IntentionallyBlank`. Imports `review` for `RunEditFlow`, `DeriveEdited`, rendering (F.3). |
| `critique.go` | `RunCritique(ctx, client, drafts, opts)` — LLM critique pipeline; one call per input draft; returns partial results (nil error on per-draft failures); always returns non-nil `SessionRecord`; hard SourceSpan integrity check (rejects mismatch, session continues). Sets `rec.CritiqueConditions` (not `rec.Conditions`) — critique apparatus is analytically distinct from extraction apparatus (#151). Private helpers: `buildCritiquePrompt`, `parseCritiqueDraft` (F.4). |
| `session_promote.go` | `PromoteSession(rec SessionRecord, observer string) (schema.Trace, error)` — converts a SessionRecord to a canonical Trace, closing the Principle 8 reflexivity gap. Field mapping: ID←rec.ID, Source←[ModelID], Target←[SourceDocRef], Mediation←Command, Observer←caller-provided (required), Tags←["session","articulation"]; WhatChanged surfaces command + SourceDocRef + ModelID for self-situated description. Dispatches on command: critique sessions read apparatus from `CritiqueConditions` (when non-nil) and use `SourceDocRef` (singular); all other sessions (extract, assist, split) use `Conditions.SourceDocRefs`/`Conditions.SourceDocRef`. Error sessions are promotable (a failed act is an observable act). Always validates the promoted trace before returning. Private helper: `sessionWhatChanged` (#138, #151). |

### Types

| Type | Key Fields | Purpose |
|------|-----------|---------|
| `LLMClient` | `Complete(ctx, system, prompt string) (string, error)` | Interface for LLM completion. Single method — models the analytical boundary: inputs in, text out. Implemented by `AnthropicClient`; tests inject a mock. |
| `AnthropicClient` | `apiKey` (unexported), `model`, `baseURL`, `httpClient` | Anthropic Messages API client. API key never serialised or logged. Uses a private `http.Client` with explicit timeout; never `http.DefaultClient`. |
| `ExtractionConditions` | `ModelID`, `PromptTemplate`, `PromptHash` (string, #150), `CriterionRef`, `SystemInstructions`, `SourceDocRefs` ([]string, #139), `SourceDocRef` (legacy string), `AdapterName`, `Timestamp` | Apparatus description for extract/assist/split sessions. Intentionally excludes API key — conditions are written to disk. `PromptHash` is the first 16 hex chars of the SHA-256 of the prompt template file for reproducibility tracking. `SourceDocRefs` is the plural field for multi-doc sessions; `SourceDocRef` is retained for backward compatibility with pre-#139 session files. |
| `CritiqueConditions` | `ModelID`, `PromptTemplate`, `PromptHash` (string, #150), `CriterionRef`, `SystemInstructions`, `SourceDocRef` (string, singular), `Timestamp` | Apparatus description for critique sessions. Analytically distinct from `ExtractionConditions`: critique input is a `TraceDraft` array, not a source document; `SourceDocRef` is singular (one critique session per source); no adapter (#151). |
| `SessionRecord` | `ID` (uuid), `Command` (string), `Conditions` (ExtractionConditions), `CritiqueConditions` (*CritiqueConditions, omitempty, #151), `DraftIDs` ([]string), `Dispositions` ([]DraftDisposition), `InputPaths` ([]string, #139), `InputPath` (legacy string), `OutputPath`, `DraftCount`, `ErrorNote`, `Timestamp` | Mandatory companion to every LLM interaction. Returned on every code path. `ID` is stamped as `SessionRef` on every produced draft. `CritiqueConditions` is set (and `Conditions` left zero) for critique sessions. `ErrorNote` makes failures inspectable from disk without re-running. `InputPaths` is the plural field for multi-doc sessions; `InputPath` is retained for backward compatibility. |
| `ExtractionOptions` | `ModelID`, `InputPaths` ([]string, #139), `SourceDocRefs` ([]string, #139), `PromptTemplatePath`, `CriterionRef`, `OutputPath` | Input parameters for `RunExtraction`. Both `InputPaths` and `SourceDocRefs` must be non-empty and equal length. |
| `AssistOptions` | `ModelID`, `InputPath`, `PromptTemplatePath`, `CriterionRef`, `SourceDocRef`, `OutputPath`, `SessionOutputPath` | Input parameters for `RunAssistSession`. `InputPath` is the spans file path — optional; recorded in SessionRecord for provenance. |
| `CritiqueOptions` | `ModelID`, `InputPath`, `PromptTemplatePath`, `CriterionRef`, `SourceDocRef`, `OutputPath`, `SessionOutputPath`, `DraftID` | Input parameters for `RunCritique`. `DraftID` filters to a single draft by ID when non-empty (F.4). |
| `SplitOptions` | `ModelID`, `InputPath`, `PromptTemplatePath`, `SourceDocRef`, `OutputPath` | Input parameters for `RunSplit`. No `CriterionRef` — split is boundary detection only; criterion enters at the assist stage (#137). |
| `ErrLLMRefusal` | `RefusalText string` | LLM explicitly declined to produce output. |
| `ErrMalformedOutput` | `RawResponse string`, `ParseErr error` | LLM returned text that does not parse as TraceDraft JSON. |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `NewAnthropicClient` | `func NewAnthropicClient(model string) (*AnthropicClient, error)` | Read API key from `MESHANT_LLM_API_KEY` (fallback `ANTHROPIC_API_KEY`); construct client with 180 s timeout. Returns error if both env vars are absent or empty. |
| `AnthropicClient.Complete` | `(c *AnthropicClient) Complete(ctx, system, prompt string) (string, error)` | Post to Anthropic Messages API; return first text content block. Caps response body at 8 MiB; scrubs auth error bodies. |
| `LoadPromptTemplate` | `func LoadPromptTemplate(path string) (string, error)` | Read prompt template file up to 1 MiB. Empty file returns `""` without error. |
| `HashPromptTemplate` | `func HashPromptTemplate(path string) (string, error)` | Compute first 16 hex chars of the SHA-256 of a prompt template file. Empty path returns `"", nil` (valid for assist sessions with no prompt file). Applies the same 1 MiB size cap as `LoadPromptTemplate`. Used by extract, assist, critique, and split to populate `PromptHash` on conditions for reproducibility tracking (#150). |
| `RunExtraction` | `func RunExtraction(ctx context.Context, client LLMClient, opts ExtractionOptions) ([]schema.TraceDraft, SessionRecord, error)` | Full extraction pipeline: validate opts → generate session ID → load prompt once → loop over opts.InputPaths (one LLM call per document) via `extractSingleDoc` → aggregate drafts → return drafts + SessionRecord. Supports multi-document ingestion (#139). Always returns non-nil SessionRecord. Stamps D2 (ExtractedBy), D3 (UncertaintyNote append), D4 (ExtractionStage "weak-draft"), D6 (SessionRef), D7 (IntentionallyBlank validation). Fails fast on mismatched or empty InputPaths/SourceDocRefs. |
| `ParseSpans` | `func ParseSpans(data []byte) ([]string, error)` | Parse a spans file into a slice of span strings. Tries JSON string array first, then newline-separated text, then single-line. Drops blank/whitespace-only entries. Returns error on empty input. |
| `RunAssistSession` | `func RunAssistSession(ctx context.Context, client LLMClient, spans []string, opts AssistOptions, in io.Reader, out io.Writer) ([]schema.TraceDraft, SessionRecord, error)` | Interactive per-span assist session. For each span: call LLM → parse draft → present to user → read action. Accept: append LLM draft (disposition "accepted"). Edit: RunEditFlow + DeriveEdited → append both (disposition "edited"). Skip: append LLM draft (disposition "skipped"; shadow preserved). EOF or Quit: return partial results without error. Always returns SessionRecord. DraftCount may exceed len(Dispositions) when edits produce two drafts per span. |
| `RunCritique` | `func RunCritique(ctx context.Context, client LLMClient, drafts []schema.TraceDraft, opts CritiqueOptions) ([]schema.TraceDraft, SessionRecord, error)` | LLM critique pipeline: one call per input draft → parse response → validate SourceSpan → stamp DerivedFrom and ExtractionStage "critiqued". Returns partial results with nil error on per-draft failures; errors accumulate in SessionRecord.ErrorNote. Returns non-nil SessionRecord on all code paths except UUID generation failure. DraftID filter (opts.DraftID non-empty) returns error with SessionRecord when ID not found (F.4). |
| `RunSplit` | `func RunSplit(ctx context.Context, client LLMClient, opts SplitOptions) ([]string, SessionRecord, error)` | LLM span-boundary detection pipeline: read source doc → load prompt → call LLM → detect refusal → parse JSON string array → filter blanks → return spans + SessionRecord. Always returns non-nil SessionRecord. `DraftCount` = len(spans); `DraftIDs` = nil. Empty result (0 non-blank spans) is an error. No criterion involvement — boundary detection is pre-analytical (#137). |
| `PromoteSession` | `func PromoteSession(rec SessionRecord, observer string) (schema.Trace, error)` | Converts a SessionRecord to a canonical schema.Trace. Observer is required — no trace without a position. Target is built from `Conditions.SourceDocRefs` (plural, #139) with fallback to legacy `Conditions.SourceDocRef`; multi-doc sessions produce a multi-element Target. WhatChanged surfaces command + all SourceDocRefs + ModelID for self-situated description. Tags: ["session", "articulation"]. Always validates before returning; invalid promoted traces are errors. Error sessions (ErrorNote non-empty) are promotable — a failed act is an observable act. Closes the Principle 8 reflexivity gap: the LLM observation apparatus enters the mesh (#138). |

### Key Design Notes

- **Zero external dependencies**: Uses only Go stdlib (`net/http`, `encoding/json`, `io`, `os`, `context`).
- **SessionRecord is mandatory**: `RunExtraction`, `RunAssistSession`, `RunCritique`, and `RunSplit` always return a SessionRecord. On error, `ErrorNote` is populated. The caller writes the record to disk; the `llm` package does not own the write.
- **Reflexivity via PromoteSession**: `PromoteSession` converts a `SessionRecord` to a canonical `Trace`, making the LLM observation apparatus visible inside the mesh it produced. This closes the Principle 8 gap left by all prior LLM pipeline work. The `"session"` and `"articulation"` tags mark these traces in articulations (#138).
- **F.1 convention enforcement**: D2 (ExtractedBy = model ID string), D3 (frameworkUncertaintyNote always appended), D4 (ExtractionStage = "weak-draft" for extraction, "critiqued" for critique), D6 (SessionRecord mandatory return), D7 (IntentionallyBlank validates against content-field allowlist). See `docs/decisions/llm-as-mediator-v1.md`. `RunSplit` enforces D6 but not D2–D5 (spans are pre-trace material).
- **Client injection**: `RunExtraction`, `RunAssistSession`, `RunCritique`, and `RunSplit` accept `LLMClient` interface — tests inject a mock; production code uses `AnthropicClient`. No live API calls in unit tests.
- **shared.go helpers**: `readSourceDoc` and `isRefusal` are unexported package-internal helpers used by both `extract.go` and `split.go`. They were moved from `extract.go` to `shared.go` when split was added to avoid duplication (#137).
- **Refusal detection**: `isRefusal()` matches conservative English-language prefixes. False negatives fall through to the malformed-output path, which is correct. The heuristic is English-only (documented in F.6 deferred notes).
- **Security**: API key is an unexported struct field; never appears in `SessionRecord`, any error message, or any serialised output. Authentication error responses (401/403) are not echoed to the user. Span text is not echoed in ErrorNote (uses length only) to avoid PII leakage into session files.
- **`llm → review` import**: `assist.go` imports `review` for rendering (`RenderDraft`, `RenderAmbiguities`, `DetectAmbiguities`) and derivation helpers (`RunEditFlow`, `DeriveEdited`). The direction is one-way and stable through F.4 (`review` has no `llm` imports). If a future feature requires `review → llm`, extract shared types into a new package to avoid a cycle.
- **DraftCount vs Dispositions**: In the assist session, `DraftCount` counts all output drafts including edit-derived ones. `Dispositions` has one entry per span processed (not per output draft). When a span is edited, two drafts appear in `DraftIDs` but one disposition entry is recorded. Downstream tools must not assume `DraftCount == len(Dispositions)`.
- **Accept asymmetry**: In the assist session, accepting a span does not produce a derived draft — the LLM draft is the output. In the review session, accepting does produce a derived draft (derived from the reviewed draft). This is intentional: in assist, the LLM's reading stands as the position; in review, the human reviewer's cut is recorded as a new derivation.
- **Critique partial results**: `RunCritique` returns nil error even when all drafts fail — per-draft errors accumulate in `SessionRecord.ErrorNote`. `cmdCritique` returns error when input was non-empty but zero critiques produced (distinguishes total LLM failure from legitimate empty input).
- **SourceSpan integrity**: `RunCritique` hard-rejects any LLM response where SourceSpan differs from the original draft — the source span anchor is immutable across the critique derivation chain.
- **parseCritiqueDraft / parseSingleDraft**: Two parallel parse implementations exist until F.6 refactor deduplicates them into a shared `parse.go` helper.

---

## Package: cmd/meshant

### Files

| File | Contains |
|------|----------|
| `main.go` | CLI entry point: `main()`, `run()` dispatcher, `usage()`, and shared helpers (`loadCriterionFile`, `stringSliceFlag`, `parseTimeFlag`, `parseTimeWindow`, `outputWriter`, `confirmOutput`, `loadTraces`, `noop`). |
| `db_factory.go` | `//go:build !neo4j` — `openDB` stub: returns "rebuild with -tags neo4j" error for any non-empty dbURL. Default binary unchanged. |
| `db_factory_neo4j.go` | `//go:build neo4j` — `openDB` implementation: reads MESHANT_DB_USER/MESHANT_DB_PASS/MESHANT_DB_NAME from env, creates `Neo4jStore`. |
| `cmd_store.go` | `cmdStore` subcommand handler — reads JSON file via `loader.Load`, writes to injected or factory-created `TraceStore`. |
| `cmd_summarize.go` | `cmdSummarize` subcommand handler. |
| `cmd_validate.go` | `cmdValidate` subcommand handler. |
| `cmd_articulate.go` | `cmdArticulate` subcommand handler (`--narrative` flag, `--db` flag). |
| `cmd_diff.go` | `cmdDiff` subcommand handler (`--db` flag). |
| `cmd_follow.go` | `cmdFollow` subcommand handler (`--criterion-file` flag, `--db` flag). |
| `cmd_draft.go` | `cmdDraft` subcommand handler (M11). |
| `cmd_promote.go` | `cmdPromote` subcommand handler (M11). |
| `cmd_rearticulate.go` | `cmdRearticulate` subcommand handler (M12). |
| `cmd_lineage.go` | `cmdLineage` subcommand handler plus 13 exclusive helpers: `lineageNode`, `lineageResult`, `buildLineage`, `detectCycleDFS`, `idPrefix`, `spanPreview`, `printLineageText`, `printLineageStep`, `lineageJSONChain`, `collectMembers`, `printLineageJSON`, `filterLineageByID`, `chainContainsID` (M12). |
| `cmd_shadow.go` | `cmdShadow` subcommand handler (`--db` flag) (M13). |
| `cmd_gaps.go` | `cmdGaps` subcommand handler (`--suggest` flag, B.2; `--db` flag) (M13). |
| `cmd_bottleneck.go` | `cmdBottleneck` subcommand handler (`--db` flag) (B.1). |
| `cmd_review.go` | `cmdReview` subcommand handler — only interactive subcommand; accepts `in io.Reader` (A.5). |
| `cmd_extraction_gap.go` | `cmdExtractionGap` subcommand handler (C.2). |
| `cmd_chain_diff.go` | `cmdChainDiff` subcommand handler (C.3). |
| `cmd_extract.go` | `cmdExtract` subcommand handler — `--source-doc` and `--source-doc-ref` are repeatable flags (multi-doc ingestion, #139) via `stringSlice` flag type; `--adapter` flag enables format-conversion before extraction (#140); adapter validated early (before any LLM call); converted text written to OS temp files (deferred removal); calls LLM via injected `llm.LLMClient`; writes TraceDraft JSON and SessionRecord JSON (0o600); session output defaulting: `<output>.session.json` (file mode) or `session_<timestamp>.json` in cwd (stdout mode) (F.2). Also houses `writeSessionRecord` shared by all LLM subcommands. |
| `cmd_convert.go` | `cmdConvert` subcommand handler — `meshant convert` two-step workflow entry: convert non-text source to plain text, write to stdout or `--output`, print confirmation with adapter name and metadata. Flags: `--adapter` (required), `--source-doc` (required), `--output` (optional) (#140). |
| `cmd_assist.go` | `cmdAssist` subcommand handler — interactive per-span LLM-assist session; reads spans file (capped at 4 MiB); accepts injected `llm.LLMClient`; reads user decisions from injected `io.Reader`; writes TraceDraft JSON and SessionRecord JSON on every code path (F.3). |
| `cmd_critique.go` | `cmdCritique` subcommand handler — calls LLM via injected `llm.LLMClient`; reads input drafts file (capped at 4 MiB); writes critiqued TraceDraft JSON and SessionRecord JSON; session output defaulting: `<output>.session.json` (file mode) or not written (stdout mode, unless `--session-output` explicit) (F.4). |
| `cmd_split.go` | `cmdSplit` subcommand handler — calls LLM via injected `llm.LLMClient`; writes candidate observation spans as JSON string array and SessionRecord JSON on every code path; session output defaulting mirrors extract; no `--criterion-file` (split is boundary detection only) (#137). |
| `cmd_promote_session.go` | `cmdPromoteSession` subcommand handler — reads SessionRecord from `--session-file` (JSON, capped at 1 MiB, no DisallowUnknownFields for forward compatibility); requires `--observer`; calls `llm.PromoteSession`; writes single-element `[]schema.Trace` JSON array to `--output` or stdout. Private helper: `readSessionFile` (#138). |
| `main_test.go` | Tests: all subcommands, flag parsing, file output, error handling, criterion file loading, draft/promote pipeline (M11). |
| `cmd_assist_test.go` | Tests: `cmdAssist` CLI handler — happy path (field assertions on ExtractionStage/ExtractedBy/UncertaintyNote), missing flags, missing file, session file written, quit-outputs-partial-results, LLM error, malformed spans, size cap (F.3). |
| `cmd_critique_test.go` | Tests: `cmdCritique` happy path, missing input, missing file, session file written, LLM error, ID filter, ID filter not found, malformed JSON input, empty array input, stdout mode, session output confirmation (F.4). |
| `cmd_split_test.go` | Tests: `cmdSplit` — missing source-doc, success (JSON array + session file), LLM error with session written, refusal, output file, default/explicit session path, --help flag, file-not-found, malformed output, stdout output (#137). |

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
| `run` | `func run(w io.Writer, args []string) error` | Command dispatcher. Routes to `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, `cmdDiff()`, `cmdFollow()`, `cmdDraft()`, `cmdPromote()`, `cmdRearticulate()`, `cmdLineage()`, `cmdShadow()`, `cmdGaps()`, `cmdBottleneck()`, `cmdExtractionGap()`, `cmdChainDiff()`, `cmdReview()`, `cmdExtract()`, `cmdAssist()`, `cmdCritique()`, `cmdSplit()`, `cmdPromoteSession()`, `cmdConvert()`, or `cmdStore()`. For `review` and `assist`, passes `os.Stdin`; for `extract`, `assist`, `critique`, `split`, passes `nil` client; for `store`, passes `nil` TraceStore (real store created from --db). |
| `loadTraces` | `func loadTraces(ctx context.Context, dbURL string, fileArgs []string) ([]schema.Trace, func(), error)` | Resolve traces from either a Neo4j database (dbURL non-empty → `openDB` + `Query(ctx, QueryOpts{})`) or a JSON file (dbURL empty → `loader.Load(fileArgs[0])`). Returns traces, a cleanup function (defer it to close the store), and any error. No pre-filtering — full substrate returned to analytical engine. Callers validate mutual exclusion before calling. |
| `openDB` | `func openDB(ctx context.Context, dbURL string) (store.TraceStore, error)` | Factory: in `!neo4j` builds, returns "rebuild with -tags neo4j" error. In `neo4j` builds, reads MESHANT_DB_USER/MESHANT_DB_PASS/MESHANT_DB_NAME from env and returns a `Neo4jStore`. |
| `cmdStore` | `func cmdStore(w io.Writer, ts store.TraceStore, args []string) error` | Subcommand: Load canonical Traces from JSON file, write to `TraceStore` via `Store()`. `ts` may be nil (openDB called from --db flag) or injected (test). Reports stored count. Idempotent on trace ID. |
| `cmdSummarize` | `func cmdSummarize(w io.Writer, args []string) error` | Subcommand: Load traces, compute mesh summary, print via `loader.PrintSummary()`. Usage: `meshant summarize <file>`. |
| `cmdValidate` | `func cmdValidate(w io.Writer, args []string) error` | Subcommand: Load and validate traces. Reports success message or errors. Usage: `meshant validate <file>`. |
| `cmdArticulate` | `func cmdArticulate(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut with `--observer` (repeatable), `--tag` (repeatable, any-match), `--from`, `--to` (RFC3339), `--format text\|json\|dot\|mermaid`, `--output <file>`. Optional `--narrative` flag appends a positioned narrative draft (text format only, skipped for JSON/DOT/Mermaid). |
| `cmdDiff` | `func cmdDiff(w io.Writer, args []string) error` | Subcommand: Load traces, articulate two cuts (`--observer-a/b`, `--tag-a/b`, per-side time windows), compute diff via `graph.Diff()`. `--format text\|json\|dot\|mermaid`, `--output <file>`. |
| `cmdFollow` | `func cmdFollow(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut, follow translation chain from --element with `--direction forward\|backward`, `--depth N`, `--observer`, `--tag`, `--from`, `--to`. Optional `--criterion-file <path>` loads an EquivalenceCriterion before trace I/O. Classify and print via `PrintChain()`. `--format text\|json`, `--output <file>`. |
| `cmdDraft` | `func cmdDraft(w io.Writer, args []string) error` | Subcommand: Load extraction JSON, assign UUIDs/timestamps, apply optional `--source-doc`, `--extracted-by`, `--stage` overrides, write TraceDraft JSON via `loader.LoadDrafts()`, print provenance summary via `PrintDraftSummary()`. `--output <file>` (M11). |
| `cmdPromote` | `func cmdPromote(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON via `loader.LoadDrafts()`, call `IsPromotable()` on each, promote qualifying drafts to canonical Traces (each tagged with `TagValueDraft`), write promoted Trace JSON, report promotion summary naming failures (M11). `--output <file>`. |
| `cmdRearticulate` | `func cmdRearticulate(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON, produce skeleton JSON array — for each draft: `source_span` copied verbatim, `derived_from` set to original ID, all content fields blank, `extraction_stage:"reviewed"`. Flags: `--id <id>` (single draft only), `--output <file>` (M12). |
| `cmdLineage` | `func cmdLineage(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON, walk DerivedFrom links, render positional reading sequences as indented trees. Anchors (drafts with no DerivedFrom) are sequence roots. Cycle detection required (DFS grey-set). Flags: `--id <id>` (single chain), `--format text\|json` (M12). |
| `cmdShadow` | `func cmdShadow(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut, print shadow summary via `graph.SummariseShadow()` + `PrintShadowSummary()`. Flags: `--observer` (repeatable, required), `--tag` (repeatable), `--from`, `--to` (RFC3339), `--output <file>` (M13). |
| `cmdGaps` | `func cmdGaps(w io.Writer, args []string) error` | Subcommand: Load traces, articulate two cuts, compare node sets via `graph.AnalyseGaps()`, print gap report. Optionally appends re-articulation suggestions via `--suggest`. Flags: `--observer-a`, `--observer-b` (required), per-side `--tag-a/b`, `--from-a/b`, `--to-a/b`, `--suggest` (bool), `--output` (M13, B.2). |
| `cmdBottleneck` | `func cmdBottleneck(w io.Writer, args []string) error` | Subcommand: Load traces, articulate a cut, identify bottlenecks via `graph.IdentifyBottlenecks()`, print notes via `PrintBottleneckNotes()`. Flags: `--observer` (optional), `--tag`, `--from`, `--to`, `--output` (B.1). |
| `cmdExtractionGap` | `func cmdExtractionGap(w io.Writer, args []string) error` | Subcommand: Load two TraceDraft JSON files, compare extractions from named positions via `loader.CompareExtractions()`, print gap report via `PrintExtractionGap()`. Flags: `--analyst-a <label>`, `--analyst-b <label>` (both required, names of analyst positions), `--output <file>` (C.2). |
| `cmdChainDiff` | `func cmdChainDiff(w io.Writer, args []string) error` | Subcommand: Load TraceDraft JSON, build span-scoped derivation chains for two named analyst positions via `loader.FollowDraftChain()` + `loader.ClassifyDraftChain()`, compare classifications via `loader.CompareChainClassifications()`, print diff report via `PrintClassificationDiffs()`. Flags: `--analyst-a <label>`, `--analyst-b <label>` (both required), `--span <source_span>` (required), `--output <file>` (C.3). |
| `cmdReview` | `func cmdReview(w io.Writer, in io.Reader, args []string) error` | Subcommand: Load TraceDraft JSON, run interactive accept/edit/skip/quit session via `review.RunReviewSession()`. Only interactive subcommand — signature diverges from all other `cmd*` functions by accepting `in io.Reader` for stdin injection (testability). Interactive prompts go to `os.Stderr`; JSON output and summary go to `w`. Flags: `--output <file>` (A.5). |
| `cmdExtract` | `func cmdExtract(w io.Writer, client llm.LLMClient, args []string) error` | Subcommand: Call LLM to produce TraceDraft records from a source document. Accepts an injected `llm.LLMClient` (nil = construct `AnthropicClient` from env). Writes TraceDraft JSON array and a `SessionRecord` JSON file on every code path (success and error). Session output defaulting: `<output>.session.json` when `--output` is a file; `session_<timestamp>.json` in cwd when stdout. Flags: `--source-doc <path>` (required), `--source-doc-ref <ref>`, `--prompt-template <path>` (default: `data/prompts/extraction_pass.md`), `--model <id>` (default: `claude-sonnet-4-6`), `--criterion-file <path>`, `--output <file>`, `--session-output <file>` (F.2). |
| `cmdAssist` | `func cmdAssist(w io.Writer, client llm.LLMClient, in io.Reader, args []string) error` | Subcommand: Read spans file, call LLM once per span, present candidate draft to user for accept/edit/skip/quit decisions via `llm.RunAssistSession()`. Accepts injected `llm.LLMClient` (nil = construct `AnthropicClient` from env) and `in io.Reader` (testability). Always writes SessionRecord on every code path. Flags: `--spans-file <path>` (required), `--prompt-template <path>`, `--model <id>`, `--source-doc-ref <ref>`, `--criterion-file <path>`, `--output <file>`, `--session-output <file>` (F.3). |
| `cmdCritique` | `func cmdCritique(w io.Writer, client llm.LLMClient, args []string) error` | Subcommand: Call LLM to produce "critiqued" derived drafts from existing TraceDrafts. Reads input drafts file (capped at 4 MiB). Accepts an injected `llm.LLMClient` (nil = construct `AnthropicClient` from env). Writes critiqued TraceDraft JSON array and a `SessionRecord` JSON file. Session output defaulting: `<output>.session.json` when `--output` is a file; not written when stdout unless `--session-output` is explicit. Returns error when input non-empty but zero critiques produced. Flags: `--input <path>` (required), `--prompt-template <path>` (default: `data/prompts/critique_pass.md`), `--model <id>` (default: `claude-sonnet-4-6`), `--source-doc-ref <ref>`, `--criterion-file <path>`, `--output <file>`, `--session-output <file>`, `--id <id>` (F.4). |
| `readSpansFile` | `func readSpansFile(path string) ([]byte, error)` | Read spans file capped at 4 MiB via `io.LimitReader`. Returns error if file exceeds cap (n > maxSpansBytes). Used only by `cmdAssist`. |
| `cmdSplit` | `func cmdSplit(w io.Writer, client llm.LLMClient, args []string) error` | Subcommand: Call LLM to split source document into candidate observation spans. Accepts injected `llm.LLMClient` (nil = construct `AnthropicClient` from env). Writes spans as JSON string array and SessionRecord JSON on every code path. Prints "proposed N candidate observation spans" summary. Session output defaulting mirrors extract. No `--criterion-file` flag. Flags: `--source-doc <path>` (required), `--source-doc-ref <ref>`, `--prompt-template <path>` (default: `data/prompts/split_pass.md`), `--model <id>` (default: `claude-sonnet-4-6`), `--output <file>`, `--session-output <file>` (#137). |
| `writeSessionRecord` | `func writeSessionRecord(path string, rec llm.SessionRecord) error` | Serialise SessionRecord as indented JSON to path (permissions 0o600 — owner read/write only). Encodes to buffer first; creates file only on success (avoids empty file on encode failure). Used by `cmdExtract`, `cmdAssist`, `cmdCritique`, and `cmdSplit`. |
| `loadCriterionFile` | `func loadCriterionFile(path string) (graph.EquivalenceCriterion, error)` | Load, decode, and validate an EquivalenceCriterion from a JSON file. Uses `DisallowUnknownFields()` for precision. Zero-value criterion is a hard error. Returns validated criterion or descriptive error. |
| `outputWriter` | `func outputWriter(w io.Writer, outputPath string) (io.Writer, error)` | Return file writer if `--output` is set, otherwise stdout. |
| `confirmOutput` | `func confirmOutput(w io.Writer, outputPath string) error` | Print "wrote <path>" confirmation to stdout when file output is used. |
| `usage` | `func usage()` | Print CLI usage message listing all subcommands and flags. |

### Key Design Notes

- **External dependencies**: `cmd/meshant` imports `adapter` which uses `github.com/ledongthuc/pdf` (pure Go, no CGo) and `golang.org/x/net/html` for format-conversion. All other packages use stdlib only.
- **Testable structure**: Core logic in `run()`, `cmdSummarize()`, `cmdValidate()`, `cmdArticulate()`, `cmdDiff()`; `main()` is thin wrapper that wires os.Stdout/os.Args and exits non-zero on error
- **Flag parsing**: Uses stdlib `flag.FlagSet` for subcommand isolation; `stringSliceFlag` enables repeatable `--observer` flags without comma-separation
- **DB backend**: `--db` flag on analytical commands + `meshant store` switches the trace source from JSON to Neo4j. The `loadTraces` helper centralises the DB/file branching. Build-tag factory (`db_factory.go`/`db_factory_neo4j.go`) keeps the default binary lean. Credentials never exposed as CLI flags — read from MESHANT_DB_USER/MESHANT_DB_PASS env vars.
- **No pre-filtering via TraceStore.Query**: `loadTraces` passes `QueryOpts{}` (no filters) so the analytical engine receives the full substrate. Cut logic (observer, time window, tags) remains exclusively in `graph.Articulate` — see decision record `store-cli-v1.md` for rationale.
- **Time handling**: RFC3339 timestamps throughout; `parseTimeFlag()` and `parseTimeWindow()` provide clear error messages with formatting hints
- **Format options**: `articulate` and `diff` both support text/json/dot/mermaid; `follow` supports text/json
- **File output**: `--output <file>` writes to file instead of stdout; `cmdReview` uses explicit `f.Close()` (not deferred) to surface write errors
- **Interactive subcommands**: `cmdReview` and `cmdAssist` both accept `in io.Reader` for stdin injection (testability). Both write interactive prompts to `os.Stderr` and JSON output to `w` (stdout). `run()` passes `os.Stdin` for both. All other subcommands are non-interactive.
- **LLM-injected subcommands**: `cmdExtract`, `cmdAssist`, `cmdCritique`, and `cmdSplit` all accept `llm.LLMClient` as a parameter (nil = real client from env). This is the same injection pattern as `in io.Reader` — enables testing without live API calls. `run()` passes `nil` for all; tests pass a mock (F.2, F.3, F.4, #137).
- **Session file permissions**: `writeSessionRecord` creates files with `0o600` (owner read/write only). Session records may contain prompt template text; world-readable permissions are inappropriate.
- **Stderr/stdout separation**: `cmdReview` writes interactive prompts to `os.Stderr` and JSON output to `w` (stdout), enabling `meshant review file.json | jq .` and `--output` file routing without prompt contamination.
- **Ingestion pipeline** (M11): `draft` command ingests LLM extraction JSON and produces TraceDraft records; `promote` command converts promotable TraceDraft records to canonical Traces (tagged with `draft` provenance signal)
- **Re-articulation pipeline** (M12): `rearticulate` command produces critique skeletons (SourceSpan + DerivedFrom set, all content fields blank); `lineage` command walks DerivedFrom links and renders positional reading sequences as CLI output
- **Shadow analysis** (M13): `shadow` subcommand summarises shadowed elements from a cut; `gaps` subcommand compares element visibility between two observer positions; neither position is authoritative
- **Re-articulation suggestions** (B.2): optional `--suggest` flag on `gaps` subcommand proposes cut adjustments (observer expansion, time-window expansion, tag relaxation) to narrow an observed gap
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

llm/
  ├─→ imports: schema (TraceDraft), loader (NewUUID), review (DeriveAccepted, DeriveEdited, RunEditFlow — one-directional; review has no llm imports)
  └─→ (used by) cmd/meshant

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

### LLM Extraction Pipeline (F.2)
1. `cmdExtract` validates flags; resolves `sessionOutputPath` (defaulting rules)
2. `llm.RunExtraction(ctx, client, opts)` — called with injected or real client
3. Inside: read source doc (capped at 1 MiB) → load prompt template → call `client.Complete()`
4. Detect refusal via prefix heuristic → parse JSON array (strips LLM preamble)
5. Per draft: validate `IntentionallyBlank`, assign UUID, stamp D2/D3/D4/D6/D7 fields
6. Return `[]TraceDraft + SessionRecord` (always non-nil, even on error)
7. `cmdExtract` writes SessionRecord JSON first (always), then TraceDraft JSON, then summary
8. On error: SessionRecord with ErrorNote is on disk; extraction error returned to caller

### LLM Assist Pipeline (F.3)
1. `cmdAssist` reads spans file (capped at 4 MiB via `readSpansFile`); calls `llm.ParseSpans()` to parse JSON array or newline-separated format; drops blank/whitespace-only entries
2. `llm.RunAssistSession(ctx, client, spans, opts, in, out)` opens interactive session loop
3. For each span: call `client.Complete()` → `parseSingleDraft()` → validate `IntentionallyBlank`; zero `DerivedFrom` (prevents false chain links); stamp D2/D3/D4/D6 fields
4. Present draft to user; read decision: `a` (accept), `e` (edit via `review.RunEditFlow`), `s` (skip — preserves draft), `q` (quit)
5. Accept: appends LLM draft with `DraftDisposition{Action:"accepted"}` — no new derived draft (distinct from `meshant review`)
6. Edit: writes LLM draft + derived-from draft via `review.DeriveEdited()`; disposition `"edited"`
7. Skip: appends LLM draft with `DraftDisposition{Action:"skipped"}` — shadow is not absence
8. EOF during edit: appends LLM draft with `DraftDisposition{Action:"abandoned"}` — interrupted articulation, not deliberate skip
9. Quit: writes partial results and returns without error
10. Return `[]TraceDraft + SessionRecord` (always non-nil, even on error); `DraftCount` = total output drafts (includes edit-derived); `Dispositions` has one entry per span (not per draft — documented asymmetry)
11. `cmdAssist` writes SessionRecord JSON (always), then TraceDraft JSON, then summary

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
| Traverse DerivedFrom chain programmatically | `loader/draftchain.go` → `FollowDraftChain()` |
| Classify draft chain steps | `loader/draftchain.go` → `ClassifyDraftChain()` |
| Summarise shadow elements from a graph | `graph/shadow.go` → `SummariseShadow()` |
| Print shadow summary | `graph/shadow.go` → `PrintShadowSummary()` |
| Compare two graph node sets | `graph/gaps.go` → `AnalyseGaps()` |
| Print observer-gap report | `graph/gaps.go` → `PrintObserverGap()` |
| Identify bottleneck elements | `graph/bottleneck.go` → `IdentifyBottlenecks()` |
| Print bottleneck analysis | `graph/bottleneck.go` → `PrintBottleneckNotes()` |
| Shadow summary via CLI | `cmd/meshant/main.go` → `cmdShadow()` |
| Observer-gap report via CLI | `cmd/meshant/main.go` → `cmdGaps()` |
| Bottleneck analysis via CLI | `cmd/meshant/main.go` → `cmdBottleneck()` |
| Suggest re-articulations from gap | `graph/suggest.go` → `SuggestRearticulations()` |
| Print re-articulation suggestions | `graph/suggest.go` → `PrintRearticSuggestions()` |
| Re-articulation suggestions via CLI | `cmd/meshant/main.go` → `cmdGaps()` with `--suggest` flag |
| Draft narrative reading of a graph | `graph/narrative.go` → `DraftNarrative()` |
| Print narrative draft | `graph/narrative.go` → `PrintNarrativeDraft()` |
| Add narrative draft to articulation output | `cmd/meshant/main.go` → `cmdArticulate()` with `--narrative` flag |
| Compare chain classifications from two analysts | `loader/classdiff.go` → `CompareChainClassifications()` |
| Print classification diff report | `loader/classdiff.go` → `PrintClassificationDiffs()` |
| Classification diff via CLI | `cmd/meshant/main.go` → `cmdChainDiff()` |
| Run interactive per-span LLM-assist session | `llm/assist.go` → `RunAssistSession()` |
| Parse spans from file (JSON array or newline-separated) | `llm/assist.go` → `ParseSpans()` |
| Interactive assist session via CLI | `cmd/meshant/cmd_assist.go` → `cmdAssist()` |
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

### Shadow Analysis (M13)
- **Shadow is a cut decision**: `SummariseShadow` reads `Cut.ShadowElements` (already computed by Articulate); "shadowed" not "missing"
- **ObserverGap is composable**: `AnalyseGaps` takes two pre-articulated `MeshGraph` values; does not re-articulate
- **Both cuts retained**: `ObserverGap.CutA` and `CutB` preserved so any report is self-situated
- **FollowDraftChain mirrors FollowTranslation**: same first-match, cycle-detection, and empty-if-not-found semantics
- **CriterionRef is string-only**: stores criterion `Name` (not the struct) to prevent `schema`→`graph` import cycle
- **DraftStepKind is v1 provisional**: mirrors StepKind; will be revisited when criteria govern draft chains

### Narrative Drafts (B.3)
- **Positioned reading, not conclusion**: DraftNarrative produces a provisional narrative from one observer cut; it organizes what the graph already contains (trace counts, mediations, shadow elements) into readable form
- **Top elements by frequency**: Selects top-3 nodes by AppearanceCount (descending), then alphabetically; lists fewer when graph is smaller
- **Mediation enumeration**: Collects up to 5 distinct non-empty Edge.Mediation strings in encounter order; appends "and N more" if more exist
- **Shadow language discipline**: Never uses "missing" (absence implies non-existence); always uses "in shadow from this position" (names what is invisible from one vantage point)
- **Mandatory caveats**: Always includes standard positioned-reading caveat; adds shadow-ratio, time-window, and tag-filter caveats based on cut parameters
- **Immutability**: NarrativeDraft fields are populated once; callers should not mutate Caveats slice
- **Empty graphs**: Returns zero-value NarrativeDraft when graph has no edges (no data to narrate)
- **CLI integration**: `--narrative` flag on `cmdArticulate` (text format only; skipped for JSON/DOT/Mermaid)

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

### Multi-Analyst Ingestion Datasets (C.4)

| Dataset | Location | Stage | Purpose |
|---------|----------|-------|---------|
| Multi-Analyst Drafts | `data/examples/multi_analyst_drafts.json` | Output | 10 TraceDraft records from two analyst positions (`analyst-a`, `analyst-b`) over a shared incident scenario; exercises `extraction-gap` and `chain-diff` operations |
| Multi-Analyst README | `data/examples/multi_analyst_drafts_README.md` | Documentation | Companion guide: scenario description, deliberate divergences between analyst positions, CLI commands to reproduce analysis output |

### Thread D Reference Datasets (Phase 5 — #147, #148, #149)

Hand-authored trace datasets covering three domains. Each exercises multi-observer articulation, mediation/intermediary distinction, and reflexive session traces. Validated with `meshant validate`. All reviews: ant-theorist ALIGNED, QA PASS, architect SHIP.

| Dataset | Location | Traces | Observers | Domain |
|---------|----------|--------|-----------|--------|
| D.1 Software Incident | `data/examples/software_incident.json` | 32 | `on-call-engineer`, `product-manager`, `customer-support-lead`, `dataset-analyst` | Payment service degradation during flash sale; `retry-buffer` as key mediator |
| D.2 Multi-Agent Pipeline | `data/examples/multi_agent_pipeline.json` | 28 | `pipeline-auditor`, `ml-engineer`, `dataset-analyst` | AI compliance pipeline; 8 agents as non-human actants; inscription conflict (taxonomy version mismatch) |
| D.3 Policy/Procurement | `data/examples/policy_procurement.json` | 27 | `procurement-officer`, `budget-approver`, `vendor-alpha`, `compliance-auditor`, `dataset-curator` | Public-sector IT procurement; 17 institutional actants; apparatus-thick, humans-thin |

All three datasets end with a structurally parallel reflexive pair: `source: ["claude-sonnet-4-6"]`, `mediation: "extract"/"critique"`, `tags: ["session", "articulation"]` — the observation apparatus recorded as a trace (Principle 8).

### Extraction Prompt Templates

| File | Purpose |
|------|---------|
| `data/prompts/critique_pass.md` | Extraction contract for the critique (re-articulation) step: what to preserve (SourceSpan verbatim), what to question (stable actor attributions, imputed intentions), what honest abstention looks like, DerivedFrom semantics, worked E3 example. Updated F.1: `extraction_stage: "critiqued"`, model ID strings for `extracted_by`, `intentionally_blank` in field guide. |
| `data/prompts/extraction_pass.md` | System instructions for `meshant extract` (Thread F). Enforces trace-first vocabulary: candidate drafts not facts, `intentionally_blank` requirement, honest abstention, worked network-operations example (F.1). |

**Dataset M8 (Incident Response):**
- **Observers:** monitoring-service, on-call-engineer, incident-commander, product-manager, customer-support
- **Actants:** alerting-pipeline, auto-scaler, circuit-breaker, sla-timer, runbook-engine, dashboard-service, connection-pool-monitor, pagerduty-webhook
- **Trace Stats:** 22 traces, 86% mediated, all 6 tag types represented, 1 graph-ref, 1 absent-source
- **Use Case:** Incident lifecycle (detection to postmortem); demonstrates observer positioning across operational roles

## Related Decision Records and Guides

- `docs/decisions/trace-schema-v2.md` — core Trace type rationale
- `docs/decisions/articulation-v2.md` — observer position and shadow design
- `docs/decisions/time-window-v1.md` — temporal filtering
- `docs/decisions/graph-diff-v2.md` — diff computation and shadow shifts
- `docs/decisions/graph-as-actor-v2.md` — identified graphs as actants
- `docs/decisions/m7-serialisation-reflexivity-v1.md` — TimeWindow JSON codec and reflexive tracing
- `docs/decisions/structured-export-v1.md` — graph export to JSON, DOT, Mermaid formats
- `docs/decisions/cli-v2.md` — CLI design decisions (M9)
- `docs/decisions/m10-tag-filter-diff-export-cli-v1.md` — Tag-filter axis, diff visual export, CLI integration (M10)
- `docs/decisions/translation-chain-v2.md` — Translation chain traversal, classification heuristics, first-match branching (M10.5)
- `docs/decisions/equivalence-criterion-v1.md` — Equivalence criterion design, three-layer model, v1 implicit criterion, second-order shadow (M10.5+)
- `docs/decisions/tracedraft-v2.md` — TraceDraft design, ingestion pipeline as analytical object, source span as anchor text, promotion criterion, provenance chain (M11)
- `docs/decisions/rearticulation-v1.md` — Re-articulation as cut not correction, SourceSpan invariant, blank scaffold as correct output, DerivedFrom positional vocabulary, cmdLineage as first-class CLI output, E3/E14 as demonstration material (M12)
- `docs/decisions/shadow-analysis-v1.md` — Shadow as cut decision, ObserverGap composability, FollowDraftChain design, CriterionRef as citation metadata, DraftStepKind v1 heuristics, shadow/gaps CLI-first design (M13)
- `docs/decisions/serve-v1.md` — HTTP server design for interactive trace graph querying: observer required on all endpoints (enforces Principle 3), Envelope response structure with CutMeta echo, four endpoints (/articulate, /diff, /shadow, /traces), error responses with ANT reasoning, stateless handlers, no silent cuts, graceful shutdown, Phase 4 (#145)
- `docs/decisions/interactive-review-v1.md` — Interactive review CLI design: session as cut, render-as-string, ExtractedBy sameness, provenance/content partition, stdin/stderr separation, main.go size debt (Thread A)
- `docs/decisions/llm-as-mediator-v1.md` — 7 conventions for LLM participation in the ingestion pipeline: mediator framing, model ID strings, framework-imposed UncertaintyNote, ExtractionStage values (incl. "critiqued"), SessionRecord mandate, IntentionallyBlank requirement; 3 named ANT tensions (F.1)
- `docs/decisions/llm-boundary-v2.md` — 9 implementation decisions for Thread F: llm package boundary, LLMClient interface, SessionRecord mandate, "critiqued" semantics, span splitting deferred, API key from env, no retry, main.go file split, no ExtractionCut type; 5 named ANT tensions; deferred items (F.6)
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
- `store/store_test.go` — integration and unit tests for TraceStore interface, JSONFileStore, and Neo4j adapter (build tag `neo4j`); atomic write verification, QueryOpts filtering
- `serve/handlers_test.go` — 82.3% coverage; black-box HTTP tests via `httptest.NewRecorder` and `httptest.NewRequest`; all four endpoints (/articulate, /diff, /shadow, /traces), observer validation, time-window parsing, tag filtering, limit application, error responses
- `cmd/demo/main_test.go` — E2E test
- `graph/shadow_test.go` — 10 tests for SummariseShadow and PrintShadowSummary (M13)
- `graph/gaps_test.go` — 9 tests for AnalyseGaps and PrintObserverGap (M13)
- `graph/narrative_test.go` — 11 tests for DraftNarrative and PrintNarrativeDraft (B.3); covers empty graphs, element sorting, mediation enumeration, shadow statements, caveat triggering (shadow ratio, time window, tag filters)
- `loader/draftchain_test.go` — 11 tests for FollowDraftChain and ClassifyDraftChain (M13)
- `schema/tracedraft_test.go` — includes tests for CriterionRef (M13) and IntentionallyBlank (M12.5)
- `cmd/meshant/main_test.go` — tests covering all CLI subcommands including follow, draft, promote (M11), rearticulate, lineage (M12), shadow, gaps (M13), narrative flag on articulate (B.3), flag parsing, file output, error handling
