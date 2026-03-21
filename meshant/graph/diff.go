// Package graph — diff.go contains the GraphDiff types, Diff function, and
// PrintDiff function. These are kept separate from graph.go (which holds
// MeshGraph, Articulate, and PrintArticulation) to keep individual files within
// the 800-line guideline.
package graph

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ShadowShiftKind names the direction of an element's movement across or
// within the shadow boundary between two graph articulations.
type ShadowShiftKind string

const (
	// ShadowShiftEmerged indicates the element moved from shadow (in g1) to a
	// visible Node (in g2).
	ShadowShiftEmerged ShadowShiftKind = "emerged"

	// ShadowShiftSubmerged indicates the element moved from a visible Node (in
	// g1) into the shadow (in g2).
	ShadowShiftSubmerged ShadowShiftKind = "submerged"

	// ShadowShiftReasonChanged indicates the element was in the shadow of both
	// g1 and g2, but with different ShadowReason values.
	ShadowShiftReasonChanged ShadowShiftKind = "reason-changed"
)

// ShadowShift records one element's movement across or within the shadow
// boundary between two graph articulations.
//
// FromReasons is empty when Kind == ShadowShiftSubmerged (element was a
// visible Node in g1). ToReasons is empty when Kind == ShadowShiftEmerged
// (element became a visible Node in g2). Both are non-empty when Kind ==
// ShadowShiftReasonChanged.
type ShadowShift struct {
	// Name is the element string as it appeared in trace source/target slices.
	Name string `json:"name"`

	// Kind describes which direction the element moved.
	Kind ShadowShiftKind `json:"kind"`

	// FromReasons are the ShadowReason values for this element in g1.
	// Empty if the element was a visible Node in g1.
	FromReasons []ShadowReason `json:"from_reasons"`

	// ToReasons are the ShadowReason values for this element in g2.
	// Empty if the element became a visible Node in g2.
	ToReasons []ShadowReason `json:"to_reasons"`
}

// PersistedNode records a node present in both graphs with its appearance count
// from each. A changed count indicates the element became more or less active.
type PersistedNode struct {
	// Name is the element name.
	Name string `json:"name"`

	// CountFrom is the AppearanceCount in g1.
	CountFrom int `json:"count_from"`

	// CountTo is the AppearanceCount in g2.
	CountTo int `json:"count_to"`
}

// GraphDiff is the result of comparing two MeshGraph articulations. It records
// what nodes and edges entered or left visibility, which elements moved across
// or within the shadow boundary, and the full cuts of both input graphs so the
// diff is self-situated.
//
// A GraphDiff is not a neutral changelog. It records what became visible or
// invisible between two specific situated cuts. The From and To fields name
// those cuts explicitly.
//
// All slice fields are sorted for deterministic output:
//   - NodesAdded, NodesRemoved: alphabetical by name
//   - NodesPersisted: alphabetical by Name field
//   - EdgesAdded, EdgesRemoved: alphabetical by TraceID
//   - ShadowShifts: alphabetical by Name field
type GraphDiff struct {
	// ID is the stable actor identifier for this diff. Empty string means the
	// diff has not been identified as an actor. Assign via graph.IdentifyDiff.
	ID string `json:"id"`

	// NodesAdded contains element names in g2.Nodes but not g1.Nodes.
	NodesAdded []string `json:"nodes_added"`

	// NodesRemoved contains element names in g1.Nodes but not g2.Nodes.
	NodesRemoved []string `json:"nodes_removed"`

	// NodesPersisted contains nodes present in both graphs with both counts.
	NodesPersisted []PersistedNode `json:"nodes_persisted"`

	// EdgesAdded contains edges whose TraceID appears in g2 but not g1.
	EdgesAdded []Edge `json:"edges_added"`

	// EdgesRemoved contains edges whose TraceID appears in g1 but not g2.
	EdgesRemoved []Edge `json:"edges_removed"`

	// ShadowShifts contains elements that moved between shadow and visible,
	// or that remained in shadow with changed reasons. Elements in the shadow
	// of both graphs with identical reasons are not included.
	ShadowShifts []ShadowShift `json:"shadow_shifts"`

	// From is the Cut of g1, stored verbatim (defensive slice copies).
	From Cut `json:"from"`

	// To is the Cut of g2, stored verbatim (defensive slice copies).
	To Cut `json:"to"`
}

// copyCut returns a deep copy of a Cut; all slice fields are duplicated.
func copyCut(c Cut) Cut {
	out := c

	out.ObserverPositions = make([]string, len(c.ObserverPositions))
	copy(out.ObserverPositions, c.ObserverPositions)

	out.ExcludedObserverPositions = make([]string, len(c.ExcludedObserverPositions))
	copy(out.ExcludedObserverPositions, c.ExcludedObserverPositions)

	out.ShadowElements = make([]ShadowElement, len(c.ShadowElements))
	for i, se := range c.ShadowElements {
		cp := se
		cp.SeenFrom = make([]string, len(se.SeenFrom))
		copy(cp.SeenFrom, se.SeenFrom)
		cp.Reasons = make([]ShadowReason, len(se.Reasons))
		copy(cp.Reasons, se.Reasons)
		out.ShadowElements[i] = cp
	}
	return out
}

