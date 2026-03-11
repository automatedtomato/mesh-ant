# Implementation Plan: M2 — Deforestation Dataset and Graph Articulation

**Status:** Ready to implement
**Branches:**
- `feat/m2-dataset` — M2.1 (cut from develop)
- `feat/m2-graph` — M2.2 + M2.3 (cut from develop, after M2.1 merges)

---

## Philosophy

Milestone 2 introduces a second dataset (richer, multi-threaded) and the first articulation:
a way to render a provisional graph from traces taken from a particular observer position.

A graph in MeshAnt is **not** a god's-eye view of the network. It is a cut — made from a stated
position, at a stated time, with a named shadow. Different observer positions compose
different graphs from the same trace dataset; none is definitive.

This milestone implements **observer position** as the primary cut axis.
Future axes (time window, tag filter) are explicitly deferred to M3+.
The graph-as-actor concept (the graph itself entering the mesh as a force) is noted
architecturally but deferred — see M2.3 decision record.

---

## Milestone 2: Deforestation Dataset and Graph Articulation

### M2.1 — Deforestation Example Dataset

**Branch:** `feat/m2-dataset`
**File:** `data/examples/deforestation.json`

#### Scenario

A 73ha deforestation event in sector BR-AM-441 (Brazilian Amazon) is detected and
acted upon across three crossing threads. The threads involve different actor-assemblages
and observer positions. Traces are dated 2026-03-11.

#### Threads

**Thread A — Satellite-to-policy (7 traces):** Satellite imagery → detection algorithm →
agency alert → inspection scheduling delay → sensor report (absent-source) →
enforcement order (joint threshold) → blockage → ministry redirection.

**Thread B — Community-observation (6 traces):** Community oral report → NGO
documentation (translation) → satellite re-task amplification → legal challenge (delay) →
collective testimony (absent-source, translation) → tribunal standing (threshold) →
international escalation (redirection).

**Thread C — Carbon-market (5 traces):** Credit invalidation notice (threshold) →
broker notification (delay) → market suspension (blockage) → automated correction
report (absent-source, translation) → credit redirection to verified project.

**Cross-thread (2 traces):** Joint verification report (multi-source amplification,
threads A+C converge) → treaty translation into dual enforcement mandate (multi-target).

#### 20 Traces

