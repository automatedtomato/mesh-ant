# MeshAnt v2.0.0 Roadmap — Rough Plan

**Threads:** B → A → C → F
**Last updated:** 2026-03-17
**Status:** rough plan — detailed per-milestone plans to be written before implementation begins

This is a directional sketch, not a locked specification. Each thread should be planned
in detail before work starts. Milestones will be numbered when cut.

---

## Thread B — Remaining Interpretive Outputs (v1.x)

**Goal:** complete the Layer 3 story. Three interpretive outputs were named in the v1.0.0
review and deferred across M13. They make analytical results actionable without hiding the cut.

### B.1 — Bottleneck note

A node that appears across many translation chains, or in many observer-position graphs,
mediates disproportionately. MeshAnt should be able to name that — provisionally.

- `graph/bottleneck.go` — `IdentifyBottlenecks(g MeshGraph) []BottleneckNote`
- `BottleneckNote`: element name, degree (in + out edges), chain appearances, reason string
- `PrintBottleneckNotes(w io.Writer, notes []BottleneckNote) error`
- CLI: `meshant bottleneck --observer <pos> [flags] traces.json`
- **Constraint:** language of provisionality throughout — "appears central from this cut",
  not "is a bottleneck". The centrality is an articulation artifact, not a fact about the world.

### B.2 — Re-articulation suggestion

When `ObserverGap` shows large `OnlyInA` or `OnlyInB` sets, MeshAnt can suggest a
re-articulation: an observer position that might bridge the gap, or a time window worth
exploring. This is a heuristic prompt, not an answer.

- `graph/suggest.go` — `SuggestRearticulations(gap ObserverGap, g1, g2 MeshGraph) []RearticSuggestion`
- `RearticSuggestion`: kind (observer / time-window / tag), rationale, suggested parameters
- `PrintRearticSuggestions(w io.Writer, suggestions []RearticSuggestion) error`
- CLI: `meshant gaps` extended with `--suggest` flag; or standalone `meshant suggest`
- **Constraint:** a suggestion is a provocation, not a recommendation. Its output must
  name what it cannot see — the shadow of the suggestion itself.

### B.3 — Incident narrative draft

A prose paragraph summarising an articulation for a non-specialist reader: which actors
appeared, what the key mediations were, what remained in shadow, which position was held.

- `graph/narrative.go` — `DraftNarrative(g MeshGraph) NarrativeDraft`
- `NarrativeDraft`: Body string, PositionStatement string, ShadowStatement string,
  Caveats []string
- `PrintNarrativeDraft(w io.Writer, n NarrativeDraft) error`
- CLI: `meshant articulate --narrative` flag, or `meshant narrative`
- **Constraint:** the narrative is a draft, always. It must carry its position explicitly
  and name what it cannot claim. No authoritative voice.

### B.4 — Decision record + codemap update

- `docs/decisions/interpretive-outputs-v1.md`
- `docs/CODEMAPS/meshant.md` updated
- `tasks/todo.md` Thread B section marked complete

---

## Thread A — Interactive Review CLI (M14, v1.x → v2.0.0 prereq)

**Goal:** human-in-the-loop refinement pass between draft extraction and promotion.
This is the last major v1.x piece before LLM-internal integration (Thread F) makes sense.
Without it, the authoring experience is still too demanding — the user must hold the full
draft context in their head across a separate review step.

### A.1 — Draft review session

An interactive terminal session that walks a user through unreviewed drafts one at a time:
shows the source span, the candidate fields, the provenance chain (DerivedFrom lineage),
and the `ClassifyDraftChain` verdict. The user accepts, revises, or skips.

- `cmd/meshant/review.go` (or inline in `main.go`) — `cmdReview`
- `meshant review [--id <id>] [--criterion-file <path>] drafts.json`
- Each draft presented as: source span → fields → derivation chain → classification
- Actions: accept (mark stage = "reviewed"), edit (open $EDITOR or inline field edit),
  skip, quit
- Output: updated drafts JSON with reviewed drafts promoted to `extraction_stage=reviewed`
- **Constraint:** the session is a cut, not a correction. The original draft is never
  overwritten — acceptance creates a new derived draft with `DerivedFrom` pointing to
  the accepted draft.

### A.2 — Provenance chain display

Inline `lineage`-style display within the review session: a compact tree showing the
derivation chain up to the current draft, with `ClassifyDraftChain` annotations.
Makes the interpretive history visible at the moment of decision.

