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
//
// A zero Start means no lower bound (all traces from the beginning of time are
// included). A zero End means no upper bound (all traces into the future are
// included). A zero TimeWindow (both fields unset) means no time filter at all
// — equivalent to a full temporal cut.
//
// The bounds are evaluated as: trace.Timestamp >= Start (if Start non-zero) AND
// trace.Timestamp <= End (if End non-zero). Both bounds are inclusive.
//
// JSON encoding: zero bounds are serialised as null (not as the RFC3339 zero
// time "0001-01-01T00:00:00Z"). See MarshalJSON and UnmarshalJSON in serial.go.
type TimeWindow struct {
	// Start is the inclusive lower bound. Zero means unbounded.
	// JSON: null when zero, RFC3339 string when non-zero.
	Start time.Time

	// End is the inclusive upper bound. Zero means unbounded.
	// JSON: null when zero, RFC3339 string when non-zero.
	End time.Time
}

// IsZero reports whether the TimeWindow has no bounds set (both Start and End
// are zero). A zero TimeWindow means no time filter is applied.
func (tw TimeWindow) IsZero() bool {
	return tw.Start.IsZero() && tw.End.IsZero()
}

// Validate returns an error if the TimeWindow is structurally invalid.
// The only invalid state is a non-zero Start that is after a non-zero End —
// this would silently produce a zero-trace articulation with no indication
// that the parameters were nonsensical. All other states (zero Start, zero End,
// both zero, Start == End) are valid.
//
// Callers should call Validate before passing a TimeWindow to Articulate.
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
	// ObserverPositions is a list of observer strings to include. When empty,
	// all traces are included (full cut). When non-empty, only traces whose
	// Observer field exactly matches one of the listed strings are included.
	ObserverPositions []string

	// TimeWindow restricts the cut to traces whose Timestamp falls within the
	// window. Zero value means no time filter (all timestamps accepted).
	// TimeWindow is a value type — it is copied automatically when opts is
	// passed by value, so no explicit deep-copy is needed.
	TimeWindow TimeWindow

	// Tags restricts the cut to traces whose Tags slice contains at least one
	// of the listed strings (any-match). When empty, all traces are included
	// (full tag cut). A defensive copy is made in Articulate so that mutations
	// to this slice after the call do not affect Cut.Tags.
	Tags []string
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
	// ID is the stable actor identifier for this graph. Empty string means the
	// graph has not been identified as an actor — it is an articulation output,
	// not yet a participant in the mesh. Assign via graph.IdentifyGraph.
	ID string `json:"id"`

	// Nodes maps element names to their node data. An element enters the graph
	// if it appeared in the Source or Target of any included trace.
	Nodes map[string]Node `json:"nodes"`

	// Edges is one edge per included trace, preserving dataset order.
	// Edges in dataset order preserves the temporal sequence, which is part
	// of what the dataset is saying about the network's structure.
	Edges []Edge `json:"edges"`

	// Cut records the articulation parameters and the shadow:
	// elements that exist in the full dataset but are invisible from
	// the chosen observer position(s).
	Cut Cut `json:"cut"`
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
	Name string `json:"name"`

	// AppearanceCount is the total number of times this element appeared in
	// Source or Target slices across all included traces. An element can
	// accumulate count from both source and target roles.
	AppearanceCount int `json:"appearance_count"`

	// ShadowCount is the number of excluded traces in which this element appears.
	// Zero for nodes that exist only in included traces. A non-zero ShadowCount
	// means this element crosses the cut boundary: it is visible here (included
	// traces) AND present in excluded traces (shadow). Methodologically, it
	// participates in more observational positions than this cut can see —
	// a partial connection rather than a clean inclusion or exclusion.
	ShadowCount int `json:"shadow_count"`
}

