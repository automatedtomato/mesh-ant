// session_promote_test.go tests PromoteSession in black-box style.
//
// All tests use the llm_test package to verify public behaviour without
// depending on internal implementation details.
package llm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- Helpers ---

// validSession returns a well-formed SessionRecord suitable for promotion.
func validSession() llm.SessionRecord {
	return llm.SessionRecord{
		ID:      "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Command: "extract",
		Conditions: llm.ExtractionConditions{
			ModelID:      "claude-sonnet-4-6",
			SourceDocRef: "data/coastal-notes.md",
			Timestamp:    time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
		},
		DraftCount: 5,
		Timestamp:  time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC),
	}
}

// --- Group: PromoteSession ---

// TestPromoteSession_success verifies that a valid session produces a trace
// that passes Validate(), with all fields correctly mapped.
func TestPromoteSession_success(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	if err := tr.Validate(); err != nil {
		t.Errorf("promoted trace fails Validate(): %v", err)
	}

	if tr.ID != rec.ID {
		t.Errorf("ID: want %q, got %q", rec.ID, tr.ID)
	}
	if !tr.Timestamp.Equal(rec.Timestamp) {
		t.Errorf("Timestamp: want %v, got %v", rec.Timestamp, tr.Timestamp)
	}
	if tr.Observer != "analyst-alice" {
		t.Errorf("Observer: want %q, got %q", "analyst-alice", tr.Observer)
	}
	if tr.Mediation != rec.Command {
		t.Errorf("Mediation: want %q (command), got %q", rec.Command, tr.Mediation)
	}
}

// TestPromoteSession_emptyObserver verifies that an empty observer string
// returns an error — no trace without an observer (Principle 8).
func TestPromoteSession_emptyObserver(t *testing.T) {
	rec := validSession()

	_, err := llm.PromoteSession(rec, "")
	if err == nil {
		t.Fatal("PromoteSession() with empty observer: want error, got nil")
	}
}

// TestPromoteSession_sourceFieldMappedFromModel verifies that the LLM model
// ID appears in the Source field — the model is the source of the act.
func TestPromoteSession_sourceFieldMappedFromModel(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	if len(tr.Source) == 0 {
		t.Fatal("Source: want non-empty (model ID), got empty")
	}
	found := false
	for _, s := range tr.Source {
		if s == rec.Conditions.ModelID {
			found = true
		}
	}
	if !found {
		t.Errorf("Source: want %q in %v", rec.Conditions.ModelID, tr.Source)
	}
}

// TestPromoteSession_targetFieldMappedFromSourceDocRef verifies that the
// source document reference appears in the Target field — the document is
// what the session processed.
func TestPromoteSession_targetFieldMappedFromSourceDocRef(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	if len(tr.Target) == 0 {
		t.Fatal("Target: want non-empty (source doc ref), got empty")
	}
	found := false
	for _, s := range tr.Target {
		if s == rec.Conditions.SourceDocRef {
			found = true
		}
	}
	if !found {
		t.Errorf("Target: want %q in %v", rec.Conditions.SourceDocRef, tr.Target)
	}
}

// TestPromoteSession_tagsIncludeSession verifies that the promoted trace
// carries the "session" tag — marking it as a reflexive session trace.
func TestPromoteSession_tagsIncludeSession(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	found := false
	for _, tag := range tr.Tags {
		if tag == string(schema.TagValueSession) {
			found = true
		}
	}
	if !found {
		t.Errorf("Tags: want %q in %v", schema.TagValueSession, tr.Tags)
	}
}

// TestPromoteSession_tagsIncludeArticulation verifies that the promoted trace
// carries the "articulation" tag — marking it as an observation-of-observation act.
func TestPromoteSession_tagsIncludeArticulation(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	found := false
	for _, tag := range tr.Tags {
		if tag == string(schema.TagValueArticulation) {
			found = true
		}
	}
	if !found {
		t.Errorf("Tags: want %q in %v", schema.TagValueArticulation, tr.Tags)
	}
}

