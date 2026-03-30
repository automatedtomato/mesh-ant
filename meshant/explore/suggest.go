// suggest.go implements the `suggest` command for the meshant explore REPL.
//
// The `suggest` command calls an LLM with a prior analytical reading (shadow,
// bottleneck, or gaps) and records the suggestion with full provenance via
// SuggestionMeta. An LLM suggestion without a named analyst and a known cut
// is unattributable — the command refuses when either is absent.
//
// SuggestClient is defined here (not in the llm package) following the Go
// principle "accept interfaces where used". llm.AnthropicClient satisfies
// SuggestClient structurally via duck typing. Tests inject a mock.
//
// See docs/decisions/explore-v1.md D4 and T172.2 for design rationale.
package explore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// SuggestClient is the LLM interface required by the suggest command.
// llm.AnthropicClient satisfies this interface via duck typing.
// Tests inject a mockSuggestClient.
//
// system is an instruction prompt (role/context); prompt is the user message
// (the serialised analytical reading). Returns the LLM's text response.
type SuggestClient interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
}

// validBases lists the basis types that suggest can use. These correspond to
// the analytical commands that produce positioned readings the LLM can reason
// from: shadow (what is excluded), bottleneck (provisional centres), gaps
// (dual-observer asymmetry). "articulate" is not a basis — it produces a
// full MeshGraph which is better sent as shadow/bottleneck context.
//
// "follow" (ClassifiedChain) is a candidate for a future basis type — it is the
// most distinctively ANT command in the toolkit and directly names translation
// steps. Deferred to v2 to keep the v1 surface minimal.
var validBases = []string{"shadow", "bottleneck", "gaps"}

// suggestSystemPrompt is the LLM system instruction for the suggest command.
// It frames the LLM's role as navigational mediator, not neutral oracle (T172.2).
const suggestSystemPrompt = `You are a navigational assistant for an Actor-Network Theory (ANT) mesh analysis session.

The analyst has performed a positioned reading of a socio-technical network from a specific observer position. You will receive a summary of that reading — either a shadow summary (what is excluded from the current cut), bottleneck notes (provisionally central elements), or an observer gap analysis (what each observer sees that the other does not).

Your task: suggest concrete next analytical steps. What elements are worth following? What observer positions might reveal more? What translations appear incomplete? Ground your suggestions in the specific elements named in the reading. Keep suggestions brief (3-5 bullet points). Use ANT vocabulary (mediator, translation, inscription, black box) where it adds precision.

Do not claim to know the network's true structure. All observations are positional readings, not ground truth.`

// cmdSuggest calls the LLM with a prior analytical reading and records the
// suggestion with full SuggestionMeta provenance.
//
// Usage: suggest [shadow|bottleneck|gaps]
//
// With no argument, suggest uses the most recent shadow, bottleneck, or gaps
// turn from the session history. With an explicit basis, it finds the most
// recent turn matching that basis type.
//
// Guards (inline errors; session continues):
//  1. No LLM client configured (sc == nil)
//  2. Analyst not set (unattributable suggestion — D4 in explore-v1.md)
//  3. No trace substrate loaded
//  4. Observer not set
//  5. No prior shadow, bottleneck, or gaps reading in the current session
func (s *AnalysisSession) cmdSuggest(ctx context.Context, rawLine string, args []string, out io.Writer) error {
	// Guard 1: LLM client required.
	if s.sc == nil {
		fmt.Fprintf(out, "suggest: no LLM client configured — set MESHANT_LLM_API_KEY and use --analyst\n")
		return nil
	}

	// Guard 2: analyst required for attribution (D4).
	if s.analyst == "" {
		fmt.Fprintf(out, "suggest: analyst not set — use --analyst flag (a suggestion without a named analyst is unattributable)\n")
		return nil
	}

	// Guard 3: store required.
	if s.ts == nil {
		fmt.Fprintf(out, "suggest: no trace substrate loaded — open a file with: meshant <file.json>\n")
		return nil
	}

	// Guard 4: observer required.
	if s.observer == "" {
		fmt.Fprintf(out, "suggest: observer not set — use 'cut <observer>' first\n")
		return nil
	}

	// Determine the requested basis (explicit or auto-detect).
	var requestedBasis string
	if len(args) > 0 {
		requestedBasis = strings.ToLower(args[0])
		if !isValidBasis(requestedBasis) {
			fmt.Fprintf(out, "suggest: unknown basis %q — valid values: shadow, bottleneck, gaps\n", args[0])
			return nil
		}
	}

	// Find the most recent qualifying turn from session history.
	basisTurn, basisLabel := s.findBasisTurn(requestedBasis)
	if basisTurn == nil {
		if requestedBasis != "" {
			fmt.Fprintf(out, "suggest: no prior %s reading in this session — run %s first\n", requestedBasis, requestedBasis)
		} else {
			fmt.Fprintf(out, "suggest: no prior shadow, bottleneck, or gaps reading — run one of those commands first\n")
		}
		return nil
	}

	// Build the LLM prompt from the basis turn's reading.
	userPrompt, err := buildSuggestPrompt(basisTurn, basisLabel)
	if err != nil {
		fmt.Fprintf(out, "suggest: failed to build prompt: %v\n", err)
		return nil
	}

	// Call the LLM. Errors are inline — the session continues.
	suggestion, err := s.sc.Complete(ctx, suggestSystemPrompt, userPrompt)
	if err != nil {
		fmt.Fprintf(out, "suggest: LLM call failed: %v\n", err)
		return nil
	}

	// Render the suggestion.
	fmt.Fprintf(out, "=== Suggestion (from %s) ===\n\n%s\n\n---\nBasis: %s | Observer: %s | Analyst: %s\n",
		basisLabel, suggestion, basisLabel, s.observer, s.analyst)

	// Build CutMeta for provenance by re-articulating from the current session state.
	// This ensures the CutMeta reflects the substrate at suggest-time (D2: live substrate).
	cutMeta, traceCount := s.buildCutMeta(ctx)

	meta := &SuggestionMeta{
		Analyst:     s.analyst,
		CutUsed:     cutMeta,
		Basis:       basisLabel,
		TraceCount:  traceCount,
		GeneratedAt: time.Now(),
	}

	s.recordTurn(rawLine, suggestion, meta)
	return nil
}

