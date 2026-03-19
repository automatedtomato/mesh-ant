# Thread B — Remaining Interpretive Outputs

**Goal:** complete the Layer 3 story. Three analytical outputs were named in the v1.0.0
review and deferred through M13. They make articulation results actionable without hiding
the cut. No god's-eye claims, no authoritative voice. Every output names its position.

**Depends on:** M13 complete (MeshGraph, ObserverGap, ShadowSummary, Cut all stable).
**Targets:** v1.x (part of v1.3.x series, before Thread A begins).
**Branch base:** `develop`
**Rough plan source:** `tasks/plan_v2_roadmap.md` §Thread B

---

## Design principles for Thread B

1. **Language discipline** — "appears central from this cut", never "is a bottleneck".
   Every output names its position and its limits. Shadow is a cut decision, not missing data.
2. **Composability** — each function takes types already produced by the analytical kernel
   (MeshGraph, ObserverGap). No re-articulation inside these functions.
3. **Provisional v1 heuristics** — same disposition as ClassifyChain and ClassifyDraftChain.
   Judgments are contestable. Reason fields make them inspectable.
4. **No import cycles** — B functions live in the `graph` package; no new schema imports.
5. **Immutable outputs** — return new values; no mutation of input graphs.

---

## B.1 — BottleneckNote

**Issue:** TBD (child of Thread B parent issue)
**File:** `meshant/graph/bottleneck.go`
**Test file:** `meshant/graph/bottleneck_test.go`
**CLI:** `meshant bottleneck --observer <pos> [--tag/--from/--to/--output] traces.json`

### What it does

From a given cut (a `MeshGraph`), identify which elements appear most central — not as a
truth claim but as a provisional reading of the articulation. An element is "central" in
three distinct senses that MeshAnt can measure from available data:

1. **Degree centrality** — how often does this element appear in trace Sources/Targets?
   (`Node.AppearanceCount`, already computed in the graph)
2. **Mediation count** — how often does this element appear as `Edge.Mediation`?
   Mediators transform what passes through them — high mediation count makes them
   disproportionately active in the articulation.
3. **Cross-cut presence** — does this element appear in both included and excluded traces?
   (`Node.ShadowCount > 0`). A cross-cut element participates in more positions than this
   cut can see — its full role is opaque from here.

No single measure determines "bottleneck". The three measures are reported separately.
The caller (and the human reader) decides what is significant.

### Types

```go
// BottleneckOptions controls which elements are included in the report.
// v1: empty struct — all non-zero elements included.
// Extension point: future fields (MinAppearanceCount, MinMediationCount, etc.)
type BottleneckOptions struct{}

// BottleneckNote records the centrality measures of one element from a given cut.
// All fields are derived from the MeshGraph passed to IdentifyBottlenecks.
// No field is a truth claim about the network — only about this articulation.
type BottleneckNote struct {
    // Element is the element name as it appears in the graph's Nodes map.
    Element string

    // AppearanceCount is the number of times this element appeared in Source or
    // Target slices across all included traces. Sourced from Node.AppearanceCount.
    AppearanceCount int

    // MediationCount is the number of edges in which this element appears as
    // Mediation. Mediators transform what passes through them — a high count
    // means this element actively redirected action in many observed traces.
    MediationCount int

    // ShadowCount is the number of excluded traces in which this element also
    // appears. Non-zero means this element crosses the cut boundary: it is
    // present in both included and excluded traces. Sourced from Node.ShadowCount.
    ShadowCount int

    // Reason is a human-readable, provisional justification naming which measure(s)
    // make this element notable. Always non-empty. Example:
    // "high mediation count (4) — actively transformed action in this cut"
    Reason string
}
```

### Functions

```go
// IdentifyBottlenecks returns BottleneckNotes for elements in g that are notable
// by at least one centrality measure. Returns notes sorted by MediationCount
// descending, then AppearanceCount descending, then name alphabetically.
//
// v1 heuristic: an element is included if MediationCount > 0 OR
// AppearanceCount >= 2 OR ShadowCount > 0. Elements with all-zero counts
// (isolated nodes) are omitted — they have no measurable centrality from this cut.
//
// The returned slice is immutable — no aliasing of the input graph.
// Returns nil if g has no nodes.
func IdentifyBottlenecks(g MeshGraph, opts BottleneckOptions) []BottleneckNote

// PrintBottleneckNotes writes a bottleneck report to w.
// The report names the cut position and lists each BottleneckNote with its
// three centrality measures and reason. Provisional language throughout.
func PrintBottleneckNotes(w io.Writer, g MeshGraph, notes []BottleneckNote) error
```

### CLI addition to main.go

```
meshant bottleneck [--observer <pos>] [--tag <t>] [--from RFC3339] [--to RFC3339] [--output <file>] traces.json
```