- Reuses `FollowDraftChain` and `ClassifyDraftChain` from `loader/draftchain.go`
- Display format: indented, with step kind (intermediary / mediator / translation)
  and reason inline

### A.3 — Ambiguity surfacing

During review, surface structural ambiguities: empty candidate fields without
`IntentionallyBlank` set, `UncertaintyNote` present (flagging unresolved provenance),
`CriterionRef` mismatch (draft uses different criterion than the review session).

- Warnings shown before each draft; user acknowledges or resolves
- Not blocking — the user may accept an ambiguous draft, but the ambiguity is named

### A.4 — Decision record + docs

- `docs/decisions/interactive-review-v1.md`
- `docs/CODEMAPS/meshant.md` updated
- `tasks/todo.md` Thread A section marked complete

---

## Thread C — Multi-Analyst Ingestion Comparison

**Goal:** make the extraction cut visible, not just the graph cut. Two analysts extract
drafts from the same source span independently. MeshAnt compares their extractions to
show where they diverged — the disagreement is analytical data, not error.

This is the ingestion-layer analogue of `ObserverGap`. The extractor is inside the mesh.

### C.1 — Multi-analyst draft set schema

A draft set tagged with `ExtractedBy` (already on `TraceDraft`) defines an analyst
position. Two sets are comparable when they share `SourceSpan` values.

- No new types required for the comparison itself — `ExtractedBy` is the analyst-position marker
- Possibly a lightweight `AnalystSet` wrapper: `Analyst string, Drafts []TraceDraft`
- Clarify: `ExtractedBy` is the analyst-position cut axis for ingestion (parallel to
  `Observer` for the graph layer)

### C.2 — Extraction gap analysis

`CompareExtractions(setA, setB []TraceDraft) ExtractionGap` — compares draft pools by
`SourceSpan`. For each span: did both analysts extract it? Did they agree on candidate
fields? On `ExtractionStage`?

- `ExtractionGap`: `OnlyInA []string` (spans only analyst A extracted),
  `OnlyInB []string`, `InBoth []string`, `Disagreements []FieldDisagreement`
- `FieldDisagreement`: SourceSpan, Field, ValueA, ValueB — where both extracted the
  same span but named it differently
- `PrintExtractionGap(w io.Writer, gap ExtractionGap) error`
- CLI: `meshant extraction-gap --analyst-a <label> --analyst-b <label> drafts.json`

### C.3 — Classification comparison

Where both analysts extracted the same span, compare their `ClassifyDraftChain` results.
Do their derivation chains produce the same step kinds? Disagreement in classification
is a signal that the criterion needs tightening or the source span is genuinely ambiguous.

- `CompareChainClassifications(chainA, chainB []DraftStepClassification) []ClassificationDiff`
- `ClassificationDiff`: StepIndex, KindA, KindB, ReasonA, ReasonB

### C.4 — Multi-analyst example dataset

A worked example: two analysts (e.g., `analyst-a` and `analyst-b`) extract drafts from
the same source document (e.g., `data/examples/evacuation_order.json`). Their extractions
deliberately differ in some spans to make the comparison meaningful.

- `data/examples/multi_analyst_drafts.json`

### C.5 — Decision record + docs

- `docs/decisions/multi-analyst-v1.md`
- `docs/CODEMAPS/meshant.md` updated
- `tasks/todo.md` Thread C section marked complete

---

## Thread F — v2.0.0: LLM-Internal Boundary

**Goal:** move the LLM boundary inside the CLI. MeshAnt can call an LLM directly to
assist with extraction, critique, and review. The LLM is not a hidden extractor — it
appears as a named mediator in the mesh. Its transformations are visible and contestable.

This thread begins only after Threads B and A are complete (and C ideally in progress).
The interactive review CLI (Thread A) is the user-facing foundation this builds on.

### F.1 — LLM mediator node

When an LLM assists with extraction or critique, it is recorded as a node in the mesh:
`extracted_by: "claude-sonnet-4-6"`, `uncertainty_note: "LLM-extracted; unverified"`.
This is not a new type — it is a convention for populating existing `TraceDraft` fields.

- `docs/decisions/llm-as-mediator-v1.md` — the discipline: LLM is a mediator, not an
  extractor. Its output is a candidate draft, not a trace. Every LLM-produced draft carries
  `ExtractedBy`, `UncertaintyNote`, and `ExtractionStage: "weak-draft"` automatically.
- Codify as a convention in `data/prompts/` and enforced in the new commands.

