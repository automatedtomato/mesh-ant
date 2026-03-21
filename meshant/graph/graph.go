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
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TimeWindow defines an inclusive temporal range for filtering traces.
// Zero Start/End means unbounded on that side. A zero TimeWindow means no filter.
// Bounds: Timestamp >= Start (if non-zero) AND Timestamp <= End (if non-zero).
// JSON: zero bounds serialise as null, not "0001-01-01T00:00:00Z". See serial.go.
type TimeWindow struct {
	// Start is the inclusive lower bound. Zero means unbounded.
	// JSON: null when zero, RFC3339 string when non-zero.
	Start time.Time

	// End is the inclusive upper bound. Zero means unbounded.
	// JSON: null when zero, RFC3339 string when non-zero.
	End time.Time
}

// IsZero reports whether both bounds are unset (no time filter).
func (tw TimeWindow) IsZero() bool {
	return tw.Start.IsZero() && tw.End.IsZero()
}

// Validate returns an error if Start is after End — an inverted window silently
// produces a zero-trace articulation. All other states are valid.
func (tw TimeWindow) Validate() error {
	if !tw.Start.IsZero() && !tw.End.IsZero() && tw.End.Before(tw.Start) {
		return fmt.Errorf("graph: TimeWindow.End (%s) is before Start (%s): inverted window would include zero traces",
			tw.End.UTC().Format(time.RFC3339), tw.Start.UTC().Format(time.RFC3339))
	}
	return nil
}

// ShadowReason describes why an element was placed in the shadow — i.e. why it
// was excluded from the current articulation. An element may be excluded for
// more than one reason simultaneously.
type ShadowReason string

const (
	// ShadowReasonObserver means the element was excluded because the trace
	// that mentioned it did not match the ObserverPositions filter.
	ShadowReasonObserver ShadowReason = "observer"

	// ShadowReasonTagFilter means the element was excluded because the trace
	// that mentioned it did not contain any of the Tags in the filter.
	// Alphabetically between "observer" and "time-window" — this ordering is
	// preserved in ShadowElement.Reasons slices.
	ShadowReasonTagFilter ShadowReason = "tag-filter"

	// ShadowReasonTimeWindow means the element was excluded because the trace
	// that mentioned it fell outside the TimeWindow filter.
	ShadowReasonTimeWindow ShadowReason = "time-window"
)

// ArticulationOptions parameterises the cut made when producing a MeshGraph.
//
// ObserverPositions filters traces to only those whose Observer field matches
// one of the listed strings. An empty slice means no filter: all traces are
// included. This models the choice to take a god's-eye position — valid as an
// option, but named so that callers cannot take it accidentally.
//
// TimeWindow filters traces to those whose Timestamp falls within the window
// (inclusive on both bounds). A zero TimeWindow means no time filter.
//
// Tags filters traces to those whose Tags slice contains at least one of the
// listed tag strings (set intersection / any-match semantics). An empty slice
// means no filter: all traces are included (full tag cut).
//
// When multiple filter fields are set, a trace must satisfy ALL active filters
// (AND semantics across ObserverPositions, TimeWindow, and Tags).
type ArticulationOptions struct {
	// ObserverPositions: only traces whose Observer matches one of these strings
	// are included. Empty means no filter (full cut).
	ObserverPositions []string

	// TimeWindow restricts the cut to traces within the window. Zero = no filter.
	// Value type — copied automatically; no explicit deep-copy needed.
	TimeWindow TimeWindow

	// Tags restricts the cut to traces carrying at least one matching tag (any-match).
	// Empty means no filter. Defensively copied in Articulate.
	Tags []string
}

// MeshGraph is a provisional, observer-positioned rendering of a trace dataset.
// It is produced by a cut that includes some traces and excludes others — not a
// neutral or definitive description. The Cut names the position and what it cannot see.
// Not safe for concurrent mutation (Nodes is a map).
type MeshGraph struct {
	// ID is the stable actor identifier. Empty means not yet identified as an
	// actor — assign via graph.IdentifyGraph.
	ID string `json:"id"`

	// Nodes maps element names to node data. An element enters the graph when
	// it appears in the Source or Target of any included trace.
	Nodes map[string]Node `json:"nodes"`

	// Edges is one edge per included trace, in dataset order (preserving
	// temporal sequence).
	Edges []Edge `json:"edges"`

	// Cut records the articulation parameters and shadow elements.
	Cut Cut `json:"cut"`
}

// Node represents a named element in the graph. AppearanceCount counts total
// source+target appearances (not unique traces) — consistent with ANT's interest
// in what is actively making a difference. ShadowCount records how many excluded
// traces also mention this element (cross-cut presence).
type Node struct {
	// Name is the element string as it appeared in source/target slices.
	Name string `json:"name"`

	// AppearanceCount is the total source+target appearances across included traces.
	AppearanceCount int `json:"appearance_count"`

	// ShadowCount is the number of excluded traces that also mention this element.
	// Non-zero means the element crosses the cut boundary — visible here but also
	// present in traces this cut cannot see.
	ShadowCount int `json:"shadow_count"`
}

