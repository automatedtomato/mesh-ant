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

## Completed Threads (v2.0.0)

All complete and merged to `develop`. v2.0.0 tagged on `main` (2026-03-22).
Full rough plan: `tasks/done/plan_v2_roadmap.md`.

**Thread B** — Interpretive outputs: `meshant bottleneck`, `meshant narrative`, `--suggest` on `meshant gaps`. Decision record + codemap.

**Thread A** — Interactive review CLI: `meshant review`; accept/edit/skip loop; ambiguity detection; provenance chain rendering. Parent #86, complete 2026-03-19.

**Thread C** — Multi-analyst ingestion comparison: `meshant extraction-gap`, `meshant chain-diff`; multi-analyst example dataset; `multi-analyst-v1.md`. Parent #103, complete 2026-03-19.

**Thread F** — LLM-internal boundary: `meshant extract`, `meshant assist`, `meshant critique`; `SessionRecord`; `LLMClient` interface; real-world example; `llm-boundary-v2.md`. Parent #115, complete 2026-03-22. **v2.0.0 released.**

Detailed plans: `tasks/done/plan_thread_{a,b,f}.md`.

---

## Deferred Items — COMPLETE (2026-03-25)

All deferred items from v2.0.0 threads resolved. Per-thread pipeline complete 2026-03-25.

- [x] **Slice equality helper naming** — renamed to `stringSlicesEqualOrdered`/`stringSlicesEqualUnordered`; fixed in per-thread refactor-clean 2026-03-24
- [ ] **buildChain closure extraction** — candidate if a second consumer appears (still deferred)
- [x] **`PromptHash` in `ExtractionConditions`** — `HashPromptTemplate` + `PromptHash` field on `ExtractionConditions` and `CritiqueConditions`; merged PR #170 2026-03-24
- [x] **`ExtractionConditions` bifurcation** — `CritiqueConditions` type added; `SessionRecord.CritiqueConditions *CritiqueConditions`; backward compat preserved; merged PR #168 2026-03-24

