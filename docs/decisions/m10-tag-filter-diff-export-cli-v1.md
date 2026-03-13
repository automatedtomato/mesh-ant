# M10 Decision Record — Tag-Filter Cut Axis, Diff Visual Export, CLI Integration

## Context

M10 closes three items deferred across earlier milestones:

1. **Tag-filter cut axis** (deferred since M3) — observer-position and time-window were the
   only cut axes. Tags were captured on traces but could not constrain an articulation.

2. **GraphDiff DOT/Mermaid export** (deferred since M8) — `PrintGraphDOT` and
   `PrintGraphMermaid` existed for `MeshGraph`, but `GraphDiff` had no visual export path.

3. **CLI wiring** — the tag-filter and diff visual export needed CLI flags to be usable.
   The user also requested file output (`--output`) to write DOT/Mermaid directly to files
   for viewing in VS Code or other tools.

---

## Decision 1 — Tag filter uses any-match (OR) semantics

`ArticulationOptions.Tags` filters traces to those whose `Tags` slice contains **at least
one** of the listed tag strings (set-intersection / any-match). A trace passes if it carries
any of the specified tags.

Rationale: the tag vocabulary is open (`[]string`), and tags are descriptive categories, not
hierarchical filters. A trace tagged `["delay", "threshold"]` should pass when the user
asks for either `delay` or `threshold`. AND semantics (require all tags) would be overly
restrictive for exploratory analysis — most traces carry only one or two tags.

The empty tag slice means no filter (full tag cut), following the same pattern as the empty
observer slice (full observer cut) and zero time window (full temporal cut).

---

## Decision 2 — Tag filter is a third shadow reason

When a trace is excluded by the tag filter, all elements it mentions enter the shadow with
reason `"tag-filter"`. This is the third `ShadowReason` alongside `"observer"` and
`"time-window"`.

Shadow reasons are accumulated per element across all excluded traces. An element can have
multiple reasons (e.g., excluded by both observer and tag filter in different traces).
Reasons are sorted alphabetically: `observer`, `tag-filter`, `time-window`.

Rationale: the shadow must name *why* each element is invisible from this position. A
tag-filtered shadow is fundamentally different from an observer-filtered shadow — they
reflect different aspects of the cut's partiality.

---

## Decision 3 — GraphDiff DOT and Mermaid use visual conventions for delta semantics

`PrintDiffDOT` and `PrintDiffMermaid` encode diff semantics through visual conventions:

| Element | DOT | Mermaid |
|---------|-----|---------|
| Added node | green, bold | green stroke |
| Removed node | red, dashed | red dashed stroke |
| Persisted node | default, label "name (N→M)" | default |
| Added edge | green arc | solid `-->` arrow |
| Removed edge | red dashed arc | dashed `-.->` arrow |
| Shadow shift: emerged | green | green stroke, dashed |
| Shadow shift: submerged | red | red stroke, dashed |
| Shadow shift: reason-changed | orange | orange stroke, dashed |

Shadow shifts appear in a dedicated subgraph (`cluster_shadow_shifts` in DOT,
`ShadowShifts` in Mermaid). The subgraph is omitted when empty.

Rationale: the color conventions are consistent with the added/removed pattern used
for nodes — green means "now visible," red means "now hidden." This makes the diagram
self-explanatory without a legend.

---

## Decision 4 — Layout: top-down direction, vertical shadow stacking

Both DOT and Mermaid use top-down layout (`rankdir=TB` in DOT, `flowchart TD` in Mermaid).
DOT nodes use `shape=box` for better readability with long element names.

Shadow nodes and shadow shift nodes are chained with invisible edges (DOT `style=invis`,
Mermaid `~~~`) to force vertical stacking. Without this, Graphviz and Mermaid place all
unconnected nodes in a subgraph on the same horizontal rank, producing very wide diagrams.

Edge labels are truncated at 28 characters (reduced from 40) to keep diagrams compact.
The JSON export remains lossless.

---

## Decision 5 — CLI `--tag` flag uses `stringSliceFlag` (same pattern as `--observer`)

The `--tag` flag on `articulate` is repeatable, using the same `stringSliceFlag` type as
`--observer`. Empty values (`--tag ""`) are rejected by `Set()`.

For `diff`, per-side tag filters are `--tag-a` and `--tag-b`, following the established
pattern of `--observer-a`/`--observer-b` and `--from-a`/`--to-a`.

---

## Decision 6 — `--output` flag writes to file; confirmation on stdout

The `--output <file>` flag on both `articulate` and `diff` writes the rendered output
(text, JSON, DOT, or Mermaid) to the specified file instead of stdout. A confirmation
message (`wrote <path>`) is printed to stdout.

The file is opened with `os.Create` and closed with a deferred `f.Close()` to prevent
file descriptor leaks on render errors.

Rationale: piping to stdout is fine for terminal use, but users working with VS Code
extensions (Graphviz Preview, Mermaid Editor) need files they can open directly. The
`--output` flag is simpler than requiring shell redirection (`> file.dot`) and provides
a confirmation message.

---

## Decision 7 — Security: all user-derived strings sanitized in DOT/Mermaid output

`ShadowShiftKind` values (typed strings that can carry arbitrary values when deserialized)
are sanitized with `stripNewlines` (DOT) and `mermaidLabel` (Mermaid) before embedding.

Tag filter values in `dotCutComment` and `PrintArticulation` are newline-stripped.

This continues the M9 security convention: no user-derived string enters DOT or Mermaid
output without sanitization.

---

## What M10 does not close

- **CLI reflexive tracing**: the CLI does not call `ArticulationTrace`/`DiffTrace` to
  record its own observational acts. The tools exist (M7); the CLI hasn't adopted them.
  This is an acknowledged tension (Principle 8), not a violation.
- **AND-semantics tag filter**: the current tag filter is OR (any-match). If a user needs
  "traces with ALL of these tags," they would need to filter programmatically. This can be
  revisited if use cases emerge.
- **Interactive visualization**: DOT and Mermaid are static outputs. A live graph laboratory
  remains a future direction.
