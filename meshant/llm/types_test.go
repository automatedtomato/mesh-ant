// types_test.go tests the type definitions in types.go, focusing on JSON
// serialization round-trips and backward-compatibility guarantees.
//
// The bifurcation of ExtractionConditions and CritiqueConditions (issue #151)
// introduces a new type-level separation between extract/assist/split sessions
// (which carry ExtractionConditions) and critique sessions (which carry
// CritiqueConditions). These tests verify the JSON representation is correct
// and that legacy session files (written before the bifurcation) decode cleanly.
package llm_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
)

// --- Group: CritiqueConditions ---

// TestCritiqueConditions_JSONRoundTrip verifies that CritiqueConditions serialises
// and deserialises correctly. Specifically:
//   - source_doc_ref (singular) is present in the JSON output.
//   - source_doc_refs (plural, from ExtractionConditions) is absent.
//   - adapter_name (from ExtractionConditions) is absent.
func TestCritiqueConditions_JSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	orig := llm.CritiqueConditions{
		ModelID:            "claude-sonnet-4-6",
		PromptTemplate:     "prompts/critique.md",
		CriterionRef:       "criteria/ant.md",
		SystemInstructions: "Produce one critique draft.",
		SourceDocRef:       "data/coastal-notes.md",
		Timestamp:          ts,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	jsonStr := string(data)

	// source_doc_ref (singular) must appear.
	if !strings.Contains(jsonStr, `"source_doc_ref"`) {
		t.Errorf("JSON output: want key %q, got: %s", "source_doc_ref", jsonStr)
	}

	// source_doc_refs (plural) must be absent — CritiqueConditions has only singular.
	if strings.Contains(jsonStr, `"source_doc_refs"`) {
		t.Errorf("JSON output: must not contain key %q (plural), got: %s", "source_doc_refs", jsonStr)
	}

	// adapter_name must be absent — no format conversion precedes critique.
	if strings.Contains(jsonStr, `"adapter_name"`) {
		t.Errorf("JSON output: must not contain key %q, got: %s", "adapter_name", jsonStr)
	}

	// Round-trip: decode and compare field values.
	var decoded llm.CritiqueConditions
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.ModelID != orig.ModelID {
		t.Errorf("ModelID: want %q, got %q", orig.ModelID, decoded.ModelID)
	}
	if decoded.PromptTemplate != orig.PromptTemplate {
		t.Errorf("PromptTemplate: want %q, got %q", orig.PromptTemplate, decoded.PromptTemplate)
	}
	if decoded.CriterionRef != orig.CriterionRef {
		t.Errorf("CriterionRef: want %q, got %q", orig.CriterionRef, decoded.CriterionRef)
	}
	if decoded.SystemInstructions != orig.SystemInstructions {
		t.Errorf("SystemInstructions: want %q, got %q", orig.SystemInstructions, decoded.SystemInstructions)
	}
	if decoded.SourceDocRef != orig.SourceDocRef {
		t.Errorf("SourceDocRef: want %q, got %q", orig.SourceDocRef, decoded.SourceDocRef)
	}
	if !decoded.Timestamp.Equal(orig.Timestamp) {
		t.Errorf("Timestamp: want %v, got %v", orig.Timestamp, decoded.Timestamp)
	}
}

