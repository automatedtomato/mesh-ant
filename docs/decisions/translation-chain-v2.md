# Decision Record: Translation Chain v1

**Date:** 2026-03-13
**Status:** Active
**Packages:** `meshant/graph`, `meshant/schema`, `meshant/cmd/meshant`
**Branch merged:** `feat/m10-chain` (part of M10.5 Translation Chain + Classification)

---

## What was decided

1. **`FollowTranslation()` reads *through* a graph via first-match branching (Layer 4 operation)**
2. **Classification assigns `StepKind` heuristics: intermediary, mediator, translation**
3. **`ClassifyChain()` produces `ClassifiedChain` with step classifications**
4. **First-match branching: when multiple edges leave a node, follow first by dataset order**
5. **Classification is cut-dependent: same chain under different observer cuts yields different results**
6. **Alternatives recorded as `branch-not-taken` breaks (consistent with shadow philosophy)**
7. **Cycle detection records closing step before adding break (asymmetry documented)**
8. **Classification uses v1 heuristics; acknowledged as outsourcing judgment to trace author**
9. **`ClassifyOptions` has `Criterion EquivalenceCriterion` field (added M10.5+); zero value preserves v1 behaviour**
10. **Intermediary is NOT a new Trace field; it emerges as analytical judgment at chain level**
11. **CLI `follow` subcommand with --observer, --tag, --from, --to, --element, --direction, --depth, --format flags**
12. **Three new files: chain.go, classify.go, chain_print.go**

---

## Context

M9 gave MeshAnt a complete CLI. M10 phases in *analytical operations* — the first is the **translation chain**, a new Layer 4 operation that reads *through* a graph by following causal chains (successive edges that form a path).

Unlike Articulate (reads traces in a cut, observes structure) and Diff (compares two cuts, observes change), FollowTranslation follows a single path of causation from source to target, asking: "what steps mediate between X and Y?"

This is the first operation that makes **mediation** — the tricky ANT concept about how one actant transforms another — analytically tractable. A translation chain names each step and classifies it as intermediary (no mediation), mediator (mediation present), or translation (mediation + "translation" tag). These classifications are provisional heuristics, acknowledged as delegating judgment to the trace author.

The chain's dependency on observer position means the same logical path looks different under different cuts. This mirrors Articulate's observer-dependence and encodes Principle 8 (the analyst is positioned).

---

## Decisions

### 1. `FollowTranslation()` reads *through* a graph via first-match branching

```go
func FollowTranslation(g MeshGraph, opts FollowOptions) TranslationChain
```

`FollowTranslation` traverses the graph from a starting element (--element flag), following edges until reaching a terminal node (no outgoing edges) or hitting a configured limit (--depth flag). At each node, if multiple outgoing edges exist, the first is followed (by dataset encounter order); all alternatives are recorded as `branch-not-taken` breaks.

**Why first-match?** Choosing one deterministically avoids explosion into combinatorial branching. The graph author's ordering of traces in the JSON is preserved as a meaningful signal — it reflects how observers encountered the events. Alternatives become *visible shadows* of the path, named in the chain's break list.

**Encounter order vs. alphabetical?** Encounter order (first appearance in the dataset) preserves the observational sequence. Alphabetical would imply false equivalence across orderings. The dataset is the archive; its order is data.

### 2. Classification assigns `StepKind` heuristics: intermediary, mediator, translation

```go
type StepClassification struct {
    StepIndex int      // index into TranslationChain.Steps
    Kind      StepKind // intermediary, mediator, or translation
    Reason    string   // human-readable justification (edge-driven, not criterion-driven)
}

type StepKind string
const (
    StepIntermediary StepKind = "intermediary"  // no mediation observed
    StepMediator     StepKind = "mediator"      // mediation present
    StepTranslation  StepKind = "translation"   // mediation + "translation" tag
)
```

Each step in a chain receives a classification derived from heuristics:

