// llm_extraction_example_test.go validates the data integrity of the
// data/examples/llm_assisted_extraction/ example directory.
//
// These tests verify provenance chain integrity across the full v2.0.0 pipeline:
//
//	source_document.md → meshant extract → raw_drafts.json
//	→ meshant assist (human review) → reviewed_drafts.json
//	→ meshant promote → promoted_traces.json
//	→ meshant articulate → articulation_output.json
//
// The tests deliberately avoid importing meshant/llm to prevent a test-binary
// import cycle (llm imports loader; loader_test importing llm would cycle).
// SessionRecord fields are decoded via a minimal local struct.
package loader_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

const llmExampleDir = "../../data/examples/llm_assisted_extraction/"

// sessionRecordStub mirrors the fields of llm.SessionRecord used for
// validation. Kept local to avoid a test-binary import cycle.
type sessionRecordStub struct {
	ID         string    `json:"id"`
	Command    string    `json:"command"`
	DraftIDs   []string  `json:"draft_ids"`
	DraftCount int       `json:"draft_count"`
	Timestamp  time.Time `json:"timestamp"`
}

// loadSessionRecord decodes extraction_session.json into a sessionRecordStub.
func loadSessionRecord(t *testing.T) sessionRecordStub {
	t.Helper()
	f, err := os.Open(llmExampleDir + "extraction_session.json")
	if err != nil {
		t.Fatalf("open extraction_session.json: %v", err)
	}
	defer f.Close()

	var rec sessionRecordStub
	if err := json.NewDecoder(f).Decode(&rec); err != nil {
		t.Fatalf("decode extraction_session.json: %v", err)
	}
	return rec
}

// TestLLMExtractionExample_SessionRecord checks that extraction_session.json
// is well-formed and internally consistent.
func TestLLMExtractionExample_SessionRecord(t *testing.T) {
	rec := loadSessionRecord(t)

	if rec.ID == "" {
		t.Error("extraction_session: id must not be empty")
	}
	if rec.Command != "extract" {
		t.Errorf("extraction_session: command: want %q, got %q", "extract", rec.Command)
	}
	if rec.DraftCount != 7 {
		t.Errorf("extraction_session: draft_count: want 7, got %d", rec.DraftCount)
	}
	if len(rec.DraftIDs) != 7 {
		t.Errorf("extraction_session: draft_ids length: want 7, got %d", len(rec.DraftIDs))
	}
	for i, id := range rec.DraftIDs {
		if id == "" {
			t.Errorf("extraction_session: draft_ids[%d] is empty", i)
		}
	}
	if rec.Timestamp.IsZero() {
		t.Error("extraction_session: timestamp must not be zero")
	}
}

// TestLLMExtractionExample_RawDrafts checks that raw_drafts.json is a valid
// TraceDraft array with correct LLM-extraction provenance on every draft.
func TestLLMExtractionExample_RawDrafts(t *testing.T) {
	rec := loadSessionRecord(t)

	drafts, err := loader.LoadDrafts(llmExampleDir + "raw_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts raw_drafts.json: %v", err)
	}

	if len(drafts) != 7 {
		t.Fatalf("raw_drafts: want 7 drafts, got %d", len(drafts))
	}

	// Build a set of session-record draft IDs for cross-reference.
	sessionDraftSet := make(map[string]bool, len(rec.DraftIDs))
	for _, id := range rec.DraftIDs {
		sessionDraftSet[id] = true
	}

	for i, d := range drafts {
		if d.ID == "" {
			t.Errorf("raw_drafts[%d]: id must not be empty", i)
		}
		if d.SourceSpan == "" {
			t.Errorf("raw_drafts[%d] (%s): source_span must not be empty", i, d.ID)
		}
		if d.ExtractionStage != "weak-draft" {
			t.Errorf("raw_drafts[%d] (%s): extraction_stage: want %q, got %q",
				i, d.ID, "weak-draft", d.ExtractionStage)
		}
		if d.ExtractedBy != "claude-sonnet-4-6" {
			t.Errorf("raw_drafts[%d] (%s): extracted_by: want %q, got %q",
				i, d.ID, "claude-sonnet-4-6", d.ExtractedBy)
		}
		if d.SessionRef == "" {
			t.Errorf("raw_drafts[%d] (%s): session_ref must not be empty", i, d.ID)
		}
		if d.SessionRef != rec.ID {
			t.Errorf("raw_drafts[%d] (%s): session_ref %q does not match extraction_session.id %q",
				i, d.ID, d.SessionRef, rec.ID)
		}
		if d.UncertaintyNote == "" {
			t.Errorf("raw_drafts[%d] (%s): uncertainty_note must not be empty (framework appends note to all LLM drafts)",
				i, d.ID)
		}
		if !sessionDraftSet[d.ID] {
			t.Errorf("raw_drafts[%d] (%s): id not found in extraction_session.draft_ids", i, d.ID)
		}
	}
}

