# MeshAnt — Task Tracking

## Language and form factor

- **Go** — primary implementation language (trace schema, loader, CLI, pipeline work)
- **Python** — LLM API integration and reference code only (when/if needed)
- **JSON/YAML** — trace data format (language-agnostic, inspectable)

## Branch strategy

- `main` — stable releases only
- `develop` — active development branch
- Feature branches from `develop`, PRs target `develop`

---

## Completed Milestones (M1–M13)

All complete and merged to `develop` / `main`. Detailed plans in `tasks/done/`.

| Milestone | Summary | Plan |
|-----------|---------|------|
| M1 | Trace schema (`Trace`, `Validate()`), minimal loader, example dataset | `done/plan_m1_*.md` |
| M2 | Deforestation dataset; `Articulate()`, `MeshGraph`, shadow mandatory | `done/plan_m2.md` |
| M3 | Longitudinal dataset; `TimeWindow` cut axis, `ShadowReason` per element | `done/plan_m3.md` |
| M4 | `Diff()`, `GraphDiff`, `ShadowShift`; situated comparison of two cuts | `done/plan_m4.md` |
| M5 | Graph-as-actor: `IdentifyGraph`, `GraphRef`; reflexivity (Principle 8 partial) | `done/plan_m5.md` |
| M6 | Minimal demo (coastal evacuation dataset); Docker; release v0.2.0 | `done/plan_m6.md` |
| M7 | JSON codec; reflexive tracing (`ArticulationTrace`, `DiffTrace`); release v0.3.0 | `done/plan_m7.md` |
| M8 | DOT/Mermaid/JSON export; persist package; incident response dataset; release v0.3.1 | — |
| M9 | CLI (`meshant`); `summarize`, `validate`, `articulate`, `diff`; authoring guide; release v1.0.0 | `done/plan_m9.md` |
| M10 | Tag-filter cut axis; `PrintDiffDOT`/`PrintDiffMermaid`; `--output`, `--tag` flags | — |
| M10.5 | Translation chain traversal; `ClassifyChain`; `FollowTranslation`; `meshant follow` | `done/plan_m10_5.md` |
| M10.5+ | `EquivalenceCriterion`; `--criterion-file` flag; classification with grounds | `done/plan_m10_5_plus.md` |
| M11 | `TraceDraft` schema; `meshant draft` + `meshant promote`; provenance-first ingestion | `done/plan_m11.md` |
| M12 | Re-articulation as second cut; `meshant rearticulate` + `meshant lineage`; critique template | `done/plan_m12.md` |
| M12.5 | `IntentionallyBlank` on `TraceDraft`; named absence in critique skeletons | — |
| M13 | `ShadowSummary`, `ObserverGapReport`; `meshant shadow`/`gaps`; `FollowDraftChain`; `CriterionRef` | — |

**Decision records:** `docs/decisions/` — one per milestone (trace-schema through shadow-analysis).

---

## Completed Threads (v2.0.0) — COMPLETE (2026-03-22)

v2.0.0 tagged on `main`. Full plan: `tasks/done/plan_v2_roadmap.md`.
Detailed plans: `tasks/done/plan_thread_{a,b,f}.md`.

| Thread | Summary |
|--------|---------|
| A | Interactive review CLI: `meshant review`; accept/edit/skip loop; ambiguity detection; provenance chain rendering |
| B | Interpretive outputs: `meshant bottleneck`, `meshant narrative`, `--suggest` on `meshant gaps` |
| C | Multi-analyst ingestion comparison: `meshant extraction-gap`, `meshant chain-diff`; multi-analyst example dataset |
| F | LLM-internal boundary: `meshant extract`, `meshant assist`, `meshant critique`; `SessionRecord`; `LLMClient` interface |

---

## Post-v2.0.0 — ANT-like Knowledge Graph (v3.0.0) — COMPLETE (2026-03-24)

v3.0.0 tagged on `main`. Full plan: `tasks/done/plan_post_v2.md`.
Per-thread pipeline complete: refactor-clean + ant-theorist ALIGNED across all phases.