- **Intermediary** (`StepIntermediary`): edge has no `Mediation` field (or empty). The source and target are connected directly without recorded transformation.
- **Mediator** (`StepMediator`): edge has a `Mediation` value. The edge transforms the source into something sent to the target.
- **Translation** (`StepTranslation`): edge has both a `Mediation` value AND the trace is tagged `TagTranslation`. Translation is a special, marked case of mediation.

Reason strings explain the classification (edge-driven, not criterion-driven — see M10.5+ / Decision 9 below):
- `"no mediation observed — action relayed without recorded transformation"`
- `"mediation present — action transformed in passage"`
- `"mediation present with translation tag — regime boundary crossed"`

*Note: the field was named `Rationale` in the design; the implementation uses `Reason`.*

**Why use Author judgment?** MeshAnt does not define what counts as mediation; the trace author does, by whether they wrote a `Mediation` value and applied tags. We outsource judgment to the observer who recorded the traces. This is epistemologically honest: the classification reflects *what the author said made a difference*, not an imposed analytical framework.

### 3. `ClassifyChain()` produces `ClassifiedChain` with step classifications

```go
type TranslationChain struct {
    StartElement string       // node where chain was entered
    Steps        []ChainStep  // ordered sequence of edge traversals
    Breaks       []ChainBreak // points where chain stopped or alternatives exist
    Cut          Cut          // articulation parameters — chain is situated within this cut
}

type ClassifiedChain struct {
    Chain           TranslationChain      // original chain
    Classifications []StepClassification  // one entry per step, same order as Chain.Steps
    Criterion       EquivalenceCriterion  // envelope metadata only; does not alter v1 heuristics
}

func ClassifyChain(chain TranslationChain, opts ClassifyOptions) ClassifiedChain
```

*Note: The design used `ClassifiedSteps []ClassifiedStep` with a `StepCount int` field. The implementation replaced these with `Classifications []StepClassification` (direct classification slice, no wrapper type) and dropped `StepCount`. `TranslationChain` dropped `Direction`, `Observer`, `GraphID` — direction is not stored post-traversal; the full `Cut` replaces the bare `Observer string` and `GraphID string` fields.*

`ClassifyChain` takes a `TranslationChain` (the raw path) and applies heuristic classification to each step. Returns a `ClassifiedChain` that preserves the original chain and adds a parallel `[]ClassifiedStep` slice with classifications.

**Why separate function, not automatic?** Classification is a second analytical pass; it is not required to traverse. A caller may want the raw chain without classification, or may classify with different criteria. Separating the operations keeps them compositional.

### 4. First-match branching: when multiple edges leave a node, follow first by dataset order

At each step, if a node has 2+ outgoing edges:

1. Follow the first (by dataset encounter order)
2. Record the others in `Breaks` as `ChainBreak` records with `Kind: BranchNotTaken`

```go
type ChainBreak struct {
    AtElement string // node where the break occurred
    Reason    string // why the chain stopped or why an alternative was not followed
}
```

Break reason values (plain strings, not a named type):
- `"element-not-in-graph"` — start element does not exist in the graph
- `"no-outgoing-edges"` — no edges leave this node (forward direction)
- `"no-incoming-edges"` — no edges enter this node (backward direction)
- `"depth-limit"` — `MaxDepth` reached
- `"cycle-detected"` — chain would revisit an already-visited node; the cycle-closing step IS recorded in `Steps`
- `"branch-not-taken"` — an alternative edge was available but not followed

*Note: The design used `Kind ChainBreakKind` (named type with constants) and embedded a `Step ChainStep` to record the alternative edge. The implementation replaced both with plain `AtElement string` and `Reason string`. The alternative edge is no longer embedded in the break record; the node name is sufficient to locate the branching point.*

This matches the shadow philosophy: alternatives exist and are named; a choice was made and is recorded. The chain is not pretending to be the only possible path — it is being transparent about what it excluded.

### 5. Classification is cut-dependent: same chain under different observer cuts yields different results

