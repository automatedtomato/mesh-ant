# M7 Plan — Graph Serialisation + Reflexive Tracing (release v0.3.0)

M7 closes the two gaps named at the end of M6.

**M7-A (serialisation):** A `MeshGraph` or `GraphDiff` produced in one process vanishes when
the process ends. A `meshgraph:<uuid>` reference in a trace points to nothing. Serialisation
makes graphs true immutable mobiles — forms that travel without deforming.

**M7-B (reflexive tracing):** The framework can articulate and diff, but it does not record
those acts as traces. Principle 8 ("the designer is inside the mesh") remains open. M7-B closes
the loop: the act of articulation becomes a trace; the resulting graph enters the mesh as an
actant.

M7-B depends on M7-A: the reflexive trace references an identified graph by its graph-ref
string, which is only meaningful if the graph can be recovered.

---

## Design decisions (resolved before implementation)

### D1 — Codec scope: codec only, no file persistence

M7-A provides `json.Marshal`/`Unmarshal` support for `MeshGraph` and `GraphDiff`. File
persistence (write identified graph to disk by UUID, load by ID) is explicitly deferred.
The caller is responsible for writing and reading the JSON bytes. This keeps M7-A minimal and
avoids coupling the graph package to a specific storage convention.

### D2 — TimeWindow zero bounds serialise as null

A zero `time.Time` marshals as `"0001-01-01T00:00:00Z"`, which is correct but misleading: zero
means "no bound", not the date 1 AD. `TimeWindow` gets a custom `MarshalJSON`/`UnmarshalJSON`
that serialises zero bounds as `null` and unmarshals `null` back to zero. The public API
(`time.Time` fields, `IsZero()`) is unchanged.

### D3 — Observer for reflexive traces is caller-supplied

The framework does not claim a fixed observer position. Every act of articulation is situated,
and the caller knows from which position they are articulating. Both `ArticulationTrace` and
`DiffTrace` take an explicit `observer string` parameter. This is consistent with the
observer-as-parameter design of `ArticulationOptions`.

### D4 — Source for ArticulationTrace is caller-supplied, absent by default

The input to an articulation is `[]schema.Trace` — a raw slice with no collective identity.
The traces are the material the act works on, not an actor that initiated it. Absent source is
the correct default (the act emerged from the framework without a named prior actor). Callers
who have a stable dataset identifier may pass it; nil means absent source. This is the first
absent-source trace produced by the framework itself, consistent with the absent-source
convention established in M1.

### D5 — Source for DiffTrace is derived from the input graphs

Unlike ArticulationTrace, a diff has two identified prior actors: the two `MeshGraph` values
it compared. Their graph-ref strings are the natural source. `DiffTrace` takes `g1, g2
MeshGraph` alongside the diff so it can derive `[GraphRef(g1), GraphRef(g2)]`. Both g1 and
g2 must be identified (non-empty ID); the function errors if either is empty.

### D6 — Tag for articulation acts: "articulation"

A new tag value `TagValueArticulation = "articulation"` is added to the schema vocabulary.
This is the first vocabulary addition since M1. Rationale: reusing `"translation"` would be
technically defensible (articulation is a translation in ANT terms), but `"articulation"` is
more precise and names the specific act the framework performs. The schema vocabulary remains
an open list; this is an addition, not a closure.

### D7 — what_changed for reflexive traces

`ArticulationTrace`: derived from `g.Cut` — observer positions and time window, in a compact
human-readable form. Example: `"articulate: observer=[meteorological-analyst] window=2026-04-14"`.

`DiffTrace`: derived from the From/To cuts of d. Example:
`"diff: [meteorological-analyst]→[local-mayor]"`.

Both are internal derivations — callers do not supply `what_changed`.

### D8 — Mediation for reflexive traces

`ArticulationTrace`: `"graph.Articulate"`
`DiffTrace`: `"graph.Diff"`

The function itself is the mediator. It transformed the inputs into the output.

---

## Sub-tasks

### M7.1 — JSON codec

**Branch:** `feat/m7-codec`

**New file: `meshant/graph/serial.go`**

- Add json struct tags to all exported types: `MeshGraph`, `GraphDiff`, `Node`, `Edge`, `Cut`,
  `ShadowElement`, `ShadowShift`, `PersistedNode`, `TimeWindow`
- Custom `TimeWindow.MarshalJSON`: marshal each bound as `null` if zero, RFC3339 string if not
- Custom `TimeWindow.UnmarshalJSON`: unmarshal `null` back to zero `time.Time`
- No changes to any existing function signatures or type definitions — tags only plus
  TimeWindow codec methods

**New file: `meshant/graph/serial_test.go`**

