# Decision Record: Interpretive Outputs v1

**Date:** 2026-03-18
**Status:** Active
**Milestone:** Thread B — Remaining Interpretive Outputs
**Packages:** `meshant/graph` (bottleneck.go, suggest.go, narrative.go), `meshant/cmd/meshant` (bottleneck, gaps --suggest, articulate --narrative)
**Related:** `docs/decisions/articulation-v2.md`, `docs/decisions/shadow-analysis-v1.md`, `tasks/plan_thread_b.md`

---

## What was decided

1. **Three interpretive outputs are Layer 3** — `BottleneckNote`, `RearticSuggestion`, `NarrativeDraft` sit above the analytical kernel. They take pre-articulated types as input; they never re-articulate internally.
2. **No composite scores** — `BottleneckNote` exposes three independent measures rather than a single centrality score. Analyst combines them with explicit judgement.
3. **Suggestion as provocation, not prescription** — `RearticSuggestion.Rationale` names what the suggestion cannot know. The suggestion engine's own shadow is named in the footer.
4. **Narrative as draft, always** — `NarrativeDraft` uses provisional language throughout. Shadow is named in every narrative. The cut is explicit in the PositionStatement.
5. **No new subcommands — flag extensions** — B.2 and B.3 are flag extensions on existing subcommands (`--suggest` on gaps, `--narrative` on articulate), not new top-level commands.
6. **Nil vs empty slice semantics** — `SuggestRearticulations` returns nil when no gap exists (no suggestion needed) and a non-nil empty slice when a gap exists but no heuristic fires (two structurally different states).

---

## Context

M13 completed the analytical kernel: `MeshGraph`, `ObserverGap`, `ShadowSummary` are stable. The gap report makes visible what two cuts diverge on; the shadow summary makes visible what the cut excludes. But the analytical surface remains structural — what is there, what changed, what is shadowed — rather than actionable.

Thread B opens Layer 3: interpretive outputs that make articulation results readable without hiding the cut. They take types already produced by the analytical kernel and produce human-facing reports. No god's-eye claims. Every output names its position and its limits.

B.1 identifies elements that appear notably active *from this cut*. B.2 suggests how to reduce a gap between two cuts *by changing your position*. B.3 summarizes an articulation *as seen from here*. All three are provisional, positioned, and contestable.

---

## Decision 1: Three interpretive outputs are Layer 3

`BottleneckNote`, `RearticSuggestion`, and `NarrativeDraft` are Layer 3 outputs. They sit above the analytical kernel (Layer 2: `MeshGraph`, `ObserverGap`, `ShadowSummary`). They take pre-articulated types as input — `MeshGraph` or `ObserverGap` — and produce human-readable reports.

This is the composability decision: each Layer 3 function is independent and composable. The caller is responsible for articulation (choosing observer positions, time windows, tag filters, and deciding what questions to ask). Layer 3 functions only read the pre-articulated results.

The alternative — Layer 3 functions accepting raw traces and options — would be convenient but would hide articulation inside the report function, making it undebuggable and non-composable. The caller may want to inspect the graph, inspect the gap, then ask for suggestions. Composability preserves that option.

Layer 3 functions never call `Articulate()` internally, never load traces, never impose a time window. They read only what the caller has already prepared.

---

## Decision 2: No composite scores

`IdentifyBottlenecks` produces `BottleneckNote` records with three independent measures:

- **AppearanceCount** — how often the element appears in Source/Target slices
- **MediationCount** — how often the element appears as Mediation in edges
- **ShadowCount** — how many excluded traces also contain this element

No single "centrality score" combines these. A composite score would hide its construction and imply a god's-eye ordering: "this element is THE bottleneck." The analyst cannot reconstruct what weights were used or why.

Instead, all three measures are reported separately. The reason field makes the heuristic visible:

> "high mediation count (4) — actively transformed action in this cut"
> "appears in both included and excluded traces — crosses the cut boundary"

The analyst reads the three measures, reads the reason, and decides what is significant *for their analytical question*. This is not weakness — it is honesty. The framework does not claim to measure "importance" or "centrality" in the abstract. It reports what is measurable from *this articulation*.

---

## Decision 3: Suggestion as provocation, not prescription

`SuggestRearticulations` produces `RearticSuggestion` records that name a direction, not an answer. The `Rationale` field always states what the suggestion cannot know.

Example rationale:

> "B sees 7 fewer elements than A; B's observer set is narrower. Expanding it might reduce the gap, but expanding the observer set cannot guarantee it. Only a re-articulation will tell whether the new elements bring new structure."

Three suggestion kinds (v1 heuristics):

- **ObserverExpansion** — if one side sees fewer elements, it might have a narrower observer set. Suggest expanding.
- **TimeWindowExpansion** — if one side has a narrower time window, suggest expanding toward the other's range.
- **TagRelaxation** — if one side has tag filters the other lacks, suggest relaxing them.