Resolved GitHub issues:
- [x] [#95](https://github.com/automatedtomato/mesh-ant/issues/95) — `ClassifyDraftChainOptions` + `ClassifiedDraftChain` envelope; C1 enforced; merged PR #170 2026-03-24
- [x] [#96](https://github.com/automatedtomato/mesh-ant/issues/96) — `DraftSubKindEndorsement` constant + `SubKind` field on `DraftStepClassification`; merged PR #169 2026-03-24
- [x] [#150](https://github.com/automatedtomato/mesh-ant/issues/150) — merged PR #170 2026-03-24
- [x] [#151](https://github.com/automatedtomato/mesh-ant/issues/151) — merged PR #168 2026-03-24

Per-thread pipeline (2026-03-25):
- Refactor-clean: MUST-FIX resolved (JSON tags on `DraftStepClassification`); 3 NICE-TO-HAVE deferred
- ANT-theorist: ALIGNED WITH TENSIONS — T1 (SubKind not criterion-governed, deepened), T2 (`CritiqueConditions.SourceDocRef` provenance-chain reading), T3 (drift→change language fixed)
- CODEMAPS updated; tests all green

---

## Post-v2.0.0 — ANT-like Knowledge Graph

**Full plan:** `tasks/plan_post_v2.md`
**Status:** Issues open (2026-03-22)

Direction confirmed in design discussion (2026-03-22). The next major form is an ANT-like Knowledge Graph — persistent, queryable, interactive. "Actors act" simulation comes much later, after the graph substrate exists.

### Phase 1 — Deferred items and ingestion gaps (parent: #132) — COMPLETE

Per-thread pipeline complete: refactor-cleaner (stringSlice consolidation, filterBlanks →
shared.go, stripPreamble extracted); ant-theorist (ALIGNED WITH TENSIONS); docs updated
(session-promote-v1.md decision record added, non-text-adapters tensions updated).

- [x] **#137 — `meshant split`** — LLM-assisted span splitting; removes the biggest `assist` friction
- [x] **#138 — Session records → Traces** — a session is an observation act; closes the ANT reflexivity gap; decision record `session-promote-v1.md`
- [x] **#139 — Multi-document ingestion** — `meshant extract` across several source documents in one session
- [x] **#140 — Non-text source adapters** — PDF, HTML, structured logs → text → existing LLM pipeline

### Phase 2 — Form 3 scoping document (parent: #133)

- [x] **#141 — KG scoping document** — storage adapter contract, query model, Web UI shape, Layer 1/2/3 boundaries

### Phase 3 — Layer 1: Trace substrate (parent: #134)

- [x] **#142 — DB adapter interface** — `TraceStore` interface in `meshant/store`; `JSONFileStore` implements it; `QueryOpts` with Observer/TimeWindow/Tags/Limit; 86.4% coverage; PR #157
- [x] **#143 — Neo4j adapter** — `Neo4jStore` + `Neo4jConfig`; `neo4j_store.go` + `neo4j_cypher.go`; build tag `neo4j`; MERGE/FOREACH Cypher; RFC3339Nano timestamps; integration tests behind same tag; 4 ANT tensions documented; decision record `neo4j-adapter-v1.md`
- [x] **#144 — `meshant store` + `--db` flag** — `meshant store` reads JSON → writes to DB via TraceStore; `--db` flag on `articulate`, `diff`, `shadow`, `gaps`, `follow`, `bottleneck`; `loadTraces` shared helper; `db_factory.go`/`db_factory_neo4j.go` build-tag factory; no pre-filtering (full substrate preserved for ANT correctness); decision record `store-cli-v1.md`

### Phase 4 — Layer 3: Interactive graph output (parent: #135)

- [x] **#145 — `meshant serve`** — localhost HTTP server; `meshant/serve` package; 4 endpoints (`/articulate`, `/diff`, `/shadow`, `/traces`); `Envelope` with `CutMeta`; observer required on all endpoints (400 with ANT-reasoning error); graceful shutdown; 82.3% coverage; 4 ANT tensions documented; decision record `serve-v1.md`
- [x] **#146 — Web UI + provenance panel** — Cytoscape.js 3.30.4 (vendored); observer gate (structural HTML); shadow panel (amber); detail panel (trace cards + provenance); `/element/{name}` endpoint; `go:embed web`; static file server; 93.4% coverage; decision record `web-ui-v1.md`

### Per-thread pipeline — Post-v2.0.0 batch (all phases) — COMPLETE (2026-03-24)

Refactor-clean: 6 MUST-FIX items resolved (stampProvenance extracted to shared.go,
validateIntentionallyBlank moved to shared.go, splitErrNotes/joinErrNotes moved to
shared.go, goto stamp eliminated in critique.go, slicesEqual/stringSlicesEqual renamed
to reflect order vs unordered semantics, LimitReader cast fixed, readSessionFile +1 guard).
7 NICE-TO-HAVE items logged; NH-3 (readSessionFile oversize) promoted and fixed.

ANT-theorist: ALIGNED WITH TENSIONS — 5 tensions, all documented, none violations.
Web UI is among the strongest embodiments of MeshAnt principles in the codebase.

Deferred items (#95, #96, #150, #151) remain open for a future phase.

### Phase 5 — Thread D datasets (parent: #136) — COMPLETE

- [x] **#147 — D.1 Software incident** — `data/examples/software_incident.json`; 32 traces; observers: `on-call-engineer`, `product-manager`, `customer-support-lead`, `dataset-analyst`; retry-buffer as key mediator; ANT aligned; merged PR #162
- [x] **#148 — D.2 Multi-agent pipeline** — `data/examples/multi_agent_pipeline.json`; 28 traces; observers: `pipeline-auditor`, `ml-engineer`, `dataset-analyst`; 8 pipeline agents as non-human actants; inscription conflict demonstrated; merged PR #163
- [x] **#149 — D.3 Policy/procurement** — `data/examples/policy_procurement.json`; 27 traces; observers: `procurement-officer`, `budget-approver`, `vendor-alpha`, `compliance-auditor`, `dataset-curator`; 17 institutional actants; 11 circular source==mediation violations fixed; ANT aligned with tensions (T1-T4); merged PR #164

---

## Post-v3 — MCP → Interactive CLI → Actors Act

**Full plan:** `tasks/plan_post_v3.md`
**Status:** Rough plan only — not yet decomposed into issues (2026-03-25)

| Version | Direction |
|---------|-----------|
| v4.0.0 | `meshant mcp` — analytical commands as MCP tools; observer-position enforced at schema level |
| v4.x | `meshant explore` — interactive analysis session; LLM suggests, analyst cuts |
| v5.0.0 | Actors act — emerged actors generate new traces, constrained by relational history |
