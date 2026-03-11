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

	// MediatedTraceCount is the number of traces that had a non-empty
	// Mediation field. This may differ from len(Mediations) if the same
	// mediation string appears in more than one trace.
	MediatedTraceCount int

	// FlaggedTraces is the subset of traces that carry a "delay" or
	// "threshold" tag. These mark structural friction points and capacity
	// boundaries in the mesh — places where time was taken or limits were
	// tested.
	FlaggedTraces []FlaggedTrace

	// GraphRefs is the deduplicated list of graph-reference strings found
	// across all Source and Target slices, in the order they were first
	// encountered. A graph-reference is a string of the form "meshgraph:<uuid>"
	// or "meshdiff:<uuid>" — it indicates that an identified MeshGraph or
	// GraphDiff appeared as an actor in the recorded traces.
	//
	// Encounter order is preserved intentionally: the first appearance of a
	// graph-reference marks when that graph became an actor in the mesh.
	GraphRefs []string
}

// FlaggedTrace is a minimal projection of a Trace that carries a delay or
// threshold tag. It carries only the fields needed to identify the trace
// and describe what happened, signalling that a summary view is not the
// same as the full trace record.
//
// Tags is a copy of the source trace's Tags slice, not a reference to it.
// Callers may safely modify FlaggedTrace.Tags without affecting the original.
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
// Files larger than 50 MB are rejected before decoding.
func Load(path string) ([]schema.Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open %q: %w", path, err)
	}
	defer f.Close()

	// Limit reads to maxFileBytes to prevent memory exhaustion on large inputs.
	limited := io.LimitReader(f, maxFileBytes)

	var traces []schema.Trace
	if err := json.NewDecoder(limited).Decode(&traces); err != nil {
		return nil, fmt.Errorf("loader: decode %q: %w", path, err)
	}

	// json.Decode sets a []T target to nil when the JSON value is null.
	// Normalise to an empty non-nil slice to honour the documented postcondition.
	if traces == nil {
		traces = []schema.Trace{}
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
// in encounter order. MediatedTraceCount records how many traces had a
// non-empty Mediation field. FlaggedTraces includes any trace with a "delay"
// or "threshold" tag; each such trace appears at most once regardless of how
// many triggering tags it carries.
func Summarise(traces []schema.Trace) MeshSummary {
	elements := make(map[string]int)
	var mediations []string
	mediationSeen := make(map[string]bool)
	mediatedCount := 0
	var flagged []FlaggedTrace
	var graphRefs []string
	graphRefSeen := make(map[string]bool)

	for _, t := range traces {
		// Count element appearances from Source and Target slices, and extract
		// any graph-reference strings (meshgraph:/meshdiff:) into GraphRefs.
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

		// Track mediation presence and deduplicate in encounter order.
		if t.Mediation != "" {
			mediatedCount++
			if !mediationSeen[t.Mediation] {
				mediations = append(mediations, t.Mediation)
				mediationSeen[t.Mediation] = true
			}
		}

		// Flag traces carrying a delay or threshold tag. Break after the
		// first match so a trace with both tags appears exactly once.
		// Copy Tags to avoid sharing the backing array with the source trace.
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
//
// Note: trace field values (element names, mediations, WhatChanged strings)
// are written to w as-is. If w is a terminal writer, values containing ANSI
// control sequences from an untrusted dataset could affect terminal state.
// For trusted local datasets this is not a concern; re-evaluate if the tool
// is ever exposed to external or user-supplied data.
//
// Returns the first write error encountered, if any.
func PrintSummary(w io.Writer, s MeshSummary) error {
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
	lines = append(lines, fmt.Sprintf("Graph references (%d):", len(s.GraphRefs)))
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