// Edge represents one trace in the graph, preserving full trace context.
// Tags, Sources, and Targets are defensive copies — safe to mutate.
type Edge struct {
	TraceID     string   `json:"trace_id"`
	WhatChanged string   `json:"what_changed"`
	// Mediation names what transformed the action. Empty if none was observed.
	// A mediator changes what passes through it — not a neutral conduit.
	Mediation   string   `json:"mediation"`
	Observer    string   `json:"observer"`
	Sources     []string `json:"sources"`
	Targets     []string `json:"targets"`
	Tags        []string `json:"tags"`
}

// Cut records the position from which a MeshGraph was articulated and the shadow
// it cannot see. Shadow is mandatory output — every representation names its limits.
type Cut struct {
	// ObserverPositions lists the filter used. Empty = full cut.
	ObserverPositions []string `json:"observer_positions"`

	// TimeWindow is the temporal filter used. Zero = no filter.
	TimeWindow TimeWindow `json:"time_window"`

	// Tags lists the tag filter used. Empty = full tag cut.
	// Defensively copied — mutations after the call do not affect this slice.
	Tags []string `json:"tags"`

	// TracesIncluded is the number of traces that passed the filter.
	TracesIncluded int `json:"traces_included"`

	// TracesTotal is the total traces in the input before filtering.
	TracesTotal int `json:"traces_total"`

	// DistinctObserversTotal is the count of distinct observer strings across
	// all input traces (before filtering).
	DistinctObserversTotal int `json:"distinct_observers_total"`

	// ShadowElements lists elements that appear only in excluded traces.
	// Sorted alphabetically — order does not imply ranking.
	ShadowElements []ShadowElement `json:"shadow_elements"`

	// ExcludedObserverPositions lists observer strings not in ObserverPositions.
	// Computed in Articulate where the full set is known; empty on a full cut.
	// Sorted alphabetically.
	ExcludedObserverPositions []string `json:"excluded_observer_positions"`
}

// ShadowElement is an element that exists in the dataset but falls outside
// the current cut. SeenFrom and Reasons are freshly allocated — safe to mutate.
type ShadowElement struct {
	// Name is the element string as it appeared in shadow trace source/target slices.
	Name string `json:"name"`

	// SeenFrom lists the distinct observer strings of shadow traces that mention
	// this element. Sorted alphabetically. May be empty when the element was
	// excluded only by the time-window filter (no observer-filter context available).
	SeenFrom []string `json:"seen_from"`

	// Reasons lists why this element is in the shadow, accumulated across all
	// excluding traces. An element can have multiple reasons even if no single
	// trace fails all filters. Sorted: observer < tag-filter < time-window.
	// Always non-empty for elements in ShadowElements.
	Reasons []ShadowReason `json:"reasons"`
}

// excludedTrace pairs a trace with the filter(s) it failed.
type excludedTrace struct {
	trace           schema.Trace
	failsObserver   bool
	failsTagFilter  bool
	failsTimeWindow bool
}

// shadowInfo accumulates shadow data for one element across all excluding traces.
type shadowInfo struct {
	seenFrom        map[string]bool
	count           int  // number of excluded traces that mention this element
	failsObserver   bool // at least one excluding trace failed the observer filter
	failsTagFilter  bool // at least one excluding trace failed the tag filter
	failsTimeWindow bool // at least one excluding trace failed the time-window filter
}

// buildEdges constructs one Edge per included trace; slice fields are defensive copies.
func buildEdges(included []schema.Trace) []Edge {
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
	return edges
}

// buildShadowData builds per-element shadow info from excluded traces.
// Count is per-trace: an element in both source and target of one trace = one mention.
func buildShadowData(excluded []excludedTrace) map[string]*shadowInfo {
	data := make(map[string]*shadowInfo)
	for _, ex := range excluded {
		elems := make(map[string]bool) // deduplicate within this trace
		for _, s := range ex.trace.Source {
			elems[s] = true
		}
		for _, tg := range ex.trace.Target {
			elems[tg] = true
		}
		for e := range elems {
			if data[e] == nil {
				data[e] = &shadowInfo{seenFrom: make(map[string]bool)}
			}
			data[e].count++
			data[e].seenFrom[ex.trace.Observer] = true
			if ex.failsObserver {
				data[e].failsObserver = true
			}
			if ex.failsTagFilter {
				data[e].failsTagFilter = true
			}
			if ex.failsTimeWindow {
				data[e].failsTimeWindow = true
			}
		}
	}
	return data
}

