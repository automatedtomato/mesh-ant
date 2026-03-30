// trace.go implements Principle 8 reflexivity for the meshant explore REPL.
//
// A completed analysis session can be promoted to the TraceStore via the `save`
// command. The promoted record is a single schema.Trace that captures the
// analytical trajectory — the sequence of observer positions visited and the
// element set visible in the final articulation.
//
// This closes the reflexivity gap: the interactive analysis apparatus enters the
// mesh it analyzed. The session was a positioned observation act; it now appears
// in the same substrate as the traces it read.
//
// Multiple saves are permitted: each call to save/Promote records the session
// state at that moment. This supports mid-session promotion (a partial record)
// as well as final promotion. Each promoted trace receives a new UUID, so they
// are independent entries in the store — not updates to a prior record.
//
// Design: D5 in docs/decisions/explore-v1.md.
// Tensions: T172.1 (acts, not readings) and T172.5 (positions as Source).
package explore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TagValueExplore marks a trace promoted from an interactive explore session.
//
// Distinct from schema.TagValueSession ("session"), which marks LLM session
// promotions. An explore session is a human-driven analytical act; an LLM
// session is a machine-driven extraction act.
//
// Typed as schema.TagValue for consistency with the schema tag vocabulary
// (TagValueSession, TagValueArticulation, TagValueDraft).
const TagValueExplore schema.TagValue = "explore"

// Promote converts the analysis session into a schema.Trace and stores it,
// closing the Principle 8 reflexivity gap: the explore session — itself an
// observation act — enters the mesh as a trace.
//
// Multiple calls are permitted — see package doc and D5 in explore-v1.md.
// Each call promotes the session state at that moment with a new UUID.
// This supports mid-session saves (partial records) as well as final saves.
//
// Guards (return error; session state is not modified):
//  1. ts is nil — no store to write to
//  2. s.analyst is empty — unattributable observation (who conducted the session?)
//
// Field mapping:
//
//	ID          ← new UUID v4 (unique per Promote call)
//	Timestamp   ← time.Now()
//	Observer    ← s.analyst (who conducted the session)
//	WhatChanged ← "explore session: N turns, observers visited: [alice, bob, ...]"
//	Mediation   ← "meshant explore"
//	Source      ← deduplicated observer positions in order of first appearance
//	             (T172.5: these are reading positions, not network elements — named, not resolved)
//	Target      ← element names from the final articulation turn's MeshGraph.Nodes;
//	             nil if no articulation turn exists, []string{} if articulation produced an empty graph
//	             (T172.1: final reading, not a full trajectory snapshot — named, not resolved)
//	Tags        ← [TagValueExplore]
func (s *AnalysisSession) Promote(ctx context.Context) error {
	if s.ts == nil {
		return errors.New("save: no store — load a trace file or connect a database first")
	}
	if s.analyst == "" {
		return errors.New("save: analyst name required — cannot promote an unattributable session (use --analyst flag)")
	}

	id, err := loader.NewUUID()
	if err != nil {
		return fmt.Errorf("save: generate UUID: %w", err)
	}

	observers := observerPositions(s.turns)
	target := finalArticulationElements(s.turns)
	whatChanged := exploreWhatChanged(len(s.turns), observers)

	t := schema.Trace{
		ID:          id,
		Timestamp:   time.Now(),
		Observer:    s.analyst,
		WhatChanged: whatChanged,
		Mediation:   "meshant explore",
		Source:      observers,
		Target:      target,
		Tags:        []string{string(TagValueExplore)},
	}

	if err := t.Validate(); err != nil {
		return fmt.Errorf("save: promoted trace invalid: %w", err)
	}

	if err := s.ts.Store(ctx, []schema.Trace{t}); err != nil {
		return fmt.Errorf("save: store failed: %w", err)
	}

	return nil
}

// cmdSave handles the "save" REPL command: promotes the current session to the
// TraceStore as a schema.Trace tagged TagValueExplore.
//
// Multiple saves are permitted — each records the session state at that moment.
// Errors are printed inline — the session continues after a failed save so
// the analyst can correct the issue (e.g. set an analyst name) without losing
// their session state.
//
// The rawLine parameter is unused: cmdSave does not record a turn because
// saving is a meta-operation that records the session, not an analytical act
// that produces a positioned reading.
func (s *AnalysisSession) cmdSave(ctx context.Context, _ string, out io.Writer) error {
	if err := s.Promote(ctx); err != nil {
		fmt.Fprintf(out, "%v\n", err)
		return nil
	}
	fmt.Fprintf(out, "session promoted: %d turns recorded as explore trace\n", len(s.turns))
	return nil
}

// observerPositions extracts deduplicated observer positions from the turn
// history in order of first appearance.
//
// Empty observer values are skipped — they arise when a turn is recorded before
// the first `cut` command sets s.observer (e.g. if a future command calls
// recordTurn while s.observer is still ""). An empty string is not a valid
// reading position in the promoted trace's Source.
//
// Returns nil when no observer was ever set (valid; the promoted Source is nil).
//
// T172.5 tension: observer positions are analytical reading positions, not network
// elements. Placing them in Source conflates "which position the analyst read from"
// with "what produced the trace." This encoding is provisional — see explore-v1.md
// T172.5. The TagValueExplore tag signals that Source/Target should be interpreted
// differently for explore-promoted traces.
func observerPositions(turns []AnalysisTurn) []string {
	seen := make(map[string]bool)
	var positions []string
	for _, t := range turns {
		if t.Observer == "" || seen[t.Observer] {
			continue
		}
		seen[t.Observer] = true
		positions = append(positions, t.Observer)
	}
	return positions
}

// finalArticulationElements returns the sorted element names from the last turn
// whose Reading is a graph.MeshGraph (i.e. the last `articulate` command).
//
// Returns:
//   - nil        — no articulation turn exists in the session history
//   - []string{} — articulation ran but the graph was empty (no traces matched the cut)
//   - []string{...} — sorted element names from the final articulation's Nodes map
//
// The nil/empty distinction preserves the difference between "analyst never
// articulated" and "analyst articulated but the cut found nothing." Both are
// valid and analytically meaningful states. Downstream consumers can read
// nil Target as "no articulation performed" and len(Target)==0 as "articulation
// returned an empty graph."
//
// T172.1 tension: this records the elements visible in the final reading, not a
// snapshot of the full analytical trajectory across all turns. Named, not resolved.
func finalArticulationElements(turns []AnalysisTurn) []string {
	for i := len(turns) - 1; i >= 0; i-- {
		g, ok := turns[i].Reading.(graph.MeshGraph)
		if !ok {
			continue
		}
		if len(g.Nodes) == 0 {
			// Articulation ran; graph was empty. Return non-nil empty slice to
			// distinguish from the "no articulation turn" case (which returns nil).
			return []string{}
		}
		names := make([]string, 0, len(g.Nodes))
		for name := range g.Nodes {
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	return nil
}

// exploreWhatChanged generates the WhatChanged string for a promoted explore trace.
//
// Example output: "explore session: 5 turns, observers visited: [alice, bob]"
// With no observers: "explore session: 0 turns, observers visited: []"
func exploreWhatChanged(turnCount int, observers []string) string {
	return fmt.Sprintf("explore session: %d turns, observers visited: [%s]",
		turnCount, strings.Join(observers, ", "))
}