If you call `FollowTranslation` on the same graph element but under two different observer positions, the chains may differ: some edges may be in the shadow of one cut but visible in another. Therefore:

- Node visibility can change (a node in one cut's shadow doesn't appear as a step in the chain)
- Edge visibility can change (an edge only visible from certain observers may disappear)
- The chain length and structure can differ

This is intentional. An element's role in mediation is observer-dependent. A graph comparison that hides certain observer positions will produce a different chain. This mirrors `Articulate`'s observer-dependence and encodes Principle 8: the analyst's position shapes what they see.

### 6. Alternatives recorded as `branch-not-taken` breaks (consistent with shadow philosophy)

Instead of hiding alternative branches, the chain names them. At each node, if 3 outgoing edges exist and we follow edge 1:

```
Step 1: [edge1: A → B]
Breaks:
  - BranchNotTaken: [edge2: A → C]
  - BranchNotTaken: [edge3: A → D]
```

This makes the path *legible*. A reader can see not just where the chain went, but where it *didn't* go. This is consistent with how shadow elements are mandatory output in `Articulate` — named absence is methodologically significant.

### 7. Cycle detection records closing step before adding break (asymmetry documented)

If following an edge leads to a node already in the current path (cycle), the step is added to `Steps`, then a `ChainBreak` with `Kind: CycleDetected` is appended to `Breaks`.

**Why include the closing step?** It completes the logical loop visibly. A reader can trace the cycle in the steps list.

**Asymmetry with depth limit:** If a depth limit (--depth N) is reached, the last step *is* included in `Steps`, but the next edge is recorded as a `DepthExceeded` break without including that edge as a step. Documented asymmetry:
- Cycle: step included, break added
- Depth limit: step included, next edge recorded as break

