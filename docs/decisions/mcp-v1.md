# Decision Record: `meshant mcp` MCP server (v1)

**Issue:** #175 (decision record) → #176 (implementation)
**Branch:** `175-mcp-v1-decision-record`
**Phase:** v4.0.0 — MCP server (parent: #171)
**ANT gate:** This record must be reviewed and aligned before #176 begins.

---

## Problem

MeshAnt's analytical engine (`articulate`, `diff`, `shadow`, `gaps`, `follow`,
`bottleneck`, `summarize`, `validate`) is currently accessible only via shell
commands. An LLM client (Claude Code, Cursor, Cline) can call MeshAnt only by
invoking subprocesses — it cannot compose multi-step analysis across commands or
carry context between calls.

The Model Context Protocol (MCP) addresses this: tools registered with an MCP
server are callable by any compliant LLM client without shell invocation. The
problem is not LLM access per se — the problem is that LLM-driven analysis must
remain observer-positioned. An LLM calling MeshAnt without declaring analyst
position is hiding the cut.

---

## Decision: `meshant mcp` + `meshant/mcp` package

Two additions:

1. **`meshant mcp --analyst <name> [--db bolt://...] [<traces.json>]`** —
   starts an MCP server (stdio transport) that exposes MeshAnt's analytical
   commands as tools. `--analyst` is required; the server refuses to start
   without it.

2. **`meshant/mcp` package** — contains `Server` struct and tool handlers.
   The CLI (`cmd_mcp.go`) is thin glue; all tool logic lives in the package
   and is testable without a live MCP connection.

---

## Design decisions

### D1: Two-level observer model — Observer and Analyst are distinct

