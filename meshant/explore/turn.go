// turn.go defines the AnalysisTurn and SuggestionMeta types for the
// meshant explore interactive session.
//
// An AnalysisTurn is a single command executed within an AnalysisSession.
// It is a positioned analytical act: the Observer, Window, and Tags fields
// snapshot the cut conditions in effect at execution time, and Reading holds
// the positioned output produced by the command.
//
// "Reading" not "Result" — a result stands independently of its conditions;
// a reading requires a position to be interpretable. The field name enforces
// this at the type level.
//
// See docs/decisions/explore-v1.md D3 for full rationale.
package explore

import (
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// AnalysisTurn records a single command executed within an AnalysisSession.
//
// Observer, Window, and Tags are snapshotted at execution time (not at session
// start). Changing the window or tags via the `window`/`tags` commands affects
// future turns only — prior turns retain the conditions under which they executed,
// preserving the full analytical record (D3 in explore-v1.md).
//
// Reading is interface{} in v1. Concrete types depend on the command:
//   - cut:         nil (cut changes state; it is not itself a mesh reading)
//   - articulate:  graph.MeshGraph
//   - shadow:      []graph.ShadowElement
//   - follow:      graph.TranslationChain
//   - bottleneck:  []graph.BottleneckElement
//   - diff:        graph.GraphDiff
//   - gaps:        graph.GapsResult
//   - summarize:   string
//   - validate:    string
//   - help:        string
//
// Suggestion is nil for all commands except `suggest`. When non-nil it carries
// the full LLM provenance for that turn (wired in #185 — stub here for type
// completeness in the skeleton).
type AnalysisTurn struct {
	Observer   string          // ANT position active when this turn executed
	Window     graph.TimeWindow // time window active when this turn executed
	Tags       []string         // tag filters active when this turn executed
	Command    string          // the command string as typed by the analyst
	Reading    interface{}     // positioned output — "Reading" not "Result" (see package doc)
	Suggestion *SuggestionMeta // non-nil only when Command == "suggest" (wired in #185)
	ExecutedAt time.Time
}

// SuggestionMeta records provenance for an LLM-generated suggestion.
//
// Every output of the `suggest` command carries SuggestionMeta so the
// suggestion can be attributed to a named cut and a known substrate size.
// An LLM suggestion without a named cut is an unattributable reading —
// it cannot be placed in the analytical record without knowing from whose
// position it was generated and what substrate it saw.
//
// This follows the same discipline as meshant/llm SuggestionMeta.
// The LLM is a mediator: its output transforms the cut into a navigational
// suggestion. That mediation must be visible in the session record.
//
// Implementation: #185. The type is declared here so that AnalysisTurn
// compiles in the skeleton.
type SuggestionMeta struct {
	Analyst     string        // who asked for the suggestion
	CutUsed     graph.CutMeta // exact cut in effect when suggest was called
	Basis       string        // "gaps", "bottleneck", or "shadow" — what the LLM saw
	TraceCount  int           // size of the substrate the LLM saw
	GeneratedAt time.Time
}
