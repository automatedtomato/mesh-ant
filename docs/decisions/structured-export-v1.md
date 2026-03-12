# M8 Decision Record — Structured Export + Second Domain (v0.4.0)

## Context

M8 closes two gaps remaining after v0.3.0:

1. **No structured output** — `MeshGraph` could round-trip to JSON (M7 codec), but there
   was no function to write that JSON to an `io.Writer`. No DOT or Mermaid output existed.
   Tools that consume graphs (jq, Graphviz, Mermaid renderers) had no path to reach them.

2. **One demo domain** — Only the evacuation order dataset shipped. A framework that only
   works for one hand-crafted scenario is a framework that hasn't been tested for generality.

---

## Decision 1 — Export functions in the `graph` package; file I/O in a separate `persist` package

`PrintGraphJSON`, `PrintDiffJSON`, `PrintGraphDOT`, and `PrintGraphMermaid` all live in
`meshant/graph/export.go`. They accept `io.Writer` and produce bytes — no file paths, no
`os.File`, no storage concerns.

File persistence (`WriteJSON`, `ReadGraphJSON`, `ReadDiffJSON`) lives in a new package
`meshant/persist/persist.go`.

Rationale: This is a direct continuation of M7 Decision 1 ("codec only — no file persistence
in graph package"). The graph package is responsible for articulation semantics and their
representations. Storage is a separate concern. Keeping file I/O out of the graph package
prevents the package from accumulating `os.*` dependencies and storage opinions.

---

## Decision 2 — DOT multi-source/multi-target edges render as Cartesian product

Each `Edge` in `MeshGraph` can have multiple `Sources` and multiple `Targets`. In DOT output,
each (source, target) pair in the Cartesian product becomes one arc, all sharing the same
edge label.

Example: an edge with `Sources: ["a", "b"]` and `Targets: ["x", "y"]` produces four arcs:
`"a" -> "x"`, `"a" -> "y"`, `"b" -> "x"`, `"b" -> "y"`.

Rationale: DOT has no native multi-source multi-target arc primitive. The Cartesian product
is the least surprising expansion. It is a **visualization simplification** — the lossless
representation remains available via `PrintGraphJSON`. The DOT comment block names the
articulation position so the cut is not hidden.

The same convention applies to Mermaid output.

---

## Decision 3 — Shadow elements are visible in DOT and Mermaid output

Shadow elements appear in a distinct visual cluster in DOT (`cluster_shadow`, dashed border,
grey colour) and in a `subgraph Shadow` block in Mermaid.

Rationale: The shadow is a first-class output of articulation — not an error state and not
optional metadata. Principle 5 (plural observers before god's-eye view) and the mandatory
shadow in `Cut` both reflect the view that what an observer cannot see is as important as
what they can see. Making shadow literally visible in diagrams is the graphical equivalent
of the shadow section in `PrintArticulation`. Omitting it from visualizations would
contradict the framework's core commitment.

---

## Decision 4 — Mermaid node ID sanitization: non-alphanumeric → underscore, digit prefix → n_

Mermaid node IDs cannot contain hyphens, spaces, or other non-alphanumeric characters.
MeshAnt element names routinely contain hyphens (e.g. `storm-sensor-network`,
`on-call-engineer`).

Sanitization rule:
- Replace any character outside `[a-zA-Z0-9_]` with `_`
- Prefix with `n_` if the sanitized ID starts with a digit
- Fall back to `n_empty` for the empty string

Collision resolution: if two names sanitize to the same ID (e.g. `a-b` and `a_b` both
become `a_b`), the later-sorted name gets a numeric suffix (`a_b_2`, `a_b_3`, etc.).

The original name is always preserved as the display label in square brackets. The
JSON export remains the lossless canonical representation.

---

## Decision 5 — Second domain: software incident response (e-commerce API outage)

The incident response dataset (`data/examples/incident_response.json`) covers a database
connection pool exhaustion during a flash sale. 22 traces, 2 days, 5 observer positions,
8 non-human actants, all 6 tag types.

Rationale for this domain: it is structurally different from the evacuation order scenario
in several ways that matter for the framework:

- **Temporal scale**: hours (Day 1), not days. Incident onset and mitigation happen within
  a single calendar day.
- **Friction type**: alert fatigue, runbook ambiguity, SLA timers — not political authority
  or physical infrastructure.
- **Observer asymmetry**: the monitoring system sees automated signals that human responders
  do not see until they are paged. The incident commander sees coordination and customer
  impact that the monitoring system never records.
- **Non-human actants**: `alerting-pipeline`, `circuit-breaker`, `auto-scaler`, `sla-timer`,
  `runbook-engine` — infrastructure actors that mediate without human direction.

The two demo cuts (`monitoring-service` Day 1, `incident-commander` Day 2) produce
near-disjoint graphs, confirming that the framework's structural blindness logic generalizes
across domains without code changes.

Note on shadow membership (Cut A): `on-call-engineer` and `incident-commander` appear as
targets of `monitoring-service` traces (the system pages them) and are therefore visible
nodes in the monitoring-service cut — not shadow elements. `product-manager` and
`database-admin` never appear in monitoring-service traces and are in shadow.

---

## Decision 6 — `WriteJSON` accepts `any`; `ReadGraphJSON`/`ReadDiffJSON` are typed

`persist.WriteJSON(path string, v any) error` is generic because the same write logic
applies to any JSON-serializable value. `ReadGraphJSON` and `ReadDiffJSON` are typed
because the read path needs to return a concrete type and cannot be usefully generic
without generics or reflection complexity.

This asymmetry is intentional: write is structurally simpler (marshal → write), read
is type-specific (read → unmarshal into concrete type). The `any` parameter means
`WriteJSON` can accept channels or functions at compile time — the marshal error is
therefore a real, reachable error path (unlike `PrintGraphJSON` where the type
constraint is enforced by the function signature).

---

## What M8 does not close

- **GraphDiff DOT/Mermaid**: `PrintDiffJSON` is available; DOT and Mermaid visualization
  for `GraphDiff` is deferred. Diff visualization requires rendering two cuts side by side
  with delta annotations — a non-trivial layout problem.
- **CLI**: no `meshant articulate`, `meshant diff` subcommands. The export functions are
  library-level; a CLI can wrap them in a future milestone.
- **Tag-filter cut axis**: deferred since M3, still not implemented. Observer-position and
  time-window remain the only cut axes.
- **Auto-recording**: `Articulate` still does not automatically call `ArticulationTrace`.
  Recording is a deliberate curatorial act (M7 Decision in `m7-serialisation-reflexivity-v1.md`).
- **Interactive visualization**: DOT and Mermaid are static outputs. A live graph laboratory
  remains a future direction (see `docs/reviews/review_12-mar-26.md`, Direction C).
