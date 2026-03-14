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

// PrintChain writes a human-readable rendering of cc to w.
//
// The output includes: start element, cut parameters, each step with its
// classification, breaks, and a philosophical footer noting that the chain
// is a situated reading — not an objective description.
func PrintChain(w io.Writer, cc ClassifiedChain) error {
	chain := cc.Chain

	// Header
	if _, err := fmt.Fprintln(w, "=== Translation Chain (provisional reading) ==="); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Start element
	if _, err := fmt.Fprintf(w, "Start element: %s\n", chain.StartElement); err != nil {
		return err
	}

	// Cut summary
	if err := printChainCut(w, chain.Cut); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Steps
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

	// Breaks
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

	// Footer — philosophical commitment
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

// printChainCut writes the cut summary line.
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

// printChainStep writes one step with its classification.
func printChainStep(w io.Writer, idx int, step ChainStep, classifications []StepClassification) error {
	// Step number and edge traversal
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

	// Classification (if available for this step index)
	if idx < len(classifications) {
		c := classifications[idx]
		if _, err := fmt.Fprintf(w, "     classification: %s — %s\n", c.Kind, c.Reason); err != nil {
			return err
		}
	}

	return nil
}

// chainJSONEnvelope is the JSON output structure for a classified chain.
type chainJSONEnvelope struct {
	StartElement    string                `json:"start_element"`
	Steps           []chainStepJSON       `json:"steps"`
	Breaks          []ChainBreak          `json:"breaks"`
	Cut             Cut                   `json:"cut"`
	Classifications []StepClassification  `json:"classifications"`
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

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
