package loader_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// examplesPath is the relative path from the meshant module root to the
// example dataset. Tests that use it must be run from the module root
// (go test ./...) — which is the standard invocation.
const examplesPath = "../../data/examples/traces.json"

// --- helpers ---

// validTrace returns a minimal Trace that passes schema.Validate().
// Tests may override individual fields to exercise specific behaviours.
func validTrace(id, whatChanged string) schema.Trace {
	return schema.Trace{
		ID:          id,
		Timestamp:   time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC),
		WhatChanged: whatChanged,
		Observer:    "test-observer/position-a",
	}
}

// writeTempJSON writes content to a temporary file and returns its path.
// The file is automatically removed at the end of the test.
func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "traces.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeTempJSON: %v", err)
	}
	return path
}

// --- Group 1: Load — Happy Path ---

func TestLoad_ReturnsCorrectCount(t *testing.T) {
	traces, err := loader.Load(examplesPath)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if len(traces) != 10 {
		t.Errorf("want 10 traces, got %d", len(traces))
	}
}

func TestLoad_AllTracesPassValidation(t *testing.T) {
	traces, err := loader.Load(examplesPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for i, tr := range traces {
		if err := tr.Validate(); err != nil {
			t.Errorf("trace %d (id=%q) failed Validate: %v", i, tr.ID, err)
		}
	}
}

func TestLoad_FieldsIntact(t *testing.T) {
	traces, err := loader.Load(examplesPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Spot-check trace #4 (rate-limiter, delay+threshold)
	want := struct {
		id          string
		whatChanged string
		tags        []string
		mediation   string
	}{
		id:          "e6a0b4d5-7c32-4fbf-cd4e-f5a6b7c8d9e0",
		whatChanged: "Application buffered 38 minutes by rate-limiter: queue throughput exceeded 50 submissions per hour",
		tags:        []string{"delay", "threshold"},
		mediation:   "queue-throughput-policy-v2",
	}

	var found schema.Trace
	for _, tr := range traces {
		if tr.ID == want.id {
			found = tr
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("trace %q not found in loaded dataset", want.id)
	}
	if found.WhatChanged != want.whatChanged {
		t.Errorf("WhatChanged: got %q, want %q", found.WhatChanged, want.whatChanged)
	}
	if found.Mediation != want.mediation {
		t.Errorf("Mediation: got %q, want %q", found.Mediation, want.mediation)
	}
	if len(found.Tags) != len(want.tags) {
		t.Fatalf("Tags: got %v (len %d), want %v (len %d)",
			found.Tags, len(found.Tags), want.tags, len(want.tags))
	}
	for i, tag := range want.tags {
		if found.Tags[i] != tag {
			t.Errorf("Tags[%d]: got %q, want %q", i, found.Tags[i], tag)
		}
	}
}

// --- Group 2: Load — Error Cases ---

func TestLoad_FileNotFound(t *testing.T) {
	_, err := loader.Load("/no/such/file/traces.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "/no/such/file/traces.json") {
		t.Errorf("error should mention the path; got: %v", err)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeTempJSON(t, `[{not valid json}]`)
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_ValidationErrorPropagated(t *testing.T) {
	// A trace with a missing observer field violates schema.Validate().
	badJSON := `[{
		"id": "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		"timestamp": "2026-03-10T09:00:00Z",
		"what_changed": "something happened",
		"observer": ""
	}]`
	path := writeTempJSON(t, badJSON)
	_, err := loader.Load(path)
	if err == nil {
		t.Fatal("expected validation error for missing observer, got nil")
	}
	if !strings.Contains(err.Error(), "observer") {
		t.Errorf("error should mention 'observer'; got: %v", err)
	}
}

func TestLoad_EmptyArray(t *testing.T) {
	path := writeTempJSON(t, `[]`)
	traces, err := loader.Load(path)
	if err != nil {
		t.Fatalf("unexpected error for empty array: %v", err)
	}
	if len(traces) != 0 {
		t.Errorf("want 0 traces, got %d", len(traces))
	}
}

// --- Group 3: Summarise — Correctness ---

func TestSummarise_ElementFrequency(t *testing.T) {
	t1 := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "first event")
	t1.Source = []string{"alpha"}
	t1.Target = []string{"beta"}

	t2 := validTrace("b2c3d4e5-f6a7-4b8c-9d0e-f1a2b3c4d5e6", "second event")
	t2.Source = []string{"alpha"}
	t2.Target = []string{"gamma"}

	s := loader.Summarise([]schema.Trace{t1, t2})

	if s.Elements["alpha"] != 2 {
		t.Errorf("alpha: want count 2, got %d", s.Elements["alpha"])
	}
	if s.Elements["beta"] != 1 {
		t.Errorf("beta: want count 1, got %d", s.Elements["beta"])
	}
	if s.Elements["gamma"] != 1 {
		t.Errorf("gamma: want count 1, got %d", s.Elements["gamma"])
	}
}

func TestSummarise_ElementsUnionOfSourceAndTarget(t *testing.T) {
	// "shared" appears once as source and once as target — total count 2.
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "shared element")
	tr.Source = []string{"shared"}
	tr.Target = []string{"shared"}

	s := loader.Summarise([]schema.Trace{tr})
	if s.Elements["shared"] != 2 {
		t.Errorf("shared: want count 2 (1 source + 1 target), got %d", s.Elements["shared"])
	}
}

func TestSummarise_MediationsDeduped(t *testing.T) {
	t1 := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "first")
	t1.Mediation = "policy-x"

	t2 := validTrace("b2c3d4e5-f6a7-4b8c-9d0e-f1a2b3c4d5e6", "second")
	t2.Mediation = "policy-x" // duplicate

	s := loader.Summarise([]schema.Trace{t1, t2})
	if len(s.Mediations) != 1 {
		t.Errorf("want 1 unique mediation, got %d: %v", len(s.Mediations), s.Mediations)
	}
	if s.Mediations[0] != "policy-x" {
		t.Errorf("want mediations[0]=%q, got %q", "policy-x", s.Mediations[0])
	}
}

func TestSummarise_MediationsEncounterOrder(t *testing.T) {
	t1 := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "first")
	t1.Mediation = "policy-a"

	t2 := validTrace("b2c3d4e5-f6a7-4b8c-9d0e-f1a2b3c4d5e6", "second")
	t2.Mediation = "policy-b"

	t3 := validTrace("c3d4e5f6-a7b8-4c9d-0e1f-a2b3c4d5e6f7", "third")
	t3.Mediation = "policy-a" // duplicate of first — should not appear again

	s := loader.Summarise([]schema.Trace{t1, t2, t3})
	if len(s.Mediations) != 2 {
		t.Fatalf("want 2 mediations, got %d: %v", len(s.Mediations), s.Mediations)
	}
	if s.Mediations[0] != "policy-a" {
		t.Errorf("mediations[0]: want %q, got %q", "policy-a", s.Mediations[0])
	}
	if s.Mediations[1] != "policy-b" {
		t.Errorf("mediations[1]: want %q, got %q", "policy-b", s.Mediations[1])
	}
}

func TestSummarise_EmptyMediationExcluded(t *testing.T) {
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "no intermediary observed")
	// tr.Mediation is zero-value ("") — should not appear in Mediations

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.Mediations) != 0 {
		t.Errorf("want 0 mediations for trace with empty Mediation, got %d: %v",
			len(s.Mediations), s.Mediations)
	}
}