// buildNodes constructs the Nodes map, annotating each with ShadowCount where applicable.
func buildNodes(includedElements map[string]int, shadow map[string]*shadowInfo) map[string]Node {
	nodes := make(map[string]Node, len(includedElements))
	for name, count := range includedElements {
		shadowCount := 0
		if sd, ok := shadow[name]; ok {
			shadowCount = sd.count
		}
		nodes[name] = Node{
			Name:            name,
			AppearanceCount: count,
			ShadowCount:     shadowCount,
		}
	}
	return nodes
}

// buildShadowElements constructs the sorted ShadowElements slice.
// Only elements that appear exclusively in excluded traces are included here;
// elements visible in both sets go into Nodes with a non-zero ShadowCount.
func buildShadowElements(shadow map[string]*shadowInfo, includedElements map[string]int) []ShadowElement {
	var elems []ShadowElement
	for name, sd := range shadow {
		if _, inIncluded := includedElements[name]; inIncluded {
			continue
		}
		seenFrom := make([]string, 0, len(sd.seenFrom))
		for obs := range sd.seenFrom {
			seenFrom = append(seenFrom, obs)
		}
		sort.Strings(seenFrom)

		var reasons []ShadowReason // stable order: observer < tag-filter < time-window
		if sd.failsObserver {
			reasons = append(reasons, ShadowReasonObserver)
		}
		if sd.failsTagFilter {
			reasons = append(reasons, ShadowReasonTagFilter)
		}
		if sd.failsTimeWindow {
			reasons = append(reasons, ShadowReasonTimeWindow)
		}

		elems = append(elems, ShadowElement{
			Name:     name,
			SeenFrom: seenFrom,
			Reasons:  reasons,
		})
	}
	sort.Slice(elems, func(i, j int) bool { // alphabetical — order must not imply ranking
		return elems[i].Name < elems[j].Name
	})
	return elems
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
//
// Callers should validate opts.TimeWindow with TimeWindow.Validate() before
// calling Articulate. An inverted window (Start after End) is a programming
// error that produces zero included traces with no further signal.
func Articulate(traces []schema.Trace, opts ArticulationOptions) MeshGraph {
	// Defensive copies so caller mutations after the call cannot affect the Cut.
	positionsCopy := make([]string, len(opts.ObserverPositions))
	copy(positionsCopy, opts.ObserverPositions)
	tw := opts.TimeWindow // value type — copy is automatic
	tagsCopy := make([]string, len(opts.Tags))
	copy(tagsCopy, opts.Tags)

	filterSet := make(map[string]bool, len(positionsCopy))
	for _, op := range positionsCopy {
		filterSet[op] = true
	}
	observerFiltered := len(filterSet) > 0
	timeFiltered := !tw.IsZero()

	tagFilterSet := make(map[string]bool, len(tagsCopy))
	for _, tag := range tagsCopy {
		tagFilterSet[tag] = true
	}
	tagFiltered := len(tagFilterSet) > 0

	// Count distinct observers across all traces before filtering.
	allObservers := make(map[string]bool)
	for _, t := range traces {
		allObservers[t.Observer] = true
	}

	// A trace is included only when it passes ALL active filters (AND semantics).
	var included []schema.Trace
	var excluded []excludedTrace
	for _, t := range traces {
		passesObs := !observerFiltered || filterSet[t.Observer]
		passesTime := !timeFiltered ||
			(tw.Start.IsZero() || !t.Timestamp.Before(tw.Start)) &&
				(tw.End.IsZero() || !t.Timestamp.After(tw.End))
		passesTags := !tagFiltered // any-match against tagFilterSet
		if tagFiltered {
			for _, tag := range t.Tags {
				if tagFilterSet[tag] {
					passesTags = true
					break
				}
			}
		}
		if passesObs && passesTime && passesTags {
			included = append(included, t)
		} else {
			excluded = append(excluded, excludedTrace{
				trace:           t,
				failsObserver:   !passesObs,
				failsTagFilter:  !passesTags,
				failsTimeWindow: !passesTime,
			})
		}
	}

	includedElements := make(map[string]int)
	for _, t := range included {
		for _, s := range t.Source {
			includedElements[s]++
		}
		for _, tg := range t.Target {
			includedElements[tg]++
		}
	}

	shadow := buildShadowData(excluded)

	// Compute excluded observers while the full set is available.
	// Stored in Cut so PrintArticulation doesn't reconstruct it from graph structure.
	var excludedObsPositions []string
	if observerFiltered {
		for obs := range allObservers {
			if !filterSet[obs] {
				excludedObsPositions = append(excludedObsPositions, obs)
			}
		}
		sort.Strings(excludedObsPositions)
	}

	return MeshGraph{
		Nodes: buildNodes(includedElements, shadow),
		Edges: buildEdges(included),
		Cut: Cut{
			ObserverPositions:         positionsCopy,
			TimeWindow:                tw,
			Tags:                      tagsCopy,
			TracesIncluded:            len(included),
			TracesTotal:               len(traces),
			DistinctObserversTotal:    len(allObservers),
			ShadowElements:            buildShadowElements(shadow, includedElements),
			ExcludedObserverPositions: excludedObsPositions,
		},
	}
}

// timeWindowLabel returns a human-readable string for the time window in a Cut.
// A zero TimeWindow is labelled "(none — full temporal cut)" to name the full
// extent as a deliberate choice, mirroring the "(all — full cut)" observer convention.
func timeWindowLabel(tw TimeWindow) string {
	if tw.IsZero() {
		return "(none — full temporal cut)"
	}
	startStr := "(unbounded)"
	if !tw.Start.IsZero() {
		startStr = tw.Start.UTC().Format(time.RFC3339)
	}
	endStr := "(unbounded)"
	if !tw.End.IsZero() {
		endStr = tw.End.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("%s – %s", startStr, endStr)
}

// shadowElementLine formats one ShadowElement into a printable line with inline
// reason annotation (e.g. [observer], [tag-filter, time-window]).
func shadowElementLine(se ShadowElement) string {
	reasonStrs := make([]string, len(se.Reasons))
	for i, r := range se.Reasons {
		reasonStrs[i] = string(r)
	}
	reasonAnnotation := fmt.Sprintf("  [%s]", strings.Join(reasonStrs, ", "))

	// SeenFrom is empty for time-window-only exclusions (no excluded-observer context).
	var mainLine string
	if len(se.SeenFrom) == 0 {
		mainLine = fmt.Sprintf("  %s → (no observer data)", se.Name)
	} else {
		mainLine = fmt.Sprintf("  %s → also seen from: %s",
			se.Name, strings.Join(se.SeenFrom, ", "))
	}
	return mainLine + reasonAnnotation
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
// M7 reflexive traces add a persistence path: ArticulationTrace and DiffTrace
// embed observer position strings (and time window bounds) directly in the
// WhatChanged field of a schema.Trace. If reflexive traces are re-ingested
// and re-printed, attacker-controlled observer strings in the original dataset
// can reach this writer via that route — not only through direct Articulate
// calls on a live dataset.
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

	// "(all — full cut)" names the position as a deliberate choice, not a neutral absence.
	obsLabel := "(all — full cut)"
	if len(g.Cut.ObserverPositions) > 0 {
		obsLabel = strings.Join(g.Cut.ObserverPositions, ", ")
	}

	twLabel := timeWindowLabel(g.Cut.TimeWindow)

	tagLabel := "(none — full tag cut)"
	if len(g.Cut.Tags) > 0 {
		sanitizedTags := make([]string, len(g.Cut.Tags))
		for i, tag := range g.Cut.Tags {
			sanitizedTags[i] = stripNewlines(tag)
		}
		tagLabel = strings.Join(sanitizedTags, ", ")
	}

	excludedObservers := g.Cut.ExcludedObserverPositions

	lines := []string{
		"=== Mesh Articulation (provisional cut) ===",
		"",
		fmt.Sprintf("Observer position(s): %s", obsLabel),
		fmt.Sprintf("Time window:          %s", twLabel),
		fmt.Sprintf("Tag filter:           %s", tagLabel),
		fmt.Sprintf("Traces included: %d of %d (distinct observers in full dataset: %d)",
			g.Cut.TracesIncluded, g.Cut.TracesTotal, g.Cut.DistinctObserversTotal),
	}

	if g.ID != "" { // only identified graphs carry a citeable reference
		ref, _ := GraphRef(g) // error only when ID is empty; guarded above
		lines = append(lines, fmt.Sprintf("Graph ID:             %s", ref))
	}

	lines = append(lines,
		"",
		"Nodes (elements visible from this position):",
	)
	for _, ne := range nodeEntries {
		lines = append(lines, fmt.Sprintf("  %-50s x%d", ne.name, ne.count))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Edges (traces in this cut: %d):", len(g.Edges)))
	for _, e := range g.Edges {
		id := e.TraceID // abbreviated below for display
		if len(id) > 8 {
			id = id[:8] + "..."
		}
		sanitizedEdgeTags := make([]string, len(e.Tags))
		for i, tag := range e.Tags {
			sanitizedEdgeTags[i] = stripNewlines(tag)
		}
		lines = append(lines, fmt.Sprintf("  %s  %v  %s", id, sanitizedEdgeTags, e.WhatChanged))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Shadow (elements invisible from this position: %d):", len(g.Cut.ShadowElements)))
	if len(g.Cut.ShadowElements) == 0 {
		lines = append(lines, "  (none — full cut taken)")
	}
	for _, se := range g.Cut.ShadowElements {
		lines = append(lines, shadowElementLine(se))
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

