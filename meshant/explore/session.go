// Package explore implements the interactive analysis session for MeshAnt.
//
// AnalysisSession is the core type: a REPL that maintains per-session state
// (observer, window, tags) and records each command as a positioned analytical
// act (AnalysisTurn). The session ends on "quit" or EOF.
//
// Design principles:
//   - Injected io.Reader / io.Writer make the REPL testable without a terminal.
//   - The TraceStore is queried live on each turn — no snapshot is taken at open.
//   - Each AnalysisTurn records the cut conditions in effect at execution time,
//     preserving the full positional trajectory for downstream promotion.
//   - Observer is mutable mid-session (ANT-native: shifting reading position).
//
// See docs/decisions/explore-v1.md for the full design rationale, including
// ANT tensions T172.1–T172.6.
//
// CLI entry point: meshant/cmd/meshant/cmd_explore.go
// Command implementations: commands.go (#183), commands_dual.go (#184),
//   suggest.go (#185), trace.go (#186)
package explore

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// AnalysisSession is the in-memory state for a single interactive explore session.
//
// ts is queried live on each turn — the substrate does not freeze at session open
// (D2 in explore-v1.md). This means traces added to the store while the session
// is open are visible to subsequent turns (T172.6: the promoted trace records acts,
// not reproducible readings — see explore-v1.md T172.6).
//
// turns is a linear slice in v1. Tree-structured (branching) sessions are deferred
// to v5 — see T172.3 in explore-v1.md. The field must not bake in a topology that
// precludes future branching.
type AnalysisSession struct {
	ts       store.TraceStore // injected; queried live on each turn (D2)
	analyst  string           // who is conducting the session; stable across turns (D1)
	observer string           // current observer position; mutable per-turn (D1)
	window   graph.TimeWindow // session-level time window; changed by `window` command
	tags     []string         // session-level tag filters; changed by `tags` command
	turns    []AnalysisTurn   // ordered linear history; no branching in v1 (T172.3)
}

// NewSession creates an AnalysisSession backed by ts, identified by analyst.
// The store is queried live on each turn — no snapshot is taken at session start (D2).
//
// ts may be nil when no trace substrate is loaded (e.g. meshant with no file arg).
// Commands that require the store will return an inline error when ts is nil.
//
// analyst may be empty if the caller does not know who is conducting the session;
// the session still works but promoted traces will carry an empty observer field.
func NewSession(ts store.TraceStore, analyst string) *AnalysisSession {
	return &AnalysisSession{
		ts:      ts,
		analyst: analyst,
	}
}

// Analyst returns the session analyst name. Stable across turns.
func (s *AnalysisSession) Analyst() string { return s.analyst }

// Observer returns the current session observer position.
// Empty until the first `cut <observer>` command.
func (s *AnalysisSession) Observer() string { return s.observer }

// Turns returns a deep-enough copy of the ordered turn history to prevent
// callers from mutating the session's internal record.
//
// Each AnalysisTurn is value-copied (struct fields are copied by value). The
// Tags slice within each turn is also deep-copied so that element mutations
// by the caller do not propagate back into the session record. This matches
// the deep-copy discipline applied in recordTurn.
func (s *AnalysisSession) Turns() []AnalysisTurn {
	result := make([]AnalysisTurn, len(s.turns))
	for i, t := range s.turns {
		result[i] = t
		if len(t.Tags) > 0 {
			tagsCopy := make([]string, len(t.Tags))
			copy(tagsCopy, t.Tags)
			result[i].Tags = tagsCopy
		}
	}
	return result
}

// Run executes the interactive REPL loop, reading commands from in and writing
// output to out. Blocking; returns when the analyst types "quit" or "q", or when
// in reaches EOF.
//
// Follows meshant/review/session.go as the reference REPL pattern (D7):
//   - bufio.Scanner for line reading
//   - All output to out, never to os.Stdout directly
//   - Testable by passing strings.NewReader and bytes.Buffer
//
// Run returns nil on normal exit (quit or EOF). It returns a non-nil error only
// for unrecoverable internal errors — inline command errors are printed to out
// and the loop continues.
func (s *AnalysisSession) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)

	for {
		fmt.Fprintf(out, "meshant> ")
		if !scanner.Scan() {
			// EOF or scanner error — treat as quit, consistent with review/session.go.
			// scanner.Err() is intentionally not checked here: in an interactive
			// terminal session a closed or errored reader is an unrecoverable condition;
			// surfacing the error would not help the analyst recover, and the session
			// ends cleanly either way. If a command-line tool discovers this silent
			// swallow is a problem in practice, checking scanner.Err() here is
			// straightforward to add.
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "quit" || line == "q" {
			break
		}
		if err := s.dispatch(ctx, line, out); err != nil {
			return err
		}
	}

	return nil
}