func TestSummarise_FlaggedTracesDelay(t *testing.T) {
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "delayed at queue")
	tr.Tags = []string{string(schema.TagDelay)}

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.FlaggedTraces) != 1 {
		t.Fatalf("want 1 flagged trace, got %d", len(s.FlaggedTraces))
	}
	if s.FlaggedTraces[0].ID != tr.ID {
		t.Errorf("FlaggedTraces[0].ID: got %q, want %q", s.FlaggedTraces[0].ID, tr.ID)
	}
}

func TestSummarise_FlaggedTracesThreshold(t *testing.T) {
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "exceeded ceiling")
	tr.Tags = []string{string(schema.TagThreshold)}

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.FlaggedTraces) != 1 {
		t.Fatalf("want 1 flagged trace, got %d", len(s.FlaggedTraces))
	}
}

func TestSummarise_FlaggedTracesOtherTagsExcluded(t *testing.T) {
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "blocked")
	tr.Tags = []string{string(schema.TagBlockage)} // blockage is not delay or threshold

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.FlaggedTraces) != 0 {
		t.Errorf("want 0 flagged traces for blockage-only tag, got %d", len(s.FlaggedTraces))
	}
}

func TestSummarise_FlaggedTracesFields(t *testing.T) {
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "threshold breached")
	tr.Tags = []string{string(schema.TagThreshold), string(schema.TagRedirection)}

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.FlaggedTraces) != 1 {
		t.Fatalf("want 1 flagged trace, got %d", len(s.FlaggedTraces))
	}
	ft := s.FlaggedTraces[0]
	if ft.ID != tr.ID {
		t.Errorf("ID: got %q, want %q", ft.ID, tr.ID)
	}
	if ft.WhatChanged != tr.WhatChanged {
		t.Errorf("WhatChanged: got %q, want %q", ft.WhatChanged, tr.WhatChanged)
	}
	// Full tags slice must be preserved, not just the triggering tag.
	if len(ft.Tags) != 2 {
		t.Fatalf("Tags: want len 2, got %d: %v", len(ft.Tags), ft.Tags)
	}
	if ft.Tags[0] != string(schema.TagThreshold) {
		t.Errorf("Tags[0]: got %q, want %q", ft.Tags[0], string(schema.TagThreshold))
	}
	if ft.Tags[1] != string(schema.TagRedirection) {
		t.Errorf("Tags[1]: got %q, want %q", ft.Tags[1], string(schema.TagRedirection))
	}
}