`CutMeta` carries two observer fields (added in #174):

- **`Observer string`** — the ANT position from which the graph was articulated;
  whose traces are being read. This is an epistemic location in the network.
- **`Analyst string`** — the human or agent that requested the cut; who is doing
  the reading. This is the `--analyst` value passed at server startup.

These are not the same. A researcher studying traces from the `sensor-floor-3`
observer position may themselves be analyst `dr-smith`. Collapsing them into one
field hides the cut: it conflates what was recorded with who is reading.

For `meshant serve`, `Analyst` is empty — the HTTP server has no session-level
reader identity. For `meshant mcp`, `Analyst` is the `--analyst` flag value,
set on every tool response. The MCP client knows which human or agent declared
this reading position.

### D2: `--analyst` is required — hard refusal, not soft suggestion

The MCP server refuses to start without `--analyst`. An LLM client calling
MeshAnt without declaring an analyst position is not performing an articulation —
it is hiding the cut behind an apparent capability. The error message must state
this explicitly:

```
meshant mcp: --analyst is required
An LLM client calling MeshAnt without declaring an analyst position is not
performing an articulation — it is hiding the cut. Provide --analyst <name>.
```

This is not a soft warning. Observer-position discipline is enforced at the
schema level (required parameter), not as a runtime suggestion.

### D3: Constructor pattern — injected TraceStore

```go
// meshant/mcp/server.go
func NewServer(ts store.TraceStore, analyst string) *Server
```

`ts` is injected. The `mcp` package does not import `cmd/meshant/main.go`'s
`loadTraces`. This mirrors `serve.NewServer` and `explore.NewSession` — all
three analytical surfaces share the same constructor discipline.

### D4: Tool set

**Batch 1 — single-observer, read-heavy (issue #177):**
- `meshant_articulate` — build the mesh graph from an observer's position
- `meshant_shadow` — find actors not visible from this observer
- `meshant_follow` — trace a translation chain from an element
- `meshant_bottleneck` — identify high-mediation actors
- `meshant_summarize` — narrative summary of the mesh
- `meshant_validate` — validate traces (no cut produced; exempt from invocation traces)

**Batch 2 — dual-observer (issue #178):**
- `meshant_diff` — diff two observer cuts
- `meshant_gaps` — find extraction gaps between two observer positions

**Deferred (write tools; require interactive session context):**
- `meshant_extract`, `meshant_assist`, `meshant_critique` — not in v4.0.0.
  Write tools involve TraceDraft creation and session provenance; they presuppose
  the interactive session model that `meshant explore` (v4.x) establishes.

### D5: MCP invocation traces — mandatory Principle 8 reflexivity

Every cut-producing tool call writes a reflexive trace to the `TraceStore`.
The MCP server is a mediator — it transforms how MeshAnt is accessed, which
observer positions are invoked, which tools are called. That mediation must be
visible in the mesh, not hidden behind a service facade.

Stored trace tags: `["mcp-invocation", "<tool-name>"]`

`meshant_validate` is exempt: it checks traces for structural validity without
producing a cut. No graph is articulated; no observer position is taken. Nothing
to record.

The implementation consequence: the `TraceStore` injected into `NewServer` must
be writable (able to `Upsert`). For read-only stores, the server logs the
failure but does not abort the tool response — the analytical result is still
returned. The failure to record is itself an absence worth naming; it is logged,
not silenced.

### D6: Fidelity guarantee

The MCP tool result for any read command must be structurally identical to the
result from the direct Go API (`graph.*` functions) wrapped in the same
`graph.Envelope`. The MCP layer is an adapter, not an analytical replacement.

A test in `meshant/mcp/mcp_test.go` asserts this for each tool: call the tool,
call the underlying Go function directly, compare the `Envelope.Data` contents.
If they diverge, the MCP surface has introduced a transformation that is not
attributable to any ANT position — the tool is producing an unattributable reading.

### D7: stdio transport first; SSE deferred

The MCP server speaks stdio. One client per server process. This is sufficient
for the primary use case (Claude Code, Cursor, Cline running `meshant mcp` as
a subprocess).

SSE (Server-Sent Events) would allow a network-accessible MCP server — one
process serving multiple clients. SSE is deferred because it requires:

1. **Authentication design**: observer-position discipline over HTTP/SSE is not
   trivial. Who sets `--analyst` when multiple clients connect? Does each client
   session have its own analyst identity? These questions do not have obvious
   answers from the current design and must not be decided in a hurry.

2. **Invocation trace fan-out**: with multiple clients, invocation traces from
   different analysts accumulate in the same store. This may be desirable, but
   the trace tagging schema (`["mcp-invocation", tool-name]`) would need an
   `analyst` tag to distinguish them.

Document the gap. Do not implement SSE in v4.0.0.

---

## Response envelope

Every cut-producing tool returns a `graph.Envelope`:

```json
{
  "cut": {
    "observer":    "string (required — the ANT position)",
    "analyst":     "string (required for mcp — who is reading)",
    "from":        "RFC3339 or absent",
    "to":          "RFC3339 or absent",
    "trace_count": 42,
    "shadow_count": 7
  },
  "data": { ... }
}
```

Note: `tags` is absent when no tag filter was applied (`omitempty`).

The `analyst` field is always present in MCP tool responses (set to the
`--analyst` value). This distinguishes MCP envelopes from HTTP server envelopes
(where `analyst` is absent).

---

## ANT tensions

**T171.1: `Analyst` optional for serve, required for mcp.**
`CutMeta.Analyst` is `omitempty` — absent in HTTP responses, present in MCP
responses. This asymmetry is intentional: the HTTP server has no session-level
analyst identity (it is a stateless service). The MCP server has one (declared
at startup). Future work may make the HTTP server accept an `?analyst=` parameter
for audit purposes. Not in v4.0.0.

**T171.2: Invocation traces as new actants — trace noise at scale.**
Storing a trace for every MCP tool call means the MCP server becomes a mediator
visible in the mesh. This is the correct ANT outcome: the server is not
transparent infrastructure; it transforms how the network is read, by whom, and
through which tools. The tension is that heavy use (many tool calls in a single
analysis session) creates trace density that may obscure the original network
structure when browsing. Mitigation: invocation traces are tagged
`"mcp-invocation"` and can be filtered out of analytical calls. The tag exists
precisely to enable this. The traces are not hidden; they are nameable.

**T171.3: `CutMeta.Observer` is singular; dual-observer tools are a mismatch.**
`meshant_diff` and `meshant_gaps` span two observer positions.
`CutMeta.Observer = observer_a`; `observer_b` lives in the result payload.
This is the same limitation documented in `serve-v1.md` (T5). The MCP layer
does not resolve it — it inherits the same structural gap. Document the
asymmetry in code comments on the diff/gaps tool handlers. Not fixed in v4.

**T171.5: The MCP layer is intermediary-like for results but mediator-like for access.**
D6 describes the MCP layer as an "adapter" — faithful transport of analytical
results, structurally identical to the direct Go API. D5 describes the MCP server
as a mediator — it transforms how MeshAnt is accessed, by whom, through which
tools, and records that transformation. Both are correct, but they apply to
different aspects. The word "adapter" in D6 must not be read as a claim of pure
intermediation. At the level of analytical results, the layer is transparent
(fidelity guarantee). At the level of access conditions — who calls, when, from
what analyst position — the layer is a mediator whose actions are inscribed in
invocation traces. The dual character is not a contradiction; it is a property
of any well-designed translation layer in ANT terms.

**T171.4: stdio requires a process per client.**
Each MCP client that connects to `meshant mcp` spawns a new server process
with its own `--analyst` value and its own store connection. This is correct
for the current use case and supports strong per-analyst isolation. It is not
a scalability problem at the analyst-workstation level. It becomes a problem
if MeshAnt is deployed as a shared service — that is an SSE/authentication
problem, documented in D7.

---

## Planned files

| File | Purpose |
|------|---------|
| `meshant/mcp/server.go` | `Server` struct, `NewServer(ts, analyst)`, tool registration |
| `meshant/mcp/tools_batch1.go` | `meshant_articulate`, `meshant_shadow`, `meshant_follow`, `meshant_bottleneck`, `meshant_summarize`, `meshant_validate` |
| `meshant/mcp/tools_batch2.go` | `meshant_diff`, `meshant_gaps` (dual-observer) |
| `meshant/mcp/invocation.go` | Reflexive trace recording; `recordInvocation(ctx, ts, toolName, meta)` |
| `meshant/mcp/mcp_test.go` | Fidelity tests — MCP result == direct Go API result for each tool |
| `meshant/cmd/meshant/cmd_mcp.go` | `cmdMcp` — flag parsing, store opening, server lifecycle |
| `meshant/cmd/meshant/cmd_mcp_test.go` | Flag/error tests for `cmdMcp` |

Modified:

| File | Change |
|------|--------|
| `meshant/cmd/meshant/main.go` | Add `"mcp"` case + usage line |

---

## Definition of done (for #176–#179)

- [ ] `meshant mcp --analyst alice traces.json` starts server on stdio
- [ ] `meshant mcp` without `--analyst` → refuses to start with ANT-reasoning error
- [ ] All cut-producing tool responses carry `analyst` field in `cut` metadata
- [ ] `meshant_validate` exempt from invocation traces
- [ ] Every other tool call writes a reflexive trace tagged `["mcp-invocation", tool-name]`
- [ ] Fidelity test passes for each tool (MCP result == direct Go API result)
- [ ] `go test ./...` passes, `go vet ./...` clean
- [ ] ANT tensions documented (T171.1–T171.5)
- [ ] SSE deferred and documented in D7

---

*This record is the ANT gate for issue #176 (MCP server skeleton + meshant_articulate).*
*Implementation must not begin until this record is reviewed and marked aligned.*
