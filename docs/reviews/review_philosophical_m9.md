# Philosophical Review — M9 (pre-v1.0.0)

## Scope

M9 additions reviewed:
- `meshant/cmd/meshant/main.go` and `main_test.go` — CLI subcommands (summarize, validate, articulate, diff)
- `docs/authoring-traces.md` — trace authoring guide
- `docs/decisions/cli-v1.md` — CLI design decisions
- `README.md` — "Who is this for?" section and CLI usage examples

Full library also reviewed per SKILLS.md process:
- `meshant/schema/trace.go`, `meshant/schema/graphref.go`
- `meshant/loader/loader.go`
- `meshant/graph/graph.go`, `meshant/graph/diff.go`, `meshant/graph/actor.go`,
  `meshant/graph/reflexive.go`, `meshant/graph/export.go`

---

## Agent A findings (trace-first and emergence)

**None.**

- No pre-defined actor types, role enumerations, or `AgentType`-style fields anywhere in schema, loader, or CLI.
- `Source` and `Target` on `Trace` are open `[]string` — the CLI flags pass through these slices unchanged.
- CLI `cmdArticulate` and `cmdDiff` require `--observer` flags; they do not accept a named actor and query traces about it. They accept traces and let the articulation surface what acts.
- `IdentifyGraph` and `IdentifyDiff` are explicit opt-ins with clear doc comments warning the caller to call them only when they intend to use the graph as an actor. No automatic identity assignment occurs anywhere in the library or CLI.
- No registry, map, or singleton accumulates actor identities across cuts.
- `docs/authoring-traces.md` gives no actor types; it gives trace field guidance and warns against `"ground-truth"` or `"objective-observer"` strings in the `observer` field.

---

## Agent B findings (articulation and cuts)

**Two violations found and fixed (see table below).**

### Violation B1 — `timeWindowLabel` used `"(none — no time filter)"`

In `meshant/graph/graph.go`, `timeWindowLabel()` returned `"(none — no time filter)"` for a zero `TimeWindow`. The phrase "no time filter" implies a neutral absence — an unset filter rather than a deliberate positional choice. Per the SKILLS.md articulation-first principle, the full-cut position must be named as a deliberate choice. The label for a zero time window should parallel `"(all — full cut)"` used for observer positions.

**Fix:** Changed to `"(none — full temporal cut)"` and added a doc comment explaining the rationale.

### Violation B2 — `dotCutComment` used `"no window"`

In `meshant/graph/export.go`, `dotCutComment()` set `win := "no window"` for a zero `TimeWindow`. This string appears in the `// observer: ... | window: ...` comment line at the top of every DOT and Mermaid output. "no window" implies neutral absence.

**Fix:** Changed to `win := "full temporal cut"` and added an inline comment explaining the same rationale.

### Other Agent B checks — clean

- `"(all — full cut)"` was already the correct label for the zero observer-positions case in `PrintArticulation` and `cutSummaryLines` in `diff.go`.
- Shadow sections are mandatory in both `PrintArticulation` and `PrintDiff` — they are emitted unconditionally, with `"(none — full cut taken)"` when the shadow is empty.
- `Cut` values are deep-copied in `Diff` via `copyCut`, and `ObserverPositions` / `Edge` slices are defensively copied in `Articulate`. Immutability is maintained.
- README "Who is this for?" presents articulations as positioned cuts: "without claiming a god's-eye view." No completeness language.
- Demo description in README explicitly names both absences as "a provisional reading, not a god's-eye account."
- `PrintSummary` footer: "this is a first look at the mesh, not a classification of actors."
- `PrintArticulation` footer: "this graph is a cut made from one position in the mesh."
- `PrintDiff` footer: "this diff is a comparison between two situated cuts, not an objective account of change."
- `docs/authoring-traces.md` warns against `"ground-truth"` or `"objective-observer"` observer strings.

---

## Agent C findings (symmetry and reflexivity)

**None.**

### Generalised symmetry
- Graph-reference strings (`meshgraph:<uuid>`, `meshdiff:<uuid>`) flow through the same `[]string` Source/Target paths as plain element names. In `Summarise` (loader), in `buildShadowData`, `buildNodes`, `buildEdges`, and `computeNodeDiff`, graph-refs are processed in the same loops as plain strings — no special branches.
- The `schema.IsGraphRef` predicate is used only to extract graph-refs into `MeshSummary.GraphRefs` for additional display; the count also appears in `Elements` (the `PrintSummary` header says "also counted in Elements above").
- No field, type, or branch differentiates human from non-human elements.

### Observer inside the mesh
- `Observer` is a required field on `Trace` and is validated by `Trace.Validate()`. The CLI's `--observer` flag is mandatory; `cmdArticulate` returns an error if no `--observer` is provided, and `stringSliceFlag.Set` rejects empty values. No defaulting or optional treatment anywhere.
- `ArticulationTrace` and `DiffTrace` are available for re-entering produced graphs into the mesh as actants. Both require an explicit `observer` argument. The CLI does not call them (deliberate: reflexive tracing is an opt-in library concern, not a CLI default), but the path to reflexivity is open.
- Every articulation-producing function (`Articulate`, `Diff`) stores the full `Cut` in the output, enabling callers to call `IdentifyGraph`/`IdentifyDiff` and then `GraphRef`/`DiffRef` to re-enter the result into the mesh.

---

## Violations fixed

| Violation | Principle | Fix |
|-----------|-----------|-----|
| `timeWindowLabel` returned `"(none — no time filter)"` — implies neutral absence, not deliberate full-temporal cut | Articulation-first (principle 2) | Changed to `"(none — full temporal cut)"` in `graph.go`; added doc comment |
| `dotCutComment` used `win := "no window"` in DOT/Mermaid comment — implies neutral absence | Articulation-first (principle 2) | Changed to `"full temporal cut"` in `export.go`; added inline comment |

---

## Tensions (not violations)

### T1 — CLI reflexivity is opt-out, not opt-in-visible

The CLI does not surface `ArticulationTrace` or `DiffTrace` as subcommands. Users who want to re-enter a produced graph into the mesh must use the library directly. This is methodologically defensible (reflexive tracing is an explicit opt-in by design — M5 decision record), but the CLI's silence on the pattern means a user reading only the CLI help text will not discover that graphs can become actants. This is a tension between usability (keeping the CLI simple) and reflexivity (making the mesh's self-referential capability discoverable). Worth tracking; not a violation.

### T2 — `FlaggedTraces` tag selection remains a provisional cut

The `Summarise` function flags only `delay` and `threshold` tags, not `blockage`, `redirection`, `amplification`, or `translation`. The doc comment on `FlaggedTraces` already names this as "a provisional cut, not a taxonomy" and lists all six types. The `docs/authoring-traces.md` guide lists all six tag types. The tension is that users who only see the `meshant summarize` output may not know that the four unflagged types are also present in their dataset. This was named and accepted in M1–M5; it remains a tracked tension, not a violation.

### T3 — `"(none — full temporal cut)"` vs `"(all — full cut)"` naming asymmetry

Observer positions use `"(all — full cut)"` and time window now uses `"(none — full temporal cut)"`. The "all" / "none" asymmetry reflects a real semantic difference — for observer positions, "all" means every observer is included; for time window, "none" means no bound is set. The labels are correct and internally consistent, but the surface asymmetry could generate questions. Noted as a tension for future terminology consistency work.

---

## Verdict

VIOLATION FOUND — REFACTORED

Two articulation-first violations in the time-window label strings were fixed. No trace-first, emergence, symmetry, or reflexivity violations were found. Three tensions are named and tracked; none reach the level of violation.