This is consistent: both record what the chain *cannot continue to do*. The difference is that cycles are semantically about repetition (the step creates the loop), while depth limits are about resource constraints (the step doesn't cause the limit, the limit does).

### 8. Classification uses v1 heuristics; acknowledged as outsourcing judgment to trace author

The heuristics are simple:
- No mediation field → intermediary
- Mediation field present → mediator
- Mediation field present AND "translation" tag → translation

These are *not* universal definitions of what makes something a mediator. They are delegations to the trace author. If the author wrote a mediation, they are claiming transformation happened. We honor that claim in the classification.

**Future:** The equivalence criterion (M11 or later) will formalize what counts as *equivalent* mediation (e.g., "only the target matters, not the mediation content"). Classification v2 will use that criterion conditionally. For now, the heuristics are explicit and provisional.

### 9. `ClassifyOptions{}` is empty struct designed as extension point

```go
type ClassifyOptions struct {
    Criterion EquivalenceCriterion // interpretive declaration; stored on ClassifiedChain as provenance
}

func ClassifyChain(chain TranslationChain, opts ClassifyOptions) ClassifiedChain
```

`ClassifyOptions` was originally designed as an empty struct — an explicit extension point for future parameterization. In M10.5+, `EquivalenceCriterion` was added as the first field. **Zero value is always safe**: empty `ClassifyOptions{}` preserves v1 behaviour for all existing callers. The `Criterion` is stored on `ClassifiedChain` as envelope metadata only — it does NOT alter the v1 step heuristics (design rule C1). Step `Reason` strings remain purely edge-driven.

### 10. Intermediary is NOT a new Trace field; it emerges as analytical judgment at chain level

`Intermediary` does not become a field on `schema.Trace`. It is a `StepKind` classification that only makes sense in the context of a translation chain. Whether a step is an intermediary depends on:
- What edges surround it (multiple edges? none?)
- What observer position is in effect (cut-dependent)
- What the analyst is asking (is this step part of a mediation chain?)

Adding it to `Trace` would reify a judgment that only exists in chains. Keeping classification at the chain level respects the composed, analytical nature of the operation.

### 11. CLI `follow` subcommand with comprehensive flag set

```
meshant follow <file> --element NAME [--observer POS] [--tag TAG] [--from RFC3339] [--to RFC3339]
                      [--direction forward|backward] [--depth N] [--format text|json] [--output FILE]
```

Flags:

- `--element NAME` (required): Start following from this element
- `--observer POS` (repeatable): Observer position cut (empty = unfiltered)
- `--tag TAG` (repeatable): Tag-filter the articulation (any-match)
- `--from RFC3339`, `--to RFC3339`: Time window (empty = unbounded)
- `--direction forward|backward` (default: forward): Follow edges as targets (forward) or sources (backward)
- `--depth N` (default: unlimited, or 0 for unlimited): Maximum chain length
- `--format text|json` (default: text): Output format
- `--output FILE`: Write to file instead of stdout

The `follow` subcommand articulates the graph with the given observer/tag/time filters, then calls `FollowTranslation(g, opts)` to produce the chain. Chains are always printed with their classifications by default (via `PrintChain`).

### 12. Three new files: chain.go, classify.go, chain_print.go

```
meshant/graph/chain.go      - TranslationChain, ChainStep, ChainBreak, Direction, FollowOptions; FollowTranslation() function
meshant/graph/classify.go   - ClassifiedChain, ClassifiedStep, StepClassification, StepKind, ClassifyOptions; ClassifyChain() function
meshant/graph/chain_print.go - PrintChain, PrintChainJSON; helpers for formatted output
```

Keeping chain logic separate from articulation/diff logic matches the existing pattern (diff.go separate from graph.go). Each file is sized for readability (<800 lines).

---

## Types and signatures

### In meshant/graph/chain.go

```go
type Direction string
const (
    DirectionForward  Direction = "forward"  // follow edges from source to target
    DirectionBackward Direction = "backward" // follow edges from target to source
)

type FollowOptions struct {
    Direction Direction // zero value means forward
    MaxDepth  int       // 0 = unlimited, >0 = max steps
}

type ChainStep struct {
    Edge           Edge   // graph edge traversed in this step
    ElementExited  string // node the chain was at before this step
    ElementEntered string // node the chain arrived at via this edge
}

type ChainBreak struct {
    AtElement string `json:"at_element"` // node where the break occurred
    Reason    string `json:"reason"`     // plain string; see known values in Decision 4
}

type TranslationChain struct {
    StartElement string       // node where chain was entered
    Steps        []ChainStep  // ordered sequence of edge traversals
    Breaks       []ChainBreak // points where chain stopped or alternatives exist
    Cut          Cut          // articulation parameters — chain is self-situated
}

func FollowTranslation(g MeshGraph, from string, opts FollowOptions) TranslationChain
```

### In meshant/graph/classify.go

```go
type StepKind string
const (
    StepIntermediary StepKind = "intermediary" // no mediation observed
    StepMediator     StepKind = "mediator"     // mediation present
    StepTranslation  StepKind = "translation"  // mediation + "translation" tag
)

type StepClassification struct {
    StepIndex int      `json:"step_index"` // index into TranslationChain.Steps
    Kind      StepKind `json:"kind"`
    Reason    string   `json:"reason"` // human-readable; edge-driven, always non-empty
}

type ClassifiedChain struct {
    Chain           TranslationChain     // original chain
    Classifications []StepClassification // one entry per step, same order as Chain.Steps
    Criterion       EquivalenceCriterion // envelope metadata; does not alter v1 heuristics
}

type ClassifyOptions struct {
    Criterion EquivalenceCriterion // stored on ClassifiedChain as provenance; zero value safe
}

func ClassifyChain(chain TranslationChain, opts ClassifyOptions) ClassifiedChain
```

### In meshant/graph/chain_print.go

```go
func PrintChain(w io.Writer, cc ClassifiedChain) error
func PrintChainJSON(w io.Writer, cc ClassifiedChain) error
```

### In meshant/cmd/meshant/main.go

```go
// New subcommand added to run() dispatcher
func cmdFollow(w io.Writer, args []string) error
```

---

## What M10.5 explicitly defers

- **Weighted chains**: all edges contribute equally; no frequency or recency weighting.
- **Temporal visibility in chain steps**: steps record which observer could see them, but not "which time window would make this element visible?"
- **Bidirectional chains**: FollowTranslation follows in one direction; a future operation might combine forward and backward into a single mediation graph.
- **Chain-as-actor**: a `TranslationChain` does not receive an ID and cannot appear in traces. Deferred; M11+ will address.
- **Equivalence criterion in classification**: v1 uses simple heuristics; the criterion for "equivalent mediation" is future work.
- **Persistence of chains**: chains remain in-memory; serialization to disk deferred.
- **Multi-step chain analysis**: no "chains of chains" or timeline of mediation evolution.

---

## Files added or modified

- `meshant/graph/chain.go` — `TranslationChain`, `ChainStep`, `ChainBreak`, `Direction`, `FollowOptions`; `FollowTranslation()` function; unexported helpers
- `meshant/graph/chain_test.go` — unit tests (groups covering first-match, cycle detection, direction reversal, depth limit)
- `meshant/graph/classify.go` — `ClassifiedChain`, `ClassifiedStep`, `StepClassification`, `StepKind`, `ClassifyOptions`; `ClassifyChain()` function
- `meshant/graph/classify_test.go` — unit tests (groups covering heuristic classification)
- `meshant/graph/chain_print.go` — `PrintChain`, `PrintChainJSON`; helpers for text/JSON formatting
- `meshant/graph/chain_print_test.go` — tests for print functions
- `meshant/graph/chain_e2e_test.go` — E2E tests against datasets (deforestation, evacuation_order, incident_response)
- `meshant/cmd/meshant/main.go` — `cmdFollow` subcommand added to dispatcher; flag parsing for --element, --observer, --tag, --from, --to, --direction, --depth, --format, --output
- `meshant/cmd/meshant/main_test.go` — tests for follow subcommand, flag parsing, error handling

---

## Relation to earlier decisions

- **Articulation (M2)**: `FollowTranslation` depends on `Articulate`; it takes a `MeshGraph` as input. Observer-dependence mirrors Articulate's observer-dependence (Principle 8).
- **Shadow (M2)**: Breaks recorded as `branch-not-taken` mirror the shadow philosophy — named absence is methodologically significant.
- **Time-window (M3)**: Classification is time-dependent; same chain under different time windows may differ.
- **Diff (M4)**: Chains enable "mediation analysis" — what M4 started (comparing two cuts) M10.5 deepens (following paths within a cut).
- **Graph-as-actor (M5)**: `TranslationChain.GraphID` records which graph the chain came from (enables reflexive tracing in M11).
- **CLI (M9)**: `cmdFollow` follows the same flag/output patterns as `cmdArticulate` and `cmdDiff`.
- **Tag-filter (M10)**: `FollowTranslation` is aware of tags; tag-filtered articulations produce tag-filtered chains.

---

## Design rationale: why mediation chains matter

Mediation is ANT's hardest concept. Most analysis conflates it with intermediation ("A passes message B to C unchanged"). MeshAnt's design **refuses** that conflation: a `Mediation` field on a trace is explicitly an author claim that transformation happened.

By following chains, we make mediation *visible* as a path. A translation chain names each step and classifies it. This is not neutral analysis — it's analytical work that makes mediation legible in practice.

The heuristics (no mediation = intermediary, mediation present = mediator, translation tag = special) are provisional and honest about that. Future work (equivalence criterion, M11+) will make the classifications more sophisticated. For now, they honor the author's judgment: if the author wrote mediation, we acknowledge it.

Chains are observer-dependent because mediation is positioned. The same causal sequence looks different from different vantage points. This is not a bug — it is methodologically *correct* (Principle 8).