// Edge represents one trace in the graph. It preserves the full trace
// context so that graph consumers can follow back to the source record.
//
// Edge.Tags is a copy of the source trace's Tags slice, not a reference to it.
// Callers may safely modify Edge.Tags without affecting subsequent operations
// on the original trace data.
type Edge struct {
	// TraceID is the UUID of the source trace.
	TraceID string `json:"trace_id"`

	// WhatChanged is the short description of the difference from the trace.
	WhatChanged string `json:"what_changed"`

	// Mediation names what transformed, redirected, or displaced the action.
	// A mediator changes what passes through it — it is not a neutral conduit.
	// Empty if no mediator was observed.
	Mediation string `json:"mediation"`

	// Observer is the observer string from the source trace.
	Observer string `json:"observer"`

	// Sources is a copy of the trace's Source slice.
	Sources []string `json:"sources"`

	// Targets is a copy of the trace's Target slice.
	Targets []string `json:"targets"`

	// Tags is a copy of the trace's Tags slice. Safe to mutate.
	Tags []string `json:"tags"`
}

// Cut records the position from which a MeshGraph was articulated and names
// the shadow: what this cut excludes. The shadow is mandatory output — every
// representation names what it cannot see.
type Cut struct {
	// ObserverPositions lists the filter used. Empty means no filter (full cut).
	// Stored verbatim from ArticulationOptions.
	ObserverPositions []string `json:"observer_positions"`

	// TimeWindow is the temporal filter used. Zero value means no time filter.
	// Stored verbatim (value copy) from ArticulationOptions.TimeWindow.
	TimeWindow TimeWindow `json:"time_window"`

	// Tags lists the tag filter used. Empty means no filter (full tag cut).
	// A defensive copy of ArticulationOptions.Tags — mutations after the call
	// do not affect this slice.
	Tags []string `json:"tags"`

	// TracesIncluded is the number of traces that passed the filter.
	TracesIncluded int `json:"traces_included"`

	// TracesTotal is the total number of traces in the input dataset,
	// before any filtering. This is always equal to len(input).
	TracesTotal int `json:"traces_total"`

	// DistinctObserversTotal is the number of distinct observer strings
	// across all traces in the input (before filtering). This names how
	// many positions exist in the full dataset, independent of which filter
	// was chosen.
	DistinctObserversTotal int `json:"distinct_observers_total"`

	// ShadowElements is the list of elements (source/target names) that appear
	// in excluded traces but not in any included trace. These are the elements
	// that this cut cannot see. Sorted alphabetically so that the shadow is not
	// implicitly ranked by order of appearance.
	ShadowElements []ShadowElement `json:"shadow_elements"`

	// ExcludedObserverPositions lists the distinct observer strings in the full
	// dataset that are NOT in ObserverPositions. Stored in Articulate where the
	// full observer set is known — PrintArticulation uses this directly rather
	// than reconstructing it from graph structure. Empty when no filter was
	// applied (full cut). Sorted alphabetically.
	ExcludedObserverPositions []string `json:"excluded_observer_positions"`
}

// ShadowElement is an element that exists in the dataset but falls outside
// the current cut. SeenFrom lists the observer positions from which this
// element would become visible — the shadow has its own trace.
//
// SeenFrom and Reasons are freshly allocated slices per element. Callers may
// safely mutate them without affecting the MeshGraph or subsequent Articulate
// calls. This is consistent with the defensive-copy guarantee on Edge.Tags.
type ShadowElement struct {
	// Name is the element string as it appeared in shadow trace source/target slices.
	Name string `json:"name"`

	// SeenFrom lists the distinct observer strings of the shadow traces in which
	// this element appears. Sorted alphabetically. This records which positions
	// in the mesh can see what this cut cannot.
	// When the element was excluded only by the time-window filter (and no
	// observer filter was set), SeenFrom may be empty because there is no
	// excluded-observer set to derive it from — only the time dimension cut.
	SeenFrom []string `json:"seen_from"`

	// Reasons lists why this element is in the shadow. May contain
	// ShadowReasonObserver, ShadowReasonTagFilter, ShadowReasonTimeWindow, or
	// any combination. The reasons are accumulated across all excluded traces
	// that mention this element: if any excluding trace fails the observer
	// filter, ShadowReasonObserver is present; if any fails the tag filter,
	// ShadowReasonTagFilter is present; if any fails the time-window filter,
	// ShadowReasonTimeWindow is present. This means an element can have
	// multiple reasons even if no single trace fails all filters simultaneously.
	// Always sorted deterministically (observer < tag-filter < time-window).
	// Non-empty whenever the element is in ShadowElements.
	Reasons []ShadowReason `json:"reasons"`
}

