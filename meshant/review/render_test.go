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

// TestRenderDraft_SliceFieldsPopulated verifies that non-empty slice fields
// (Source, Tags) are rendered as comma-joined values in the output.
// This exercises the non-empty branch of sliceOrEmpty.
func TestRenderDraft_SliceFieldsPopulated(t *testing.T) {
	d := schema.TraceDraft{
		Source: []string{"actor-a", "actor-b"},
		Tags:   []string{"policy", "consent"},
	}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "actor-a, actor-b") {
		t.Errorf("expected comma-joined source 'actor-a, actor-b' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "policy, consent") {
		t.Errorf("expected comma-joined tags 'policy, consent' in output, got:\n%s", out)
	}
}

// TestRenderDraft_EmptyPlaceholder verifies that blank fields render as
// "(empty)" rather than being omitted from the output.
func TestRenderDraft_EmptyPlaceholder(t *testing.T) {
	d := schema.TraceDraft{}
	out := review.RenderDraft(d, 1, 1)
	if !strings.Contains(out, "(empty)") {
		t.Errorf("expected '(empty)' placeholder for blank fields, got:\n%s", out)
	}
}

// TestRenderAmbiguities_EmptySlice verifies that a non-nil empty slice also
// renders "(none)" — same contract as nil.
func TestRenderAmbiguities_EmptySlice(t *testing.T) {
	out := review.RenderAmbiguities([]review.AmbiguityWarning{})
	if !strings.Contains(strings.ToLower(out), "none") {
		t.Errorf("expected '(none)' for empty non-nil slice, got: %q", out)
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
