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
	"reflect"
	"sort"
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

// articulationStub is a minimal local struct for decoding articulation_output.json.
// The top-level id field is intentionally empty for articulations produced by
// meshant articulate — MeshGraph.ID is user-assigned and never auto-populated
// by the articulate command itself. Tests assert structural content, not the ID.
type articulationStub struct {
	Nodes map[string]json.RawMessage `json:"nodes"`
	Edges []json.RawMessage          `json:"edges"`
	Cut   struct {
		TracesIncluded int `json:"traces_included"`
		TracesTotal    int `json:"traces_total"`
	} `json:"cut"`
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

// TestLLMExtractionExample_ReviewedDrafts checks that reviewed_drafts.json is a
// valid TraceDraft array with the expected total count, correct provenance on
// human-derived drafts, and the analytical signals for each divergence.
func TestLLMExtractionExample_ReviewedDrafts(t *testing.T) {
	rawDrafts, err := loader.LoadDrafts(llmExampleDir + "raw_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts raw_drafts.json: %v", err)
	}

	reviewed, err := loader.LoadDrafts(llmExampleDir + "reviewed_drafts.json")
	if err != nil {
		t.Fatalf("LoadDrafts reviewed_drafts.json: %v", err)
	}

	// reviewed_drafts.json contains all 7 carried-forward LLM drafts plus 2
	// human-derived corrections (Divergences B and C).
	if len(reviewed) != 9 {
		t.Fatalf("reviewed_drafts: want 9 drafts (7 LLM + 2 human-derived), got %d", len(reviewed))
	}

	// Build index of raw draft IDs and a map for looking up reviewed drafts by ID.
	rawIDs := make(map[string]bool, len(rawDrafts))
	for _, d := range rawDrafts {
		rawIDs[d.ID] = true
	}
	reviewedByID := make(map[string]schema.TraceDraft, len(reviewed))
	for _, d := range reviewed {
		reviewedByID[d.ID] = d
	}

	// assistSessionRef is the session ID stamped on human-derived reviewed drafts
	// by the meshant assist session (separate from the extract session).
	const assistSessionRef = "f5000000-0000-4000-8000-000000000002"

	humanDraftCount := 0
	for i, d := range reviewed {
		if d.ID == "" {
			t.Errorf("reviewed_drafts[%d]: id must not be empty", i)
		}
		if d.SourceSpan == "" {
			t.Errorf("reviewed_drafts[%d] (%s): source_span must not be empty", i, d.ID)
		}
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
			// Reviewed drafts carry the assist session ref, not the extract session ref.
			if d.SessionRef != assistSessionRef {
				t.Errorf("reviewed_drafts[%d] (%s): reviewed-stage draft session_ref: want %q, got %q",
					i, d.ID, assistSessionRef, d.SessionRef)
			}
		}
	}

	// The example includes exactly 2 human-edited spans (Divergences B and C).
	if humanDraftCount != 2 {
		t.Errorf("reviewed_drafts: want exactly 2 reviewed-stage drafts (divergences B and C), got %d", humanDraftCount)
	}

	// --- Divergence A (span 2): LLM reading accepted without editing ---
	// The structural signal of Divergence A is the absence of a reviewed-stage
	// derived draft for span 2. If span 2 had been edited, a reviewed draft with
	// derived_from == span2LLMId would appear in reviewed_drafts.json.
	const span2LLMId = "f5000001-0000-4000-8000-000000000002"
	for _, d := range reviewed {
		if d.ExtractionStage == "reviewed" && d.DerivedFrom == span2LLMId {
			t.Errorf("Divergence A: span 2 (%s) should have been accepted without editing; "+
				"found a reviewed-stage derived draft %s pointing to it", span2LLMId, d.ID)
		}
	}
	// Span 2's LLM draft must be present in reviewed_drafts (accepted, carried through).
	if _, ok := reviewedByID[span2LLMId]; !ok {
		t.Errorf("Divergence A: span 2 LLM draft (%s) must appear in reviewed_drafts (accepted)", span2LLMId)
	}

	// --- Divergence B (span 5): LLM generalized mediation; reviewer corrected it ---
	const (
		span5LLMId    = "f5000001-0000-4000-8000-000000000005"
		span5HumanId  = "f5000002-0000-4000-8000-000000000005"
		llmMediation  = "majority-vote-protocol"
		goodMediation = "foundation-bylaws-article-7"
	)
	if d, ok := reviewedByID[span5LLMId]; ok {
		if d.Mediation != llmMediation {
			t.Errorf("Divergence B: LLM draft (%s) mediation: want %q (LLM reading), got %q",
				span5LLMId, llmMediation, d.Mediation)
		}
	} else {
		t.Errorf("Divergence B: LLM progenitor draft (%s) missing from reviewed_drafts", span5LLMId)
	}
	if d, ok := reviewedByID[span5HumanId]; ok {
		if d.Mediation != goodMediation {
			t.Errorf("Divergence B: human-derived draft (%s) mediation: want %q, got %q",
				span5HumanId, goodMediation, d.Mediation)
		}
		if d.DerivedFrom != span5LLMId {
			t.Errorf("Divergence B: human-derived draft (%s) derived_from: want %q, got %q",
				span5HumanId, span5LLMId, d.DerivedFrom)
		}
	} else {
		t.Errorf("Divergence B: human-derived draft (%s) missing from reviewed_drafts", span5HumanId)
	}

	// --- Divergence C (span 7): LLM read dissent as blockage; reviewer read it as translation ---
	const (
		span7LLMId   = "f5000001-0000-4000-8000-000000000007"
		span7HumanId = "f5000002-0000-4000-8000-000000000007"
	)
	if d, ok := reviewedByID[span7LLMId]; ok {
		wantTags := []string{"blockage"}
		if !reflect.DeepEqual(d.Tags, wantTags) {
			t.Errorf("Divergence C: LLM draft (%s) tags: want %v, got %v", span7LLMId, wantTags, d.Tags)
		}
	} else {
		t.Errorf("Divergence C: LLM progenitor draft (%s) missing from reviewed_drafts", span7LLMId)
	}
	if d, ok := reviewedByID[span7HumanId]; ok {
		wantTags := []string{"translation"}
		if !reflect.DeepEqual(d.Tags, wantTags) {
			t.Errorf("Divergence C: human-derived draft (%s) tags: want %v (translation, not blockage), got %v",
				span7HumanId, wantTags, d.Tags)
		}
		wantTarget := []string{"low-resource-contributors"}
		if !reflect.DeepEqual(d.Target, wantTarget) {
			t.Errorf("Divergence C: human-derived draft (%s) target: want %v, got %v",
				span7HumanId, wantTarget, d.Target)
		}
		if d.Observer != "dissenting-maintainers" {
			t.Errorf("Divergence C: human-derived draft (%s) observer: want %q, got %q",
				span7HumanId, "dissenting-maintainers", d.Observer)
		}
		if d.DerivedFrom != span7LLMId {
			t.Errorf("Divergence C: human-derived draft (%s) derived_from: want %q, got %q",
				span7HumanId, span7LLMId, d.DerivedFrom)
		}
	} else {
		t.Errorf("Divergence C: human-derived draft (%s) missing from reviewed_drafts", span7HumanId)
	}
}

