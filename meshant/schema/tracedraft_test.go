package schema_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// validDraft returns a TraceDraft that passes Validate() and IsPromotable().
func validDraft() schema.TraceDraft {
	return schema.TraceDraft{
		ID:              "d0000000-0000-4000-8000-000000000001",
		Timestamp:       time.Date(2026, 6, 2, 18, 0, 0, 0, time.UTC),
		SourceSpan:      "The routing matrix redirected the form to a secondary queue.",
		WhatChanged:     "routing-matrix redirected form-submission to secondary-queue",
		Source:          []string{"form-submission"},
		Target:          []string{"secondary-queue"},
		Mediation:       "secondary-queue routing rule",
		Observer:        "queue-monitor",
		Tags:            []string{"translation"},
		UncertaintyNote: "",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-pass1",
	}
}

// --- Validate ---

func TestTraceDraft_Validate_ZeroValue(t *testing.T) {
	var d schema.TraceDraft
	if err := d.Validate(); err == nil {
		t.Fatal("zero value TraceDraft: expected error from Validate(), got nil")
	}
}

func TestTraceDraft_Validate_EmptySourceSpan(t *testing.T) {
	d := validDraft()
	d.SourceSpan = ""
	if err := d.Validate(); err == nil {
		t.Fatal("empty SourceSpan: expected error, got nil")
	}
}

func TestTraceDraft_Validate_SourceSpanOnly(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "The form was redirected."}
	if err := d.Validate(); err != nil {
		t.Fatalf("SourceSpan-only draft: expected nil error, got %v", err)
	}
}

func TestTraceDraft_Validate_FullFields(t *testing.T) {
	d := validDraft()
	if err := d.Validate(); err != nil {
		t.Fatalf("full-field draft: expected nil error, got %v", err)
	}
}

// --- IsPromotable ---

func TestTraceDraft_IsPromotable_MissingWhatChanged(t *testing.T) {
	d := validDraft()
	d.WhatChanged = ""
	if d.IsPromotable() {
		t.Fatal("missing WhatChanged: expected IsPromotable() = false, got true")
	}
}

func TestTraceDraft_IsPromotable_MissingObserver(t *testing.T) {
	d := validDraft()
	d.Observer = ""
	if d.IsPromotable() {
		t.Fatal("missing Observer: expected IsPromotable() = false, got true")
	}
}

func TestTraceDraft_IsPromotable_MissingID(t *testing.T) {
	d := validDraft()
	d.ID = ""
	if d.IsPromotable() {
		t.Fatal("missing ID: expected IsPromotable() = false, got true")
	}
}

func TestTraceDraft_IsPromotable_InvalidID(t *testing.T) {
	d := validDraft()
	d.ID = "not-a-uuid"
	if d.IsPromotable() {
		t.Fatal("invalid UUID ID: expected IsPromotable() = false, got true")
	}
}

func TestTraceDraft_IsPromotable_AllRequiredPresent(t *testing.T) {
	d := validDraft()
	if !d.IsPromotable() {
		t.Fatal("fully populated draft: expected IsPromotable() = true, got false")
	}
}

// IsPromotable does not require SourceSpan — a draft with SourceSpan empty
// but all promotability fields present should still be promotable.
func TestTraceDraft_IsPromotable_EmptySourceSpanDoesNotBlock(t *testing.T) {
	d := validDraft()
	d.SourceSpan = ""
	if !d.IsPromotable() {
		t.Fatal("empty SourceSpan should not block IsPromotable(); expected true, got false")
	}
}

// --- Promote ---

func TestTraceDraft_Promote_Success(t *testing.T) {
	d := validDraft()
	tr, err := d.Promote()
	if err != nil {
		t.Fatalf("Promote() on valid draft: unexpected error: %v", err)
	}
	// Promoted trace must pass Trace.Validate().
	if err := tr.Validate(); err != nil {
		t.Fatalf("promoted Trace fails Validate(): %v", err)
	}
	// Must carry TagValueDraft.
	hasDraftTag := false
	for _, tag := range tr.Tags {
		if tag == string(schema.TagValueDraft) {
			hasDraftTag = true
			break
		}
	}
	if !hasDraftTag {
		t.Fatalf("promoted Trace missing %q tag; tags = %v", schema.TagValueDraft, tr.Tags)
	}
	// Core fields must be transferred faithfully.
	if tr.ID != d.ID {
		t.Errorf("ID mismatch: got %q want %q", tr.ID, d.ID)
	}
	if tr.WhatChanged != d.WhatChanged {
		t.Errorf("WhatChanged mismatch: got %q want %q", tr.WhatChanged, d.WhatChanged)
	}
	if tr.Observer != d.Observer {
		t.Errorf("Observer mismatch: got %q want %q", tr.Observer, d.Observer)
	}
}