// TestPromoteSession_whatChangedMentionsCommand verifies that WhatChanged
// includes the session command, making the act readable as a trace.
func TestPromoteSession_whatChangedMentionsCommand(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	if !strings.Contains(tr.WhatChanged, rec.Command) {
		t.Errorf("WhatChanged %q: want command %q mentioned", tr.WhatChanged, rec.Command)
	}
}

// TestPromoteSession_whatChangedSurfacesModelID verifies that WhatChanged
// includes the model ID — making the conditions of the act visible in the
// trace's most human-readable field (self-situated description, not a god's-eye report).
func TestPromoteSession_whatChangedSurfacesModelID(t *testing.T) {
	rec := validSession()

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	if !strings.Contains(tr.WhatChanged, rec.Conditions.ModelID) {
		t.Errorf("WhatChanged %q: want model ID %q surfaced in description", tr.WhatChanged, rec.Conditions.ModelID)
	}
}

// TestPromoteSession_errorSession verifies that a session with ErrorNote is
// still promotable — a failed LLM session is an observable act.
func TestPromoteSession_errorSession(t *testing.T) {
	rec := validSession()
	rec.ErrorNote = "network timeout after 30s"

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with error session: want no error, got: %v", err)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("error session promoted trace fails Validate(): %v", err)
	}
}

// TestPromoteSession_splitSession verifies that a split session (Command="split",
// DraftIDs=nil) promotes correctly — split is a valid observation act.
func TestPromoteSession_splitSession(t *testing.T) {
	rec := validSession()
	rec.Command = "split"
	rec.DraftIDs = nil

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with split session: want no error, got: %v", err)
	}
	if tr.Mediation != "split" {
		t.Errorf("Mediation: want %q, got %q", "split", tr.Mediation)
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("split session promoted trace fails Validate(): %v", err)
	}
}

// TestPromoteSession_emptySourceDocRef verifies that a session without a
// source doc ref promotes without error — Target is nil when ref is empty.
func TestPromoteSession_emptySourceDocRef(t *testing.T) {
	rec := validSession()
	rec.Conditions.SourceDocRef = ""

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with empty source doc ref: want no error, got: %v", err)
	}
	// Target must not contain a blank string entry.
	for _, s := range tr.Target {
		if s == "" {
			t.Error("Target: must not contain blank string when SourceDocRef is empty")
		}
	}
}

// TestPromoteSession_emptyModelID verifies that a session without a model ID
// promotes without error — Source is nil when model ID is empty.
func TestPromoteSession_emptyModelID(t *testing.T) {
	rec := validSession()
	rec.Conditions.ModelID = ""

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with empty model ID: want no error, got: %v", err)
	}
	for _, s := range tr.Source {
		if s == "" {
			t.Error("Source: must not contain blank string when ModelID is empty")
		}
	}
}

// TestPromoteSession_multiDocSourceDocRefs verifies that when a session has
// SourceDocRefs (plural, multi-doc), all refs appear in the promoted Target.
func TestPromoteSession_multiDocSourceDocRefs(t *testing.T) {
	rec := validSession()
	// Simulate a multi-doc session: SourceDocRefs carries all doc refs.
	rec.Conditions.SourceDocRefs = []string{"data/doc-a.md", "data/doc-b.md"}

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with multi-doc session: want no error, got: %v", err)
	}

	// All source doc refs must appear in Target.
	for _, wantRef := range rec.Conditions.SourceDocRefs {
		found := false
		for _, s := range tr.Target {
			if s == wantRef {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Target: want %q in %v", wantRef, tr.Target)
		}
	}

	// WhatChanged must surface all source doc refs so the act is human-readable.
	for _, wantRef := range rec.Conditions.SourceDocRefs {
		if !strings.Contains(tr.WhatChanged, wantRef) {
			t.Errorf("WhatChanged %q: want ref %q mentioned", tr.WhatChanged, wantRef)
		}
	}
}