// TestLLMExtractionExample_ReviewedDrafts checks that reviewed_drafts.json
// is a valid TraceDraft array and that derived (human-edited) drafts have
// DerivedFrom pointing to a draft in raw_drafts.json.
func TestLLMExtractionExample_ReviewedDrafts(t *testing.T) {
	rawDrafts, err := loader.LoadDrafts(llmExampleDir + "raw_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts raw_drafts.json: %v", err)
	}

	reviewed, err := loader.LoadDrafts(llmExampleDir + "reviewed_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts reviewed_drafts.json: %v", err)
	}

	// Build index of raw draft IDs.
	rawIDs := make(map[string]bool, len(rawDrafts))
	for _, d := range rawDrafts {
		rawIDs[d.ID] = true
	}

	// reviewed_drafts.json contains both carried-forward LLM drafts (accepted/skipped)
	// and human-derived drafts (edited). Count each kind.
	humanDraftCount := 0
	for i, d := range reviewed {
		if d.ID == "" {
			t.Errorf("reviewed_drafts[%d]: id must not be empty", i)
		}
		if d.SourceSpan == "" {
			t.Errorf("reviewed_drafts[%d] (%s): source_span must not be empty", i, d.ID)
		}
		// Human-derived drafts must have DerivedFrom pointing to a raw draft.
		if d.ExtractionStage == "reviewed" {
			humanDraftCount++
			if d.DerivedFrom == "" {
				t.Errorf("reviewed_drafts[%d] (%s): reviewed-stage draft must have derived_from set", i, d.ID)
			}
			if !rawIDs[d.DerivedFrom] {
				t.Errorf("reviewed_drafts[%d] (%s): derived_from %q not found in raw_drafts.json",
					i, d.ID, d.DerivedFrom)
			}
			if d.ExtractedBy == "" {
				t.Errorf("reviewed_drafts[%d] (%s): reviewed-stage draft must have extracted_by set", i, d.ID)
			}
		}
	}

	// The example includes at least 2 human-edited spans (divergences B and C).
	if humanDraftCount < 2 {
		t.Errorf("reviewed_drafts: want at least 2 reviewed-stage drafts (divergences B and C), got %d", humanDraftCount)
	}
}

// TestLLMExtractionExample_PromotedTraces checks that promoted_traces.json
// is a valid Trace array where every trace carries the TagValueDraft provenance tag.
func TestLLMExtractionExample_PromotedTraces(t *testing.T) {
	traces, err := loader.Load(llmExampleDir + "promoted_traces.json")
	if err != nil {
		t.Fatalf("Load promoted_traces.json: %v", err)
	}

	if len(traces) == 0 {
		t.Fatal("promoted_traces: want at least 1 trace, got 0")
	}

	for i, tr := range traces {
		if err := tr.Validate(); err != nil {
			t.Errorf("promoted_traces[%d] (%s): Validate failed: %v", i, tr.ID, err)
		}

		hasDraftTag := false
		for _, tag := range tr.Tags {
			if schema.TagValue(tag) == schema.TagValueDraft {
				hasDraftTag = true
				break
			}
		}
		if !hasDraftTag {
			t.Errorf("promoted_traces[%d] (%s): missing %q provenance tag", i, tr.ID, schema.TagValueDraft)
		}
	}
}

// TestLLMExtractionExample_ArticulationOutput checks that articulation_output.json
// is valid JSON containing the expected top-level fields for an articulated graph.
func TestLLMExtractionExample_ArticulationOutput(t *testing.T) {
	f, err := os.Open(llmExampleDir + "articulation_output.json")
	if err != nil {
		t.Fatalf("open articulation_output.json: %v", err)
	}
	defer f.Close()

	var articulationJSON map[string]json.RawMessage
	if err := json.NewDecoder(f).Decode(&articulationJSON); err != nil {
		t.Fatalf("decode articulation_output.json: %v", err)
	}

	for _, field := range []string{"nodes", "edges"} {
		if _, ok := articulationJSON[field]; !ok {
			t.Errorf("articulation_output.json: missing required field %q", field)
		}
	}
}

// TestLLMExtractionExample_ProvenanceChainIntegrity verifies that the provenance
// chain is coherent end-to-end: session → raw drafts → reviewed drafts → promoted traces.
func TestLLMExtractionExample_ProvenanceChainIntegrity(t *testing.T) {
	rec := loadSessionRecord(t)

	rawDrafts, err := loader.LoadDrafts(llmExampleDir + "raw_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts raw_drafts.json: %v", err)
	}

	reviewed, err := loader.LoadDrafts(llmExampleDir + "reviewed_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts reviewed_drafts.json: %v", err)
	}

	traces, err := loader.Load(llmExampleDir + "promoted_traces.json")
	if err != nil {
		t.Fatalf("Load promoted_traces.json: %v", err)
	}

	// Every raw draft's session_ref must equal the extraction session ID.
	for i, d := range rawDrafts {
		if d.SessionRef != rec.ID {
			t.Errorf("chain integrity: raw_drafts[%d] (%s) session_ref %q != session.id %q",
				i, d.ID, d.SessionRef, rec.ID)
		}
	}

	// Build reviewed draft ID set.
	reviewedIDs := make(map[string]bool, len(reviewed))
	for _, d := range reviewed {
		reviewedIDs[d.ID] = true
	}

	// Every promoted trace ID must correspond to a draft in reviewed_drafts.json.
	for i, tr := range traces {
		if !reviewedIDs[tr.ID] {
			t.Errorf("chain integrity: promoted_traces[%d] (%s) ID not found in reviewed_drafts.json", i, tr.ID)
		}
	}
}