func TestTraceDraft_Promote_PreservesExistingTags(t *testing.T) {
	d := validDraft()
	d.Tags = []string{"translation", "threshold"}
	tr, err := d.Promote()
	if err != nil {
		t.Fatalf("Promote(): unexpected error: %v", err)
	}
	// Both original tags and TagValueDraft must be present.
	tagSet := make(map[string]bool)
	for _, tag := range tr.Tags {
		tagSet[tag] = true
	}
	for _, want := range []string{"translation", "threshold", string(schema.TagValueDraft)} {
		if !tagSet[want] {
			t.Errorf("promoted Trace missing tag %q; tags = %v", want, tr.Tags)
		}
	}
}

func TestTraceDraft_Promote_NotPromotable(t *testing.T) {
	d := validDraft()
	d.WhatChanged = ""
	_, err := d.Promote()
	if err == nil {
		t.Fatal("Promote() on non-promotable draft: expected error, got nil")
	}
}

func TestTraceDraft_Promote_ErrorMessageDescriptive(t *testing.T) {
	d := validDraft()
	d.WhatChanged = ""
	d.Observer = ""
	_, err := d.Promote()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	// Error should name the missing fields so callers can report them.
	if !strings.Contains(msg, "what_changed") {
		t.Errorf("error message should mention what_changed; got: %q", msg)
	}
	if !strings.Contains(msg, "observer") {
		t.Errorf("error message should mention observer; got: %q", msg)
	}
}

// --- DerivedFrom chain ---

// TestTraceDraftChain verifies that two drafts can be linked via DerivedFrom,
// forming a structurally followable revision chain.
func TestTraceDraftChain(t *testing.T) {
	parent := schema.TraceDraft{
		ID:              "d0000000-0000-4000-8000-000000000001",
		SourceSpan:      "Raw span extracted by LLM.",
		ExtractionStage: "weak-draft",
		ExtractedBy:     "llm-pass1",
	}
	if err := parent.Validate(); err != nil {
		t.Fatalf("parent Validate(): %v", err)
	}

	child := schema.TraceDraft{
		ID:              "d0000000-0000-4000-8000-000000000002",
		SourceSpan:      "Raw span extracted by LLM.",
		WhatChanged:     "reviewed draft with corrected observer",
		Observer:        "human-analyst",
		ExtractionStage: "reviewed",
		ExtractedBy:     "reviewer",
		DerivedFrom:     parent.ID, // links revision to parent
	}
	if err := child.Validate(); err != nil {
		t.Fatalf("child Validate(): %v", err)
	}
	if child.DerivedFrom != parent.ID {
		t.Errorf("DerivedFrom: got %q want %q", child.DerivedFrom, parent.ID)
	}
}

// --- IntentionallyBlank ---

// TestTraceDraft_IntentionallyBlank_RoundTrip verifies that IntentionallyBlank
// is preserved through struct construction and that Validate succeeds regardless
// of whether the field is set.
func TestTraceDraft_IntentionallyBlank_RoundTrip(t *testing.T) {
	d := schema.TraceDraft{
		SourceSpan:         "Raw span text.",
		ExtractionStage:    "reviewed",
		DerivedFrom:        "d0000000-0000-4000-8000-000000000001",
		IntentionallyBlank: []string{"what_changed", "source", "target", "mediation", "observer", "tags"},
	}

	if err := d.Validate(); err != nil {
		t.Fatalf("Validate() with IntentionallyBlank set: unexpected error: %v", err)
	}

	if len(d.IntentionallyBlank) != 6 {
		t.Errorf("IntentionallyBlank length: got %d want 6", len(d.IntentionallyBlank))
	}
	for _, field := range []string{"what_changed", "source", "target", "mediation", "observer", "tags"} {
		found := false
		for _, b := range d.IntentionallyBlank {
			if b == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("IntentionallyBlank missing %q", field)
		}
	}
}