// TestCritiqueConditions_OmitemptyFields verifies that CriterionRef and
// SourceDocRef are omitted when empty (omitempty semantics).
func TestCritiqueConditions_OmitemptyFields(t *testing.T) {
	cc := llm.CritiqueConditions{
		ModelID:            "claude-sonnet-4-6",
		SystemInstructions: "Produce one critique draft.",
		Timestamp:          time.Now(),
	}

	data, err := json.Marshal(cc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	jsonStr := string(data)

	// Empty CriterionRef must be omitted.
	if strings.Contains(jsonStr, `"criterion_ref"`) {
		t.Errorf("JSON output: empty CriterionRef must be omitted, got: %s", jsonStr)
	}

	// Empty SourceDocRef must be omitted.
	if strings.Contains(jsonStr, `"source_doc_ref"`) {
		t.Errorf("JSON output: empty SourceDocRef must be omitted, got: %s", jsonStr)
	}
}

// --- Group: SessionRecord.CritiqueConditions field ---

// TestSessionRecord_CritiqueConditionsOmittedForExtract verifies that a
// SessionRecord with Command="extract" and nil CritiqueConditions serialises
// without a "critique_conditions" key in the JSON output.
func TestSessionRecord_CritiqueConditionsOmittedForExtract(t *testing.T) {
	rec := llm.SessionRecord{
		ID:      "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Command: "extract",
		Conditions: llm.ExtractionConditions{
			ModelID:   "claude-sonnet-4-6",
			Timestamp: time.Now(),
		},
		Timestamp: time.Now(),
		// CritiqueConditions is nil (zero value for pointer)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	jsonStr := string(data)

	// critique_conditions must not appear for non-critique sessions.
	if strings.Contains(jsonStr, `"critique_conditions"`) {
		t.Errorf("JSON output: must not contain key %q for extract session, got: %s", "critique_conditions", jsonStr)
	}
}

// TestSessionRecord_CritiqueConditionsPresentForCritique verifies that a
// SessionRecord with Command="critique" and non-nil CritiqueConditions
// serialises with a "critique_conditions" key in the JSON output.
func TestSessionRecord_CritiqueConditionsPresentForCritique(t *testing.T) {
	rec := llm.SessionRecord{
		ID:      "c3d4e5f6-a7b8-9012-cdef-012345678912",
		Command: "critique",
		CritiqueConditions: &llm.CritiqueConditions{
			ModelID:            "claude-sonnet-4-6",
			SystemInstructions: "Produce one critique draft.",
			SourceDocRef:       "data/coastal-notes.md",
			Timestamp:          time.Now(),
		},
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	jsonStr := string(data)

	// critique_conditions must appear for critique sessions.
	if !strings.Contains(jsonStr, `"critique_conditions"`) {
		t.Errorf("JSON output: want key %q for critique session, got: %s", "critique_conditions", jsonStr)
	}

	// The source_doc_ref inside critique_conditions must also be present.
	if !strings.Contains(jsonStr, `"source_doc_ref"`) {
		t.Errorf("JSON output: want %q inside critique_conditions, got: %s", "source_doc_ref", jsonStr)
	}
}

// TestSessionRecord_LegacyCritiqueDecodes verifies backward compatibility:
// a legacy critique session JSON file (with "conditions" but no
// "critique_conditions") decodes correctly. CritiqueConditions must be nil
// and Conditions.ModelID must be populated.
func TestSessionRecord_LegacyCritiqueDecodes(t *testing.T) {
	// This is what a critique session file looked like before issue #151.
	// It has "conditions" populated and no "critique_conditions" key.
	legacyJSON := `{
		"id": "c3d4e5f6-a7b8-9012-cdef-012345678912",
		"command": "critique",
		"conditions": {
			"model_id": "claude-sonnet-4-6",
			"prompt_template": "prompts/critique.md",
			"system_instructions": "Produce one critique draft.",
			"source_doc_ref": "data/coastal-notes.md",
			"timestamp": "2026-03-24T12:00:00Z"
		},
		"draft_ids": ["d1", "d2"],
		"draft_count": 2,
		"timestamp": "2026-03-24T12:00:00Z"
	}`

	var rec llm.SessionRecord
	if err := json.Unmarshal([]byte(legacyJSON), &rec); err != nil {
		t.Fatalf("json.Unmarshal legacy critique session: %v", err)
	}

	// CritiqueConditions must be nil — it was not in the JSON.
	if rec.CritiqueConditions != nil {
		t.Error("CritiqueConditions: want nil for legacy critique session, got non-nil")
	}

	// Conditions.ModelID must be populated from the legacy "conditions" field.
	if rec.Conditions.ModelID != "claude-sonnet-4-6" {
		t.Errorf("Conditions.ModelID: want %q, got %q", "claude-sonnet-4-6", rec.Conditions.ModelID)
	}

	// Command must be preserved.
	if rec.Command != "critique" {
		t.Errorf("Command: want %q, got %q", "critique", rec.Command)
	}

	// DraftCount must be preserved.
	if rec.DraftCount != 2 {
		t.Errorf("DraftCount: want 2, got %d", rec.DraftCount)
	}
}
