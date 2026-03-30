// commands.go implements the single-observer analytical commands and filter-setter
// commands for the meshant explore REPL.
//
// Commands in this file:
//   - articulate: cuts the mesh graph from the current session position
//   - shadow:     summarises what the current cut leaves in shadow
//   - follow:     follows a translation chain from a named element
//   - bottleneck: surfaces provisionally central elements
//   - window:     sets or clears the session-level time window filter
//   - tags:       sets or clears the session-level tag filter
//
// Dual-observer commands (diff, gaps) live in commands_dual.go.
//
// Shared preamble: articulateForSession performs the nil-store guard, observer
// guard, full-substrate query, and graph.Articulate call common to all
// single-observer analytical commands.
//
// See docs/decisions/explore-v1.md for the full design rationale, particularly
// D3 (per-turn positional snapshotting) and T172.6 (live substrate).
package explore

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// articulateForSession performs the shared preamble for single-observer
// analytical commands: nil-store guard, observer guard, full-substrate query,
// and graph.Articulate with the current session cut conditions.
//
// Returns (graph, true, nil) on success. On guard or query failure, prints an
// inline error to out prefixed with cmdName and returns (zero, false, nil).
// Returns a non-nil error only for unrecoverable failures (currently none;
// reserved for future use).
//
// The cmdName parameter is used to prefix inline error messages so that the
// analyst can identify which command failed (e.g. "articulate: no trace
// substrate loaded" vs "follow: no trace substrate loaded").
func (s *AnalysisSession) articulateForSession(ctx context.Context, cmdName string, out io.Writer) (graph.MeshGraph, bool, error) {
	if s.ts == nil {
		fmt.Fprintf(out, "%s: no trace substrate loaded — open a file with: meshant <file.json>\n", cmdName)
		return graph.MeshGraph{}, false, nil
	}
	if s.observer == "" {
		fmt.Fprintf(out, "%s: observer not set — use 'cut <observer>' first\n", cmdName)
		return graph.MeshGraph{}, false, nil
	}
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		fmt.Fprintf(out, "%s: failed to load traces: %v\n", cmdName, err)
		return graph.MeshGraph{}, false, nil
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{s.observer},
		TimeWindow:        s.window,
		Tags:              s.tags,
	})
	return g, true, nil
}

// cmdArticulate cuts the mesh graph from the current session observer position
// and renders the result via graph.PrintArticulation.
//
// A turn is recorded only on success. The Reading field carries the full
// graph.MeshGraph produced by this cut.
func (s *AnalysisSession) cmdArticulate(ctx context.Context, rawLine string, out io.Writer) error {
	g, ok, err := s.articulateForSession(ctx, "articulate", out)
	if err != nil || !ok {
		return err
	}
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
func (s *AnalysisSession) cmdShadow(ctx context.Context, rawLine string, out io.Writer) error {
	g, ok, err := s.articulateForSession(ctx, "shadow", out)
	if err != nil || !ok {
		return err
	}
	summary := graph.SummariseShadow(g)
	if err := graph.PrintShadowSummary(out, summary); err != nil {
		return err
	}
	s.recordTurn(rawLine, summary, nil)
	return nil
}

// cmdFollow follows a translation chain from a named element through the
// current session cut and renders the classified chain via graph.PrintChain.
//
// Usage: follow <element> [max_depth]
//
// max_depth is optional; 0 means unlimited (the graph engine default).
// A turn is recorded only on success. The Reading field carries the
// graph.ClassifiedChain produced by this traversal.
func (s *AnalysisSession) cmdFollow(ctx context.Context, rawLine string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintf(out, "follow: element name required — usage: follow <element> [max_depth]\n")
		return nil
	}
	element := args[0]

	// Parse optional max_depth.
	maxDepth := 0
	if len(args) >= 2 {
		d, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(out, "follow: invalid max_depth %q: expected integer\n", args[1])
			return nil
		}
		// graph.FollowOptions.MaxDepth: 0 means unlimited; negative values also satisfy
		// MaxDepth <= 0, so they are treated identically to 0 (unlimited). No guard is
		// needed — the behaviour is intentional and tested.
		maxDepth = d
	}

	g, ok, err := s.articulateForSession(ctx, "follow", out)
	if err != nil || !ok {
		return err
	}

	chain := graph.FollowTranslation(g, element, graph.FollowOptions{MaxDepth: maxDepth})
	cc := graph.ClassifyChain(chain, graph.ClassifyOptions{})
	if err := graph.PrintChain(out, cc); err != nil {
		return err
	}
	s.recordTurn(rawLine, cc, nil)
	return nil
}

// cmdBottleneck identifies provisionally central elements in the current
// session cut and renders them via graph.PrintBottleneckNotes.
//
// A turn is recorded only on success. The Reading field carries the
// []graph.BottleneckNote slice produced by this analysis.
func (s *AnalysisSession) cmdBottleneck(ctx context.Context, rawLine string, out io.Writer) error {
	g, ok, err := s.articulateForSession(ctx, "bottleneck", out)
	if err != nil || !ok {
		return err
	}
	notes := graph.IdentifyBottlenecks(g, graph.BottleneckOptions{})
	if err := graph.PrintBottleneckNotes(out, g, notes); err != nil {
		return err
	}
	s.recordTurn(rawLine, notes, nil)
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