// TestTraceDraft_IntentionallyBlank_ValidateStillRequiresSourceSpan verifies
// that Validate still requires source_span even when IntentionallyBlank is set —
// IntentionallyBlank does not relax the minimum invariant.
func TestTraceDraft_IntentionallyBlank_ValidateStillRequiresSourceSpan(t *testing.T) {
	d := schema.TraceDraft{
		IntentionallyBlank: []string{"what_changed", "source"},
		// SourceSpan deliberately absent.
	}

	if err := d.Validate(); err == nil {
		t.Fatal("Validate() with empty SourceSpan: want error, got nil")
	}
}

// TestTraceDraft_IntentionallyBlank_EmptyByDefault verifies that a TraceDraft
// created without IntentionallyBlank has a nil/empty slice — not implicitly
// populated.
func TestTraceDraft_IntentionallyBlank_EmptyByDefault(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "some span"}
	if len(d.IntentionallyBlank) != 0 {
		t.Errorf("IntentionallyBlank: want empty by default, got %v", d.IntentionallyBlank)
	}
}

// --- CriterionRef ---

func TestTraceDraft_CriterionRef_EmptyByDefault(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "span"}
	if d.CriterionRef != "" {
		t.Errorf("CriterionRef: want empty by default, got %q", d.CriterionRef)
	}
}

func TestTraceDraft_CriterionRef_ValidateStillRequiresSourceSpan(t *testing.T) {
	d := schema.TraceDraft{CriterionRef: "my-criterion"}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate() with CriterionRef but no SourceSpan: want error, got nil")
	}
}

func TestTraceDraft_CriterionRef_DoesNotAffectIsPromotable(t *testing.T) {
	// A draft with CriterionRef but missing WhatChanged/Observer is not promotable.
	d := schema.TraceDraft{
		ID:           "a0000000-0000-4000-8000-000000000001",
		SourceSpan:   "span",
		CriterionRef: "my-criterion",
	}
	if d.IsPromotable() {
		t.Error("IsPromotable(): want false (WhatChanged and Observer missing), got true")
	}
}

func TestTraceDraft_CriterionRef_OmitEmptyInJSON(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "span"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), "criterion_ref") {
		t.Errorf("criterion_ref should be omitted when empty; JSON: %s", b)
	}
}

func TestTraceDraft_CriterionRef_PresentInJSON(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "span", CriterionRef: "actor-stability-v1"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"criterion_ref":"actor-stability-v1"`) {
		t.Errorf("criterion_ref missing from JSON: %s", b)
	}
}

// --- SessionRef ---

// TestTraceDraft_SessionRef_JSONRoundtrip verifies that SessionRef survives
// a marshal/unmarshal cycle intact.
func TestTraceDraft_SessionRef_JSONRoundtrip(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "span", SessionRef: "sess-abc"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var got schema.TraceDraft
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got.SessionRef != "sess-abc" {
		t.Errorf("SessionRef roundtrip: want %q, got %q", "sess-abc", got.SessionRef)
	}
}

// TestTraceDraft_SessionRef_OmitEmpty verifies that an empty SessionRef is
// omitted from JSON output (omitempty tag).
func TestTraceDraft_SessionRef_OmitEmpty(t *testing.T) {
	d := schema.TraceDraft{SourceSpan: "span"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), "session_ref") {
		t.Errorf("session_ref should be omitted when empty; JSON: %s", b)
	}
}

// TestTraceDraft_SessionRef_NotTransferredByPromote verifies that SessionRef
// is a draft-layer provenance field only and is NOT transferred to a
// canonical Trace by Promote(). SessionRef names the ingestion session, not
// the analytical content — it has no place in a canonical record.
func TestTraceDraft_SessionRef_NotTransferredByPromote(t *testing.T) {
	d := schema.TraceDraft{
		ID:          "a0000000-0000-4000-8000-000000000099",
		SourceSpan:  "span",
		WhatChanged: "something changed",
		Observer:    "analyst-1",
		SessionRef:  "sess-xyz",
	}
	tr, err := d.Promote()
	if err != nil {
		t.Fatalf("Promote(): unexpected error: %v", err)
	}
	// Trace has no SessionRef field — verify via JSON that the key is absent.
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("json.Marshal(Trace): %v", err)
	}
	if strings.Contains(string(b), "session_ref") {
		t.Errorf("session_ref must not appear in promoted Trace JSON; got: %s", b)
	}
}
