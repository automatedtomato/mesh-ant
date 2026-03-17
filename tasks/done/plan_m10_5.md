# M10.5: Translation Chain + Mediator/Intermediary Classification

## Overview

Layer 4 analytical operation — the first that reads *through* a graph (following
paths) rather than *across* graphs (comparing cuts). Two connected capabilities:

1. **`FollowTranslation`** — traverse connected elements through an articulated
   MeshGraph, producing a `TranslationChain`
2. **`ClassifyChain`** — judge each step as intermediary-like, mediator-like, or
   translation. Classification is cut-dependent, not intrinsic.

No new fields on `schema.Trace`. Intermediary emerges as analytical judgment.

---

## Phase 1: Chain Types + Traversal (M10.5.1)

**File:** `meshant/graph/chain.go`

### Types

```go
type Direction string // "forward" | "backward"

type FollowOptions struct {
    Direction Direction // default "forward"
    MaxDepth  int       // 0 = unlimited
}

type ChainStep struct {
    Edge           Edge   // the edge traversed
    ElementEntered string // the node entered via this edge
    ElementExited  string // the node exited
}

type ChainBreak struct {
    AtElement string // where the chain could not continue
    Reason    string // "no-outgoing-edges" | "no-incoming-edges" | "depth-limit" | "cycle-detected" | "branch-not-taken" | "element-not-in-graph"
}

type TranslationChain struct {
    StartElement string
    Steps        []ChainStep
    Breaks       []ChainBreak
    Cut          Cut
}
```

### Function

```go
func FollowTranslation(g MeshGraph, from string, opts FollowOptions) TranslationChain
```

### Key decisions

- First-match branching: follows first edge by dataset order, records alternatives
  as breaks (shadow philosophy — names what was not followed)
- Cycle detection terminates and records a break
- Multi-source/multi-target edges: current position determines which element "exits"

### Tests (~25)

`meshant/graph/chain_test.go` — 9 groups:

1. Linear chain (A→B→C→D), forward and backward
2. Start element not in graph (break immediately)
3. Cycle detection (A→B→C→A)
4. Depth limit (chain of 5, limit 3)
5. Branch-not-taken breaks (A→B, A→C, follow from A)
6. Multi-source/multi-target edges
7. Empty graph (no edges)
8. Single-element chain (no connections)
9. Zero-value FollowOptions defaults (forward, unlimited)

---

## Phase 2: Step Classification (M10.5.2)

**File:** `meshant/graph/classify.go`

### Types

```go
type StepKind string // "intermediary" | "mediator" | "translation"

type StepClassification struct {
    StepIndex int
    Kind      StepKind
    Reason    string
}

type ClassifiedChain struct {
    Chain           TranslationChain
    Classifications []StepClassification
}

type ClassifyOptions struct {
    // Empty for v1. Extension point for future contextual heuristics.
}
```

### Function

```go
func ClassifyChain(chain TranslationChain, opts ClassifyOptions) ClassifiedChain
```

### v1 heuristics (provisional)

- **Translation**: non-empty `Edge.Mediation` AND `"translation"` tag present
- **Mediator-like**: non-empty `Edge.Mediation`, no translation tag
- **Intermediary-like**: empty `Edge.Mediation` — "no mediation *observed*"

### Tests (~15)

`meshant/graph/classify_test.go` — 7 groups:

1. All-intermediary chain (no mediation on any edge)
2. All-mediator chain (mediation present, no translation tag)
3. Mixed chain
4. Translation step (mediation + translation tag)
5. **Same trace data, two cuts, different classifications (Question ④)**
6. Empty chain (zero steps)
7. Reason strings non-empty

---

## Phase 3: Output + CLI (M10.5.3)

### Output

**File:** `meshant/graph/chain_print.go`

```go
func PrintChain(w io.Writer, cc ClassifiedChain) error
func PrintChainJSON(w io.Writer, cc ClassifiedChain) error
```

PrintChain footer: *"This chain is a reading through one situated cut.
Classification is an analytical judgment, not an intrinsic property.
The same chain from a different cut may yield different classifications."*

### CLI

**File:** `meshant/cmd/meshant/main.go` — new `follow` subcommand

Flags:
- `--observer`, `--tag`, `--from`, `--to` (reused articulation params)
- `--element` (required — start element)
- `--direction` (default "forward"), `--depth` (default 0)
- `--format text|json`, `--output <file>`

Pipeline: Load → Articulate → FollowTranslation → ClassifyChain → Print

### Tests (~10)

In `meshant/cmd/meshant/main_test.go`

---

## Phase 4: E2E + Decision Record + Codemap (M10.5.4)

### E2E tests

**File:** `meshant/graph/chain_e2e_test.go`

- Forward from `buoy-array-atlantic-sector-7` (meteorological-analyst, 2026-04-14)
- Backward from `evacuation-order-16apr` (local-mayor, 2026-04-16)
- Same start element, two observer cuts → different chain/classification outcomes

### Decision record

**File:** `docs/decisions/translation-chain-v2.md` — 10 decisions

### Codemap update

**File:** `docs/CODEMAPS/meshant.md`

---

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Heuristics too simplistic | HIGH | Named as provisional; `ClassifyOptions` extension point; Reason field inspectable |
| "intermediary" from absence | HIGH | Label is "-like"; Reason says "no mediation *observed*"; footer names as judgment |
| Cut-dependence hard to test | MEDIUM | Two cuts from evacuation data with different mediation visibility |
| Multi-source/target ambiguity | MEDIUM | Documented convention in chain.go |

## Deferred

- Multi-branch following (all paths)
- Chain comparison / `ChainDiff`
- DOT/Mermaid chain visualization
- Contextual heuristics (adjacent-step classification)
- User-defined classification rules

## Estimated

~55 new tests, 4 new Go files + CLI extension, 1 decision record, 1 codemap update.
