# Plan: v4.0.0 — MCP Server

**Date:** 2026-03-25
**Parent issue:** #171
**Status:** Issues open — not yet started
**Follows:** v3.1.0 (deferred items batch)

---

## What

Expose MeshAnt's analytical commands as MCP (Model Context Protocol) tools. Any LLM client
(Claude Code, Cursor, Cline, etc.) can call `articulate`, `diff`, `shadow`, `gaps`, `follow`,
`bottleneck`, `summarize`, and `validate` without the analyst writing shell commands.

The MCP server is a new surface for MeshAnt's existing analytical engine — not new analytics.

---

## Design decisions (confirmed after two-round planner + architect + ant-theorist discussion)

### Two-level observer model

`CutMeta` carries two observer fields:
- `Observer string` — the cut-axis observer (whose traces are being read)
- `Analyst string` — the session observer (who is doing the reading, i.e. the human or LLM
  calling the MCP tool)

These are not the same. Collapsing them hides the cut. `Analyst` is the `--observer` value
passed at server startup; `Observer` is the per-call input parameter.

### `--observer` required flag

The MCP server refuses to start without `--observer`. An LLM client calling MeshAnt without
declaring an analyst position is not performing an articulation — it is hiding the cut.
Hard requirement, not soft.

### MCP invocation traces (mandatory, Principle 8)

Every cut-producing tool call produces a reflexive trace stored in the `TraceStore`. The MCP
server becomes a visible actant in the mesh. Tags: `["mcp-invocation", tool-name]`.
`meshant_validate` is exempt (no cut produced).

### Constructor pattern

```go
// meshant/mcp/server.go
func NewServer(ts store.TraceStore, observer string) *Server
```

`ts` is injected. The MCP package does not import `cmd/meshant/main.go`'s `loadTraces`.

### Shared infrastructure: graph/envelope.go

`CutMeta`, `Envelope`, and `cutMetaFromGraph` are extracted from `meshant/serve/response.go`
to `meshant/graph/envelope.go`. Both `serve` and `mcp` (and later `explore`) import from there.
This is the critical prerequisite — without it, the packages produce incompatible wrappers.

### Tool set

**Batch 1 (single-observer, read-heavy):**
`meshant_articulate`, `meshant_shadow`, `meshant_follow`, `meshant_bottleneck`,
`meshant_summarize`, `meshant_validate`

**Batch 2 (dual-observer):**
`meshant_diff`, `meshant_gaps`

**Deferred (write tools, interactive-CLI context):**
`meshant_extract`, `meshant_assist`, `meshant_critique`

**Transport:** stdio first. SSE (network-accessible) deferred — not a v4.0.0 requirement.

### Fidelity guarantee

The MCP tool result for any read command must be structurally identical to the result from
the direct Go API (`graph.*` functions) wrapped in the same `Envelope`. A test in
`meshant/mcp/mcp_test.go` asserts this for each tool.

---

## Known tensions (documented, not resolved)

**T171.1** — Two-level observer model: `Analyst` field is new to `CutMeta`. `serve` package
has no Analyst (it does not have a session-level observer). The field is optional for `serve`,
required for `mcp`. Needs careful handling at the `graph/envelope.go` extraction point.

**T171.2** — Invocation traces as new actants: storing a trace for every MCP call means the
MCP server itself becomes a mediator visible in the mesh. This is the correct ANT outcome.
The tension is that heavy use creates trace noise. The decision record (#175) must address
this directly.

**T171.3** — `CutMeta.Observer` single string for dual-observer results: `diff` and `gaps`
span two observer positions. `CutMeta.Observer = observer_a`; `observer_b` lives in the result
payload. Known limitation, documented in code comments, not fixed in v4.

**T171.4** — SSE transport deferred: stdio requires a process per client. SSE would allow
network-accessible use. Deferred because it requires authentication design (observer-position
discipline over HTTP/SSE is not trivial). Document the gap.

---

## Child issues

| # | Type | Scope | Notes |
|---|------|-------|-------|
| #174 | refactor | Extract CutMeta/Envelope to graph/envelope.go; add Analyst field | Shared prereq with #172 |
| #175 | docs | mcp-v1.md decision record | **ANT gate** before #176 |
| #176 | feat | MCP server skeleton + meshant_articulate + meshant mcp CLI | Fidelity test required |
| #177 | feat | MCP tools batch 1 (shadow, follow, bottleneck, summarize, validate) | |
| #178 | feat | MCP tools batch 2 (diff, gaps — dual-observer) | T171.3 documented in code |
| #179 | feat | MCP invocation trace recording (mandatory) | **ANT gate** required |

---

## Sequencing

```
#174 (CutMeta extraction) → #175 (decision record, ANT gate)
                          → #176 (skeleton + articulate)
                               → #177 (batch 1)
                                    → #178 (batch 2)
                                         → #179 (invocation traces, ANT gate)
```

#174 and #175 can proceed in parallel (different types of work).

---

## What does not change

- The `graph.*` functions are not modified. MCP is an adapter, not a replacement.
- The `serve` package is not modified (except to import from `graph/envelope.go`).
- Observer-position discipline is enforced at the tool schema level (required parameter), not as a runtime suggestion.
- No god's-eye tools. Every read tool requires a declared observer position.

---

*This is a plan — a provisional articulation. It will be revisited after #175 (decision record).*
*Before any phase begins: confirm design, run per-issue pipeline.*
