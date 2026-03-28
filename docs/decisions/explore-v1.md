# Decision Record: `meshant explore` Interactive Analysis Session (v1)

**Issue:** #181 (decision record) → #182–#186 (implementation)
**Branch:** `181-explore-v1-decision-record`
**Phase:** v4.x — Interactive CLI (parent: #172)
**ANT gate:** This record must be reviewed and aligned before #182 begins.

---

## Problem

MeshAnt's analytical surfaces — CLI subcommands, MCP tools, HTTP endpoints — are
stateless. Each invocation is a single cut with no memory of prior turns. An analyst
refining a reading across multiple observer positions, time windows, and commands has
no session context: each command must re-specify position, re-load traces, re-articulate
the network.

The problem is not absence of a REPL. The problem is that multi-turn analytical work is
invisible to the mesh. An analyst who reads from alice's position, then shifts to bob's,
then compares the two, has performed a sequence of positioned acts. That sequence is not
recorded anywhere — the analytical labour dissolves when the terminal closes.

`meshant explore` is the surface for situated, multi-turn analysis where each turn is
positioned, the full sequence is recoverable, and completed sessions can be promoted to
the mesh as first-class reflexive traces.

---

## Decision: `meshant explore` + `meshant/explore` package

Two additions:

1. **`meshant explore [--db bolt://...] [<traces.json>]`** — starts an interactive
   analysis session. No `--analyst` flag: the analyst identifies themselves at session
   start via the `cut` command or an initial prompt. The session ends with `quit`.

2. **`meshant/explore` package** — contains `AnalysisSession`, `AnalysisTurn`,
   `SuggestionMeta`, and `AnalysisTrace` types. The CLI (`cmd_explore.go`) is thin
   glue; all session logic lives in the package and is testable via injected
   `io.Reader`/`io.Writer`.

---

## Design decisions

### D1: Session observer model — mutable per-turn, stable session analyst

Two observer concepts coexist in `meshant explore`:

- **`Analyst string`** — the human conducting the session; set once at session start
  (via the `cut` command's first invocation or an explicit `--analyst` flag on the CLI).
  Populated in `CutMeta.Analyst` on every turn's envelope. Identifies who is reading.

- **`Observer string`** — the ANT position from which each turn's graph is articulated;
  whose traces are being read. Per-turn; mutable across the session. Populated in
  `CutMeta.Observer`.

Changing `Observer` mid-session is ANT-native — it is a shift in reading position, not
a breach of session integrity. An analyst studying a network from multiple vantage points
in sequence is performing a legitimate analytical act. The session records the trajectory.

This contrasts with `meshant mcp`, where `--analyst` is required at startup and fixed for
the life of the server process. The distinction is intentional:

- MCP is a service: each tool call is an atomic act; the analyst context is a deployment
  decision.
- `explore` is a session: multi-turn, exploratory, deliberately positioned. The observer
  may change because the analyst is navigating the network.

The `cut <observer>` command changes `Observer` for the current and all future turns
until changed again. Each `AnalysisTurn` records the `Observer` that was active when it
executed — the full positional trajectory is preserved in `turns`.

### D2: `AnalysisSession` holds injected `TraceStore`, not a snapshot

```go
// NewSession creates an AnalysisSession backed by the given store and
// identified by the given analyst name. The store is queried live on each
// turn — no snapshot is taken at session start.
func NewSession(ts store.TraceStore, analyst string) *AnalysisSession
```

```go
type AnalysisSession struct {
    ts      store.TraceStore  // injected; queried live on each turn
    analyst string            // who is conducting the session
    turns   []AnalysisTurn   // linear turn history; no branching in v1
    observer string           // current observer position; mutable
    window  graph.TimeWindow  // session-level; changed by `window` command
    tags    []string          // session-level; changed by `tags` command
}
```

Each turn calls `ts.Query(ctx, store.QueryOpts{})` at execution time. This means:

- Traces added to the store while the session is open are visible to subsequent turns.
- The substrate does not freeze at session open.
- The analyst sees a live, evolving mesh — not a snapshot of what existed when they began.

**No single-branch assumption.** `turns []AnalysisTurn` is a linear slice in v1.
`AnalysisSession` must not bake in a topology that precludes future branching.
Branching (multiple live cuts diffed against each other within one session) is deferred
to v5 — see T172.3.

### D3: `AnalysisTurn` — each turn is a positioned analytical act

```go
type AnalysisTurn struct {
    Observer   string          // ANT position active when this turn executed
    Window     graph.TimeWindow // time window active when this turn executed
    Tags       []string        // tag filters active when this turn executed
    Command    string          // the command string as typed
    Reading    interface{}     // MeshGraph, GraphDiff, ShadowSummary, etc. — named
                               // "Reading" not "Result" to signal that the output is
                               // a positioned act, not a context-free finding.
    Suggestion *SuggestionMeta // non-nil only when Command == "suggest"
    ExecutedAt time.Time
}
```

`Window` and `Tags` are snapshotted per-turn at execution time. Changing the window or
tags via the `window`/`tags` commands affects future turns only — prior turns retain the
conditions under which they were executed. This preserves the analytical record: the
analyst can reconstruct what cut was in effect for any turn by reading that turn's fields.

`Reading` is `interface{}` in v1. Concrete types: `graph.MeshGraph` (articulate, shadow),
`graph.GraphDiff` (diff), `graph.GapsResult` (gaps), string (summarize, validate, help).
The field name `Reading` — not `Result` — signals that the output is a positioned act,
not a context-free finding. A result can stand alone; a reading requires a position.

`Suggestion` is nil for all commands except `suggest`. When non-nil, it carries the full
provenance of the LLM suggestion — see D4.

### D4: `SuggestionMeta` — the LLM never suggests without a named cut

```go
type SuggestionMeta struct {
    Analyst     string        // who asked
    CutUsed     graph.CutMeta // the exact cut in effect when suggest was called
    Basis       string        // what was passed to the LLM: "gaps", "bottleneck", "shadow"
    TraceCount  int           // size of the substrate the LLM saw
    GeneratedAt time.Time
}
```

Every output of `suggest` carries `SuggestionMeta`. An LLM suggestion without a named
cut is an unattributable reading — it cannot be placed in the analytical record without
knowing from whose position the suggestion was generated and what substrate it saw.

This follows the same discipline as `meshant assist` and `SuggestionMeta` in the LLM
package. The LLM is a mediator: it transforms the cut into a navigational suggestion.
That transformation must be visible in the session record — not hidden behind a "the
AI suggested" annotation without provenance.

`suggest` without a prior `articulate`, `gaps`, or `bottleneck` in the current turn is
an error. There is no cut to suggest from. The command must refuse with a clear message.

`suggest` is optional. A complete `meshant explore` session that never invokes `suggest`
is a fully valid, fully attributable analytical act.

### D5: `AnalysisTrace` + `TagValueExplore` — Principle 8 reflexivity

Completed sessions can be promoted to the `TraceStore` via the `save` command. The
promoted record is a single `schema.Trace` (mirroring `llm.PromoteSession`):

```
Observer:    session.analyst          — who conducted the session
WhatChanged: "explore session: N turns, observers visited: [alice, bob, ...]"
Mediation:   "meshant explore"
Source:      deduplicated observer positions visited, in order of first appearance
Target:      elements from the final articulation's node set (if available)
Tags:        [TagValueExplore]
```

`TagValueExplore = "explore"` — distinct from `TagValueSession = "session"` (LLM session
promotes). The distinction matters for downstream filtering and for knowing which
analytical surface generated the trace.

**Why `Source` = observer positions?** The analytical trajectory — the sequence of
positions visited — is what the session records. Recording which observer positions were
visited in `Source` captures the relational path of the analysis: the analyst moved
through the network by occupying different reading positions in sequence. This is
not the same as recording network elements in `Source`. An ANT tension (T172.5) is
named for this.

**Why `Target` = elements from the final articulation?** The final cut reveals what
the session ultimately saw. Recording those elements as `Target` creates a trace that
connects the analytical act (reading from those positions) to the network fragment that
emerged from it. If no articulation was performed, `Target` is empty.

**`save` command semantics.** `save` promotes the current session to the `TraceStore`
immediately. It is callable mid-session (a partial record) or at the end. `quit` without
`save` discards the session — there is no implicit promotion. The promotion is always an
explicit act.

**Why no auto-promote on `quit`?** Auto-promoting on exit would record sessions the
analyst did not intend to preserve — exploratory dead-ends, debugging runs, or sessions
opened by mistake. The analyst must declare that their session is worth recording. This
is consistent with MeshAnt's broader principle: no trace enters the mesh without a
deliberate act of inscription.

### D6: `gaps` is dual-observer — same pattern as `meshant_gaps`

`gaps <observer-b>` takes a second observer argument. The session's current `Observer`
is `observer_a`; `<observer-b>` is `observer_b`. This produces a `graph.GapsResult`
comparing the two positioned cuts.

Exactly the same dual-observer pattern as `meshant_gaps` in the MCP server
(`mcp-v1.md D4`, `T178.2–T178.4`). The same tensions apply — including T172.4
(`CutMeta.Observer` is a single string; `observer_b` lives in the result payload only).

`diff <observer-b>` follows the same pattern for `graph.GraphDiff`.

### D7: REPL pattern — `bufio.Scanner`, injected `io.Reader`/`io.Writer`

```go
// Run executes the interactive loop, reading commands from in and writing
// output to out. Blocking; returns when the analyst types `quit`.
func (s *AnalysisSession) Run(ctx context.Context, in io.Reader, out io.Writer) error
```

Follows `meshant/review/session.go` as the reference pattern:
- `bufio.NewScanner(in)` for line reading
- All output to `out`, never to `os.Stdout` directly
- Testable without a live terminal: pass `strings.NewReader(...)` and
  `bytes.Buffer` in tests

The CLI (`cmd_explore.go`) wires `os.Stdin` and `os.Stdout` and calls `session.Run`.
Nothing else touches the terminal.

---

## Command set

| Command | Arguments | Observer | Returns | Notes |
|---------|-----------|----------|---------|-------|
| `cut` | `<observer>` | sets | — | Change the session observer position |
| `articulate` | — | current | `graph.MeshGraph` | Build graph from current position |
| `shadow` | — | current | `[]graph.ShadowElement` | Elements not visible from current position |
| `follow` | `<element>` | current | `graph.TranslationChain` | Translation chain from element |
| `bottleneck` | — | current | `[]graph.BottleneckElement` | High-mediation actors |
| `diff` | `<observer-b>` | dual | `graph.GraphDiff` | Current vs. observer-b |
| `gaps` | `<observer-b>` | dual | `graph.GapsResult` | Extraction gaps, current vs. observer-b |
| `window` | `<from> <to>` | — | — | Set time window for future turns; `window reset` clears |
| `tags` | `<tag...>` | — | — | Set tag filters for future turns; `tags reset` clears |
| `suggest` | — | current | string | LLM suggests next step; requires prior cut context |
| `save` | — | — | — | Promote session to TraceStore as `AnalysisTrace` |
| `quit` | — | — | — | End session; discards if not saved |
| `help` | — | — | string | List commands |

`window` and `tags` affect future turns only — prior turns retain the conditions under
which they were executed (D3).

---

## ANT tensions

**T172.1: Observer mutability vs. session coherence.**
Per-turn observer recording is ANT-native: each turn is a positioned analytical act.
But the promoted `AnalysisTrace` must record the full sequence of positions — not just
the final one. If only the final observer is preserved, the analytical trajectory is
collapsed: the mesh knows the session ended at position X but not that it passed through
Y and Z first. The `Source` field in the promoted trace carries the deduplicated sequence
(D5). If a session visits the same observer multiple times, that observer appears once
in `Source` (in order of first appearance). The trajectory is compressed but not erased.

**T172.2: `suggest` and LLM dependency.**
Without `suggest`, `meshant explore` is pure Go — no LLM call, no external dependency.
With `suggest`, it introduces the same LLM boundary as `meshant assist`. The LLM is a
mediator: its output transforms the cut into a navigational suggestion. That mediation
must be visible in the session record via `SuggestionMeta`. The LLM must not be treated
as a neutral oracle that produces suggestions without a known cut and provenance. `suggest`
is optional — an explore session without it is a fully valid analytical act.

**T172.3: Linear session only — branching deferred.**
First version supports linear cut refinement: one observer position at a time, sequential
turns, no forking. `AnalysisSession.turns` is a slice; the struct does not preclude a
tree-structured future. Branching (holding multiple live cuts, diffing them against each
other within a single session) is deferred to v5. This limitation means an analyst who
wants to compare two divergent inquiry paths must open two separate sessions or use
`diff`/`gaps` from within a single linear session.

**T172.4: `CutMeta.Observer` is singular; dual-observer results are a mismatch.**
`diff` and `gaps` span two observer positions. `CutMeta.Observer = current observer
(observer_a)`; `observer_b` lives in the `Result` payload only. This is the same
limitation as `serve-v1.md T5` and `mcp-v1.md T171.3`. Resolving it would require a
schema change to `CutMeta` (e.g. `Observers []string`). Not in v4.x.

**T172.6: Live substrate means the promoted trace is a record of acts, not a record of readings.**
D2 specifies that the `TraceStore` is queried live on each turn. If traces are added to
the store between turn N and turn N+1, those two turns operate on different substrates.
The `AnalysisSession` records each turn's cut parameters (`Observer`, `Window`, `Tags`,
`Command`) but not the substrate state at the time of each turn. The promoted
`AnalysisTrace` is therefore a record of *what the analyst did* (positions occupied,
commands issued) — not a fully reproducible record of *what the analyst saw* (the exact
network the cut revealed). Re-running the same parameters against the current store may
produce a different reading if the substrate has changed. This is analogous to T171.2
(MCP invocation traces alter the substrate that future invocations read) but is sharper
here because the explore session is explicitly framed as a recoverable analytical
trajectory. Named, not resolved: freezing the substrate at session open would introduce
snapshot-staleness and memory costs, and is deferred until a concrete use case demands it.

**T172.5: `Source` = observer positions conflates analytical positions with network elements.**
In the `schema.Trace` model, `Source` and `Target` are element names — actors in the
network. Using `Source` to record observer positions (D5) is a provisional encoding: it
captures the analytical trajectory in the nearest available field, but observer positions
are not network elements in the ANT sense. They are reading positions, not actants. A
future schema extension (e.g. `AnalyticalPositions []string` distinct from `Source`) would
resolve this cleanly. For v1, the `TagValueExplore` tag signals that `Source`/`Target`
should be interpreted differently for explore-promoted traces.

---

## Planned files

| File | Purpose | Issue |
|------|---------|-------|
| `meshant/explore/session.go` | `AnalysisSession`, `NewSession`, `Run` | #182 |
| `meshant/explore/turn.go` | `AnalysisTurn`, `SuggestionMeta` types | #182 |
| `meshant/explore/commands.go` | Command dispatch: articulate, shadow, follow, bottleneck, window, tags, help | #183 |
| `meshant/explore/commands_dual.go` | Dual-observer commands: diff, gaps | #184 |
| `meshant/explore/suggest.go` | `suggest` command + `SuggestionMeta` population | #185 |
| `meshant/explore/trace.go` | `AnalysisTrace`, `Promote()`, `TagValueExplore` | #186 |
| `meshant/explore/explore_test.go` | Session, command, and promotion tests | #182–#186 |
| `meshant/cmd/meshant/cmd_explore.go` | CLI glue: flag parsing, store opening, `session.Run` | #182 |

Modified:

| File | Change | Issue |
|------|--------|-------|
| `meshant/cmd/meshant/main.go` | Add `"explore"` case + usage line | #182 |

---

## Definition of done (for #182–#186)

- [ ] `meshant explore traces.json` opens an interactive session
- [ ] `cut <observer>` changes the session observer; subsequent turns use the new position
- [ ] Each turn records `Observer`, `Window`, `Tags`, `Command`, `Reading`, `ExecutedAt`
- [ ] `articulate`, `shadow`, `follow`, `bottleneck` produce results consistent with direct Go API (fidelity test)
- [ ] `diff <observer-b>` and `gaps <observer-b>` produce dual-observer results
- [ ] `window` and `tags` affect future turns only; past turns unchanged
- [ ] `suggest` refuses without prior cut context; produces output with `SuggestionMeta`
- [ ] `save` promotes session to `TraceStore`; `quit` without `save` discards
- [ ] Promoted trace carries `TagValueExplore`, `Source` = observer sequence, `Target` = final articulation elements
- [ ] `go test ./...` passes, `go vet ./...` clean
- [ ] ANT tensions T172.1–T172.6 documented in code and this record
- [ ] Branching deferred and documented (T172.3)
- [ ] Session branching, SSE transport, `CutMeta` multi-observer field deferred and documented

---

## Deferred items

| Item | Why deferred | Where noted |
|------|-------------|-------------|
| Session branching (tree-structured turns) | Requires branching turn history, inter-branch diff UX. v5 scope. | T172.3 |
| SSE transport for remote sessions | Same auth/multi-client concerns as MCP SSE (`mcp-v1.md D7`). | Inherited |
| `CutMeta.Observers []string` | Schema change affecting all surfaces. Not a v4.x change. | T172.4 |
| `/session/{id}` Web UI endpoint | Web UI cannot display explore session provenance without a new endpoint. | `web-ui-v1.md T3` |
| `PromptHash` in `SuggestionMeta` | `suggest` uses LLM prompts; hash should be recorded for reproducibility. | Deferred items batch (PR #170) |
| `AnalyticalPositions []string` in schema | Clean separation of observer positions from network elements in promoted traces. | T172.5 |
| Substrate snapshot per turn | Freezing store state per-turn would make readings reproducible from the record alone, at cost of memory/complexity. | T172.6 |

---

*This record is the ANT gate for issue #182 (AnalysisSession types + REPL skeleton).*
*Implementation must not begin until this record is reviewed and marked aligned.*