```json
[
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e601",
    "timestamp": "2026-03-11T02:14:00Z",
    "what_changed": "Multispectral imagery band ratio translated into a raw spectral anomaly report flagging 73ha NDVI deviation in sector BR-AM-441",
    "source": ["landsat-9-overpass-3147"],
    "target": ["raw-spectral-anomaly-report-20260311"],
    "mediation": "deforestation-detection-algorithm-v3",
    "tags": ["translation"],
    "observer": "satellite-operator"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e602",
    "timestamp": "2026-03-11T03:07:00Z",
    "what_changed": "Spectral anomaly of 73ha exceeds 50ha alert threshold; agency notification triggered via anomaly-threshold-ruleset-v2",
    "source": ["raw-spectral-anomaly-report-20260311"],
    "target": ["national-forest-agency-alert-0312"],
    "mediation": "anomaly-threshold-ruleset-v2",
    "tags": ["threshold"],
    "observer": "deforestation-detection-algorithm"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e603",
    "timestamp": "2026-03-11T08:22:00Z",
    "what_changed": "Inspection request entered 14-day scheduling backlog; field visit deferred to 2026-03-25 by inspection-scheduling-queue",
    "source": ["national-forest-agency-alert-0312"],
    "target": ["field-inspection-request-br-am-441"],
    "mediation": "inspection-scheduling-queue",
    "tags": ["delay"],
    "observer": "national-forest-agency"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e604",
    "timestamp": "2026-03-11T09:15:00Z",
    "what_changed": "Autonomous ground sensor array generated structured deforestation report without human attribution; source unidentifiable by observing system",
    "target": ["field-sensor-report-br-am-441-auto"],
    "mediation": "autonomous-sensor-array-protocol",
    "tags": ["translation"],
    "observer": "national-forest-agency"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e605",
    "timestamp": "2026-03-11T11:45:00Z",
    "what_changed": "Joint signature of agency officer and field inspector required by enforcement-authorization-protocol-v4 to issue enforcement order above 60ha threshold",
    "source": ["national-forest-agency", "field-inspector-ramirez"],
    "target": ["enforcement-order-br-am-441"],
    "mediation": "enforcement-authorization-protocol-v4",
    "tags": ["threshold"],
    "observer": "policy-enforcement-officer"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e606",
    "timestamp": "2026-03-11T13:10:00Z",
    "what_changed": "Logging operation cerrado-timber-operations-4412 suspended by enforcement order; equipment access revoked pending investigation",
    "source": ["enforcement-order-br-am-441"],
    "target": ["cerrado-timber-operations-4412"],
    "tags": ["blockage"],
    "observer": "policy-enforcement-officer"
  },
  {
    "id": "3a1f2b4c-d5e6-4f70-8091-a2b3c4d5e607",
    "timestamp": "2026-03-11T14:30:00Z",
    "what_changed": "Enforcement case exceeds regional agency jurisdiction (>60ha); redirected to Ministry of Environment via inter-agency-escalation-protocol",
    "source": ["enforcement-order-br-am-441"],
    "target": ["ministry-of-environment-case-2026-0087"],
    "mediation": "inter-agency-escalation-protocol",
    "tags": ["redirection"],
    "observer": "national-forest-agency"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b801",
    "timestamp": "2026-03-11T07:30:00Z",
    "what_changed": "Oral testimony from community monitor translated to written documented report by ngo-documentation-protocol-v2; spatial coordinates added, identity anonymized on request",
    "source": ["local-community-monitor-tupinamba"],
    "target": ["community-deforestation-report-20260311"],
    "mediation": "ngo-documentation-protocol-v2",
    "tags": ["translation"],
    "observer": "ngo-field-coordinator"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b802",
    "timestamp": "2026-03-11T09:00:00Z",
    "what_changed": "Community report amplified into formal satellite re-task request; alert-routing-protocol matched report to active satellite pass schedule for 2026-03-12 window",
    "source": ["community-deforestation-report-20260311"],
    "target": ["landsat-9-retask-request-20260312"],
    "mediation": "alert-routing-protocol",
    "tags": ["amplification"],
    "observer": "ngo-field-coordinator"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b803",
    "timestamp": "2026-03-11T10:15:00Z",
    "what_changed": "Legal challenge filed but entered 45-day docket queue; federal-court-filing-registry assigned hearing date 2026-04-25",
    "source": ["community-deforestation-report-20260311"],
    "target": ["legal-challenge-filing-2026-0041"],
    "mediation": "federal-court-filing-registry",
    "tags": ["delay"],
    "observer": "ngo-field-coordinator"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b804",
    "timestamp": "2026-03-11T10:45:00Z",
    "what_changed": "Multiple community accounts aggregated into structured impact assessment by testimony-aggregation-protocol-v3; no single source attributable — testimony is collective",
    "target": ["community-impact-assessment-20260311"],
    "mediation": "testimony-aggregation-protocol-v3",
    "tags": ["translation"],
    "observer": "ngo-field-coordinator"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b805",
    "timestamp": "2026-03-11T13:50:00Z",
    "what_changed": "Legal challenge admitted by tribunal-admissibility-framework; documented 73ha loss meets 50ha standing threshold for expedited hearing",
    "source": ["legal-challenge-filing-2026-0041"],
    "target": ["forest-protection-tribunal-case-2026-0041"],
    "mediation": "tribunal-admissibility-framework",
    "tags": ["threshold"],
    "observer": "ngo-field-coordinator"
  },
  {
    "id": "b3c4d5e6-f7a8-4901-b2c3-d4e5f6a7b806",
    "timestamp": "2026-03-11T15:00:00Z",
    "what_changed": "Community complaint escalated from national tribunal to UNFF complaint mechanism; national legal remedy deemed insufficient by un-forest-complaint-mechanism",
    "source": ["local-community-monitor-tupinamba", "ngo-field-coordinator"],
    "target": ["international-treaty-body-complaint-2026-0018"],
    "mediation": "un-forest-complaint-mechanism",
    "tags": ["redirection"],
    "observer": "international-treaty-body"
  },
  {
    "id": "c5d6e7f8-a9b0-4c12-d3e4-f5a6b7c8d901",
    "timestamp": "2026-03-11T08:00:00Z",
    "what_changed": "73ha deforestation triggers invalidation of 14,600 carbon credits in VCS project 2847; credit loss exceeds 5% buffer-pool threshold in vcs-permanence-buffer-protocol",
    "source": ["raw-spectral-anomaly-report-20260311"],
    "target": ["carbon-credit-invalidation-notice-vcs-2847"],
    "mediation": "vcs-permanence-buffer-protocol",
    "tags": ["threshold"],
    "observer": "carbon-registry-auditor"
  },
  {
    "id": "c5d6e7f8-a9b0-4c12-d3e4-f5a6b7c8d902",
    "timestamp": "2026-03-11T10:00:00Z",
    "what_changed": "Broker notification entered 48-hour reconciliation cycle; registry-reconciliation-scheduler queues batched notifications — broker verde-carbon-ltd not notified until 2026-03-13",
    "source": ["carbon-credit-invalidation-notice-vcs-2847"],
    "target": ["broker-notification-verde-carbon-ltd"],
    "mediation": "registry-reconciliation-scheduler",
    "tags": ["delay"],
    "observer": "carbon-registry-auditor"
  },
  {
    "id": "c5d6e7f8-a9b0-4c12-d3e4-f5a6b7c8d903",
    "timestamp": "2026-03-11T11:00:00Z",
    "what_changed": "Trading in VCS-2847 credits suspended; market temporarily blocked pending confirmation of deforestation extent and buffer-pool recalculation",
    "source": ["carbon-credit-invalidation-notice-vcs-2847"],
    "target": ["voluntary-carbon-market-trading-vcs-2847"],
    "tags": ["blockage"],
    "observer": "carbon-credit-broker"
  },
  {
    "id": "c5d6e7f8-a9b0-4c12-d3e4-f5a6b7c8d904",
    "timestamp": "2026-03-11T12:30:00Z",
    "what_changed": "Automated reconciliation engine generated market-correction-report without human attribution; source is the registry system itself, not a named actor",
    "target": ["market-correction-report-vcs-2847-20260311"],
    "mediation": "registry-automated-reconciliation-engine",
    "tags": ["translation"],
    "observer": "carbon-registry-auditor"
  },
  {
    "id": "c5d6e7f8-a9b0-4c12-d3e4-f5a6b7c8d905",
    "timestamp": "2026-03-11T14:00:00Z",
    "what_changed": "Invalidated VCS-2847 credits redirected to verified reforestation project mata-viva-0031 via buffer-pool-replacement-protocol; buyer contracts reissued with new project reference",
    "source": ["carbon-credit-broker", "carbon-registry-auditor"],
    "target": ["reforestation-replacement-credits-mata-viva-0031"],
    "mediation": "buffer-pool-replacement-protocol",
    "tags": ["redirection"],
    "observer": "carbon-credit-broker"
  },
  {
    "id": "d7e8f9a0-b1c2-4d34-e5f6-a7b8c9d0e101",
    "timestamp": "2026-03-11T16:00:00Z",
    "what_changed": "Satellite deforestation data and carbon-registry invalidation data jointly amplified through inter-agency-data-fusion-protocol into verified enforcement report; two independent threads converge as corroborating evidence",
    "source": ["national-forest-agency", "carbon-registry-auditor"],
    "target": ["joint-verification-report-br-am-441-20260311"],
    "mediation": "inter-agency-data-fusion-protocol",
    "tags": ["amplification"],
    "observer": "policy-enforcement-officer"
  },
  {
    "id": "d7e8f9a0-b1c2-4d34-e5f6-a7b8c9d0e102",
    "timestamp": "2026-03-11T17:30:00Z",
    "what_changed": "UNFF article-5 obligation translated into dual mandate: national-forest-agency must report under REDD+ protocol; carbon-registry-auditor must verify credit removals under Paris Agreement article-5-implementation-guide",
    "source": ["international-treaty-body"],
    "target": ["national-forest-agency", "carbon-registry-auditor"],
    "mediation": "paris-agreement-article-5-implementation-guide",
    "tags": ["translation"],
    "observer": "international-treaty-body"
  }
]
```

