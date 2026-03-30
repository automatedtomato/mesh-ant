// commands_dual.go implements the dual-observer analytical commands for the
// meshant explore REPL: diff and gaps.
//
// Both commands compare two observer positions against the same trace substrate.
// The current session observer (s.observer) is always position A. The second
// observer is provided as a required argument. Both sides share the session's
// active window and tag filters (KISS in v1; per-side overrides are deferred).
//
// The articulateDual helper performs the shared preamble: nil-store guard,
// observer guard, full-substrate query, and two independent graph.Articulate
// calls. This cannot reuse articulateForSession (which returns one graph) but
// mirrors its contract: print inline error and return false on failure.
//
// See docs/decisions/explore-v1.md T172.4 for the known tension around
// AnalysisTurn.Observer being a single string for dual-observer results.
package explore

import (
	"context"
	"fmt"
	"io"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// articulateDual performs the shared preamble for dual-observer commands.
//
// Returns (gA, gB, true, nil) on success. On guard or query failure, prints an
// inline error prefixed with cmdName and returns (zero, zero, false, nil).
// Returns non-nil error only for unrecoverable failures (currently none).
//
// Both graphs are articulated from the same full-substrate query (D2 in
// explore-v1.md: live substrate, not a snapshot). Both use the session's
// current window and tags so the comparison is symmetric under the same filters.
func (s *AnalysisSession) articulateDual(ctx context.Context, cmdName, observerB string, out io.Writer) (gA, gB graph.MeshGraph, ok bool, err error) {
	if s.ts == nil {
		fmt.Fprintf(out, "%s: no trace substrate loaded — open a file with: meshant <file.json>\n", cmdName)
		return graph.MeshGraph{}, graph.MeshGraph{}, false, nil
	}
	if s.observer == "" {
		fmt.Fprintf(out, "%s: observer not set — use 'cut <observer>' first\n", cmdName)
		return graph.MeshGraph{}, graph.MeshGraph{}, false, nil
	}

	traces, qErr := s.ts.Query(ctx, store.QueryOpts{})
	if qErr != nil {
		fmt.Fprintf(out, "%s: failed to load traces: %v\n", cmdName, qErr)
		return graph.MeshGraph{}, graph.MeshGraph{}, false, nil
	}

	gA = graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{s.observer},
		TimeWindow:        s.window,
		Tags:              s.tags,
	})
	gB = graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{observerB},
		TimeWindow:        s.window,
		Tags:              s.tags,
	})
	return gA, gB, true, nil
}

// cmdDiff compares the current session cut (observer A) against a second
// observer position (observer B) and renders the positioned difference via
// graph.PrintDiff.
//
// Usage: diff <observer-b>
//
// Both cuts share the session's active window and tag filters. The Reading
// field carries the graph.GraphDiff produced by this comparison.
//
// ANT tension T172.4 (explore-v1.md): AnalysisTurn.Observer is a single string;
// it records observer A (the session position). Observer B is carried by the
// GraphDiff payload. A richer multi-observer turn record is deferred.
func (s *AnalysisSession) cmdDiff(ctx context.Context, rawLine string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintf(out, "diff: observer-b required — usage: diff <observer-b>\n")
		return nil
	}

	gA, gB, ok, err := s.articulateDual(ctx, "diff", args[0], out)
	if err != nil || !ok {
		return err
	}

	d := graph.Diff(gA, gB)
	if err := graph.PrintDiff(out, d); err != nil {
		return err
	}
	s.recordTurn(rawLine, d, nil)
	return nil
}

// cmdGaps analyses what each observer can see that the other cannot, using
// graph.AnalyseGaps on two independently articulated cuts.
//
// Usage: gaps <observer-b>
//
// Both cuts share the session's active window and tag filters. The Reading
// field carries the graph.ObserverGap produced by this analysis.
//
// ANT tension T172.4 applies here too: Turn.Observer records observer A.
func (s *AnalysisSession) cmdGaps(ctx context.Context, rawLine string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintf(out, "gaps: observer-b required — usage: gaps <observer-b>\n")
		return nil
	}

	gA, gB, ok, err := s.articulateDual(ctx, "gaps", args[0], out)
	if err != nil || !ok {
		return err
	}

	gap := graph.AnalyseGaps(gA, gB)
	if err := graph.PrintObserverGap(out, gap); err != nil {
		return err
	}
	s.recordTurn(rawLine, gap, nil)
	return nil
}
