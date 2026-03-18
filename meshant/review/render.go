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

// RenderDraft formats d for display in the review session.
//
// index is the 1-based position of this draft in the session queue; total is
// the total queue length. Output is a multi-line string containing all
// candidate and provenance fields. Empty values are shown as "(empty)" so the
// reviewer can see at a glance which fields need attention.
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

// RenderAmbiguities formats warnings for display below a rendered draft.
//
// Each warning is shown on its own line as "[field] message". Returns
// "(none)" when warnings is empty or nil, so the reviewer always sees an
// explicit no-ambiguity signal rather than blank space.
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

// chainIDLen is the maximum number of runes shown for a draft ID in RenderChain.
// Eight characters are enough to distinguish siblings within a typical chain.
const chainIDLen = 8

// chainWhatChangedLen is the maximum number of runes shown for what_changed
// in RenderChain. Longer values are truncated with "..." to keep each line
// readable at a standard terminal width.
const chainWhatChangedLen = 60

// truncateString returns s unchanged when its rune length is <= maxLen.
// Otherwise it returns the first maxLen runes followed by "...".
// Rune slicing is used instead of byte slicing so that multi-byte UTF-8
// codepoints are never split.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// RenderChain formats a derivation chain for display in the review session.
//
// Each draft is shown with its 1-based index, truncated ID (first 8 chars),
// extraction_stage, extracted_by, and truncated what_changed (first 60 chars).
// The last draft in the chain is marked as the current draft under review.
// Between consecutive drafts the matching DraftStepClassification is shown
// with its Kind and Reason.
//
// Returns "(no derivation chain)\n" if chain is empty or nil.
// Classifications may be nil or shorter than len(chain)-1 — missing steps
// are silently omitted.
func RenderChain(chain []schema.TraceDraft, classifications []loader.DraftStepClassification) string {
	if len(chain) == 0 {
		return "(no derivation chain)\n"
	}

	// Index classifications by StepIndex for O(1) lookup.
	// StepIndex is 1-based: StepIndex=1 means chain[0]→chain[1].
	byStep := make(map[int]loader.DraftStepClassification, len(classifications))
	for _, c := range classifications {
		byStep[c.StepIndex] = c
	}

	var b strings.Builder
	for i, d := range chain {
		// Mark the final draft so the reviewer knows it is the one under review.
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

		// If there is a next draft, show the classification for this step.
		// byStep key i+1 = StepIndex for the transition from chain[i] to chain[i+1].
		if i < len(chain)-1 {
			if c, ok := byStep[i+1]; ok {
				fmt.Fprintf(&b, "    | %s: %s\n", string(c.Kind), c.Reason)
			}
		}
	}
	return b.String()
}

// valueOrEmpty returns s if non-empty, or the placeholder "(empty)".
// Used by RenderDraft to make absent fields visible to the reviewer.
func valueOrEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}
	return s
}

// sliceOrEmpty returns a comma-joined string of ss if non-empty, or "(empty)".
// Used by RenderDraft to render slice fields (source, target, tags,
// intentionally_blank) in a single readable line.
func sliceOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return "(empty)"
	}
	return strings.Join(ss, ", ")
}