#### Coverage summary

| Tag          | Count | Traces                                          |
|--------------|-------|-------------------------------------------------|
| translation  | 6     | A01, A04, B01, B04, C04, X02                   |
| threshold    | 4     | A02, A05, B05, C01                             |
| delay        | 3     | A03, B03, C02                                   |
| redirection  | 3     | A07, B06, C05                                   |
| blockage     | 2     | A06, C03                                        |
| amplification| 2     | B02, X01                                        |

Absent-source traces: A04, B04, C04 (3 traces — automated systems, collective attribution)
Multi-source traces: A05, B06, C05, X01 (4 traces)
Multi-target traces: X02 (1 trace — international-treaty-body → 2 targets)

Observer positions (9 distinct):
`satellite-operator`, `deforestation-detection-algorithm`, `national-forest-agency`,
`ngo-field-coordinator`, `carbon-registry-auditor`, `carbon-credit-broker`,
`policy-enforcement-officer`, `international-treaty-body`

(Note: `local-community-monitor` appears as source but never as observer — this is
intentional. Community monitors are observed by the NGO field coordinator, not
self-reporting into this dataset. Their observer position is absent — which is itself
a trace of asymmetry worth noting.)

---

### M2.2 — Graph Articulation Package

**Branch:** `feat/m2-graph`
**Files:**
```
meshant/
  graph/
    graph.go       — all exported types and functions
    graph_test.go  — black-box tests in package graph_test
    e2e_test.go    — full pipeline test against deforestation dataset
```

