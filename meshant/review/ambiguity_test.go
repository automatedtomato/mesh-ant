package review_test

import (
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// TestDetectAmbiguities_EmptyDraft verifies that a zero-value TraceDraft
// produces exactly 6 warnings — one per candidate content field — since all
// fields are blank and IntentionallyBlank is empty (no suppression).
func TestDetectAmbiguities_EmptyDraft(t *testing.T) {
	d := schema.TraceDraft{}
	warnings := review.DetectAmbiguities(d)
	// 6 candidate fields: what_changed, source, target, mediation, observer, tags.
	// No IntentionallyBlank suppression, no UncertaintyNote → no criterion_ref mismatch.
	if len(warnings) != 6 {
		t.Errorf("expected 6 warnings for zero-value draft, got %d: %v", len(warnings), warnings)
	}
}

// TestDetectAmbiguities_AllFieldsPopulated verifies that a draft with all six
// candidate fields populated returns an empty (nil or zero-length) slice.
func TestDetectAmbiguities_AllFieldsPopulated(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "policy shifted from opt-in to opt-out",
		Source:      []string{"regulators"},
		Target:      []string{"platform-users"},
		Mediation:   "GDPR compliance layer",
		Observer:    "privacy-analyst",
		Tags:        []string{"policy", "consent"},
	}
	warnings := review.DetectAmbiguities(d)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for fully-populated draft, got %d: %v", len(warnings), warnings)
	}
}

// TestDetectAmbiguities_WhatChangedBlank verifies that when WhatChanged is
// empty and IntentionallyBlank does not include "what_changed", a warning is
// returned for the "what_changed" field.
func TestDetectAmbiguities_WhatChangedBlank(t *testing.T) {
	d := schema.TraceDraft{
		// WhatChanged intentionally left blank
		Source:    []string{"actor-a"},
		Target:    []string{"actor-b"},
		Mediation: "some mediator",
		Observer:  "analyst",
		Tags:      []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "what_changed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'what_changed', got: %v", warnings)
	}
}

// TestDetectAmbiguities_SourceEmpty verifies that when Source is nil (empty
// slice) and IntentionallyBlank does not include "source", a warning is
// returned for the "source" field.
func TestDetectAmbiguities_SourceEmpty(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "something shifted",
		// Source intentionally left nil
		Target:    []string{"actor-b"},
		Mediation: "some mediator",
		Observer:  "analyst",
		Tags:      []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "source" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'source', got: %v", warnings)
	}
}

// TestDetectAmbiguities_UncertaintyNoteWithNoCriterionRef verifies that when
// UncertaintyNote is non-empty but CriterionRef is empty, a warning is
// returned for "criterion_ref mismatch".
func TestDetectAmbiguities_UncertaintyNoteWithNoCriterionRef(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged:     "alignment unclear",
		Source:          []string{"actor-a"},
		Target:          []string{"actor-b"},
		Mediation:       "some mediator",
		Observer:        "analyst",
		Tags:            []string{"tag1"},
		UncertaintyNote: "the direction of mediation is unclear from the span",
		// CriterionRef deliberately empty
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "criterion_ref mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for 'criterion_ref mismatch', got: %v", warnings)
	}
}

// TestDetectAmbiguities_IntentionallyBlankSuppressesWarnings verifies that
// when IntentionallyBlank lists content fields, those fields do not produce
// ambiguity warnings even when their values are empty.
func TestDetectAmbiguities_IntentionallyBlankSuppressesWarnings(t *testing.T) {
	// All six candidate fields are blank, but all six are listed in
	// IntentionallyBlank — no content warnings should be returned.
	d := schema.TraceDraft{
		IntentionallyBlank: []string{
			"what_changed",
			"source",
			"target",
			"mediation",
			"observer",
			"tags",
		},
	}
	warnings := review.DetectAmbiguities(d)
	for _, w := range warnings {
		switch w.Field {
		case "what_changed", "source", "target", "mediation", "observer", "tags":
			t.Errorf("field %q should be suppressed by IntentionallyBlank, but warning was returned: %v", w.Field, w)
		}
	}
}

// TestDetectAmbiguities_LanguageDiscipline verifies that no warning message
// contains "missing", "error", or "incomplete" — ANT language discipline
// requires invitational, non-deficit language.
func TestDetectAmbiguities_LanguageDiscipline(t *testing.T) {
	// Trigger as many warnings as possible: all candidate fields blank, no
	// IntentionallyBlank entries, and an UncertaintyNote with no CriterionRef.
	d := schema.TraceDraft{
		UncertaintyNote: "everything is uncertain",
	}
	warnings := review.DetectAmbiguities(d)
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning to test language discipline, got none")
	}
	forbiddenTerms := []string{"missing", "error", "incomplete"}
	for _, w := range warnings {
		lower := strings.ToLower(w.Message)
		for _, term := range forbiddenTerms {
			if strings.Contains(lower, term) {
				t.Errorf("warning for field %q contains forbidden term %q: %q", w.Field, term, w.Message)
			}
		}
	}
}