`cmdBottleneck` follows the same pattern as `cmdShadow`: load traces, articulate,
call `IdentifyBottlenecks`, call `PrintBottleneckNotes`. All flags reuse existing helpers
(`stringSliceFlag`, `parseTimeWindow`, `outputWriter`).

### Tests (bottleneck_test.go, ~12 tests)

```
TestIdentifyBottlenecks_ByMediationCount       — element appears as Mediation in 3 edges; included, MediationCount=3
TestIdentifyBottlenecks_ByAppearanceCount      — element appears in 2+ edges as source/target; included
TestIdentifyBottlenecks_ByShadowCount          — element has ShadowCount>0; included
TestIdentifyBottlenecks_ZeroCountsOmitted      — element with all-zero counts not in result
TestIdentifyBottlenecks_SortOrder              — sorted by MediationCount desc, then AppearanceCount desc
TestIdentifyBottlenecks_ReasonNonEmpty         — Reason field never empty
TestIdentifyBottlenecks_EmptyGraph             — nil return for empty graph
TestIdentifyBottlenecks_ImmutableResult        — modifying returned slice does not affect graph
TestIdentifyBottlenecks_CrossCutElement        — element with ShadowCount>0 gets cross-cut reason note
TestPrintBottleneckNotes_ContainsCutPosition   — output includes observer position
TestPrintBottleneckNotes_ProvisionalLanguage   — output does not claim authoritative centrality
TestCmdBottleneck_HappyPath                    — full run via run() dispatcher
```

### Key design constraint

`IdentifyBottlenecks` must not scan the full dataset or re-articulate. It only reads
`g.Nodes` (for AppearanceCount and ShadowCount) and `g.Edges` (for Mediation strings).
It does not accept `[]schema.Trace`. All the data it needs is already in the graph.

---

## B.2 — RearticSuggestion

**Issue:** TBD (child of Thread B parent issue)
**File:** `meshant/graph/suggest.go`
**Test file:** `meshant/graph/suggest_test.go`
**CLI:** `meshant gaps --suggest` flag (extends existing `cmdGaps`)

### What it does

When `ObserverGap` shows structural asymmetry, MeshAnt can produce a heuristic
provocation: a description of what kind of re-articulation might reduce the gap, and why.
A suggestion is not a recommendation — it is a provocation that names what it cannot know.

Suggestions are generated from the gap structure and cut metadata alone (CutA, CutB,
OnlyInA counts, OnlyInB counts). No access to original traces is needed. This keeps
`suggest.go` composable: a caller can run `AnalyseGaps` then `SuggestRearticulations`
in sequence without re-loading the dataset.

Three heuristic suggestion kinds:

1. **ObserverExpansion** — if OnlyInA or OnlyInB is large, the position with fewer visible
   elements might benefit from including additional observer positions. Suggests expanding
   the observer set of the less-visible side.
2. **TimeWindowExpansion** — if one side has a tighter time window than the other, and the
   gap is large, suggest expanding the narrower window toward the other's range.
3. **TagRelaxation** — if one side has tag filters the other lacks, and the gap is large,
   suggest relaxing the tag filter on the filtered side.

### Types

```go
// SuggestionKind identifies the type of re-articulation provocation.
type SuggestionKind string

const (
    SuggestionObserverExpansion  SuggestionKind = "observer-expansion"
    SuggestionTimeExpansion      SuggestionKind = "time-window-expansion"
    SuggestionTagRelaxation      SuggestionKind = "tag-relaxation"
)

// RearticSuggestion is one heuristic provocation for a re-articulation.
// It names a direction, not an answer. The Rationale always states what
// the suggestion cannot know — the shadow of the suggestion itself.
type RearticSuggestion struct {
    // Kind identifies the type of suggestion.
    Kind SuggestionKind

    // Side is "A" or "B" — which articulation the suggestion applies to.
    Side string

    // Rationale is a human-readable explanation of why this suggestion is
    // generated and what it cannot claim. Always non-empty. Example:
    // "B sees 7 fewer elements than A; B's observer set is narrower —
    // expanding it might reduce the gap, but cannot guarantee it"
    Rationale string

    // SuggestedParams describes the suggested change in plain language.
    // Not a ready-to-use CLI invocation — a description of the direction.
    // Example: "add A's observer positions to B's cut"
    SuggestedParams string
}
```

### Functions