#### Exported API

```go
// Package graph provides functions to articulate a MeshGraph from a trace dataset.
//
// Articulation is a cut: a provisional rendering of the mesh from a particular
// observer position. It does not produce a neutral, definitive graph. The cut
// always names what it excludes: the shadow elements visible from other positions
// but not from the chosen one.
//
// See docs/decisions/articulation-v1.md for the rationale behind these design
// choices and what has been explicitly deferred to future milestones.
package graph

// ArticulationOptions parameterises the cut made when producing a MeshGraph.
//
// ObserverPositions filters traces to only those whose Observer field matches
// one of the listed strings. An empty slice means no filter: all traces are
// included. This models the choice to take a god's-eye position — valid as an
// option, but named so that callers cannot take it accidentally.
type ArticulationOptions struct {
    ObserverPositions []string
}

// MeshGraph is a provisional, observer-positioned rendering of a trace dataset.
// It is not a definitive description of the network. The Cut field names the
// position from which it was made and the shadow elements it cannot see.
type MeshGraph struct {
    // Nodes maps element names to their node data. An element enters the graph
    // if it appeared in the Source or Target of any included trace.
    Nodes map[string]Node

    // Edges is one edge per included trace, preserving dataset order.
    Edges []Edge

    // Cut records the articulation parameters and the shadow:
    // elements that exist in the full dataset but are invisible from
    // the chosen observer position(s).
    Cut Cut
}

// Node represents a named element in the graph. It counts how many times the
// element appeared across included traces (AppearanceCount) and how many
// additional traces would mention it if the observer filter were removed
// (ShadowCount — from traces in the shadow, not the included set).
type Node struct {
    Name            string
    AppearanceCount int
    // ShadowCount is the number of shadow traces in which this element appears.
    // Zero for nodes that are not also shadow elements.
    ShadowCount int
}

// Edge represents one trace in the graph. It preserves the full trace
// context so that graph consumers can follow back to the source record.
type Edge struct {
    TraceID     string
    WhatChanged string
    Mediation   string
    Observer    string
    Sources     []string
    Targets     []string
    Tags        []string
}

// Cut records the position from which a MeshGraph was articulated and names
// the shadow: what this cut excludes.
type Cut struct {
    // ObserverPositions lists the filter used. Empty means no filter (full cut).
    ObserverPositions []string

    // TracesIncluded is the number of traces that passed the filter.
    TracesIncluded int

    // TracesTotal is the total number of traces in the input dataset.
    TracesTotal int

    // DistinctObserversTotal is the number of distinct observer strings
    // across all traces in the input (before filtering).
    DistinctObserversTotal int

    // ShadowElements is the list of elements (source/target names) that appear
    // in excluded traces but not in any included trace. These are the elements
    // that this cut cannot see. Sorted alphabetically.
    ShadowElements []ShadowElement
}

// ShadowElement is an element that exists in the dataset but falls outside
// the current cut. SeenFrom lists the observer positions from which this
// element would become visible.
type ShadowElement struct {
    Name     string
    // SeenFrom lists the distinct observer strings of the shadow traces
    // in which this element appears. Sorted alphabetically.
    SeenFrom []string
}

// Articulate builds a MeshGraph from a slice of already-validated traces and
// the given ArticulationOptions. It does not call schema.Validate() —
// that is the loader's responsibility.
//
// If opts.ObserverPositions is empty, all traces are included (full cut).
// The Cut.ShadowElements field is always populated relative to the chosen
// filter, even when no filter is applied (in which case it will be empty).
func Articulate(traces []schema.Trace, opts ArticulationOptions) MeshGraph

// PrintArticulation writes a provisional mesh graph to w.
// The shadow section is mandatory output — it encodes the methodological
// commitment that this graph is a cut, not a complete account.
//
// Returns the first write error encountered, if any.
func PrintArticulation(w io.Writer, g MeshGraph) error
```

