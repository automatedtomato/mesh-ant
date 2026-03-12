// Package graph — reflexive.go contains functions that produce Traces recording
// the act of articulation or diffing. This is M7.2: reflexive tracing.
//
// Reflexive tracing embodies two core ANT commitments:
//
//  1. Principle 8 (the designer is inside the mesh): when you articulate a graph
//     or diff two graphs, that act is itself a happening in the network. Recording
//     it as a Trace makes the observation apparatus traceable.
//
//  2. Generalised symmetry: a MeshGraph or GraphDiff can appear as Source or Target
//     in a trace (via their graph-ref strings), just like any other actant.
//
// Both functions are explicit opt-ins. ArticulationTrace and DiffTrace are not
// called automatically by Articulate or Diff — the caller decides when an act of
// observation should enter the mesh record. Not every articulation needs to be one.
package graph

import (
	"fmt"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// ArticulationTrace produces a Trace recording the act of articulation.
//
// g must be an identified MeshGraph (non-empty g.ID). Call graph.IdentifyGraph
// first if g was returned directly from Articulate.
//
// observer is the caller's position — who performed this articulation. It maps
// to schema.Trace.Observer and is required by schema.Validate.
//
// source may be nil — absent source is valid when the input traces have no
// collective identity. When non-nil, source values appear in the trace's Source
// field verbatim. Target is always set to the graph-ref string of g, so the
// trace passes schema.Validate even when source is nil.
//
// The produced trace always passes schema.Validate(). The caller should not need
// to call Validate separately unless they mutate the returned trace.
//
// Returns an error if:
//   - g.ID is empty (call IdentifyGraph first)
//   - observer is empty (required by schema.Validate)
func ArticulationTrace(g MeshGraph, observer string, source []string) (schema.Trace, error) {
	// Fail fast — these errors would produce a trace that fails schema.Validate.
	if g.ID == "" {
		return schema.Trace{}, fmt.Errorf("graph.ArticulationTrace: graph has no ID; call IdentifyGraph first")
	}
	if observer == "" {
		return schema.Trace{}, fmt.Errorf("graph.ArticulationTrace: observer must be non-empty")
	}

	// Derive what_changed from the Cut parameters. This describes the position
	// from which the articulation was made, keeping the reflexive trace self-situated.
	whatChanged := articulationWhatChanged(g.Cut)

	// Copy source to avoid capturing caller-owned slice.
	var sourceCopy []string
	if len(source) > 0 {
		sourceCopy = make([]string, len(source))
		copy(sourceCopy, source)
	}

	tr := schema.Trace{
		ID:          newUUID4(),
		Timestamp:   time.Now().UTC(),
		WhatChanged: whatChanged,
		Source:      sourceCopy,
		Target:      []string{graphRefPrefixGraph + g.ID},
		Mediation:   "graph.Articulate",
		Tags:        []string{string(schema.TagValueArticulation)},
		Observer:    observer,
	}
	return tr, nil
}

// DiffTrace produces a Trace recording the act of diffing two graphs.
//
// d, g1, and g2 must all be identified (non-empty ID). Source is derived as
// ["meshgraph:<g1.ID>", "meshgraph:<g2.ID>"]. Target is set to
// ["meshdiff:<d.ID>"]. This makes both input graphs and the resulting diff
// traceable as actants in the mesh.
//
// observer is the caller's position, required by schema.Validate.
//
// The produced trace always passes schema.Validate(). Returns an error if any
// of d, g1, or g2 have an empty ID, or if observer is empty.
func DiffTrace(d GraphDiff, g1, g2 MeshGraph, observer string) (schema.Trace, error) {
	// Validate all required IDs and observer up-front for clear error messages.
	if d.ID == "" {
		return schema.Trace{}, fmt.Errorf("graph.DiffTrace: diff has no ID; call IdentifyDiff first")
	}
	if g1.ID == "" {
		return schema.Trace{}, fmt.Errorf("graph.DiffTrace: g1 has no ID; call IdentifyGraph first")
	}
	if g2.ID == "" {
		return schema.Trace{}, fmt.Errorf("graph.DiffTrace: g2 has no ID; call IdentifyGraph first")
	}
	if observer == "" {
		return schema.Trace{}, fmt.Errorf("graph.DiffTrace: observer must be non-empty")
	}

	whatChanged := diffWhatChanged(d.From, d.To)

	tr := schema.Trace{
		ID:          newUUID4(),
		Timestamp:   time.Now().UTC(),
		WhatChanged: whatChanged,
		Source:      []string{graphRefPrefixGraph + g1.ID, graphRefPrefixGraph + g2.ID},
		Target:      []string{graphRefPrefixDiff + d.ID},
		Mediation:   "graph.Diff",
		Tags:        []string{string(schema.TagValueArticulation)},
		Observer:    observer,
	}
	return tr, nil
}

// articulationWhatChanged derives a human-readable description of the cut for
// use in an ArticulationTrace's WhatChanged field. The description is derived
// from the Cut parameters so the reflexive trace is self-situated.
//
// Format:
//   - observers set, no window:   "articulate: observer=[pos1, pos2]"
//   - observers set, window set:  "articulate: observer=[pos1, pos2] window=START–END"
//   - no observers, window set:   "articulate: window=START–END"
//   - neither set (full cut):     "articulate: full cut"
func articulationWhatChanged(c Cut) string {
	var parts []string

	if len(c.ObserverPositions) > 0 {
		parts = append(parts, fmt.Sprintf("observer=[%s]", strings.Join(c.ObserverPositions, ", ")))
	}

	if !c.TimeWindow.IsZero() {
		startStr := "(unbounded)"
		if !c.TimeWindow.Start.IsZero() {
			startStr = c.TimeWindow.Start.UTC().Format(time.RFC3339)
		}
		endStr := "(unbounded)"
		if !c.TimeWindow.End.IsZero() {
			endStr = c.TimeWindow.End.UTC().Format(time.RFC3339)
		}
		parts = append(parts, fmt.Sprintf("window=%s\u2013%s", startStr, endStr))
	}

	if len(parts) == 0 {
		return "articulate: full cut"
	}
	return "articulate: " + strings.Join(parts, " ")
}

// diffWhatChanged derives a human-readable description of the diff for use in a
// DiffTrace's WhatChanged field. The format is directional: "from → to".
//
// Format: "diff: [from-observers]→[to-observers]"
// When observer lists are empty (full cut), the label is "(full cut)".
func diffWhatChanged(from, to Cut) string {
	fromLabel := "(full cut)"
	if len(from.ObserverPositions) > 0 {
		fromLabel = strings.Join(from.ObserverPositions, ", ")
	}
	toLabel := "(full cut)"
	if len(to.ObserverPositions) > 0 {
		toLabel = strings.Join(to.ObserverPositions, ", ")
	}
	return fmt.Sprintf("diff: [%s]\u2192[%s]", fromLabel, toLabel)
}
