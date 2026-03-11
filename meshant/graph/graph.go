// Package graph provides functions to articulate a MeshGraph from a trace dataset.
//
// Articulation is a cut: a provisional rendering of the mesh from a particular
// observer position. It does not produce a neutral, definitive graph. Every cut
// names what it excludes — the shadow elements visible from other positions but
// not from the chosen one. There is no god's-eye view; every articulation is
// made from somewhere, by someone, at some time.
//
// See docs/decisions/articulation-v1.md for the rationale behind these design
// choices and what has been explicitly deferred to future milestones.
package graph

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// ArticulationOptions parameterises the cut made when producing a MeshGraph.
//
// ObserverPositions filters traces to only those whose Observer field matches
// one of the listed strings. An empty slice means no filter: all traces are
// included. This models the choice to take a god's-eye position — valid as an
// option, but named so that callers cannot take it accidentally.
type ArticulationOptions struct {
	// ObserverPositions is a list of observer strings to include. When empty,
	// all traces are included (full cut). When non-empty, only traces whose
	// Observer field exactly matches one of the listed strings are included.
	ObserverPositions []string
}

// MeshGraph is a provisional, observer-positioned rendering of a trace dataset.
// It is not a definitive description of the network. The Cut field names the
// position from which it was made and the shadow elements it cannot see.
//
// A MeshGraph is not neutral. It is produced by a cut that includes some
// traces and excludes others. The shadow records what the cut cannot see.
//
// MeshGraph is not safe for concurrent mutation. Nodes is a map; concurrent
// reads and writes to the same MeshGraph from multiple goroutines require
// external synchronisation.
type MeshGraph struct {
	// Nodes maps element names to their node data. An element enters the graph
	// if it appeared in the Source or Target of any included trace.
	Nodes map[string]Node

	// Edges is one edge per included trace, preserving dataset order.
	// Edges in dataset order preserves the temporal sequence, which is part
	// of what the dataset is saying about the network's structure.
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
//
// AppearanceCount counts total appearances (source + target), not unique traces.
// This is consistent with ANT's interest in what is actively making a difference,
// not just what nominally exists.
type Node struct {
	// Name is the element string as it appeared in the trace source/target slices.
	Name string

	// AppearanceCount is the total number of times this element appeared in
	// Source or Target slices across all included traces. An element can
	// accumulate count from both source and target roles.
	AppearanceCount int

	// ShadowCount is the number of shadow traces in which this element appears.
	// Zero for nodes that are not also shadow elements. A non-zero ShadowCount
	// indicates this element has a larger presence in the mesh than is visible
	// from the current observer position.
	ShadowCount int
}

// Edge represents one trace in the graph. It preserves the full trace
// context so that graph consumers can follow back to the source record.
//
// Edge.Tags is a copy of the source trace's Tags slice, not a reference to it.
// Callers may safely modify Edge.Tags without affecting subsequent operations
// on the original trace data.
type Edge struct {
	// TraceID is the UUID of the source trace.
	TraceID string

	// WhatChanged is the short description of the difference from the trace.
	WhatChanged string

	// Mediation is the intermediary that transformed, redirected, or relayed
	// the action. Empty if no intermediary was observed.
	Mediation string

	// Observer is the observer string from the source trace.
	Observer string

	// Sources is a copy of the trace's Source slice.
	Sources []string

	// Targets is a copy of the trace's Target slice.
	Targets []string

	// Tags is a copy of the trace's Tags slice. Safe to mutate.
	Tags []string
}

// Cut records the position from which a MeshGraph was articulated and names
// the shadow: what this cut excludes. The shadow is mandatory output — every
// representation names what it cannot see.
type Cut struct {
	// ObserverPositions lists the filter used. Empty means no filter (full cut).
	// Stored verbatim from ArticulationOptions.
	ObserverPositions []string

	// TracesIncluded is the number of traces that passed the filter.
	TracesIncluded int

	// TracesTotal is the total number of traces in the input dataset,
	// before any filtering. This is always equal to len(input).
	TracesTotal int

	// DistinctObserversTotal is the number of distinct observer strings
	// across all traces in the input (before filtering). This names how
	// many positions exist in the full dataset, independent of which filter
	// was chosen.
	DistinctObserversTotal int

	// ShadowElements is the list of elements (source/target names) that appear
	// in excluded traces but not in any included trace. These are the elements
	// that this cut cannot see. Sorted alphabetically so that the shadow is not
	// implicitly ranked by order of appearance.
	ShadowElements []ShadowElement

	// ExcludedObserverPositions lists the distinct observer strings in the full
	// dataset that are NOT in ObserverPositions. Stored in Articulate where the
	// full observer set is known — PrintArticulation uses this directly rather
	// than reconstructing it from graph structure. Empty when no filter was
	// applied (full cut). Sorted alphabetically.
	ExcludedObserverPositions []string
}

// ShadowElement is an element that exists in the dataset but falls outside
// the current cut. SeenFrom lists the observer positions from which this
// element would become visible — the shadow has its own trace.
type ShadowElement struct {
	// Name is the element string as it appeared in shadow trace source/target slices.
	Name string

	// SeenFrom lists the distinct observer strings of the shadow traces in which
	// this element appears. Sorted alphabetically. This records which positions
	// in the mesh can see what this cut cannot.
	SeenFrom []string
}

// Articulate builds a MeshGraph from a slice of already-validated traces and
// the given ArticulationOptions. It does not call schema.Validate() —
// that is the loader's responsibility.
//
// If opts.ObserverPositions is empty, all traces are included (full cut).
// The Cut.ShadowElements field is always populated relative to the chosen
// filter, even when no filter is applied (in which case it will be empty,
// since no traces are excluded).
//
// Nodes contains only elements from included traces. ShadowElements contains
// elements that appear exclusively in excluded traces. Elements that appear in
// both included and excluded traces are in Nodes (with a non-zero ShadowCount)
// but not in ShadowElements.
//
// Edges are in dataset order, preserving the temporal sequence of the input.
// Edge.Tags, Edge.Sources, Edge.Targets, and Cut.ObserverPositions are always
// copies — mutating them does not affect subsequent calls, the original trace
// data, or the ArticulationOptions passed in.
func Articulate(traces []schema.Trace, opts ArticulationOptions) MeshGraph {
	// Copy ObserverPositions so the caller cannot affect the returned Cut
	// by mutating opts after the call. Consistent with the copy treatment
	// applied to Edge.Tags, Edge.Sources, and Edge.Targets below.
	positionsCopy := make([]string, len(opts.ObserverPositions))
	copy(positionsCopy, opts.ObserverPositions)

	// Build observer filter set for O(1) lookup.
	filterSet := make(map[string]bool, len(positionsCopy))
	for _, op := range positionsCopy {
		filterSet[op] = true
	}
	filtered := len(filterSet) > 0

	// Count distinct observers across ALL traces before any filtering.
	allObservers := make(map[string]bool)
	for _, t := range traces {
		allObservers[t.Observer] = true
	}

	// Split traces into included (pass filter) and excluded (fail filter).
	var included, excluded []schema.Trace
	for _, t := range traces {
		if !filtered || filterSet[t.Observer] {
			included = append(included, t)
		} else {
			excluded = append(excluded, t)
		}
	}

	// Count element appearances across included traces.
	// AppearanceCount is total appearances (source + target), not unique traces.
	includedElements := make(map[string]int)
	for _, t := range included {
		for _, s := range t.Source {
			includedElements[s]++
		}
		for _, tg := range t.Target {
			includedElements[tg]++
		}
	}

	// Build edges in dataset order. Each slice field is a copy so callers
	// cannot affect the input or subsequent Articulate calls.
	edges := make([]Edge, 0, len(included))
	for _, t := range included {
		tags := make([]string, len(t.Tags))
		copy(tags, t.Tags)
		sources := make([]string, len(t.Source))
		copy(sources, t.Source)
		targets := make([]string, len(t.Target))
		copy(targets, t.Target)
		edges = append(edges, Edge{
			TraceID:     t.ID,
			WhatChanged: t.WhatChanged,
			Mediation:   t.Mediation,
			Observer:    t.Observer,
			Sources:     sources,
			Targets:     targets,
			Tags:        tags,
		})
	}

	// Build shadow data from excluded traces.
	// For each element in excluded traces, track: how many excluded traces
	// mention it (count) and from which observer positions (seenFrom).
	// Count is per-trace, not per-appearance: if a trace has X in both
	// source and target, it counts as one mention.
	type shadowInfo struct {
		seenFrom map[string]bool
		count    int // number of excluded traces that mention this element
	}
	shadowData := make(map[string]*shadowInfo)
	for _, t := range excluded {
		// Collect this trace's elements, deduplicating within the trace.
		traceElems := make(map[string]bool)
		for _, s := range t.Source {
			traceElems[s] = true
		}
		for _, tg := range t.Target {
			traceElems[tg] = true
		}
		for e := range traceElems {
			if shadowData[e] == nil {
				shadowData[e] = &shadowInfo{seenFrom: make(map[string]bool)}
			}
			shadowData[e].count++
			shadowData[e].seenFrom[t.Observer] = true
		}
	}

	// Build Nodes from included elements, adding ShadowCount for elements
	// that also appear in excluded traces.
	nodes := make(map[string]Node, len(includedElements))
	for name, count := range includedElements {
		shadowCount := 0
		if sd, ok := shadowData[name]; ok {
			shadowCount = sd.count
		}
		nodes[name] = Node{
			Name:            name,
			AppearanceCount: count,
			ShadowCount:     shadowCount,
		}
	}

	// Build ShadowElements: elements that appear ONLY in excluded traces.
	// Elements present in both included and excluded traces are in Nodes
	// (with ShadowCount > 0) but do not appear in ShadowElements.
	var shadowElems []ShadowElement
	for name, sd := range shadowData {
		if _, inIncluded := includedElements[name]; inIncluded {
			continue // visible from included traces → Nodes, not shadow
		}
		seenFrom := make([]string, 0, len(sd.seenFrom))
		for obs := range sd.seenFrom {
			seenFrom = append(seenFrom, obs)
		}
		sort.Strings(seenFrom)
		shadowElems = append(shadowElems, ShadowElement{
			Name:     name,
			SeenFrom: seenFrom,
		})
	}
	// Sort shadow elements alphabetically — order must not imply ranking.
	sort.Slice(shadowElems, func(i, j int) bool {
		return shadowElems[i].Name < shadowElems[j].Name
	})

	// Compute excluded observer positions now, while the full observer set is
	// available. Stored in Cut so PrintArticulation does not need to reconstruct
	// it from graph structure (which would miss observers whose traces are
	// entirely in the shadow-count zone, not in ShadowElements).
	var excludedObsPositions []string
	if filtered {
		for obs := range allObservers {
			if !filterSet[obs] {
				excludedObsPositions = append(excludedObsPositions, obs)
			}
		}
		sort.Strings(excludedObsPositions)
	}

	return MeshGraph{
		Nodes: nodes,
		Edges: edges,
		Cut: Cut{
			ObserverPositions:         positionsCopy,
			TracesIncluded:            len(included),
			TracesTotal:               len(traces),
			DistinctObserversTotal:    len(allObservers),
			ShadowElements:            shadowElems,
			ExcludedObserverPositions: excludedObsPositions,
		},
	}
}

// PrintArticulation writes a provisional mesh graph to w. The shadow section
// is mandatory output — it encodes the methodological commitment that this
// graph is a cut, not a complete account. Even when the shadow is empty
// (full cut taken), the shadow section appears and states that explicitly.
//
// Output includes:
//   - Header naming the articulation as provisional
//   - Observer position(s) used
//   - Nodes section with element names and appearance counts
//   - Edges section with trace summaries in dataset order
//   - Shadow section listing invisible elements and which positions can see them
//   - Footer note encoding Principle 8: the graph is a cut, not truth
//
// Note: trace field values (element names, WhatChanged, Observer, Mediation,
// shadow SeenFrom strings) are written to w as-is. If w is a terminal writer,
// values containing ANSI control sequences from an untrusted dataset could
// affect terminal state. For trusted local datasets this is not a concern;
// re-evaluate if the tool is ever exposed to external or user-supplied data.
//
// Returns the first write error encountered, if any.
func PrintArticulation(w io.Writer, g MeshGraph) error {
	// Sort nodes by descending appearance count, then alphabetically.
	type nodeEntry struct {
		name  string
		count int
	}
	nodeEntries := make([]nodeEntry, 0, len(g.Nodes))
	for name, node := range g.Nodes {
		nodeEntries = append(nodeEntries, nodeEntry{name, node.AppearanceCount})
	}
	sort.Slice(nodeEntries, func(i, j int) bool {
		if nodeEntries[i].count != nodeEntries[j].count {
			return nodeEntries[i].count > nodeEntries[j].count
		}
		return nodeEntries[i].name < nodeEntries[j].name
	})

	// Observer positions label.
	obsLabel := "(all — no filter)"
	if len(g.Cut.ObserverPositions) > 0 {
		obsLabel = strings.Join(g.Cut.ObserverPositions, ", ")
	}

	// Excluded observer positions are pre-computed in Articulate where the full
	// observer set is known. Use them directly rather than approximating from
	// graph structure.
	excludedObservers := g.Cut.ExcludedObserverPositions

	lines := []string{
		"=== Mesh Articulation (provisional cut) ===",
		"",
		fmt.Sprintf("Observer position(s): %s", obsLabel),
		fmt.Sprintf("Traces included: %d of %d (distinct observers in full dataset: %d)",
			g.Cut.TracesIncluded, g.Cut.TracesTotal, g.Cut.DistinctObserversTotal),
		"",
		"Nodes (elements visible from this position):",
	}
	for _, ne := range nodeEntries {
		lines = append(lines, fmt.Sprintf("  %-50s x%d", ne.name, ne.count))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Edges (traces in this cut: %d):", len(g.Edges)))
	for _, e := range g.Edges {
		// Abbreviate UUID to first 8 chars for readability.
		id := e.TraceID
		if len(id) > 8 {
			id = id[:8] + "..."
		}
		lines = append(lines, fmt.Sprintf("  %s  %v  %s", id, e.Tags, e.WhatChanged))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Shadow (elements invisible from this position: %d):", len(g.Cut.ShadowElements)))
	if len(g.Cut.ShadowElements) == 0 {
		lines = append(lines, "  (none — full cut taken)")
	}
	for _, se := range g.Cut.ShadowElements {
		lines = append(lines, fmt.Sprintf("  %s → also seen from: %s",
			se.Name, strings.Join(se.SeenFrom, ", ")))
	}

	lines = append(lines,
		"",
		"---",
		"Note: this graph is a cut made from one position in the mesh.",
		"Elements in the shadow are not absent — they are invisible from here.",
	)
	if len(excludedObservers) > 0 {
		lines = append(lines,
			fmt.Sprintf("Observer position(s) not included: %s", strings.Join(excludedObservers, ", ")),
		)
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("graph: PrintArticulation: %w", err)
		}
	}
	return nil
}