#### Output format (example — deforestation dataset, observer: carbon-registry-auditor)

```
=== Mesh Articulation (provisional cut) ===
Observer position(s): carbon-registry-auditor
Traces included: 4 of 20 (distinct observers in full dataset: 8)

Nodes (elements visible from this position):
  raw-spectral-anomaly-report-20260311      x1
  carbon-credit-invalidation-notice-vcs-2847 x2
  broker-notification-verde-carbon-ltd      x1
  market-correction-report-vcs-2847-20260311 x1
  reforestation-replacement-credits-mata-viva-0031 x1

Edges (traces in this cut):
  c5d6e7f8...  [threshold]  73ha deforestation triggers invalidation...
  c5d6e7f8...  [delay]      Broker notification entered 48-hour...
  c5d6e7f8...  [translation] Automated reconciliation engine...
  c5d6e7f8...  [redirection] Invalidated VCS-2847 credits redirected...

Shadow (elements invisible from this position: 16):
  broker-notification-verde-carbon-ltd → also seen from: carbon-credit-broker
  cerrado-timber-operations-4412 → also seen from: policy-enforcement-officer
  community-deforestation-report-20260311 → also seen from: ngo-field-coordinator
  ... (etc.)

---
Note: this graph is a cut made from one position in the mesh.
Elements in the shadow are not absent — they are invisible from here.
Observer position(s) not included: deforestation-detection-algorithm,
  international-treaty-body, national-forest-agency, ngo-field-coordinator,
  policy-enforcement-officer, satellite-operator.
```

**Format decisions:**
- Nodes sorted by descending appearance count, then alphabetically
- Edges in dataset order (preserves temporal sequence)
- Shadow section is mandatory — absence of shadow means full cut was taken
- Footer names excluded observer positions explicitly
- When no filter (full cut): shadow section still appears, stated empty

---

#### Test Cases

**File:** `meshant/graph/graph_test.go` — package `graph_test`

##### Group 1: Articulate — Full cut (no filter)
- `TestArticulate_FullCut_IncludesAllTraces` — empty opts, TracesIncluded == TracesTotal
- `TestArticulate_FullCut_EmptyShadow` — no shadow elements when all traces included
- `TestArticulate_FullCut_NodeCount` — expected number of distinct elements
- `TestArticulate_FullCut_EdgeCount` — one edge per trace

##### Group 2: Articulate — Observer filter
- `TestArticulate_SingleObserver_TracesIncluded` — filter to one observer, correct count
- `TestArticulate_SingleObserver_ShadowPopulated` — shadow non-empty when filter applied
- `TestArticulate_SingleObserver_ShadowNotInNodes` — shadow elements not in Nodes map
- `TestArticulate_SingleObserver_NodesFromIncludedOnly` — Nodes only contain elements from included traces
- `TestArticulate_MultiObserver_Union` — two observer positions, union of their traces
- `TestArticulate_UnknownObserver_ZeroTraces` — filter to unknown observer → 0 included, all shadow

##### Group 3: Articulate — Cut metadata
- `TestArticulate_Cut_TracesTotal` — always equals len(input)
- `TestArticulate_Cut_DistinctObserversTotal` — counts unique observer strings in full input
- `TestArticulate_Cut_ObserverPositionsStored` — filter stored in Cut.ObserverPositions
- `TestArticulate_Cut_ShadowSeenFrom` — ShadowElement.SeenFrom lists correct observer positions
- `TestArticulate_Cut_ShadowSortedAlphabetically` — ShadowElements in alpha order