### F.2 — `meshant extract` command

Calls an LLM with a source document and the extraction prompt template. Returns a draft
JSON array ready for `meshant review`. The LLM appears in the mesh; its output enters the
existing ingestion pipeline unchanged.

- Requires: LLM client integration (Claude API via `anthropic-go` or HTTP)
- Input: source document (text file or stdin) + `--criterion-file` (optional)
- Output: `TraceDraft` JSON array (`extraction_stage: "weak-draft"`, `extracted_by: <model>`)
- The LLM call is a single, inspectable step — not a pipeline hidden from the user

### F.3 — `meshant assist` — interactive authoring companion

A session interface: present a source span, let the LLM suggest candidate trace fields,
show its reasoning, let the user accept/revise/reject. Each interaction is one derivation
step. The provenance chain is built live.

- Builds on `cmdReview` (Thread A.1) — same session UX, with LLM suggestions surfaced
  as pre-filled candidates rather than blank fields
- The LLM's suggestion is a `weak-draft`; the user's acceptance/revision is the next
  derivation step (`DraftMediator` or `DraftIntermediary` in the chain)
- `meshant assist [--criterion-file <path>] <source-document>`

### F.4 — LLM critique pass

`meshant critique` — automated critique of an existing draft set using the critique prompt
template (`data/prompts/critique_pass.md`). The LLM produces a new derived draft for each
input draft with `DerivedFrom` set and `ExtractionStage: "reviewed"`. Human review follows.

- This is the automated version of `meshant rearticulate` — same output shape, LLM-assisted
- The output feeds directly into `meshant review` (Thread A)

### F.5 — Real-world example: LLM-assisted extraction

A full worked example: raw source document → `meshant extract` → `meshant review` →
`meshant promote` → `meshant articulate`. Shows the complete pipeline end-to-end with an
LLM as a visible mediator.

- `data/examples/llm_assisted_extraction/` — source doc, draft JSON, promoted traces,
  articulation output
- Documents where the LLM agreed with human review, and where it diverged

### F.6 — Decision record + docs + v2.0.0 release

- `docs/decisions/llm-boundary-v2.md`
- `docs/CODEMAPS/meshant.md` updated
- `README.md` updated — v2.0.0 section, new commands
- `tasks/todo.md` Thread F marked complete
- Release: v2.0.0

---

## Thread D — Real-World Datasets (runs alongside all threads)

**Goal:** validate authoring conventions and interpretation patterns across domains.
Not a standalone milestone — a running commitment. A new real-world dataset should
accompany each major thread (B, A, C, F) to ground the work.

### D.1 — Software incident trace dataset
A real-world software incident (e.g., a database outage, a deploy failure, an API
cascade). The incident is already fully resolved — MeshAnt traces what mediated the
failure and the recovery. Two observer positions: on-call engineer, product manager.

- `data/examples/software_incident.json`
- Demonstrates: threshold crossings, non-human mediators (alerting system, circuit breaker,
  runbook), asymmetric observer positions

### D.2 — Multi-agent pipeline trace dataset
A multi-agent AI pipeline (e.g., a research + summarization + review pipeline). Traces
what each agent mediated, what was translated between steps, where friction appeared.
Directly relevant to MeshAnt's eventual use case in AI system analysis.

- `data/examples/multi_agent_pipeline.json`
- Demonstrates: LLM steps as mediators, translation between prompt formats, shadow
  from system prompt opacity

### D.3 — Policy / procurement trace dataset
A policy decision or procurement process (e.g., a budget approval chain, a regulation
implementation). Human and non-human actors; delays and thresholds central.

- `data/examples/policy_process.json`

---

## Sequencing summary

```
v1.x now
  └── Thread B: Interpretive outputs (bottleneck, re-articulation suggestion, narrative)
        └── Thread A: Interactive review CLI (human-in-the-loop authoring)
              ├── Thread C: Multi-analyst comparison (runs parallel or after A)
              └── Thread F: v2.0.0 — LLM-internal boundary
                    └── v2.0.0 release

Thread D: Real-world datasets — runs alongside all threads, one dataset per thread
```

**Invariant across all threads:**
- Shadow is a cut decision, not missing data — language discipline throughout
- Every LLM step is a mediator, not a neutral extractor
- No god's-eye outputs — every summary names its position
- Detailed plans written and reviewed before implementation begins

---

*This is a rough plan. Cut detailed milestone plans from it when work begins.*