// buildShadowLookup converts a ShadowElement slice into a name→ShadowElement map.
func buildShadowLookup(elements []ShadowElement) map[string]ShadowElement {
	m := make(map[string]ShadowElement, len(elements))
	for _, se := range elements {
		m[se.Name] = se
	}
	return m
}

// shadowReasonsEqual reports whether two ShadowReason slices are identical.
func shadowReasonsEqual(a, b []ShadowReason) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// copyReasons returns a copy of a ShadowReason slice; returns nil for nil input.
func copyReasons(src []ShadowReason) []ShadowReason {
	if src == nil {
		return nil
	}
	out := make([]ShadowReason, len(src))
	copy(out, src)
	return out
}

// computeNodeDiff builds NodesAdded, NodesRemoved, and NodesPersisted (all sorted).
func computeNodeDiff(g1nodes, g2nodes map[string]Node) (added, removed []string, persisted []PersistedNode) {
	for name, n2 := range g2nodes {
		if n1, ok := g1nodes[name]; ok {
			persisted = append(persisted, PersistedNode{Name: name, CountFrom: n1.AppearanceCount, CountTo: n2.AppearanceCount})
		} else {
			added = append(added, name)
		}
	}
	for name := range g1nodes {
		if _, ok := g2nodes[name]; !ok {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Slice(persisted, func(i, j int) bool { return persisted[i].Name < persisted[j].Name })
	return added, removed, persisted
}

// computeEdgeDiff builds EdgesAdded and EdgesRemoved by comparing TraceIDs (sorted).
func computeEdgeDiff(g1edges, g2edges []Edge) (added, removed []Edge) {
	g1ids := make(map[string]bool, len(g1edges))
	for _, e := range g1edges {
		g1ids[e.TraceID] = true
	}
	g2ids := make(map[string]bool, len(g2edges))
	for _, e := range g2edges {
		g2ids[e.TraceID] = true
	}
	for _, e := range g2edges {
		if !g1ids[e.TraceID] {
			added = append(added, e)
		}
	}
	for _, e := range g1edges {
		if !g2ids[e.TraceID] {
			removed = append(removed, e)
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].TraceID < added[j].TraceID })
	sort.Slice(removed, func(i, j int) bool { return removed[i].TraceID < removed[j].TraceID })
	return added, removed
}

// computeShadowShifts builds the ShadowShifts slice by examining elements that
// appear in at least one graph's shadow or as a node in one graph while in the
// shadow of the other. Sorted alphabetically by Name.
func computeShadowShifts(g1 MeshGraph, g2 MeshGraph) []ShadowShift {
	s1 := buildShadowLookup(g1.Cut.ShadowElements)
	s2 := buildShadowLookup(g2.Cut.ShadowElements)

	candidates := make(map[string]bool) // union of names in either shadow
	for name := range s1 {
		candidates[name] = true
	}
	for name := range s2 {
		candidates[name] = true
	}

	var shifts []ShadowShift
	for name := range candidates {
		_, visibleG1 := g1.Nodes[name]
		_, visibleG2 := g2.Nodes[name]
		se1, shadG1 := s1[name]
		se2, shadG2 := s2[name]

		switch {
		case shadG1 && visibleG2: // emerged
			shifts = append(shifts, ShadowShift{
				Name:        name,
				Kind:        ShadowShiftEmerged,
				FromReasons: copyReasons(se1.Reasons),
			})
		case visibleG1 && shadG2: // submerged
			shifts = append(shifts, ShadowShift{
				Name:      name,
				Kind:      ShadowShiftSubmerged,
				ToReasons: copyReasons(se2.Reasons),
			})
		case shadG1 && shadG2 && !shadowReasonsEqual(se1.Reasons, se2.Reasons): // reason-changed
			shifts = append(shifts, ShadowShift{
				Name:        name,
				Kind:        ShadowShiftReasonChanged,
				FromReasons: copyReasons(se1.Reasons),
				ToReasons:   copyReasons(se2.Reasons),
			})
		}
	}
	sort.Slice(shifts, func(i, j int) bool { return shifts[i].Name < shifts[j].Name })
	return shifts
}

// Diff compares two MeshGraph articulations and returns a GraphDiff recording
// what became visible or invisible between them. The diff is directional:
// Diff(g1, g2) reads as "moving from g1 to g2."
//
// All output slices are sorted deterministically — see GraphDiff for the sort
// key used per field.
func Diff(g1, g2 MeshGraph) GraphDiff {
	added, removed, persisted := computeNodeDiff(g1.Nodes, g2.Nodes)
	edgesAdded, edgesRemoved := computeEdgeDiff(g1.Edges, g2.Edges)
	shifts := computeShadowShifts(g1, g2)

	return GraphDiff{
		NodesAdded:     added,
		NodesRemoved:   removed,
		NodesPersisted: persisted,
		EdgesAdded:     edgesAdded,
		EdgesRemoved:   edgesRemoved,
		ShadowShifts:   shifts,
		From:           copyCut(g1.Cut),
		To:             copyCut(g2.Cut),
	}
}

// cutSummaryLines returns the cut description lines for PrintDiff From/To sections.
func cutSummaryLines(label string, c Cut) []string {
	obsLabel := "(all — full cut)"
	if len(c.ObserverPositions) > 0 {
		obsLabel = strings.Join(c.ObserverPositions, ", ")
	}
	return []string{
		fmt.Sprintf("%s cut:", label),
		fmt.Sprintf("  Observer position(s): %s", obsLabel),
		fmt.Sprintf("  Time window:          %s", timeWindowLabel(c.TimeWindow)),
		fmt.Sprintf("  Traces included: %d of %d", c.TracesIncluded, c.TracesTotal),
	}
}

// shadowShiftLine formats a single ShadowShift entry for PrintDiff output.
// Format: "  <name>  <kind>  [fromReasons] → [toReasons]"
// When FromReasons or ToReasons is empty, the empty side renders as "(visible)"
// to make the direction of movement unambiguous.
func shadowShiftLine(s ShadowShift) string {
	fromStr := "(visible)"
	if len(s.FromReasons) > 0 {
		parts := make([]string, len(s.FromReasons))
		for i, r := range s.FromReasons {
			parts[i] = string(r)
		}
		fromStr = "[" + strings.Join(parts, ", ") + "]"
	}
	toStr := "(visible)"
	if len(s.ToReasons) > 0 {
		parts := make([]string, len(s.ToReasons))
		for i, r := range s.ToReasons {
			parts[i] = string(r)
		}
		toStr = "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("  %-40s  %-16s  %s → %s", s.Name, string(s.Kind), fromStr, toStr)
}

// PrintDiff writes a human-readable comparison of two articulations to w.
// All sections are rendered unconditionally — an empty section emits "(none)"
// rather than being skipped. The From/To cut metadata is always printed first,
// encoding the commitment that this comparison is situated.
//
// Returns the first write error encountered, if any.
func PrintDiff(w io.Writer, d GraphDiff) error {
	lines := []string{"=== Mesh Diff (situated comparison) ===", ""}

	if d.ID != "" { // only identified diffs carry a citeable reference
		ref, _ := DiffRef(d) // error only when ID is empty; guarded above
		lines = append(lines, fmt.Sprintf("Diff ID: %s", ref), "")
	}

	lines = append(lines, cutSummaryLines("From", d.From)...)
	lines = append(lines, "")
	lines = append(lines, cutSummaryLines("To", d.To)...)
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Nodes added (%d):", len(d.NodesAdded)))
	if len(d.NodesAdded) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, name := range d.NodesAdded {
			lines = append(lines, "  "+name)
		}
	}
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Nodes removed (%d):", len(d.NodesRemoved)))
	if len(d.NodesRemoved) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, name := range d.NodesRemoved {
			lines = append(lines, "  "+name)
		}
	}
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Nodes persisted (%d):", len(d.NodesPersisted)))
	if len(d.NodesPersisted) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, p := range d.NodesPersisted {
			lines = append(lines, fmt.Sprintf("  %-50s x%d → x%d", p.Name, p.CountFrom, p.CountTo))
		}
	}
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Edges added (%d):", len(d.EdgesAdded)))
	if len(d.EdgesAdded) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, e := range d.EdgesAdded {
			id := e.TraceID
			if len(id) > 8 {
				id = id[:8] + "..."
			}
			lines = append(lines, fmt.Sprintf("  %s  %v  %s", id, e.Tags, e.WhatChanged))
		}
	}
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Edges removed (%d):", len(d.EdgesRemoved)))
	if len(d.EdgesRemoved) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, e := range d.EdgesRemoved {
			id := e.TraceID
			if len(id) > 8 {
				id = id[:8] + "..."
			}
			lines = append(lines, fmt.Sprintf("  %s  %v  %s", id, e.Tags, e.WhatChanged))
		}
	}
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("Shadow shifts (%d):", len(d.ShadowShifts)))
	if len(d.ShadowShifts) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, s := range d.ShadowShifts {
			lines = append(lines, shadowShiftLine(s))
		}
	}

	lines = append(lines,
		"",
		"---",
		"Note: this diff is a comparison between two situated cuts, not an objective account of change.",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("graph: PrintDiff: %w", err)
		}
	}
	return nil
}