```go
// SuggestRearticulations analyses the gap between two articulations and
// returns heuristic provocations for reducing it. It reads only the gap
// structure (OnlyInA, OnlyInB counts) and the two Cut values (CutA, CutB).
//
// Returns nil if the gap is empty (OnlyInA and OnlyInB both empty) — no
// suggestion is meaningful when both positions already see the same elements.
//
// v1 heuristics are provisional. Each suggestion names what it cannot know.
func SuggestRearticulations(gap ObserverGap) []RearticSuggestion

// PrintRearticSuggestions writes the suggestions to w.
// Each suggestion is printed with its kind, side, rationale, and suggested
// parameters. A caveat section names the limits of the heuristic.
func PrintRearticSuggestions(w io.Writer, suggestions []RearticSuggestion) error
```

### CLI: `--suggest` flag on `meshant gaps`

```
meshant gaps --observer-a <pos> --observer-b <pos> [flags] --suggest traces.json
```

When `--suggest` is set, `cmdGaps` calls `SuggestRearticulations(gap)` after
`PrintObserverGap` and appends the suggestion report. No new subcommand needed.

### Tests (suggest_test.go, ~10 tests)

```
TestSuggestRearticulations_ObserverExpansion   — A has wider observer set, B has large OnlyInB gap
TestSuggestRearticulations_TimeExpansion       — A has wider time window, large OnlyInA
TestSuggestRearticulations_TagRelaxation       — A has tags filter, B does not; large OnlyInA
TestSuggestRearticulations_EmptyGap            — nil return when OnlyInA and OnlyInB both empty
TestSuggestRearticulations_SymmetricGap        — both sides equally visible; no suggestion generated
TestSuggestRearticulations_RationaleNonEmpty   — Reason always non-empty
TestSuggestRearticulations_SuggestedParamsNonEmpty — SuggestedParams always non-empty
TestSuggestRearticulations_NamesLimits         — Rationale contains caveat language
TestPrintRearticSuggestions_Output             — output contains side, kind, rationale
TestCmdGaps_SuggestFlag                        — --suggest flag triggers suggestion output
```

---

## B.3 — NarrativeDraft

**Issue:** TBD (child of Thread B parent issue)
**File:** `meshant/graph/narrative.go`
**Test file:** `meshant/graph/narrative_test.go`
**CLI:** `meshant articulate --narrative` flag (extends existing `cmdArticulate`)

### What it does

A prose paragraph summarising an articulation for a non-specialist reader. Three sections:
- **PositionStatement** — what position this reading is taken from
- **Body** — what the articulation shows: key actors, key mediations, edge count
- **ShadowStatement** — what this reading cannot see, named explicitly
- **Caveats** — provisional language reminders; always includes a standard caveat

This is a template-based draft, not LLM-generated (Thread F handles that). It is called
a "draft" because it is always incomplete — a starting point for a human author.

### Types

```go
// NarrativeDraft is a provisional prose reading of an articulation.
// It is always a draft — never a final account. Every section is populated
// from the graph data; nothing is inferred from outside the cut.
type NarrativeDraft struct {
    // PositionStatement names the observer position(s) and time window
    // from which this reading is taken. One sentence.
    PositionStatement string

    // Body is a prose summary of what the articulation shows:
    // how many elements, how many traces, the most active elements,
    // key mediations present. Two to four sentences.
    Body string

    // ShadowStatement names what this reading cannot claim to see.
    // One to two sentences.
    ShadowStatement string

    // Caveats lists reminders about the provisional nature of this draft.
    // At minimum: one standard caveat about positional incompleteness.
    Caveats []string
}
```

### Functions

```go
// DraftNarrative produces a NarrativeDraft from a MeshGraph.
// All text is derived from the graph's nodes, edges, and cut.
// No external data or LLM is used.
//
// The draft is always marked as provisional. Caveats are always non-empty.
// Returns a zero-value NarrativeDraft if g has no edges.
func DraftNarrative(g MeshGraph) NarrativeDraft

// PrintNarrativeDraft writes the narrative draft to w.
// Sections are printed with clear headers. The draft nature is prominent.
func PrintNarrativeDraft(w io.Writer, n NarrativeDraft) error
```

### Narrative generation rules

- **PositionStatement**: built from `cutLabel(g.Cut)` — same helper used by gaps/reflexive.
- **Body**: enumerate top-3 elements by AppearanceCount; list distinct Mediation strings
  present in edges (up to 5, with "and N more" if longer); report `TracesIncluded` count.
- **ShadowStatement**: report `len(g.Cut.ShadowElements)` shadowed elements; name the
  shadow reasons present; use "in shadow from this position" not "missing".
- **Caveats**: always include "This draft is a positioned reading, not a complete account.
  A different cut would produce a different narrative." Additional caveats if shadow is
  large (> 50% of total traces) or if time window is set.

### CLI: `--narrative` flag on `meshant articulate`

```
meshant articulate --observer <pos> [flags] --narrative traces.json
```

