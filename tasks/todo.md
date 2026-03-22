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

## Deferred Items (still open)

Small items deferred during v2.0.0 threads. Not yet assigned to a phase.

- [ ] **Slice equality helper naming** — `slicesEqual`/`stringSlicesEqual` inconsistency; next refactor-clean pass
- [ ] **buildChain closure extraction** — candidate if a second consumer appears
- [ ] **`PromptHash` in `ExtractionConditions`** — content hash of prompt template for reproducibility; deferred from Thread F
- [ ] **`ExtractionConditions` bifurcation** — extract vs. critique conditions may need distinct types; deferred from Thread F ant-theorist review

Open GitHub issues (deferred, lower priority):
- [#95](https://github.com/automatedtomato/mesh-ant/issues/95) — Govern `classifyDraftChain` heuristic with `EquivalenceCriterion`
- [#96](https://github.com/automatedtomato/mesh-ant/issues/96) — sub-kind field for stage-only endorsement steps in draft chain
- [#150](https://github.com/automatedtomato/mesh-ant/issues/150) — `PromptHash` in `ExtractionConditions` (reproducibility)
- [#151](https://github.com/automatedtomato/mesh-ant/issues/151) — `ExtractionConditions` bifurcation (extract vs critique conditions)

---

## Post-v2.0.0 — ANT-like Knowledge Graph

**Full plan:** `tasks/plan_post_v2.md`
**Status:** Issues open (2026-03-22)

Direction confirmed in design discussion (2026-03-22). The next major form is an ANT-like Knowledge Graph — persistent, queryable, interactive. "Actors act" simulation comes much later, after the graph substrate exists.

### Phase 1 — Deferred items and ingestion gaps (parent: #132)

- [x] **#137 — `meshant split`** — LLM-assisted span splitting; removes the biggest `assist` friction
- [x] **#138 — Session records → Traces** — a session is an observation act; closes the ANT reflexivity gap
- [ ] **#139 — Multi-document ingestion** — `meshant extract` across several source documents in one session
- [ ] **#140 — Non-text source adapters** — PDF, HTML, structured logs → text → existing LLM pipeline

### Phase 2 — Form 3 scoping document (parent: #133)

- [x] **#141 — KG scoping document** — storage adapter contract, query model, Web UI shape, Layer 1/2/3 boundaries

### Phase 3 — Layer 1: Trace substrate (parent: #134)

- [ ] **#142 — DB adapter interface** — `TraceStore` interface in `meshant/store`; JSON loader implements it
- [ ] **#143 — Neo4j adapter** — implement `TraceStore` against Neo4j-compatible backend
- [ ] **#144 — `meshant store` + `--db` flag** — ingest JSON to DB; `--db` flag on all analytical commands

### Phase 4 — Layer 3: Interactive graph output (parent: #135)

- [ ] **#145 — `meshant serve`** — localhost HTTP server; cut endpoints; provenance enforcement
- [ ] **#146 — Web UI + provenance panel** — D3.js/Cytoscape.js; observer selector required; shadow named

### Phase 5 — Thread D datasets (parent: #136)

- [ ] **#147 — D.1 Software incident** — multi-service outage; competing observer positions; full LLM pipeline
- [ ] **#148 — D.2 Multi-agent pipeline** — agents as actants; AI workflow domain
- [ ] **#149 — D.3 Policy/procurement** — institutional non-human actants; regulatory/procurement domain
