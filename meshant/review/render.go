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
