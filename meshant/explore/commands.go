// commands.go implements the batch-1 analytical and filter-setter commands
// for the meshant explore REPL.
//
// Commands in this file:
//   - articulate: cuts the mesh graph from the current session position
//   - shadow:     summarises what the current cut leaves in shadow
//   - window:     sets or clears the session-level time window filter
//   - tags:       sets or clears the session-level tag filter
//
// Naming note: "articulate" and "shadow" are analytical commands — they query
// the store, produce a Reading, and record a turn. "window" and "tags" are
// filter setters — they mutate session state that is snapshotted by subsequent
// analytical commands, but they do not record turns themselves.
//
// See docs/decisions/explore-v1.md for the full design rationale, particularly
// D3 (per-turn positional snapshotting) and T172.6 (live substrate).
package explore

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// cmdArticulate cuts the mesh graph from the current session observer position
// and renders the result via graph.PrintArticulation.
//
// Guards:
//   - s.ts == nil: inline error directing the analyst to load a file.
//   - s.observer == "": inline error directing the analyst to use cut first.
//   - query error: inline error; session continues.
//
// A turn is recorded only on success. The Reading field carries the full
// graph.MeshGraph produced by this cut.
func (s *AnalysisSession) cmdArticulate(ctx context.Context, rawLine string, out io.Writer) error {
	if s.ts == nil {
		fmt.Fprintf(out, "articulate: no trace substrate loaded — open a file with: meshant <file.json>\n")
		return nil
	}
	if s.observer == "" {
		fmt.Fprintf(out, "articulate: observer not set — use 'cut <observer>' first\n")
		return nil
	}

	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		fmt.Fprintf(out, "articulate: failed to load traces: %v\n", err)
		return nil
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{s.observer},
		TimeWindow:        s.window,
		Tags:              s.tags,
	}
	g := graph.Articulate(traces, opts)

	// Print errors from io.Writer are terminal — the analyst cannot recover
	// from a broken output stream.
	if err := graph.PrintArticulation(out, g); err != nil {
		return err
	}

	s.recordTurn(rawLine, g, nil)
	return nil
}

// cmdShadow articulates the mesh graph from the current session observer
// position, then summarises what the cut leaves in shadow.
//
// The shadow is not missing data — it is the structured record of what this
// cut cannot see. The Reading field carries a graph.ShadowSummary so the
// positional record includes what was excluded, not just what was visible.
//
// Guards and error handling follow the same pattern as cmdArticulate.
func (s *AnalysisSession) cmdShadow(ctx context.Context, rawLine string, out io.Writer) error {
	if s.ts == nil {
		fmt.Fprintf(out, "shadow: no trace substrate loaded — open a file with: meshant <file.json>\n")
		return nil
	}
	if s.observer == "" {
		fmt.Fprintf(out, "shadow: observer not set — use 'cut <observer>' first\n")
		return nil
	}

	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		fmt.Fprintf(out, "shadow: failed to load traces: %v\n", err)
		return nil
	}

	opts := graph.ArticulationOptions{
		ObserverPositions: []string{s.observer},
		TimeWindow:        s.window,
		Tags:              s.tags,
	}
	g := graph.Articulate(traces, opts)
	summary := graph.SummariseShadow(g)

	if err := graph.PrintShadowSummary(out, summary); err != nil {
		return err
	}

	s.recordTurn(rawLine, summary, nil)
	return nil
}

// cmdWindow sets or clears the session-level time window filter.
//
// Usage:
//
//	window                   — clear (same as "window clear")
//	window clear             — clear the active time window
//	window <from> <to>       — set a time window; both are RFC3339 strings
//
// The window filter is snapshotted into each subsequent analytical turn
// (D3 in explore-v1.md). This command does not record a turn itself — it
// is a filter setter, not an analytical act.
func (s *AnalysisSession) cmdWindow(_ string, args []string, out io.Writer) error {
	// 0 args or the single keyword "clear" both reset the window.
	if len(args) == 0 || (len(args) == 1 && args[0] == "clear") {
		s.window = graph.TimeWindow{}
		fmt.Fprintln(out, "window cleared")
		return nil
	}

	if len(args) != 2 {
		fmt.Fprintf(out, "window: usage: window <from> <to> | window clear\n")
		return nil
	}

	from, err := time.Parse(time.RFC3339, args[0])
	if err != nil {
		fmt.Fprintf(out, "window: invalid from value %q: expected RFC3339\n", args[0])
		return nil
	}
	to, err := time.Parse(time.RFC3339, args[1])
	if err != nil {
		fmt.Fprintf(out, "window: invalid to value %q: expected RFC3339\n", args[1])
		return nil
	}

	tw := graph.TimeWindow{Start: from, End: to}
	if err := tw.Validate(); err != nil {
		// Validate returns a non-nil error when End precedes Start.
		fmt.Fprintf(out, "window: %v\n", err)
		return nil
	}

	s.window = tw
	fmt.Fprintf(out, "window → %s to %s\n", args[0], args[1])
	return nil
}

// cmdTags sets or clears the session-level tag filter.
//
// Usage:
//
//	tags                     — clear (same as "tags clear")
//	tags clear               — clear the active tag filter
//	tags <t1> [t2...]        — set the filter to the listed tags (replaces, not appends)
//
// The tag filter is snapshotted into each subsequent analytical turn (D3).
// This command does not record a turn itself.
//
// Note: "tags clear foo bar" treats all three arguments as literal tags —
// "clear" is only a keyword when it is the sole argument.
func (s *AnalysisSession) cmdTags(_ string, args []string, out io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "clear") {
		s.tags = nil
		fmt.Fprintln(out, "tags cleared")
		return nil
	}

	// Deep-copy args so that the session's tag slice is independent of the
	// strings.Fields result in dispatch (which is safe in Go, but explicit
	// is better than implicit here).
	tags := make([]string, len(args))
	copy(tags, args)
	s.tags = tags
	fmt.Fprintf(out, "tags → %s\n", strings.Join(tags, ", "))
	return nil
}
