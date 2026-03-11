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

// MeshSummary holds a provisional first-pass reading of a trace dataset.
// It is named "summary" rather than "report" or "analysis" to signal that
// this is a cut made from a particular position at a particular time —
// not a finished account. It should remain revisable as more traces are
// followed.
type MeshSummary struct {
	// Elements maps every string that appeared in any Source or Target
	// slice across all traces to the total number of times it appeared.
	// An element can accumulate count from both source and target roles
	// across different traces. This counts involvement, not unique presence —
	// consistent with ANT's interest in what is actively making a difference,
	// not just what exists.
	Elements map[string]int

	// Mediations is a deduplicated list of all non-empty Mediation values
	// observed across the dataset, in the order they were first encountered.
	// Encounter order is preserved intentionally: the sequence in which
	// mediations appear is part of what the dataset is saying about the
	// network's structure.
	Mediations []string

	// FlaggedTraces is the subset of traces that carry a "delay" or
	// "threshold" tag. These mark structural friction points and capacity
	// boundaries in the mesh — places where time was taken or limits were
	// tested.
	FlaggedTraces []FlaggedTrace
}

// FlaggedTrace is a minimal projection of a Trace that carries a delay or
// threshold tag. It carries only the fields needed to identify the trace
// and describe what happened, signalling that a summary view is not the
// same as the full trace record.
type FlaggedTrace struct {
	ID          string
	WhatChanged string
	Tags        []string
}

// Load reads a JSON file at path, decodes each Trace, and validates it via
// schema.Validate(). If any trace fails validation, Load returns nil and an
// error wrapping the validation message along with the trace's index and ID.
// Load stops at the first invalid trace.
//
// An empty JSON array is valid and returns an empty (non-nil) slice.
func Load(path string) ([]schema.Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open %q: %w", path, err)
	}
	defer f.Close()

	var traces []schema.Trace
	if err := json.NewDecoder(f).Decode(&traces); err != nil {
		return nil, fmt.Errorf("loader: decode %q: %w", path, err)
	}

	for i, t := range traces {
		if err := t.Validate(); err != nil {
			return nil, fmt.Errorf("loader: trace %d (id=%q): %w", i, t.ID, err)
		}
	}

	return traces, nil
}

// Summarise builds a MeshSummary from a slice of already-validated traces.
// It does not call Validate() — that responsibility belongs to Load.
//
// Elements counts each string's total appearances across all Source and
// Target slices (not unique traces). Mediations are deduplicated and listed
// in encounter order. FlaggedTraces includes any trace with a "delay" or
// "threshold" tag; each such trace appears at most once regardless of how
// many triggering tags it carries.
func Summarise(traces []schema.Trace) MeshSummary {
	elements := make(map[string]int)
	var mediations []string
	mediationSeen := make(map[string]bool)
	var flagged []FlaggedTrace

	for _, t := range traces {
		// Count element appearances from Source and Target slices.
		for _, s := range t.Source {
			elements[s]++
		}
		for _, tg := range t.Target {
			elements[tg]++
		}

		// Deduplicate mediations in encounter order.
		if t.Mediation != "" && !mediationSeen[t.Mediation] {
			mediations = append(mediations, t.Mediation)
			mediationSeen[t.Mediation] = true
		}

		// Flag traces carrying a delay or threshold tag. Break after the
		// first match so a trace with both tags appears exactly once.
		for _, tag := range t.Tags {
			if tag == string(schema.TagDelay) || tag == string(schema.TagThreshold) {
				flagged = append(flagged, FlaggedTrace{
					ID:          t.ID,
					WhatChanged: t.WhatChanged,
					Tags:        t.Tags,
				})
				break
			}
		}
	}

	return MeshSummary{
		Elements:      elements,
		Mediations:    mediations,
		FlaggedTraces: flagged,
	}
}

// PrintSummary writes a provisional mesh summary to w. It takes an io.Writer
// rather than printing directly to os.Stdout so the output can be captured
// and tested without redirecting standard output.
//
// Elements are sorted by descending frequency, then alphabetically within
// the same frequency. Mediations are listed in encounter order. Flagged
// traces are listed in dataset order.
//
// The footer note is mandatory output — it encodes the methodological
// commitment that the element list is not an actor list, and that this
// summary is a provisional cut, not a finished ontology.
func PrintSummary(w io.Writer, s MeshSummary) {
	fmt.Fprintln(w, "=== Mesh Summary (provisional) ===")
	fmt.Fprintln(w)

	// Build a sortable slice from the elements map.
	type entry struct {
		name  string
		count int
	}
	entries := make([]entry, 0, len(s.Elements))
	for name, count := range s.Elements {
		entries = append(entries, entry{name, count})
	}
	// Sort descending by count, then ascending by name within the same count.
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})

	fmt.Fprintln(w, "Elements (source/target appearances across all traces):")
	for _, e := range entries {
		fmt.Fprintf(w, "  %-45s x%d\n", e.name, e.count)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Observed mediations (%d traces, %d unique):\n",
		countTracesWithMediation(s), len(s.Mediations))
	for _, m := range s.Mediations {
		fmt.Fprintf(w, "  %s\n", m)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Traces tagged delay or threshold (%d):\n", len(s.FlaggedTraces))
	for _, ft := range s.FlaggedTraces {
		fmt.Fprintf(w, "  %s  %v  %s\n", ft.ID, ft.Tags, ft.WhatChanged)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "Note: this is a first look at the mesh, not a classification of actors.")
	fmt.Fprintln(w, "Elements listed here are names that appear in traces — they may be human,")
	fmt.Fprintln(w, "non-human, or assemblages. Their roles are not yet determined.")
}

// countTracesWithMediation counts the number of mediation entries in the
// Mediations slice as a proxy for "traces that had a mediation observed".
// Used only for the PrintSummary header line.
func countTracesWithMediation(s MeshSummary) int {
	return len(s.Mediations)
}