// TestLLMExtractionExample_PromotedTraces checks that promoted_traces.json is a
// valid Trace array of the expected size where every trace carries TagValueDraft.
func TestLLMExtractionExample_PromotedTraces(t *testing.T) {
	traces, err := loader.Load(llmExampleDir + "promoted_traces.json")
	if err != nil {
		t.Fatalf("Load promoted_traces.json: %v", err)
	}

	// All 9 drafts in reviewed_drafts.json are promotable; all 9 are promoted.
	if len(traces) != 9 {
		t.Fatalf("promoted_traces: want 9 traces (all reviewedDrafts promoted), got %d", len(traces))
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
// is a well-formed articulated graph with the expected structural content for
// the registry-security-team observer cut.
//
// The top-level "id" field is empty by design — MeshGraph.ID is user-assigned and
// meshant articulate does not auto-populate it. Tests assert structural content.
func TestLLMExtractionExample_ArticulationOutput(t *testing.T) {
	f, err := os.Open(llmExampleDir + "articulation_output.json")
	if err != nil {
		t.Fatalf("open articulation_output.json: %v", err)
	}
	defer f.Close()

	var art articulationStub
	if err := json.NewDecoder(f).Decode(&art); err != nil {
		t.Fatalf("decode articulation_output.json: %v", err)
	}

	// The registry-security-team cut includes 6 of the 9 promoted traces
	// (the 3 traces from governance-working-group and dissenting-maintainers observer
	// positions are excluded). 8 distinct actor nodes are visible.
	wantEdges := 6
	if len(art.Edges) != wantEdges {
		t.Errorf("articulation_output: edges: want %d, got %d", wantEdges, len(art.Edges))
	}
	wantNodes := 8
	if len(art.Nodes) != wantNodes {
		t.Errorf("articulation_output: nodes: want %d, got %d", wantNodes, len(art.Nodes))
	}

	// PROPOSAL-CSRP-001 appears as a target in 4 edges from this cut.
	if _, ok := art.Nodes["PROPOSAL-CSRP-001"]; !ok {
		t.Error("articulation_output: nodes: expected PROPOSAL-CSRP-001 to be present")
	}

	// The cut records traces_included (6) vs traces_total (9), confirming the
	// observer-position filter was applied.
	if art.Cut.TracesIncluded != 6 {
		t.Errorf("articulation_output: cut.traces_included: want 6, got %d", art.Cut.TracesIncluded)
	}
	if art.Cut.TracesTotal != 9 {
		t.Errorf("articulation_output: cut.traces_total: want 9, got %d", art.Cut.TracesTotal)
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

	// The 2 reviewed-stage (human-derived) drafts carry the assist session ref,
	// not the extract session ref. Verify the assist session ref is stamped
	// consistently across both.
	const assistSessionRef = "f5000000-0000-4000-8000-000000000002"
	assistRefSeen := make(map[string]bool)
	for _, d := range reviewed {
		if d.ExtractionStage == "reviewed" {
			if d.SessionRef != assistSessionRef {
				t.Errorf("chain integrity: reviewed draft (%s) session_ref: want assist session %q, got %q",
					d.ID, assistSessionRef, d.SessionRef)
			}
			assistRefSeen[d.SessionRef] = true
		}
	}
	// Confirm the assist session ref is distinct from the extract session ref.
	if assistRefSeen[rec.ID] {
		t.Errorf("chain integrity: assist session ref %q must differ from extract session ref %q", assistSessionRef, rec.ID)
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

	// All promoted trace IDs are distinct.
	traceIDSeen := make(map[string]bool, len(traces))
	for i, tr := range traces {
		if traceIDSeen[tr.ID] {
			t.Errorf("chain integrity: promoted_traces[%d] (%s) duplicate ID", i, tr.ID)
		}
		traceIDSeen[tr.ID] = true
	}

	// The set of promoted trace IDs equals the set of reviewed draft IDs —
	// confirm no draft was silently dropped or duplicated.
	traceIDs := make([]string, 0, len(traces))
	for id := range traceIDSeen {
		traceIDs = append(traceIDs, id)
	}
	sort.Strings(traceIDs)
	reviewedIDList := make([]string, 0, len(reviewedIDs))
	for id := range reviewedIDs {
		reviewedIDList = append(reviewedIDList, id)
	}
	sort.Strings(reviewedIDList)
	if !reflect.DeepEqual(traceIDs, reviewedIDList) {
		t.Errorf("chain integrity: promoted trace IDs do not match reviewed draft IDs\n  promoted: %v\n  reviewed: %v",
			traceIDs, reviewedIDList)
	}
}
