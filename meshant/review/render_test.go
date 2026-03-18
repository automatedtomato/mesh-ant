package review_test

import (
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TestRenderDraft_ContainsIndex verifies that the rendered output contains the
// index/total position indicator (e.g. "1/3") so the reviewer knows where
// they are in the queue.
func TestRenderDraft_ContainsIndex(t *testing.T) {
	d := schema.TraceDraft{WhatChanged: "something shifted"}
	out := review.RenderDraft(d, 1, 3)
	if !strings.Contains(out, "1/3") {
		t.Errorf("expected output to contain '1/3', got:\n%s", out)
	}
}

// TestRenderDraft_ContainsWhatChanged verifies that the rendered output
// contains the "what_changed:" label and the field's value.
func TestRenderDraft_ContainsWhatChanged(t *testing.T) {
	d := schema.TraceDraft{WhatChanged: "consensus collapsed"}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "what_changed:") {
		t.Errorf("expected output to contain 'what_changed:', got:\n%s", out)
	}
	if !strings.Contains(out, "consensus collapsed") {
		t.Errorf("expected output to contain 'consensus collapsed', got:\n%s", out)
	}
}

// TestRenderDraft_ContainsExtractionStage verifies that the rendered output
// contains the "extraction_stage:" label and the field's value.
func TestRenderDraft_ContainsExtractionStage(t *testing.T) {
	d := schema.TraceDraft{ExtractionStage: "weak-draft"}
	out := review.RenderDraft(d, 2, 5)
	if !strings.Contains(out, "extraction_stage:") {
		t.Errorf("expected output to contain 'extraction_stage:', got:\n%s", out)
	}
	if !strings.Contains(out, "weak-draft") {
		t.Errorf("expected output to contain 'weak-draft', got:\n%s", out)
	}
}

// TestRenderDraft_EmptyFieldsRendered verifies that a zero-value draft renders
// without panic, and that field labels are still present in the output.
// Empty values should appear as "(empty)" rather than being omitted.
func TestRenderDraft_EmptyFieldsRendered(t *testing.T) {
	d := schema.TraceDraft{}
	// Should not panic.
	out := review.RenderDraft(d, 1, 1)
	expectedLabels := []string{
		"what_changed:",
		"source:",
		"target:",
		"mediation:",
		"observer:",
		"tags:",
		"extraction_stage:",
		"extracted_by:",
		"uncertainty_note:",
		"intentionally_blank:",
		"derived_from:",
		"criterion_ref:",
	}
	for _, label := range expectedLabels {
		if !strings.Contains(out, label) {
			t.Errorf("expected output to contain label %q, got:\n%s", label, out)
		}
	}
}

// TestRenderAmbiguities_NoWarnings verifies that an empty warnings slice
// renders a clear "(none)" (or similar) indicator rather than blank output.
func TestRenderAmbiguities_NoWarnings(t *testing.T) {
	out := review.RenderAmbiguities(nil)
	lowerOut := strings.ToLower(out)
	if !strings.Contains(lowerOut, "none") {
		t.Errorf("expected '(none)' or similar for empty warnings, got: %q", out)
	}
}

// TestRenderAmbiguities_ContainsWarningMessage verifies that when warnings are
// present, the warning's Message text appears in the rendered output.
func TestRenderAmbiguities_ContainsWarningMessage(t *testing.T) {
	warnings := []review.AmbiguityWarning{
		{
			Field:   "what_changed",
			Message: "what_changed is unregistered from this position — the nature of the change is in shadow",
		},
	}
	out := review.RenderAmbiguities(warnings)
	if !strings.Contains(out, "what_changed is unregistered from this position") {
		t.Errorf("expected warning message in output, got:\n%s", out)
	}
}
