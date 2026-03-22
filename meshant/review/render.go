// render.go formats TraceDraft records and AmbiguityWarning slices for
// display in an interactive review session.
//
// Output is plain multi-line text intended for terminal display. No colour
// codes or terminal-specific formatting are used so that output can also be
// piped or logged cleanly.
//
// All field values are shown — empty values are rendered as "(empty)" rather
// than being omitted, so the reviewer sees the full shape of every draft.
package review

import (
	"fmt"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// RenderDraft formats d for review display. Empty values shown as "(empty)".
func RenderDraft(d schema.TraceDraft, index, total int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Draft [%d/%d]\n", index, total)
	fmt.Fprintf(&b, "  what_changed:       %s\n", valueOrEmpty(d.WhatChanged))
	fmt.Fprintf(&b, "  source:             %s\n", sliceOrEmpty(d.Source))
	fmt.Fprintf(&b, "  target:             %s\n", sliceOrEmpty(d.Target))
	fmt.Fprintf(&b, "  mediation:          %s\n", valueOrEmpty(d.Mediation))
	fmt.Fprintf(&b, "  observer:           %s\n", valueOrEmpty(d.Observer))
	fmt.Fprintf(&b, "  tags:               %s\n", sliceOrEmpty(d.Tags))
	fmt.Fprintf(&b, "  extraction_stage:   %s\n", valueOrEmpty(d.ExtractionStage))
	fmt.Fprintf(&b, "  extracted_by:       %s\n", valueOrEmpty(d.ExtractedBy))
	fmt.Fprintf(&b, "  uncertainty_note:   %s\n", valueOrEmpty(d.UncertaintyNote))
	fmt.Fprintf(&b, "  intentionally_blank: %s\n", sliceOrEmpty(d.IntentionallyBlank))
	fmt.Fprintf(&b, "  derived_from:       %s\n", valueOrEmpty(d.DerivedFrom))
	fmt.Fprintf(&b, "  criterion_ref:      %s\n", valueOrEmpty(d.CriterionRef))

	return b.String()
}

// RenderAmbiguities formats warnings for display. Returns "(none)\n" for empty/nil input.
func RenderAmbiguities(warnings []AmbiguityWarning) string {
	if len(warnings) == 0 {
		return "(none)\n"
	}

	var b strings.Builder
	for _, w := range warnings {
		fmt.Fprintf(&b, "[%s] %s\n", w.Field, w.Message)
	}
	return b.String()
}

const chainIDLen = 8          // truncated ID rune length for RenderChain
const chainWhatChangedLen = 60 // truncated what_changed rune length for RenderChain

// truncateString returns the first maxLen runes of s followed by "..." when exceeded.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// RenderChain formats a derivation chain for review display.
// Returns "(no derivation chain)\n" for empty/nil input.
func RenderChain(chain []schema.TraceDraft, classifications []loader.DraftStepClassification) string {
	if len(chain) == 0 {
		return "(no derivation chain)\n"
	}

	byStep := make(map[int]loader.DraftStepClassification, len(classifications))
	for _, c := range classifications {
		byStep[c.StepIndex] = c
	}

	var b strings.Builder
	for i, d := range chain {
		current := ""
		if i == len(chain)-1 {
			current = "  <-- current"
		}
		fmt.Fprintf(&b, "  [%d] id:%-8s  stage:%-12s  by:%s%s\n",
			i+1,
			truncateString(d.ID, chainIDLen),
			valueOrEmpty(d.ExtractionStage),
			valueOrEmpty(d.ExtractedBy),
			current,
		)
		fmt.Fprintf(&b, "      what_changed: %s\n", truncateString(valueOrEmpty(d.WhatChanged), chainWhatChangedLen))

		if i < len(chain)-1 {
			if c, ok := byStep[i+1]; ok {
				fmt.Fprintf(&b, "    | %s: %s\n", string(c.Kind), c.Reason)
			}
		}
	}
	return b.String()
}

// valueOrEmpty returns s if non-empty, or "(empty)".
func valueOrEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

// sliceOrEmpty returns a comma-joined string of ss if non-empty, or "(empty)".
func sliceOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return "(empty)"
	}
	return strings.Join(ss, ", ")
}