// excludedTrace pairs a trace with the filter(s) it failed, for shadow reason tracking.
type excludedTrace struct {
	trace           schema.Trace
	failsObserver   bool
	failsTagFilter  bool
	failsTimeWindow bool
}

// shadowInfo accumulates data about an element that appears in excluded traces.
// Reasons are accumulated across all excluded traces that mention the element:
// if any such trace fails the observer filter, failsObserver is set true;
// if any fails the tag filter, failsTagFilter is set true;
// if any fails the time-window filter, failsTimeWindow is set true.
// An element can have multiple reasons even if no single trace fails all filters.
type shadowInfo struct {
	seenFrom        map[string]bool
	count           int  // number of excluded traces that mention this element
	failsObserver   bool // at least one excluding trace failed the observer filter
	failsTagFilter  bool // at least one excluding trace failed the tag filter
	failsTimeWindow bool // at least one excluding trace failed the time-window filter
}

// buildEdges constructs one Edge per included trace in dataset order.
// Each slice field (Tags, Sources, Targets) is a defensive copy.
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

// buildShadowData builds per-element shadow information from excluded traces.
// Count is per-trace (not per-appearance): an element in both source and target
// of the same trace counts as one mention.
func buildShadowData(excluded []excludedTrace) map[string]*shadowInfo {
	data := make(map[string]*shadowInfo)
	for _, ex := range excluded {
		// Deduplicate elements within this trace before counting.
		elems := make(map[string]bool)
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

// buildNodes constructs the Nodes map from included element counts, annotating
// each node with a ShadowCount from the shadow data where applicable.
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

// buildShadowElements constructs the sorted ShadowElements slice. Elements that
// appear in both included and excluded traces are in Nodes (not here). Only
// elements that appear EXCLUSIVELY in excluded traces enter ShadowElements.
func buildShadowElements(shadow map[string]*shadowInfo, includedElements map[string]int) []ShadowElement {
	var elems []ShadowElement
	for name, sd := range shadow {
		if _, inIncluded := includedElements[name]; inIncluded {
			continue // visible from included traces → Nodes, not shadow
		}
		seenFrom := make([]string, 0, len(sd.seenFrom))
		for obs := range sd.seenFrom {
			seenFrom = append(seenFrom, obs)
		}
		sort.Strings(seenFrom)

		// Reasons are in stable alphabetical order: observer < tag-filter < time-window.
		var reasons []ShadowReason
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
	// Alphabetical sort — order must not imply ranking.
	sort.Slice(elems, func(i, j int) bool {
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
	// Copy ObserverPositions so caller mutations after the call cannot affect
	// the returned Cut. Consistent with the copy discipline on Edge slices.
	positionsCopy := make([]string, len(opts.ObserverPositions))
	copy(positionsCopy, opts.ObserverPositions)

	// TimeWindow is a value type — opts copy is automatic. No deep-copy needed.
	tw := opts.TimeWindow

	// Copy Tags for the same defensive-copy reason as ObserverPositions.
	tagsCopy := make([]string, len(opts.Tags))
	copy(tagsCopy, opts.Tags)

	// Build observer filter set for O(1) lookup.
	filterSet := make(map[string]bool, len(positionsCopy))
	for _, op := range positionsCopy {
		filterSet[op] = true
	}
	observerFiltered := len(filterSet) > 0
	timeFiltered := !tw.IsZero()

	// Build tag filter set for O(1) lookup.
	tagFilterSet := make(map[string]bool, len(tagsCopy))
	for _, tag := range tagsCopy {
		tagFilterSet[tag] = true
	}
	tagFiltered := len(tagFilterSet) > 0

	// Count distinct observers across ALL traces before filtering.
	allObservers := make(map[string]bool)
	for _, t := range traces {
		allObservers[t.Observer] = true
	}

	// Split traces: a trace is included only if it passes ALL active filters
	// (AND semantics across observer, tag, and time-window axes).
	var included []schema.Trace
	var excluded []excludedTrace
	for _, t := range traces {
		passesObs := !observerFiltered || filterSet[t.Observer]
		passesTime := !timeFiltered ||
			(tw.Start.IsZero() || !t.Timestamp.Before(tw.Start)) &&
				(tw.End.IsZero() || !t.Timestamp.After(tw.End))
		// Tag filter: passes if no filter is active, or if any of the trace's
		// tags appear in the filter set (set-intersection / any-match semantics).
		passesTags := !tagFiltered
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

	// Count element appearances across included traces.
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

	// Compute excluded observer positions while the full set is available.
	// Stored in Cut so PrintArticulation does not reconstruct it from graph
	// structure (which would miss observers entirely in the shadow-count zone).
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

// timeWindowLabel returns a human-readable string for the time window stored in
// a Cut. The label is always emitted in PrintArticulation so readers know the
// temporal scope of the cut even when no filter was applied.
//
// A zero TimeWindow is labelled "(none — full temporal cut)" to name it as a
// deliberate choice — the full temporal extent of the dataset — rather than
// implying a neutral absence. This mirrors the "(all — full cut)" convention
// used for observer positions (per articulation-v1.md Decision 3).
func timeWindowLabel(tw TimeWindow) string {
	if tw.IsZero() {
		return "(none — full temporal cut)"
	}
	// Format both bounds in RFC3339 for unambiguous machine-readable output.
	// A zero bound means unbounded; render it as "(unbounded)" so that
	// half-open windows are legible (e.g. "(unbounded) – 2026-03-14T23:59:59Z").
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

// shadowElementLine formats a single ShadowElement into a printable line.
// The reason annotation (e.g. [observer], [tag-filter, time-window]) is
// appended inline on the same line for compactness. Reasons are in the
// sorted order guaranteed by Articulate (observer < tag-filter < time-window).
func shadowElementLine(se ShadowElement) string {
	reasonStrs := make([]string, len(se.Reasons))
	for i, r := range se.Reasons {
		reasonStrs[i] = string(r)
	}
	reasonAnnotation := fmt.Sprintf("  [%s]", strings.Join(reasonStrs, ", "))

	// When SeenFrom is empty (time-window-only exclusion with no observer filter
	// context), show a placeholder rather than an empty "also seen from:" line.
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

	// Observer positions label.
	// "(all — full cut)" names the full-cut position as a deliberate choice,
	// not as the absence of a filter (per articulation-v1.md Decision 3).
	obsLabel := "(all — full cut)"
	if len(g.Cut.ObserverPositions) > 0 {
		obsLabel = strings.Join(g.Cut.ObserverPositions, ", ")
	}

	// Time-window label. Shown on every articulation output regardless of
	// whether a window was set, so readers always know the temporal scope.
	twLabel := timeWindowLabel(g.Cut.TimeWindow)

	// Tag filter label. Shown on every articulation output so readers know which
	// tags were used as a filter — or that no filter was applied (full tag cut).
	// A zero Tags slice is named explicitly to mirror the observer "(all — full cut)"
	// convention and the time-window "(none — full temporal cut)" convention.
	tagLabel := "(none — full tag cut)"
	if len(g.Cut.Tags) > 0 {
		sanitizedTags := make([]string, len(g.Cut.Tags))
		for i, tag := range g.Cut.Tags {
			sanitizedTags[i] = stripNewlines(tag)
		}
		tagLabel = strings.Join(sanitizedTags, ", ")
	}

	// Excluded observer positions are pre-computed in Articulate where the full
	// observer set is known. Use them directly rather than approximating from
	// graph structure.
	excludedObservers := g.Cut.ExcludedObserverPositions

	lines := []string{
		"=== Mesh Articulation (provisional cut) ===",
		"",
		fmt.Sprintf("Observer position(s): %s", obsLabel),
		fmt.Sprintf("Time window:          %s", twLabel),
		fmt.Sprintf("Tag filter:           %s", tagLabel),
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

