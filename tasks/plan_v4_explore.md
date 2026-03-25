# Plan: v4.x — Interactive Analysis Session + Web UI Time Series

**Date:** 2026-03-25
**Parent issue:** #172
**Status:** Issues open — not yet started
**Follows:** v4.0.0 (MCP server)

---

## What

Two deliverables in this version:

1. **`meshant explore`** — a stateful REPL-like analysis session where the analyst refines
   cuts across multiple turns. The LLM suggests; the analyst cuts. Session state persists
   across turns. Completed sessions can be promoted to the mesh (Principle 8 reflexivity).

2. **Web UI time series controls** — time window picker/slider in the browser. Independent
   of `explore` — can ship before or after. Backend already supports `?from`/`?to`.

---

## Design decisions (confirmed after two-round planner + architect + ant-theorist discussion)

### Session observer model — mutable across turns

The session observer (Analyst) is mutable. Each `AnalysisTurn` carries its own `Observer string`.
Changing position mid-session is ANT-native — it is a shift in the reading position, not a
breach of session integrity. No forced restart on position change.

This contrasts with the MCP server (where `--observer` is fixed at startup). The difference
is intentional: MCP is a service; `explore` is an analytical session.

### Shared infrastructure with MCP

Both `explore` and `mcp` depend on `graph/envelope.go` (#174). Neither imports
`cmd/meshant/main.go`'s `loadTraces`. Constructor pattern:

```go
explore.NewSession(ts store.TraceStore, observer string) *AnalysisSession
```

### AnalysisSession and AnalysisTurn

```go
type AnalysisSession struct {
    ts      store.TraceStore  // injected, not snapshot
    turns   []AnalysisTurn
    window  graph.TimeWindow  // session-level, changeable via `window` command
    tags    []string          // session-level, changeable via `tags` command
    // no single-branch assumption — branching is v5
}

type AnalysisTurn struct {
    Observer      string       // per-turn; mutable
    Command       string
    Result        interface{}  // MeshGraph, GraphDiff, ShadowSummary, etc.
    Suggestion    *SuggestionMeta  // nil unless Command == "suggest"
}
```

Each turn queries `ts.Query()` live — not a snapshot of the state at session open.

### SuggestionMeta — required on every suggest output

```go
type SuggestionMeta struct {
    SessionObserver string
    CutUsed         graph.CutMeta
    Basis           string     // e.g. "gaps", "bottleneck", "shadow-density"
    TraceCount      int
    GeneratedAt     time.Time
}
```

The LLM never suggests without a named cut. Suggestions carry their own provenance.
`suggest` follows the same discipline as `meshant assist`.

### AnalysisTrace + TagValueExplore (Principle 8)

Completed sessions are promoted via `AnalysisTrace.Promote()`. The promoted trace:
- Records the full sequence of observer positions (not just the final one)
- Records commands issued and results (summary form)
- Records suggestions received and analyst decisions
- Carries `TagValueExplore` constant (distinct from `TagValueSession`)

This follows `llm.PromoteSession` as the reference pattern.

### REPL pattern

Follows `meshant/review/session.go`: `bufio.NewScanner(in)`, `in io.Reader`/`out io.Writer`
injection for testability.

Commands: `cut <observer>`, `articulate`, `diff <observer-b>`, `gaps <observer-b>`,
`shadow`, `follow <element>`, `bottleneck`, `window <from> <to>`, `tags <tag...>`,
`suggest`, `save`, `quit`, `help`

### gaps is dual-observer

`graph.AnalyseGaps` takes two `MeshGraph` inputs — confirmed dual-observer. The `gaps`
command in explore takes a second observer argument (same as `diff`). Same T172.4 tension
as MCP.

### Web UI time series controls

Frontend-only work in `meshant/web/`. No backend changes. Time window picker sends `from`/`to`
to existing API endpoints. Cut metadata panel shows active time window. Independent of explore.

---

## Known tensions (documented, not resolved)

**T172.1** — Observer mutability vs. session coherence: per-turn observer recording is
ANT-native (each turn is a positioned act). But the promoted `AnalysisTrace` must record
the full sequence, not just the final position. If not, the reflexivity record is incomplete.
Decision record (#181) must address this.

**T172.2** — `suggest` and LLM dependency: without `suggest`, `meshant explore` is pure Go
(no LLM). With `suggest`, it introduces the same LLM boundary as `meshant assist`. The LLM
is a mediator (visible in session provenance), not an extractor. Must not be treated as
neutral input.

**T172.3** — REPL scope: first version is narrow (linear cut refinement, one observer
position at a time). `AnalysisSession` must not bake in a single-branch assumption.
Branching (multiple live cuts diffed against each other) is deferred to v5.

**T172.4** — `CutMeta.Observer` single string for dual-observer results: `diff` and `gaps`
span two observer positions. Known limitation, documented in code comments, not fixed in v4.

---

## Child issues

| # | Type | Scope | Notes |
|---|------|-------|-------|
| #180 | feat | Web UI time series controls | Independent — can ship any time |
| #181 | docs | explore-v1.md decision record | **ANT gate** before #182 |
| #182 | feat | AnalysisSession types + meshant explore REPL skeleton | Prereq: #174, #181 |
| #183 | feat | explore commands batch 1 (articulate, shadow, window, tags) | |
| #184 | feat | explore commands batch 2 (diff, gaps, follow, bottleneck) | |
| #185 | feat | suggest command with SuggestionMeta | **ANT gate** required |
| #186 | feat | AnalysisTrace + TagValueExplore + promote-explore | **ANT gate** required |

---

## Sequencing

```
#174 (CutMeta extraction, from #171) — shared prereq
                                     ↓
#181 (decision record, ANT gate) → #182 (skeleton) → #183 (batch 1) → #184 (batch 2)
                                                                             ↓
                                                           #185 (suggest, ANT gate)
                                                                             ↓
                                                           #186 (AnalysisTrace, ANT gate)

#180 (Web UI time series) — independent, parallel
```

---

## What does not change

The core inversion holds:

> Traces come first. The analyst cuts. The LLM suggests. The mesh constrains.

- `suggest` proposes; the analyst decides. No automated cuts.
- `AnalysisTrace` is always attributed (TagValueExplore + session provenance).
- The promoted session trace is a new observation act in the mesh, not a summary document.

---

*This is a plan — a provisional articulation. It will be revisited after #181 (decision record).*
*Before any phase begins: confirm design, run per-issue pipeline.*