##### Group 4: Articulate — Node and Edge content
- `TestArticulate_NodeAppearanceCount` — element appearing in 3 traces counts 3
- `TestArticulate_NodeShadowCount` — shadow element's ShadowCount matches trace count
- `TestArticulate_EdgeFields` — Edge carries correct TraceID, WhatChanged, Mediation, etc.
- `TestArticulate_Edge_TagsCopied` — Edge.Tags is a copy, not a shared reference
- `TestArticulate_EmptyInput` — Articulate(nil, opts) returns zero MeshGraph, no panic

##### Group 5: PrintArticulation — Output
- `TestPrintArticulation_ContainsHeader`
- `TestPrintArticulation_ContainsObserverLine`
- `TestPrintArticulation_ContainsNodesSection`
- `TestPrintArticulation_ContainsEdgesSection`
- `TestPrintArticulation_ContainsShadowSection` — shadow section present even if empty
- `TestPrintArticulation_ContainsFooter`
- `TestPrintArticulation_WriterErrorPropagated`
- `TestPrintArticulation_EmptyGraph_DoesNotPanic`

**File:** `meshant/graph/e2e_test.go` — package `graph_test`

- `TestE2E_FullCut` — Load deforestation.json → Articulate(all) → PrintArticulation: 20 traces, no shadow
- `TestE2E_CarbonRegistryAuditorCut` — filter to carbon-registry-auditor: 4 traces included, shadow non-empty
- `TestE2E_NGOCut` — filter to ngo-field-coordinator: 5 traces, shadow elements visible from other positions
- `TestE2E_CrossThreadCut` — filter to policy-enforcement-officer: 3 traces (A05, A06, X01), shadow covers B+C threads

---

### M2.3 — Articulation Decision Record

**Branch:** `feat/m2-graph`
**File:** `docs/decisions/articulation-v1.md`

Content to record:
1. Observer position as primary cut axis — rationale (ANT: no god's-eye view)
2. Shadow as mandatory output — rationale (Strathern: every cut names its exclusions)
3. Empty filter = full cut, not error — rationale (full cut is valid; must be named)
4. Deferred axes: time window, tag filter — recorded with explicit deferral note
5. Graph-as-actor: noted architecturally for M3+. Once produced, a graph enters
   the mesh as a potential source/target. A deforestation map triggers policy;
   the graph is not neutral. Implementation deferred.
6. ShadowElement.SeenFrom: records which positions would make the invisible visible —
   this is the shadow's own trace.

---

## Implementation Order

### M2.1 — Dataset first

1. Create `feat/m2-dataset` branch from `develop`
2. Write `data/examples/deforestation.json` (exact JSON from plan)
3. Validate locally: `go test ./meshant/loader/... -run TestLoad` with a quick loader
   call, or write a one-off validation script
4. Pre-merge review → merge to `develop`

### M2.2 — Graph package (TDD)

1. Create `feat/m2-graph` branch from `develop` (after M2.1 merges)
2. Scaffold `graph.go` — stubs, compilable
3. Write `graph_test.go` — all groups RED
4. Write `e2e_test.go` — RED
5. Implement `Articulate` — GREEN groups 1–4
6. Implement `PrintArticulation` — GREEN group 5 + e2e
7. `go test -race -cover ./...` — all tests pass, ≥80% coverage
8. `gofmt` + review doc comments

### M2.3 — Decision record

1. Write `docs/decisions/articulation-v1.md`
2. Include in same branch/PR as M2.2

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| Observer position as primary cut axis | ANT: no god's-eye view; agency is situated |
| Empty filter = full cut (not error) | Allows full-mesh view but forces naming it as a position |
| Shadow section is mandatory output | Strathern: every representation names what it excludes |
| ShadowElement.SeenFrom | The shadow has its own trace — names who can see what you cannot |
| ShadowElements sorted alphabetically | Shadow is not ordered by importance — order would impose a ranking |
| Node.ShadowCount | Lets a consumer see how much more of the mesh is behind each element |
| Edges in dataset order | Temporal sequence is part of the dataset's structure |
| Edge.Tags is a copy | Same immutability principle as FlaggedTrace.Tags in loader |
| Graph-as-actor: deferred | Architecturally noted; implementing it now would force premature form |
| Time window, tag filter: deferred | M3+ axes; implementing them now over-specifies the cut machinery |
| No LoadAndArticulate convenience wrapper | Avoids forcing form factor prematurely |
