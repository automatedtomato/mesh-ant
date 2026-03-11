# Implementation Plan: M1.3 — Minimal Trace Loader

**Branch:** `feat/m1-loader` (cut from develop)
**Status:** Confirmed — ready to implement

---

## Package Structure

```
meshant/
  loader/
    loader.go        — all exported types and functions
    loader_test.go   — black-box tests in package loader_test
```

No subdirectories. No `main` package — this is a callable package, not a binary.

---

## Exported API Surface

### Types

```go
// MeshSummary holds a provisional first-pass reading of the trace dataset.
type MeshSummary struct {
    // Elements maps every string from any Source or Target slice to
    // the number of times it appeared across all traces.
    Elements map[string]int

    // Mediations is a deduplicated list of all non-empty Mediation values,
    // in encounter order (first seen). Encounter order is intentional —
    // it reflects the dataset's own structure, not an analyst's ranking.
    Mediations []string

    // FlaggedTraces is the subset of traces carrying a "delay" or
    // "threshold" tag.
    FlaggedTraces []FlaggedTrace
}

// FlaggedTrace is a minimal projection of a trace that carries a delay
// or threshold tag.
type FlaggedTrace struct {
    ID          string
    WhatChanged string
    Tags        []string
}
```

### Functions

```go
// Load reads a JSON file at path, decodes each Trace, validates it via
// schema.Validate(), and returns the full slice or the first validation error.
func Load(path string) ([]schema.Trace, error)

// Summarise builds a MeshSummary from a slice of already-validated traces.
// Does not call Validate() — that is Load's responsibility.
func Summarise(traces []schema.Trace) MeshSummary

// PrintSummary writes a provisional mesh summary to w.
// Takes io.Writer (not os.Stdout) so output is testable.
func PrintSummary(w io.Writer, s MeshSummary)
```

---

## Output Format (example — vendor registration dataset)

```
=== Mesh Summary (provisional) ===

Elements (source/target appearances across all traces):
  vendor-registration-application-00142  x8
  intake-queue                           x2
  vendor-web-portal                      x1
  vendor-registration-form-v4            x1
  intake-form-validator-v4               x1
  rate-limiter                           x1
  queue-throughput-policy-v2             x1
  classification-ruleset-v7              x1
  approval-threshold-rule-tier2          x1
  escalation-routing-matrix-v3           x1
  background-check-service               x1
  compliance-reviewer                    x1
  compliance-approval-checklist-v2       x1

Observed mediations (8 traces, 7 unique):
  intake-form-validator-v4
  queue-throughput-policy-v2
  classification-ruleset-v7
  approval-threshold-rule-tier2
  escalation-routing-matrix-v3
  background-check-webhook-endpoint
  compliance-approval-checklist-v2

Traces tagged delay or threshold (5):
  e6a0b4d5...  [delay threshold]  Application buffered 38 minutes by rate-limiter...
  a8c2d6f7...  [threshold redirection]  Contract value $340,000 exceeds $250,000...
  c0e4f8b9...  [delay]  Application entered 72-hour hold...
  d1f5a9c0...  [translation]  Background check result received via inbound webhook...
  e2a6b0d1...  [threshold]  Application approved by compliance reviewer...

---
Note: this is a first look at the mesh, not a classification of actors.
Elements listed here are names that appear in traces — they may be human,
non-human, or assemblages. Their roles are not yet determined.
```

**Format decisions:**
- Elements sorted by descending frequency, then alphabetically within same frequency
- Mediations in encounter order (intentional — preserves dataset structure)
- All tags shown on flagged traces, not just the triggering one
- Footer disclaimer is mandatory output, not a comment (encodes Principle 8)

---

## Test Cases

**File:** `meshant/loader/loader_test.go` — package `loader_test` (black-box)

### Group 1: Load — Happy Path
- `TestLoad_ReturnsCorrectCount` — load examples/traces.json, check 10 traces, no error
- `TestLoad_AllTracesPassValidation` — spot-validate each returned trace
- `TestLoad_FieldsIntact` — spot-check one trace by ID for field correctness

### Group 2: Load — Error Cases
- `TestLoad_FileNotFound` — non-existent path → error containing path
- `TestLoad_InvalidJSON` — temp file with bad JSON → non-nil error
- `TestLoad_ValidationErrorPropagated` — temp file with missing observer → error mentioning observer
- `TestLoad_EmptyArray` — temp file with `[]` → 0 traces, no error

### Group 3: Summarise — Correctness
- `TestSummarise_ElementFrequency` — known traces, check element counts
- `TestSummarise_ElementsUnionOfSourceAndTarget` — name in both source+target counted twice
- `TestSummarise_MediationsDeduped` — same mediation in two traces → one entry
- `TestSummarise_MediationsEncounterOrder` — order preserved
- `TestSummarise_EmptyMediationExcluded` — no-mediation trace adds no empty string
- `TestSummarise_FlaggedTracesDelay` — `["delay"]` trace appears in FlaggedTraces
- `TestSummarise_FlaggedTracesThreshold` — `["threshold"]` trace appears
- `TestSummarise_FlaggedTracesOtherTagsExcluded` — `["blockage"]` trace excluded
- `TestSummarise_FlaggedTracesFields` — correct ID, WhatChanged, full Tags
- `TestSummarise_EmptyInput` — `Summarise(nil)` → zero MeshSummary, no panic

### Group 4: PrintSummary — Output
- `TestPrintSummary_ContainsElementsHeader`
- `TestPrintSummary_ContainsMediationsHeader`
- `TestPrintSummary_ContainsFlaggedHeader`
- `TestPrintSummary_ContainsProvisionalNote` — footer disclaimer present
- `TestPrintSummary_ElementAppearsWithCount` — known element + `x{N}` in output
- `TestPrintSummary_EmptySummary_DoesNotPanic`

---

## Implementation Steps

1. Scaffold `loader.go` — stub types and function signatures (compilable)
2. Write `loader_test.go` with all tests (RED — most will fail)
3. Implement `Load` (GREEN for Groups 1 + 2)
   - `os.Open` → `json.NewDecoder` → `Decode` → validate each trace
   - Stop at first validation error, wrap with trace index + ID context
4. Implement `Summarise` (GREEN for Group 3)
   - Count elements from Source + Target slices
   - Deduplicate mediations in encounter order
   - Flag traces with delay or threshold; use `break` to avoid double-counting
5. Implement `PrintSummary` (GREEN for Group 4)
   - Build sorted element slice (desc count, asc name) via `sort.Slice`
   - Mediations in encounter order (slice order from Summarise)
   - Footer note is mandatory output
6. Run `go test -race -cover ./...` — all 27 schema tests + all loader tests must pass
7. `gofmt` and review doc comments

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| Three separate functions (Load / Summarise / PrintSummary) | Each layer independently testable; no forced form factor |
| `io.Writer` on PrintSummary | Canonical Go; output testable via bytes.Buffer without stdout capture |
| `FlaggedTrace` projection (not `[]schema.Trace`) | Signals summary ≠ full record; ANT-consistent (don't overload the element) |
| Element count = appearances not unique-traces | More faithful to "following what is active" for future heterogeneous assemblages |
| Mediations in encounter order | Encounter order is part of the dataset's structure; alphabetical sort would erase it |
| Load stops at first validation error | Simpler signature; sufficient at this stage |
| Footer disclaimer in PrintSummary output | Encodes Principle 8 (provisional, not ontology) — tested for presence |
| No `LoadAndPrint` convenience wrapper | Avoids locking in form factor prematurely |