- Round-trip `MeshGraph`: identified, unidentified (empty ID), zero TimeWindow, non-zero
  TimeWindow, half-open window (Start only, End only)
- Round-trip `GraphDiff`: with and without ShadowShifts, with both From/To cuts
- JSON snapshot: one test that marshals a known MeshGraph and asserts the exact JSON string,
  pinning the output format against accidental changes
- TimeWindow nil/null round-trip: zero → null → zero; non-zero → string → non-zero
- Unmarshal error paths: invalid JSON, wrong type for time fields

**Coverage target:** 100% of serial.go

---

### M7.2 — Reflexive tracing

**Branch:** `feat/m7-reflexive`

**Schema addition: `meshant/schema/trace.go`**

- Add `TagValueArticulation TagValue = "articulation"` to the tag vocabulary constants
- Add one test to `meshant/schema/trace_test.go` covering the new constant (validate a trace
  with tag `"articulation"`)

**New file: `meshant/graph/reflexive.go`**

```go
// ArticulationTrace produces a Trace recording the act of articulation.
// g must be identified (non-empty g.ID). observer is the caller's position.
// source may be nil — absent source is the correct default when the input
// traces have no collective identity. The produced trace always passes
// schema.Validate().
func ArticulationTrace(g MeshGraph, observer string, source []string) (schema.Trace, error)

// DiffTrace produces a Trace recording the act of diffing two graphs.
// d, g1, and g2 must all be identified (non-empty ID). Source is derived
// as [GraphRef(g1), GraphRef(g2)]. observer is the caller's position.
func DiffTrace(d GraphDiff, g1, g2 MeshGraph, observer string) (schema.Trace, error)
```

Internal behaviour:
- Both functions generate a fresh UUID4 for the trace ID
- Both use `time.Now()` for Timestamp
- `ArticulationTrace` target: `["meshgraph:<g.ID>"]`; tags: `["articulation"]`;
  mediation: `"graph.Articulate"`; what_changed: derived from g.Cut
- `DiffTrace` target: `["meshdiff:<d.ID>"]`; source: `["meshgraph:<g1.ID>", "meshgraph:<g2.ID>"]`;
  tags: `["articulation"]`; mediation: `"graph.Diff"`; what_changed: derived from d.From/d.To

**New file: `meshant/graph/reflexive_test.go`**

- `ArticulationTrace`: produced trace passes `schema.Validate()`; target contains graph-ref;
  tags contain `"articulation"`; mediation is `"graph.Articulate"`; observer matches input;
  absent source (nil) produces valid trace; non-nil source appears in trace
- `DiffTrace`: produced trace passes `schema.Validate()`; source contains both graph-refs;
  target contains diff-ref; tags, mediation, observer correct
- Error cases: unidentified graph (empty ID) → error; empty observer string → error (schema
  requires observer); unidentified g1 or g2 in DiffTrace → error

**Coverage target:** 100% of reflexive.go

---

### M7.3 — Decision record

**Branch:** can be bundled with `feat/m7-reflexive` or as a separate commit on develop.

**New file: `docs/decisions/m7-serialisation-reflexivity-v1.md`**

Decisions to record:
1. Codec only — no file persistence in M7
2. TimeWindow null convention and rationale
3. Observer as parameter — not a fixed framework identity
4. Source absent by default for ArticulationTrace — rationale (traces have no collective identity)
5. Source derived for DiffTrace — rationale (prior graphs are natural actors)
6. New tag `"articulation"` — first schema vocabulary addition since M1; why not `"translation"`
7. Mediation = function name
8. What M7 does not close: file persistence, registry, auto-recording (recording is still a
   curatorial act — the caller must call ArticulationTrace explicitly)

---

## Merge chain and release

```
feat/m7-codec       → develop
feat/m7-reflexive   → develop
develop             → main
git tag v0.3.0
```

Release notes should name:
- Graph serialisation (JSON round-trip for MeshGraph and GraphDiff)
- Reflexive tracing (ArticulationTrace, DiffTrace)
- New tag value: "articulation"
- Known remaining gap: file persistence (no WriteGraph/ReadGraph helpers yet)
- Known remaining gap: auto-recording is still not done — recording is a curatorial act

---

## What M7 does not do

- **No file persistence.** Callers marshal to `[]byte` and decide where to store it.
- **No auto-recording.** The framework does not automatically call `ArticulationTrace` after
  every `Articulate`. Recording is an explicit curatorial act. This is intentional: Principle 8
  says the designer is inside the mesh, but it does not say every act must be recorded.
- **No registry.** There is still no package-level map of ID → MeshGraph.
- **No new cut axes.** Time-window and observer-position remain the only cut types.
- **No CLI.** Form factor is still unforced.