// findBasisTurn scans the session turn history in reverse to find the most
// recent turn whose Reading qualifies as a suggest basis.
//
// If requestedBasis is non-empty, it finds the most recent turn matching that
// specific basis type. If empty, it returns the most recent turn of any valid
// basis type.
//
// Returns (nil, "") when no qualifying turn is found.
func (s *AnalysisSession) findBasisTurn(requestedBasis string) (*AnalysisTurn, string) {
	for i := len(s.turns) - 1; i >= 0; i-- {
		t := &s.turns[i]
		label := basisLabelForReading(t.Reading)
		if label == "" {
			continue // not a qualifying basis reading
		}
		if requestedBasis == "" || requestedBasis == label {
			return t, label
		}
	}
	return nil, ""
}

// basisLabelForReading returns the basis label for a turn's Reading, or ""
// if the Reading type is not a valid suggest basis.
//
// Valid basis types and their labels:
//   - graph.ShadowSummary    → "shadow"
//   - []graph.BottleneckNote → "bottleneck"
//   - graph.ObserverGap      → "gaps"
func basisLabelForReading(reading interface{}) string {
	switch reading.(type) {
	case graph.ShadowSummary:
		return "shadow"
	case []graph.BottleneckNote:
		return "bottleneck"
	case graph.ObserverGap:
		return "gaps"
	default:
		return ""
	}
}

// isValidBasis reports whether s is one of the allowed explicit basis labels.
func isValidBasis(s string) bool {
	for _, v := range validBases {
		if s == v {
			return true
		}
	}
	return false
}

// buildSuggestPrompt serialises the basis turn's Reading to JSON and wraps it
// in a structured user prompt for the LLM.
//
// The prompt includes the basis label, the observer position, and the
// serialised reading so the LLM has the positioned context it needs.
func buildSuggestPrompt(t *AnalysisTurn, basisLabel string) (string, error) {
	readingJSON, err := json.MarshalIndent(t.Reading, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal reading: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Observer position: %s\n", t.Observer)
	fmt.Fprintf(&sb, "Basis: %s\n", basisLabel)
	fmt.Fprintf(&sb, "Reading:\n%s\n", string(readingJSON))
	fmt.Fprintf(&sb, "\nSuggest concrete next analytical steps based on this positioned reading.")

	return sb.String(), nil
}

// buildCutMeta constructs a graph.CutMeta for the SuggestionMeta by performing
// a fresh articulation from the current session state (D2: live substrate).
//
// This is a metadata-only articulation — the graph produced here is not shown
// to the analyst and does not appear in the session record. It exists solely to
// provide authoritative provenance for SuggestionMeta.CutUsed (observer, trace
// count, shadow count at suggest-time). This is distinct from analytical
// articulations where the graph is the reading returned to the analyst.
//
// Returns (zero CutMeta, 0) on store query failure — errors are soft here since
// the LLM call already succeeded; failing to build metadata should not suppress
// the suggestion output. Note: a zero CutMeta.TraceCount will look like genuine
// data to downstream consumers (#186 save/promote) — this is a known limitation
// documented as a tension in explore-v1.md.
func (s *AnalysisSession) buildCutMeta(ctx context.Context) (graph.CutMeta, int) {
	traces, err := s.ts.Query(ctx, store.QueryOpts{})
	if err != nil {
		return graph.CutMeta{}, 0
	}
	g := graph.Articulate(traces, graph.ArticulationOptions{
		ObserverPositions: []string{s.observer},
		TimeWindow:        s.window,
		Tags:              s.tags,
	})
	meta := graph.CutMetaFromGraph(g)
	meta.Analyst = s.analyst
	return meta, meta.TraceCount
}