None of these is a recommendation. All three are incomplete provocations. The footer of `PrintRearticSuggestions` names the suggestion engine's own shadow:

> "This suggestion engine can only suggest changes to observer, time, and tag parameters. It cannot suggest changes to element boundaries, equivalence criteria, or the trace dataset itself. Only an analyst can judge whether a suggestion is worth pursuing."

The heuristics are v1 and provisional — acknowledged as contestable. A future version may add different suggestion kinds or change the thresholds. The current version names what it does and does not do.

---

## Decision 4: Narrative as draft, always

`DraftNarrative` produces a `NarrativeDraft` from a `MeshGraph`. It is template-based in v1 (LLM-assisted narrative draft is deferred to Thread F). The output has four sections:

- **PositionStatement** — names the observer position(s) and time window. One sentence. Built from `cutLabel()` — the same helper used by gaps and shadow reports.
- **Body** — what the articulation shows: top-3 elements by AppearanceCount, distinct Mediation strings, trace count. Two to four sentences.
- **ShadowStatement** — what this reading cannot claim to see: number of shadowed elements, the reasons they are shadowed (observer, tag, time). Uses "in shadow from this position" language, never "missing."
- **Caveats** — reminders about provisional nature. Always includes: "This draft is a positioned reading, not a complete account. A different cut would produce a different narrative." Additional caveats if shadow is large or time window is narrow.

The type name "Draft" encodes revisability. The narrative is never presented as final or authoritative. Shadow is named in every narrative — the cut is explicit in the PositionStatement. An analyst reading a narrative draft understands immediately: this is one reading from one position. A different analyst would write differently.

No god's-eye language. No claims about "what really happened." No hidden framing. The draft says: here is what the articulation shows from this position, here is what it cannot see, and here are reminders about what that means.

---

## Decision 5: No new subcommands — flag extensions

B.2 (`RearticSuggestion`) and B.3 (`NarrativeDraft`) are implemented as flag extensions, not new subcommands:

- **`--suggest` flag on `meshant gaps`** — When set, `cmdGaps` calls `SuggestRearticulations(gap)` and appends the suggestion report after the standard gap report.
- **`--narrative` flag on `meshant articulate`** — When set, `cmdArticulate` calls `DraftNarrative(g)` and appends the narrative draft after the standard articulation output.

This avoids proliferating the CLI surface. Bottleneck is a new subcommand (`meshant bottleneck`) because it is a distinct operation: identify elements by centrality measures. But gaps with suggestions is still fundamentally a gap report — the suggestion is an augmentation of the gap output. Likewise, articulate with narrative is still fundamentally an articulation — the narrative is an augmentation.

The flags are optional. `meshant gaps` without `--suggest` works as before. `meshant articulate` without `--narrative` works as before. This treats Layer 3 outputs as optional analytical augmentations, not primary outputs.

---

## Decision 6: Nil vs empty slice semantics for SuggestRearticulations

`SuggestRearticulations(gap ObserverGap) []RearticSuggestion` returns:

- **nil** when `OnlyInA` and `OnlyInB` are both empty — no gap exists, no suggestion is meaningful
- **non-nil empty slice** when a gap exists but no heuristic fires — a gap is present, but none of the three suggestion kinds apply

This preserves the caller's ability to distinguish two structurally different states:

1. "There is no gap" (nil) — the two articulations already see the same elements
2. "There is a gap but no suggestion available" (non-nil empty) — the gap exists, but the v1 heuristics do not match it

The first state means nothing needs to change. The second state means a gap exists that the heuristics cannot explain — an analytical puzzle worth manual investigation.

This design mirrors Go's idiomatic distinction between "missing" (nil) and "empty but present" (empty slice). It avoids collapsing two distinct conditions into one.

---

## What Thread B does NOT do

- **Automated decisions** — Layer 3 outputs are provocations and drafts, not decisions. The analyst reads them and decides what to do.
- **LLM-generated narratives** — `DraftNarrative` is template-based in v1. LLM-assisted narrative generation is deferred to Thread F.
- **Composite centrality ranking** — `BottleneckNote` reports three measures separately. No unified ranking.
- **Criterion-driven suggestions** — `SuggestRearticulations` uses structural gap heuristics. Criterion-aware suggestion logic is deferred.
- **New schema types** — Layer 3 functions live in the `graph` package and accept only types already produced by the analytical kernel. No new schema imports.

---

## Related

- `docs/decisions/articulation-v2.md` — observer axis as primary cut, shadow mandatory
- `docs/decisions/shadow-analysis-v1.md` — shadow as cut decision, `AnalyseGaps` composability
- `docs/decisions/graph-diff-v2.md` — `PrintDiff` pattern: situate the comparison in both positions
- `tasks/plan_thread_b.md` — detailed design rules and test plans for B.1, B.2, B.3
- `docs/glossary.md` — mediation, intermediary, cut, shadow, articulation vocabulary