// TestDetectAmbiguities_TargetEmpty verifies that when Target is nil and
// IntentionallyBlank does not include "target", a warning is returned.
func TestDetectAmbiguities_TargetEmpty(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "something shifted",
		Source:      []string{"actor-a"},
		Mediation:   "some mediator",
		Observer:    "analyst",
		Tags:        []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "target" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'target', got: %v", warnings)
	}
}

// TestDetectAmbiguities_MediationEmpty verifies that when Mediation is empty
// and IntentionallyBlank does not include "mediation", a warning is returned.
func TestDetectAmbiguities_MediationEmpty(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "something shifted",
		Source:      []string{"actor-a"},
		Target:      []string{"actor-b"},
		Observer:    "analyst",
		Tags:        []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "mediation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'mediation', got: %v", warnings)
	}
}

// TestDetectAmbiguities_ObserverEmpty verifies that when Observer is empty
// and IntentionallyBlank does not include "observer", a warning is returned.
func TestDetectAmbiguities_ObserverEmpty(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "something shifted",
		Source:      []string{"actor-a"},
		Target:      []string{"actor-b"},
		Mediation:   "some mediator",
		Tags:        []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "observer" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'observer', got: %v", warnings)
	}
}

// TestDetectAmbiguities_TagsEmpty verifies that when Tags is nil and
// IntentionallyBlank does not include "tags", a warning is returned.
func TestDetectAmbiguities_TagsEmpty(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged: "something shifted",
		Source:      []string{"actor-a"},
		Target:      []string{"actor-b"},
		Mediation:   "some mediator",
		Observer:    "analyst",
	}
	warnings := review.DetectAmbiguities(d)
	found := false
	for _, w := range warnings {
		if w.Field == "tags" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for field 'tags', got: %v", warnings)
	}
}

// TestDetectAmbiguities_PartialIntentionallyBlank verifies that IntentionallyBlank
// suppresses exactly the listed field and does not suppress adjacent fields.
// Suppressing "source" only should still produce warnings for the remaining 5 fields.
func TestDetectAmbiguities_PartialIntentionallyBlank(t *testing.T) {
	// All 6 candidate fields are blank; only "source" is intentionally blank.
	// Expected: 5 warnings (all fields except "source").
	d := schema.TraceDraft{
		IntentionallyBlank: []string{"source"},
	}
	warnings := review.DetectAmbiguities(d)
	for _, w := range warnings {
		if w.Field == "source" {
			t.Errorf("field 'source' is in IntentionallyBlank but produced a warning: %v", w)
		}
	}
	if len(warnings) != 5 {
		t.Errorf("expected 5 warnings (all except 'source'), got %d: %v", len(warnings), warnings)
	}
}

// TestDetectAmbiguities_CriterionRefPresent verifies that when both
// UncertaintyNote and CriterionRef are set, no criterion_ref mismatch warning
// is returned.
func TestDetectAmbiguities_CriterionRefPresent(t *testing.T) {
	d := schema.TraceDraft{
		WhatChanged:     "alignment unclear",
		Source:          []string{"actor-a"},
		Target:          []string{"actor-b"},
		Mediation:       "some mediator",
		Observer:        "analyst",
		Tags:            []string{"tag1"},
		UncertaintyNote: "the direction of mediation is unclear",
		CriterionRef:    "c-001", // present — no mismatch
	}
	warnings := review.DetectAmbiguities(d)
	for _, w := range warnings {
		if w.Field == "criterion_ref mismatch" {
			t.Errorf("expected no criterion_ref mismatch warning when CriterionRef is set, got: %v", w)
		}
	}
}

// TestDetectAmbiguities_MultipleAmbiguities verifies that a draft with both
// WhatChanged=="" and Source==nil returns at least 2 warnings.
func TestDetectAmbiguities_MultipleAmbiguities(t *testing.T) {
	d := schema.TraceDraft{
		// WhatChanged blank and Source nil — two distinct ambiguities
		Target:    []string{"actor-b"},
		Mediation: "some mediator",
		Observer:  "analyst",
		Tags:      []string{"tag1"},
	}
	warnings := review.DetectAmbiguities(d)
	foundWhatChanged := false
	foundSource := false
	for _, w := range warnings {
		if w.Field == "what_changed" {
			foundWhatChanged = true
		}
		if w.Field == "source" {
			foundSource = true
		}
	}
	if !foundWhatChanged || !foundSource {
		t.Errorf("expected warnings for both 'what_changed' and 'source', got: %v", warnings)
	}
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings, got %d: %v", len(warnings), warnings)
	}
}