func TestSummarise_FlaggedTracesNoDuplication(t *testing.T) {
	// A trace with both delay and threshold should appear only once.
	tr := validTrace("a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5", "buffered past threshold")
	tr.Tags = []string{string(schema.TagDelay), string(schema.TagThreshold)}

	s := loader.Summarise([]schema.Trace{tr})
	if len(s.FlaggedTraces) != 1 {
		t.Errorf("want 1 flagged trace (not 2) for delay+threshold, got %d", len(s.FlaggedTraces))
	}
}

func TestSummarise_EmptyInput(t *testing.T) {
	// Summarise(nil) must not panic and must return a usable zero-value summary.
	s := loader.Summarise(nil)
	if s.Elements == nil {
		t.Error("Elements map should be non-nil even for empty input")
	}
	if len(s.Mediations) != 0 {
		t.Errorf("want 0 mediations, got %d", len(s.Mediations))
	}
	if len(s.FlaggedTraces) != 0 {
		t.Errorf("want 0 flagged traces, got %d", len(s.FlaggedTraces))
	}
}

// --- Group 4: PrintSummary — Output ---

// summaryFromExamples loads the example dataset and builds a summary,
// failing the test if either step fails. Used by PrintSummary tests that
// want realistic output rather than minimal hand-built input.
func summaryFromExamples(t *testing.T) loader.MeshSummary {
	t.Helper()
	traces, err := loader.Load(examplesPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loader.Summarise(traces)
}

func TestPrintSummary_ContainsElementsHeader(t *testing.T) {
	var buf bytes.Buffer
	loader.PrintSummary(&buf, summaryFromExamples(t))
	if !strings.Contains(buf.String(), "Elements") {
		t.Errorf("output missing 'Elements' header; got:\n%s", buf.String())
	}
}

func TestPrintSummary_ContainsMediationsHeader(t *testing.T) {
	var buf bytes.Buffer
	loader.PrintSummary(&buf, summaryFromExamples(t))
	if !strings.Contains(buf.String(), "mediations") {
		t.Errorf("output missing 'mediations' header; got:\n%s", buf.String())
	}
}

func TestPrintSummary_ContainsFlaggedHeader(t *testing.T) {
	var buf bytes.Buffer
	loader.PrintSummary(&buf, summaryFromExamples(t))
	if !strings.Contains(buf.String(), "Traces tagged") {
		t.Errorf("output missing 'Traces tagged' header; got:\n%s", buf.String())
	}
}

func TestPrintSummary_ContainsProvisionalNote(t *testing.T) {
	var buf bytes.Buffer
	loader.PrintSummary(&buf, summaryFromExamples(t))
	out := buf.String()
	if !strings.Contains(out, "first look at the mesh") {
		t.Errorf("output missing footer disclaimer; got:\n%s", out)
	}
}

func TestPrintSummary_ElementAppearsWithCount(t *testing.T) {
	var buf bytes.Buffer
	loader.PrintSummary(&buf, summaryFromExamples(t))
	out := buf.String()
	// vendor-registration-application-00142 appears 8 times (target in traces 2–10 minus #1,#3)
	if !strings.Contains(out, "vendor-registration-application-00142") {
		t.Errorf("output missing expected element name; got:\n%s", out)
	}
	if !strings.Contains(out, "x8") {
		t.Errorf("output missing count x8 for top element; got:\n%s", out)
	}
}

func TestPrintSummary_EmptySummary_DoesNotPanic(t *testing.T) {
	// PrintSummary with a zero-value MeshSummary must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintSummary panicked on empty summary: %v", r)
		}
	}()
	var buf bytes.Buffer
	loader.PrintSummary(&buf, loader.MeshSummary{})
}