// dispatch routes a trimmed, non-empty, non-quit command line to its handler.
// Returns nil for unknown commands and inline errors (they are printed to out).
// Returns a non-nil error only for unrecoverable internal failures.
func (s *AnalysisSession) dispatch(ctx context.Context, line string, out io.Writer) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}
	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "cut":
		return s.cmdCut(line, args, out)
	case "articulate":
		return s.cmdArticulate(ctx, line, out)
	case "shadow":
		return s.cmdShadow(ctx, line, out)
	case "follow":
		return s.cmdFollow(ctx, line, args, out)
	case "bottleneck":
		return s.cmdBottleneck(ctx, line, out)
	case "diff":
		return s.cmdDiff(ctx, line, args, out)
	case "gaps":
		return s.cmdGaps(ctx, line, args, out)
	case "window":
		return s.cmdWindow(line, args, out)
	case "tags":
		return s.cmdTags(line, args, out)
	case "help", "h":
		fmt.Fprint(out, helpText())
		return nil
	default:
		fmt.Fprintf(out, "unknown command %q — type 'help' for a list of commands\n", cmd)
		return nil
	}
}

// cmdCut changes the session observer position and records a turn.
//
// Usage: cut <observer>
//
// The turn records the observer that became active as a result of the cut
// (not the prior observer). This preserves the positional trajectory in turns:
// each turn.Observer shows where the analyst was reading from after that cut.
//
// No turn is recorded when the cut fails (e.g. missing observer name).
func (s *AnalysisSession) cmdCut(rawLine string, args []string, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintf(out, "cut: observer name required — usage: cut <observer>\n")
		return nil // inline error; session continues
	}
	s.observer = args[0]
	fmt.Fprintf(out, "observer → %s\n", s.observer)
	s.recordTurn(rawLine, nil, nil)
	return nil
}

// helpText returns the help listing for the current command set.
// Extended in #185–#186 as new commands are added.
func helpText() string {
	return `Commands:
  cut <observer>           set the observer position for subsequent turns
  articulate               articulate the mesh graph from the current position
  shadow                   summarise what the current cut leaves in shadow
  diff <observer-b>        compare current cut against observer-b's reading
  gaps <observer-b>        analyse what each observer sees that the other does not
  follow <element> [depth] follow a translation chain from the named element
  bottleneck               surface provisionally central elements in the current cut
  window <from> <to>       set a time window filter (RFC3339); 'window clear' to reset
  tags <t1> [t2...]        set tag filters; 'tags clear' to reset
  help  (h)                show this help
  quit  (q)                end the session; discards unsaved turns
`
}

// recordTurn appends a new AnalysisTurn to the session history.
//
// command is the full line as typed by the analyst (e.g. "cut alice",
// "articulate", "shadow"). This populates Turn.Command and preserves the
// full invocation in the session record — callers must pass rawLine, not
// just the verb.
//
// Window and Tags are deep-copied so that future changes to s.window / s.tags
// do not retroactively alter a completed turn's recorded conditions (D3).
// Reading and Suggestion are stored by reference — their concrete types
// (graph.MeshGraph, graph.ShadowSummary, etc.) are immutable values from
// the analytical engine.
func (s *AnalysisSession) recordTurn(command string, reading interface{}, suggestion *SuggestionMeta) {
	// Deep-copy tags: nil input → nil copy (no empty slice created for zero-tag sessions).
	var tagsCopy []string
	if len(s.tags) > 0 {
		tagsCopy = make([]string, len(s.tags))
		copy(tagsCopy, s.tags)
	}

	s.turns = append(s.turns, AnalysisTurn{
		Observer:   s.observer,
		Window:     s.window, // graph.TimeWindow is a value type; no deep copy needed
		Tags:       tagsCopy,
		Command:    command,
		Reading:    reading,
		Suggestion: suggestion,
		ExecutedAt: time.Now(),
	})
}
