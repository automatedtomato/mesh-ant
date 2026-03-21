// chain_print.go provides human-readable and JSON output for ClassifiedChain.
//
// PrintChain produces a situated, annotated reading of a translation chain.
// The footer explicitly names the output as an analytical judgment — not an
// objective description. PrintChainJSON produces a machine-readable envelope
// for piping or storage.
package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PrintChain writes a human-readable rendering of cc to w, including start
// element, cut, classified steps, breaks, and a positioned-reading footer.
func PrintChain(w io.Writer, cc ClassifiedChain) error {
	chain := cc.Chain

	if _, err := fmt.Fprintln(w, "=== Translation Chain (provisional reading) ==="); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Start element: %s\n", chain.StartElement); err != nil {
		return err
	}

	if err := printChainCut(w, chain.Cut); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Criterion block positioned after cut, before steps, so interpretive
	// conditions appear before judgments. Only rendered when non-zero.
	if err := printChainCriterion(w, cc.Criterion); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Steps (%d):\n", len(chain.Steps)); err != nil {
		return err
	}
	for i, step := range chain.Steps {
		if err := printChainStep(w, i, step, cc.Classifications); err != nil {
			return err
		}
	}
	if len(chain.Steps) == 0 {
		if _, err := fmt.Fprintln(w, "  (none)"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if len(chain.Breaks) > 0 {
		if _, err := fmt.Fprintf(w, "Breaks (%d):\n", len(chain.Breaks)); err != nil {
			return err
		}
		for _, b := range chain.Breaks {
			if _, err := fmt.Fprintf(w, "  %s: %s\n", b.AtElement, b.Reason); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "---"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Note: this chain is a reading through one situated cut."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Classification is an analytical judgment, not an intrinsic property."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "The same chain from a different cut may yield different classifications."); err != nil {
		return err
	}

	return nil
}

// printChainCut writes the cut summary line to w.
func printChainCut(w io.Writer, c Cut) error {
	parts := []string{}

	if len(c.ObserverPositions) > 0 {
		parts = append(parts, fmt.Sprintf("observer=[%s]", strings.Join(c.ObserverPositions, ", ")))
	}

	if !c.TimeWindow.IsZero() {
		tw := ""
		if !c.TimeWindow.Start.IsZero() {
			tw += c.TimeWindow.Start.UTC().Format("2006-01-02")
		}
		tw += ".."
		if !c.TimeWindow.End.IsZero() {
			tw += c.TimeWindow.End.UTC().Format("2006-01-02")
		}
		parts = append(parts, fmt.Sprintf("window=%s", tw))
	}

	if len(c.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags=[%s]", strings.Join(c.Tags, ", ")))
	}

	if len(parts) > 0 {
		_, err := fmt.Fprintf(w, "Cut: %s\n", strings.Join(parts, ", "))
		return err
	}

	_, err := fmt.Fprintln(w, "Cut: (full cut — no filters)")
	return err
}

// printChainCriterion writes the equivalence criterion block to w when non-zero.
// The heuristics disclaimer (v1 edge-driven, criterion not a dispatch key) is
// always emitted for non-zero criteria (C1). Trailing blank line before steps.
func printChainCriterion(w io.Writer, c EquivalenceCriterion) error {
	if c.IsZero() {
		return nil
	}

	// Name-only criteria are analytically incomplete but structurally valid; still print.
	if c.Name != "" {
		if _, err := fmt.Fprintf(w, "Criterion: %s\n", c.Name); err != nil {
			return err
		}
	}

	// ANT T2: a name without a declaration is a handle with no interpretive
	// grounding — signal the weakness explicitly rather than silence.
	if c.Name != "" && c.Declaration == "" {
		if _, err := fmt.Fprintln(w, "(handle only — no declaration grounds this reading)"); err != nil {
			return err
		}
	}

	if c.Declaration != "" {
		if _, err := fmt.Fprintf(w, "Declaration: %s\n", c.Declaration); err != nil {
			return err
		}
	}

	if len(c.Preserve) > 0 {
		if _, err := fmt.Fprintf(w, "Preserve: [%s]\n", strings.Join(c.Preserve, ", ")); err != nil {
			return err
		}
	}

	if len(c.Ignore) > 0 {
		if _, err := fmt.Fprintf(w, "Ignore: [%s]\n", strings.Join(c.Ignore, ", ")); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "(criterion carried — classification uses v1 heuristics)"); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return nil
}

// printChainStep writes one step with its classification to w.
func printChainStep(w io.Writer, idx int, step ChainStep, classifications []StepClassification) error {
	mediation := "(none)"
	if step.Edge.Mediation != "" {
		mediation = step.Edge.Mediation
	}

	if _, err := fmt.Fprintf(w, "  %d. %s --[%s]--> %s\n",
		idx+1, step.ElementExited, step.Edge.WhatChanged, step.ElementEntered); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "     mediation: %s\n", mediation); err != nil {
		return err
	}

	if idx < len(classifications) {
		c := classifications[idx]
		if _, err := fmt.Fprintf(w, "     classification: %s — %s\n", c.Kind, c.Reason); err != nil {
			return err
		}
	}

	return nil
}

// chainJSONEnvelope is the JSON output structure for a classified chain.
// Criterion is a pointer with omitempty so the key is fully absent (not `{}`)
// when no criterion was declared (design rule A2).
type chainJSONEnvelope struct {
	StartElement    string                `json:"start_element"`
	Steps           []chainStepJSON       `json:"steps"`
	Breaks          []ChainBreak          `json:"breaks"`
	Cut             Cut                   `json:"cut"`
	Classifications []StepClassification  `json:"classifications"`
	Criterion       *EquivalenceCriterion `json:"criterion,omitempty"`
}

type chainStepJSON struct {
	Edge           Edge   `json:"edge"`
	ElementExited  string `json:"element_exited"`
	ElementEntered string `json:"element_entered"`
}

// PrintChainJSON writes cc as indented JSON to w.
func PrintChainJSON(w io.Writer, cc ClassifiedChain) error {
	chain := cc.Chain

	steps := make([]chainStepJSON, len(chain.Steps))
	for i, s := range chain.Steps {
		steps[i] = chainStepJSON{
			Edge:           s.Edge,
			ElementExited:  s.ElementExited,
			ElementEntered: s.ElementEntered,
		}
	}

	breaks := chain.Breaks
	if breaks == nil {
		breaks = []ChainBreak{}
	}

	classifications := cc.Classifications
	if classifications == nil {
		classifications = []StepClassification{}
	}

	env := chainJSONEnvelope{
		StartElement:    chain.StartElement,
		Steps:           steps,
		Breaks:          breaks,
		Cut:             chain.Cut,
		Classifications: classifications,
	}

	if !cc.Criterion.IsZero() { // pointer+omitempty: absent (not null/`{}`) for zero criterion
		crit := cc.Criterion
		env.Criterion = &crit
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
