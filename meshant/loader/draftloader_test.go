package loader_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// --- helpers ---

// writeTempDraftJSON writes content to a temp file and returns its path.
func writeTempDraftJSON(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "drafts-*.json")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f.Name()
}

// writeTempDrafts serialises drafts to a temp JSON file and returns its path.
func writeTempDrafts(t *testing.T, drafts []schema.TraceDraft) string {
	t.Helper()
	data, err := json.Marshal(drafts)
	if err != nil {
		t.Fatalf("marshal drafts: %v", err)
	}
	return writeTempDraftJSON(t, string(data))
}

// isLowercaseUUID checks that s matches the pattern expected by TraceDraft.IsPromotable.
func isLowercaseUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return false
			}
		}
	}
	return true
}

// --- LoadDrafts ---

func TestLoadDrafts_ValidFile_CorrectCount(t *testing.T) {
	drafts := []schema.TraceDraft{
		{SourceSpan: "span one"},
		{SourceSpan: "span two"},
		{SourceSpan: "span three"},
	}
	path := writeTempDrafts(t, drafts)

	got, err := loader.LoadDrafts(path)
	if err != nil {
		t.Fatalf("LoadDrafts: unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("count: got %d want 3", len(got))
	}
}

func TestLoadDrafts_IDAutoAssigned_WhenMissing(t *testing.T) {
	path := writeTempDraftJSON(t, `[{"source_span":"needs an id"}]`)

	got, err := loader.LoadDrafts(path)
	if err != nil {
		t.Fatalf("LoadDrafts: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("count: got %d want 1", len(got))
	}
	if !isLowercaseUUID(got[0].ID) {
		t.Errorf("auto-assigned ID is not a valid lowercase UUID: %q", got[0].ID)
	}
}

func TestLoadDrafts_IDPreserved_WhenPresent(t *testing.T) {
	want := "a1000000-0000-4000-8000-000000000001"
	path := writeTempDraftJSON(t, `[{"id":"`+want+`","source_span":"has id"}]`)

	got, err := loader.LoadDrafts(path)
	if err != nil {
		t.Fatalf("LoadDrafts: unexpected error: %v", err)
	}
	if got[0].ID != want {
		t.Errorf("ID: got %q want %q", got[0].ID, want)
	}
}

func TestLoadDrafts_EmptySourceSpan_ValidationError(t *testing.T) {
	path := writeTempDraftJSON(t, `[{"source_span":"ok"},{"source_span":""}]`)

	_, err := loader.LoadDrafts(path)
	if err == nil {
		t.Fatal("expected validation error for empty SourceSpan, got nil")
	}
}

func TestLoadDrafts_EmptyArray_ReturnsEmptySlice(t *testing.T) {
	path := writeTempDraftJSON(t, `[]`)

	got, err := loader.LoadDrafts(path)
	if err != nil {
		t.Fatalf("LoadDrafts: unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("count: got %d want 0", len(got))
	}
}

func TestLoadDrafts_MalformedJSON_Error(t *testing.T) {
	path := writeTempDraftJSON(t, `[{bad json}]`)

	_, err := loader.LoadDrafts(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoadDrafts_FileNotFound_Error(t *testing.T) {
	_, err := loader.LoadDrafts(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadDrafts_TimestampStamped(t *testing.T) {
	before := time.Now().UTC()
	path := writeTempDraftJSON(t, `[{"source_span":"needs timestamp"}]`)

	got, err := loader.LoadDrafts(path)
	if err != nil {
		t.Fatalf("LoadDrafts: unexpected error: %v", err)
	}
	after := time.Now().UTC()

	ts := got[0].Timestamp
	if ts.IsZero() {
		t.Fatal("Timestamp not stamped — expected non-zero time")
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("Timestamp %v outside expected window [%v, %v]", ts, before, after)
	}
}

// --- SummariseDrafts ---

func TestSummariseDrafts_CountsByStage(t *testing.T) {
	drafts := []schema.TraceDraft{
		{SourceSpan: "a", ExtractionStage: "weak-draft"},
		{SourceSpan: "b", ExtractionStage: "weak-draft"},
		{SourceSpan: "c", ExtractionStage: "reviewed"},
	}
	s := loader.SummariseDrafts(drafts)

	if s.ByStage["weak-draft"] != 2 {
		t.Errorf("ByStage[weak-draft]: got %d want 2", s.ByStage["weak-draft"])
	}
	if s.ByStage["reviewed"] != 1 {
		t.Errorf("ByStage[reviewed]: got %d want 1", s.ByStage["reviewed"])
	}
}

func TestSummariseDrafts_CountsByExtractedBy(t *testing.T) {
	drafts := []schema.TraceDraft{
		{SourceSpan: "a", ExtractedBy: "llm-pass1"},
		{SourceSpan: "b", ExtractedBy: "llm-pass1"},
		{SourceSpan: "c", ExtractedBy: "human"},
	}
	s := loader.SummariseDrafts(drafts)

	if s.ByExtractedBy["llm-pass1"] != 2 {
		t.Errorf("ByExtractedBy[llm-pass1]: got %d want 2", s.ByExtractedBy["llm-pass1"])
	}
	if s.ByExtractedBy["human"] != 1 {
		t.Errorf("ByExtractedBy[human]: got %d want 1", s.ByExtractedBy["human"])
	}
}

func TestSummariseDrafts_PromotableCount(t *testing.T) {
	promotable := schema.TraceDraft{
		ID:          "a1000000-0000-4000-8000-000000000001",
		SourceSpan:  "span",
		WhatChanged: "something changed",
		Observer:    "analyst",
	}
	notPromotable := schema.TraceDraft{
		ID:         "a1000000-0000-4000-8000-000000000002",
		SourceSpan: "span only",
	}
	s := loader.SummariseDrafts([]schema.TraceDraft{promotable, notPromotable})

	if s.Total != 2 {
		t.Errorf("Total: got %d want 2", s.Total)
	}
	if s.Promotable != 1 {
		t.Errorf("Promotable: got %d want 1", s.Promotable)
	}
}

func TestSummariseDrafts_FieldFillRate(t *testing.T) {
	drafts := []schema.TraceDraft{
		{SourceSpan: "a", WhatChanged: "x", Observer: "o1", Mediation: "m"},
		{SourceSpan: "b", WhatChanged: "y"},
		{SourceSpan: "c"},
	}
	s := loader.SummariseDrafts(drafts)

	if s.FieldFillRate["source_span"] != 3 {
		t.Errorf("FieldFillRate[source_span]: got %d want 3", s.FieldFillRate["source_span"])
	}
	if s.FieldFillRate["what_changed"] != 2 {
		t.Errorf("FieldFillRate[what_changed]: got %d want 2", s.FieldFillRate["what_changed"])
	}
	if s.FieldFillRate["observer"] != 1 {
		t.Errorf("FieldFillRate[observer]: got %d want 1", s.FieldFillRate["observer"])
	}
	if s.FieldFillRate["mediation"] != 1 {
		t.Errorf("FieldFillRate[mediation]: got %d want 1", s.FieldFillRate["mediation"])
	}
}

// --- PrintDraftSummary ---

func TestPrintDraftSummary_ContainsExpectedFields(t *testing.T) {
	s := loader.DraftSummary{
		Total:          5,
		Promotable:     2,
		ByStage:        map[string]int{"weak-draft": 3, "reviewed": 2},
		ByExtractedBy:  map[string]int{"llm-pass1": 4, "human": 1},
		FieldFillRate:  map[string]int{"source_span": 5, "what_changed": 3},
	}

	var buf bytes.Buffer
	if err := loader.PrintDraftSummary(&buf, s); err != nil {
		t.Fatalf("PrintDraftSummary: unexpected error: %v", err)
	}
	out := buf.String()

	checks := []string{"5", "2", "weak-draft", "reviewed", "llm-pass1", "human", "source_span", "what_changed"}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; output:\n%s", want, out)
		}
	}
}
