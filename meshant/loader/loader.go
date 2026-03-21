// Package loader provides functions to load, summarise, and print MeshAnt
// trace datasets.
//
// The three exported functions are intentionally separate: Load handles I/O
// and validation, Summarise builds a provisional reading of the mesh,
// and PrintSummary renders that reading to any io.Writer. Keeping them
// separate means each layer is independently testable and no form factor
// is forced onto the caller.
package loader

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// maxFileBytes caps the size of a JSON file accepted by Load.
// This prevents accidental memory exhaustion from an unexpectedly large file.
// 50 MB is generous for any realistic trace dataset at this stage.
const maxFileBytes = 50 * 1024 * 1024 // 50 MB

// MeshSummary holds a provisional first-pass reading of a trace dataset.
// Named "summary" (not "report") to signal a cut from one position, not a finished account.
type MeshSummary struct {
	// Elements maps every Source/Target string to total appearance count.
	// Counts involvement, not unique presence.
	Elements map[string]int

	// Mediations is a deduplicated list of non-empty Mediation values in encounter order.
	Mediations []string

	// MediatedTraceCount is the number of traces with a non-empty Mediation field.
	MediatedTraceCount int

	// FlaggedTraces contains traces with a "delay" or "threshold" tag — proxies for
	// measurable friction. A provisional cut, not a taxonomy; other tag types are
	// equally significant and accessible via the full trace records.
	FlaggedTraces []FlaggedTrace

	// GraphRefs is the deduplicated list of graph-reference strings from Source/Target
	// slices in encounter order (meshgraph:/meshdiff: prefixes).
	GraphRefs []string
}

// FlaggedTrace is a minimal projection of a delay/threshold-tagged Trace.
// Tags is a copy — callers may modify it without affecting the original.
type FlaggedTrace struct {
	ID          string
	WhatChanged string
	Tags        []string
}

// Load reads a JSON file at path, validates each Trace, and returns the slice.
// Stops at the first invalid trace. Empty array is valid. Files >50 MB are rejected.
func Load(path string) ([]schema.Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxFileBytes)

	var traces []schema.Trace
	if err := json.NewDecoder(limited).Decode(&traces); err != nil {
		return nil, fmt.Errorf("loader: decode %q: %w", path, err)
	}

	if traces == nil { // json.Decode sets target to nil for JSON null
		traces = []schema.Trace{}
	}

	for i, t := range traces {
		if err := t.Validate(); err != nil {
			return nil, fmt.Errorf("loader: trace %d (id=%q): %w", i, t.ID, err)
		}
	}

	return traces, nil
}

// Summarise builds a MeshSummary from already-validated traces. Does not call Validate.
// Each flagged trace appears at most once, regardless of how many triggering tags it carries.
func Summarise(traces []schema.Trace) MeshSummary {
	elements := make(map[string]int)
	var mediations []string
	mediationSeen := make(map[string]bool)
	mediatedCount := 0
	var flagged []FlaggedTrace
	var graphRefs []string
	graphRefSeen := make(map[string]bool)

	for _, t := range traces {
		for _, s := range t.Source {
			elements[s]++
			if schema.IsGraphRef(s) && !graphRefSeen[s] {
				graphRefs = append(graphRefs, s)
				graphRefSeen[s] = true
			}
		}
		for _, tg := range t.Target {
			elements[tg]++
			if schema.IsGraphRef(tg) && !graphRefSeen[tg] {
				graphRefs = append(graphRefs, tg)
				graphRefSeen[tg] = true
			}
		}

		if t.Mediation != "" {
			mediatedCount++
			if !mediationSeen[t.Mediation] {
				mediations = append(mediations, t.Mediation)
				mediationSeen[t.Mediation] = true
			}
		}

		for _, tag := range t.Tags {
			if tag == string(schema.TagDelay) || tag == string(schema.TagThreshold) {
				tags := make([]string, len(t.Tags))
				copy(tags, t.Tags)
				flagged = append(flagged, FlaggedTrace{
					ID:          t.ID,
					WhatChanged: t.WhatChanged,
					Tags:        tags,
				})
				break
			}
		}
	}

	return MeshSummary{
		Elements:           elements,
		Mediations:         mediations,
		MediatedTraceCount: mediatedCount,
		FlaggedTraces:      flagged,
		GraphRefs:          graphRefs,
	}
}

// PrintSummary writes a provisional mesh summary to w.
// Elements sorted by descending frequency then alphabetically; mediations in encounter order.
// Returns the first write error encountered, if any.
func PrintSummary(w io.Writer, s MeshSummary) error {
	type entry struct {
		name  string
		count int
	}
	entries := make([]entry, 0, len(s.Elements))
	for name, count := range s.Elements {
		entries = append(entries, entry{name, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})

	lines := []string{
		"=== Mesh Summary (provisional) ===",
		"",
		"Elements (source/target appearances across all traces):",
	}
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("  %-45s x%d", e.name, e.count))
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Observed mediations (%d traces, %d unique):",
		s.MediatedTraceCount, len(s.Mediations)))
	for _, m := range s.Mediations {
		lines = append(lines, "  "+m)
	}
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Traces tagged delay or threshold (%d):", len(s.FlaggedTraces)))
	for _, ft := range s.FlaggedTraces {
		lines = append(lines, fmt.Sprintf("  %s  %v  %s", ft.ID, ft.Tags, ft.WhatChanged))
	}
	lines = append(lines, "")
	// Graph-refs also appear in Elements (ANT symmetry: identified graphs are actants).
	lines = append(lines, fmt.Sprintf("Graph references (%d, also counted in Elements above):", len(s.GraphRefs)))
	if len(s.GraphRefs) == 0 {
		lines = append(lines, "  (none)")
	} else {
		for _, ref := range s.GraphRefs {
			lines = append(lines, "  "+ref)
		}
	}
	lines = append(lines,
		"",
		"---",
		"Note: this is a first look at the mesh, not a classification of actors.",
		"Elements listed here are names that appear in traces — they may be human,",
		"non-human, or assemblages. Their roles are not yet determined.",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("loader: PrintSummary: %w", err)
		}
	}
	return nil
}