// TestPromoteSession_SourceDocRefsPriorityOverLegacy verifies that when both
// SourceDocRefs (plural) and the legacy SourceDocRef (singular) are present,
// SourceDocRefs takes precedence and SourceDocRef is ignored for the Target field.
// This covers the deserialization scenario where an older session file that had
// SourceDocRef is re-saved with SourceDocRefs also populated.
func TestPromoteSession_SourceDocRefsPriorityOverLegacy(t *testing.T) {
	rec := validSession()
	// Simulate a session file with both fields present.
	rec.Conditions.SourceDocRefs = []string{"data/new-doc.md"}
	rec.Conditions.SourceDocRef = "data/old-doc.md" // legacy field — must be ignored

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() with both ref fields: want no error, got: %v", err)
	}

	// Target must be built from SourceDocRefs only.
	if len(tr.Target) != 1 || tr.Target[0] != "data/new-doc.md" {
		t.Errorf("Target: want [%q], got %v (SourceDocRefs should take priority over SourceDocRef)", "data/new-doc.md", tr.Target)
	}
	// The legacy value must not appear.
	for _, s := range tr.Target {
		if s == "data/old-doc.md" {
			t.Errorf("Target contains legacy SourceDocRef %q — SourceDocRefs must take priority", "data/old-doc.md")
		}
	}
}

// --- Group: PromoteSession — CritiqueConditions bifurcation (#151) ---

// TestPromoteSession_critiqueSession_newFormat verifies that a critique session
// with CritiqueConditions populated (new format, post-#151) maps Source from
// CritiqueConditions.ModelID and Target from CritiqueConditions.SourceDocRef.
func TestPromoteSession_critiqueSession_newFormat(t *testing.T) {
	rec := llm.SessionRecord{
		ID:      "c3d4e5f6-a7b8-9012-cdef-012345678912",
		Command: "critique",
		CritiqueConditions: &llm.CritiqueConditions{
			ModelID:      "claude-opus-4-6",
			SourceDocRef: "data/incident-log.md",
			Timestamp:    time.Now(),
		},
		Timestamp: time.Now(),
	}

	tr, err := llm.PromoteSession(rec, "analyst-alice")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	// Source must come from CritiqueConditions.ModelID.
	if len(tr.Source) == 0 {
		t.Fatal("Source: want non-empty for new-format critique session, got empty")
	}
	if tr.Source[0] != "claude-opus-4-6" {
		t.Errorf("Source[0]: want %q (CritiqueConditions.ModelID), got %q", "claude-opus-4-6", tr.Source[0])
	}

	// Target must come from CritiqueConditions.SourceDocRef.
	if len(tr.Target) == 0 {
		t.Fatal("Target: want non-empty for new-format critique session, got empty")
	}
	if tr.Target[0] != "data/incident-log.md" {
		t.Errorf("Target[0]: want %q (CritiqueConditions.SourceDocRef), got %q", "data/incident-log.md", tr.Target[0])
	}

	// Trace must still pass Validate().
	if err := tr.Validate(); err != nil {
		t.Errorf("promoted trace fails Validate(): %v", err)
	}
}