| Phase | Summary |
|-------|---------|
| 1 (#132) | #137 `meshant split`, #138 session→traces, #139 multi-doc ingestion, #140 non-text adapters |
| 2 (#133) | #141 KG scoping document (`kg-scoping-v1.md`) |
| 3 (#134) | #142 `TraceStore` interface + `JSONFileStore`, #143 Neo4j adapter, #144 `meshant store` + `--db` flag |
| 4 (#135) | #145 `meshant serve` (4 HTTP endpoints), #146 Web UI + provenance panel (Cytoscape.js) |
| 5 (#136) | #147 D.1 software incident (32 traces), #148 D.2 multi-agent pipeline (28 traces), #149 D.3 policy/procurement (27 traces) |

Deferred items resolved (v3.1.0, 2026-03-25): #95 `ClassifyDraftChainOptions`, #96 `DraftSubKindEndorsement`, #150 `PromptHash`, #151 `CritiqueConditions`. One open deferred: `buildChain` closure extraction (candidate if second consumer appears).

---

## Post-v3 — MCP → Interactive CLI → Actors Act

**Detailed plans:** `tasks/plan_v4_mcp.md`, `tasks/plan_v4_explore.md`, `tasks/plan_v5_actors.md`
**Rough plan (archived):** `tasks/done/plan_post_v3.md`
**Status:** Issues open (2026-03-25)

| Version | Direction |
|---------|-----------|
| v4.0.0 | `meshant mcp` — analytical commands as MCP tools; observer-position enforced at schema level |
| v4.x | `meshant explore` — interactive analysis session; LLM suggests, analyst cuts |
| v4.x | Web UI time series controls — time window picker/slider; backend already supports `?from`/`?to` |
| v5.0.0 | Actors act — emerged actors generate new traces, constrained by relational history |

### v4.0.0 — MCP server (parent: #171)

- [x] **#174 — CutMeta/Envelope extraction** — move `CutMeta`/`Envelope`/`cutMetaFromGraph` from `serve/response.go` → `graph/envelope.go`; add `Analyst` field; shared prereq for MCP + explore
- [x] **#175 — mcp-v1.md decision record** (ANT gate) — two-level observer model, tool set rationale, invocation traces, SSE deferred; `--observer`→`--analyst` rename; T171.1–T171.5
- [x] **#176 — MCP server skeleton + meshant_articulate + meshant mcp CLI** — `mcp.NewServer(ts, analyst)`; stdio; fidelity test; `bufio.Scanner` 4MiB buffer; `recordInvocation` (includes #179)
- [x] **#179 — MCP invocation trace recording** — folded into #176; `recordInvocation` writes tag `["mcp-invocation", toolName]`; soft-fail policy; Observer attribution documented
- [x] **#177 — MCP tools batch 1** — shadow, follow, bottleneck, summarize, validate; `filterByTagsOR` (OR tag semantics for validate); input validation guards (length, max_depth bounds); 39 tests, 80.1% coverage
- [x] **#178 — MCP tools batch 2** — diff, gaps (dual-observer); `GapsResult` exported; T178.2–T178.4 documented; 87.1% coverage
- [ ] **Deferred (architect N1)** — `newUUID4` duplicated between `graph/actor.go` and `mcp/tools.go`; extract to `meshant/internal/uuid` if a third consumer appears
- [x] **Deferred (architect N2)** — `tags` property schema `items: {type: "string"}` — fixed in #177 for all batch-1 tools; `TestMCPServer_ToolsList_TagsHaveItems` asserts it

### v4.x — Interactive CLI + Web UI time series (parent: #172)

- [x] **#180 — Web UI time series controls** — `datetime-local` From/To picker; T1–T4 documented; `TestHandleShadow_WithTimeWindow` added; `time-window-controls-v1.md`
- [x] **#181 — explore-v1.md decision record** (ANT gate) — mutable session observer, AnalysisSession design, SuggestionMeta, AnalysisTrace; `Reading` not `Result`; T172.1–T172.6; ANT gate ALIGNED
- [x] **#182 — AnalysisSession types + meshant explore REPL skeleton** — `meshant/explore/` package; `AnalysisSession`, `AnalysisTurn`, `SuggestionMeta`; cut/quit/help; `meshant` (no args) enters REPL; D7a amends explore-v1.md; 95.8% coverage; PR #194
- [x] **#183 — explore commands batch 1** — articulate, shadow, window/tag filters; `commands.go`; errStore stub; 96.6% coverage; PR #195
- [x] **#184 — explore commands batch 2** — follow, bottleneck, diff, gaps; `commands_dual.go` new file; `articulateForSession`/`articulateDual` helpers; 95.5% coverage; PR #196
- [ ] **#185 — suggest command with SuggestionMeta** (ANT gate) — LLM suggestions with named provenance
- [ ] **#186 — AnalysisTrace + TagValueExplore + promote-explore** (ANT gate) — Principle 8 reflexivity for explore sessions

### v5.0.0 — Actors Act (parent: #173)

Not yet decomposed into child issues. Revisit after v4.x complete and four open design questions have provisional answers. See `tasks/plan_v5_actors.md`.