When `--narrative` is set, `cmdArticulate` calls `DraftNarrative(g)` and appends
`PrintNarrativeDraft` output after the standard articulation output.

### Tests (narrative_test.go, ~11 tests)

```
TestDraftNarrative_PositionStatement           — contains observer position string
TestDraftNarrative_BodyMentionsTopElements     — top-N elements by AppearanceCount appear in Body
TestDraftNarrative_BodyMentionsMediations      — Mediation strings from edges appear in Body
TestDraftNarrative_ShadowStatement             — names shadow count and reason
TestDraftNarrative_ShadowLanguage              — uses "in shadow" not "missing"
TestDraftNarrative_CaveatsNonEmpty             — at least one caveat always present
TestDraftNarrative_EmptyGraph                  — returns zero-value NarrativeDraft, no panic
TestDraftNarrative_ImmutableInput              — does not modify the input graph
TestPrintNarrativeDraft_ContainsSections       — output includes all four section headers
TestPrintNarrativeDraft_DraftLabel             — output clearly labels as DRAFT
TestCmdArticulate_NarrativeFlag                — --narrative flag triggers narrative output
```

---

## B.4 — Decision Record + Codemap Update

**Issue:** TBD (child of Thread B parent issue)
**Files:** `docs/decisions/interpretive-outputs-v1.md`, `docs/CODEMAPS/meshant.md`,
           `tasks/todo.md`

### Decision record content

Key decisions to record:
1. Bottleneck as three independent measures, not a single score — avoids false precision
2. `IdentifyBottlenecks` reads only the MeshGraph (not raw traces) — compositional discipline
3. `SuggestRearticulations` takes only `ObserverGap` — composable, no hidden re-loading
4. `DraftNarrative` is template-based in v1 (LLM-assisted version deferred to Thread F)
5. Shadow language enforced throughout — "in shadow from this position", never "missing"
6. `--suggest` added to `gaps`, `--narrative` to `articulate` rather than new subcommands
   for B.2 and B.3 — avoids subcommand proliferation for flags that are output variants

### Codemap update

Add to `meshant/graph/` file table: `bottleneck.go`, `suggest.go`, `narrative.go`
Add to type table: `BottleneckNote`, `BottleneckOptions`, `RearticSuggestion`, `SuggestionKind`, `NarrativeDraft`
Add to function table: `IdentifyBottlenecks`, `PrintBottleneckNotes`, `SuggestRearticulations`, `PrintRearticSuggestions`, `DraftNarrative`, `PrintNarrativeDraft`
Update CLI table: `meshant bottleneck`, `--suggest` on `gaps`, `--narrative` on `articulate`

---

## File summary

| File | Status | What changes |
|------|--------|--------------|
| `meshant/graph/bottleneck.go` | new | `BottleneckNote`, `BottleneckOptions`, `IdentifyBottlenecks`, `PrintBottleneckNotes` |
| `meshant/graph/bottleneck_test.go` | new | ~12 tests |
| `meshant/graph/suggest.go` | new | `RearticSuggestion`, `SuggestionKind`, `SuggestRearticulations`, `PrintRearticSuggestions` |
| `meshant/graph/suggest_test.go` | new | ~10 tests |
| `meshant/graph/narrative.go` | new | `NarrativeDraft`, `DraftNarrative`, `PrintNarrativeDraft` |
| `meshant/graph/narrative_test.go` | new | ~11 tests |
| `meshant/cmd/meshant/main.go` | modified | `cmdBottleneck`; `--suggest` on `cmdGaps`; `--narrative` on `cmdArticulate` |
| `meshant/cmd/meshant/main_test.go` | modified | ~8 tests for cmdBottleneck + flag tests |
| `docs/decisions/interpretive-outputs-v1.md` | new | design decisions for B.1–B.3 |
| `docs/CODEMAPS/meshant.md` | modified | new files, types, functions |
| `tasks/todo.md` | modified | Thread B tasks marked complete |

---

## Issue structure

- **Parent:** "Thread B: Remaining Interpretive Outputs — bottleneck, re-articulation
  suggestion, narrative draft"
- **Children (one per B.x):**
  - B.1: BottleneckNote — centrality measures from a cut
  - B.2: RearticSuggestion — heuristic gap-reduction provocations
  - B.3: NarrativeDraft — positioned prose summary of an articulation
  - B.4: Decision record + codemap

Each child issue maps to one branch (`B1-bottleneck-note`, etc.) and one PR targeting develop.

---

## Invariants

- No god's-eye language anywhere in output text
- `Reason` / `Rationale` / `Caveats` fields always non-empty — every judgment is inspectable
- All new functions operate only on types returned by existing analytical functions
- `go test ./...` and `go vet ./...` green before each PR merge
- TDD order: write tests first, then implement