// TestPromoteSession_critiqueSession_backwardCompat verifies that a legacy
// critique session (CritiqueConditions nil, Conditions populated) falls back
// to Conditions for Source and Target mapping.
func TestPromoteSession_critiqueSession_backwardCompat(t *testing.T) {
	rec := llm.SessionRecord{
		ID:      "d4e5f6a7-b8c9-0123-defa-bc1234567890",
		Command: "critique",
		// CritiqueConditions is nil — legacy format
		Conditions: llm.ExtractionConditions{
			ModelID:      "claude-haiku-4-5-20251001",
			SourceDocRef: "data/legacy-source.md",
			Timestamp:    time.Now(),
		},
		Timestamp: time.Now(),
	}

	tr, err := llm.PromoteSession(rec, "analyst-bob")
	if err != nil {
		t.Fatalf("PromoteSession() want no error for legacy critique session, got: %v", err)
	}

	// Source must come from Conditions.ModelID (fallback path).
	found := false
	for _, s := range tr.Source {
		if s == "claude-haiku-4-5-20251001" {
			found = true
		}
	}
	if !found {
		t.Errorf("Source: want %q from Conditions.ModelID (legacy fallback), got %v", "claude-haiku-4-5-20251001", tr.Source)
	}

	// Target must come from Conditions.SourceDocRef (fallback path).
	foundTarget := false
	for _, s := range tr.Target {
		if s == "data/legacy-source.md" {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Errorf("Target: want %q from Conditions.SourceDocRef (legacy fallback), got %v", "data/legacy-source.md", tr.Target)
	}

	// Trace must pass Validate().
	if err := tr.Validate(); err != nil {
		t.Errorf("legacy critique promoted trace fails Validate(): %v", err)
	}
}

// TestPromoteSession_critiqueConditionsPriorityOverConditions verifies that
// when both CritiqueConditions and Conditions are populated, CritiqueConditions
// takes priority for Source and Target mapping.
func TestPromoteSession_critiqueConditionsPriorityOverConditions(t *testing.T) {
	rec := llm.SessionRecord{
		ID:      "e5f6a7b8-c9d0-1234-efab-cd1234567890",
		Command: "critique",
		CritiqueConditions: &llm.CritiqueConditions{
			ModelID:      "claude-opus-4-6",
			SourceDocRef: "data/new-source.md",
			Timestamp:    time.Now(),
		},
		// Conditions also populated (e.g. from a mis-migration scenario).
		Conditions: llm.ExtractionConditions{
			ModelID:      "claude-haiku-4-5-20251001",
			SourceDocRef: "data/old-source.md",
			Timestamp:    time.Now(),
		},
		Timestamp: time.Now(),
	}

	tr, err := llm.PromoteSession(rec, "analyst-carol")
	if err != nil {
		t.Fatalf("PromoteSession() want no error, got: %v", err)
	}

	// Source must come from CritiqueConditions (takes priority).
	if len(tr.Source) == 0 || tr.Source[0] != "claude-opus-4-6" {
		t.Errorf("Source: want %q from CritiqueConditions (priority), got %v", "claude-opus-4-6", tr.Source)
	}

	// Target must come from CritiqueConditions (takes priority).
	if len(tr.Target) == 0 || tr.Target[0] != "data/new-source.md" {
		t.Errorf("Target: want %q from CritiqueConditions (priority), got %v", "data/new-source.md", tr.Target)
	}

	// Legacy values must not appear.
	for _, s := range tr.Source {
		if s == "claude-haiku-4-5-20251001" {
			t.Errorf("Source: legacy ModelID %q must not appear when CritiqueConditions is present", s)
		}
	}
	for _, s := range tr.Target {
		if s == "data/old-source.md" {
			t.Errorf("Target: legacy SourceDocRef %q must not appear when CritiqueConditions is present", s)
		}
	}

	// Trace must pass Validate().
	if err := tr.Validate(); err != nil {
		t.Errorf("promoted trace fails Validate(): %v", err)
	}
}

// TestPromoteSession_promotedTraceAlwaysValidates verifies Validate() passes
// across a range of valid session configurations.
func TestPromoteSession_promotedTraceAlwaysValidates(t *testing.T) {
	cases := []struct {
		name string
		rec  llm.SessionRecord
	}{
		{"extract", validSession()},
		{
			"assist",
			llm.SessionRecord{
				ID:      "b2c3d4e5-f6a7-8901-bcde-f01234567891",
				Command: "assist",
				Conditions: llm.ExtractionConditions{
					ModelID:      "claude-opus-4-6",
					SourceDocRef: "data/spans.json",
					Timestamp:    time.Now(),
				},
				Timestamp: time.Now(),
			},
		},
		{
			"critique",
			llm.SessionRecord{
				ID:      "c3d4e5f6-a7b8-9012-cdef-012345678912",
				Command: "critique",
				Conditions: llm.ExtractionConditions{
					ModelID:  "claude-haiku-4-5-20251001",
					Timestamp: time.Now(),
				},
				Timestamp: time.Now(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := llm.PromoteSession(tc.rec, "analyst-bob")
			if err != nil {
				t.Fatalf("PromoteSession() want no error, got: %v", err)
			}
			if err := tr.Validate(); err != nil {
				t.Errorf("promoted trace fails Validate(): %v", err)
			}
		})
	}
}
