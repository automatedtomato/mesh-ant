// draftloader.go provides functions to load, summarise, and print TraceDraft
// datasets produced by the ingestion pipeline.
//
// Three exported functions parallel the existing loader pattern:
//   - LoadDrafts handles I/O, UUID assignment, and validation
//   - SummariseDrafts builds a provenance-aware reading of the draft set
//   - PrintDraftSummary renders that reading to any io.Writer
//
// The design principle mirrors the main loader: each layer is independently
// testable and no output format is forced on the caller.
package loader

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// DraftSummary holds a provenance-aware reading of a TraceDraft dataset.
type DraftSummary struct {
	// Total is the number of drafts in the dataset.
	Total int

	// Promotable is the number of drafts for which IsPromotable() returns true.
	Promotable int

	// ByStage maps ExtractionStage to draft count.
	ByStage map[string]int

	// ByExtractedBy maps ExtractedBy label to draft count.
	ByExtractedBy map[string]int

	// FieldFillRate maps field names to the count of drafts with a non-empty value.
	// Empty = honest abstention, not missing data.
	FieldFillRate map[string]int

	// WithIntentionallyBlank counts drafts declaring at least one intentionally blank field
	// (critique-pass skeletons where blank is a correct choice, not missing data).
	WithIntentionallyBlank int

	// WithCriterionRef counts drafts carrying a non-empty CriterionRef — self-situated
	// skeletons whose interpretive frame is named, not implicit.
	WithCriterionRef int

	// WithSessionRef counts drafts linked to the ingestion session that produced them.
	WithSessionRef int
}

// LoadDrafts reads a JSON array of TraceDraft records from path, assigns UUIDs
// to records without IDs, stamps zero timestamps, and validates each record.
// Stops at the first validation error. Empty array valid. Files >50 MB rejected.
func LoadDrafts(path string) ([]schema.TraceDraft, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open draft file %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxFileBytes)

	var drafts []schema.TraceDraft
	if err := json.NewDecoder(limited).Decode(&drafts); err != nil {
		return nil, fmt.Errorf("loader: decode draft file %q: %w", path, err)
	}

	if drafts == nil {
		drafts = []schema.TraceDraft{}
	}

	now := time.Now().UTC()
	for i := range drafts {
		// Ingestion JSON may omit IDs; the loader assigns them.
		if drafts[i].ID == "" {
			id, err := NewUUID()
			if err != nil {
				return nil, fmt.Errorf("loader: draft %d: generate UUID: %w", i, err)
			}
			drafts[i].ID = id
		}
		if drafts[i].Timestamp.IsZero() {
			drafts[i].Timestamp = now
		}
		if err := drafts[i].Validate(); err != nil {
			return nil, fmt.Errorf("loader: draft %d (id=%q): %w", i, drafts[i].ID, err)
		}
	}

	return drafts, nil
}

// SummariseDrafts builds a DraftSummary from already-validated TraceDraft records.
// Does not call Validate.
func SummariseDrafts(drafts []schema.TraceDraft) DraftSummary {
	s := DraftSummary{
		Total:         len(drafts),
		ByStage:       make(map[string]int),
		ByExtractedBy: make(map[string]int),
		FieldFillRate: make(map[string]int),
	}

	for _, d := range drafts {
		if d.IsPromotable() {
			s.Promotable++
		}
		if d.ExtractionStage != "" {
			s.ByStage[d.ExtractionStage]++
		}
		if d.ExtractedBy != "" {
			s.ByExtractedBy[d.ExtractedBy]++
		}

		if d.SourceSpan != "" {
			s.FieldFillRate["source_span"]++
		}
		if d.SourceDocRef != "" {
			s.FieldFillRate["source_doc_ref"]++
		}
		if d.WhatChanged != "" {
			s.FieldFillRate["what_changed"]++
		}
		if len(d.Source) > 0 {
			s.FieldFillRate["source"]++
		}
		if len(d.Target) > 0 {
			s.FieldFillRate["target"]++
		}
		if d.Mediation != "" {
			s.FieldFillRate["mediation"]++
		}
		if d.Observer != "" {
			s.FieldFillRate["observer"]++
		}
		if len(d.Tags) > 0 {
			s.FieldFillRate["tags"]++
		}
		if d.UncertaintyNote != "" {
			s.FieldFillRate["uncertainty_note"]++
		}
		if d.ExtractionStage != "" {
			s.FieldFillRate["extraction_stage"]++
		}
		if d.ExtractedBy != "" {
			s.FieldFillRate["extracted_by"]++
		}
		if d.DerivedFrom != "" {
			s.FieldFillRate["derived_from"]++
		}
		if d.SessionRef != "" {
			s.FieldFillRate["session_ref"]++
			s.WithSessionRef++
		}
		if len(d.IntentionallyBlank) > 0 {
			s.WithIntentionallyBlank++
		}
		if d.CriterionRef != "" {
			s.WithCriterionRef++
		}
	}

	return s
}

// PrintDraftSummary writes a draft provenance summary to w.
// Returns the first write error encountered, if any.
func PrintDraftSummary(w io.Writer, s DraftSummary) error {
	lines := []string{
		"=== Draft Summary (provenance) ===",
		"",
		fmt.Sprintf("Total drafts: %d  |  Promotable: %d  |  Not promotable: %d",
			s.Total, s.Promotable, s.Total-s.Promotable),
	}

	lines = append(lines, "", "By extraction stage:")
	if len(s.ByStage) == 0 {
		lines = append(lines, "  (none recorded)")
	} else {
		stages := sortedKeys(s.ByStage)
		for _, stage := range stages {
			lines = append(lines, fmt.Sprintf("  %-30s %d", stage, s.ByStage[stage]))
		}
	}

	lines = append(lines, "", "By extracted_by:")
	if len(s.ByExtractedBy) == 0 {
		lines = append(lines, "  (none recorded)")
	} else {
		bys := sortedKeys(s.ByExtractedBy)
		for _, by := range bys {
			lines = append(lines, fmt.Sprintf("  %-30s %d", by, s.ByExtractedBy[by]))
		}
	}

	orderedFields := []string{
		"source_span", "source_doc_ref", "what_changed", "source", "target",
		"mediation", "observer", "tags", "uncertainty_note",
		"extraction_stage", "extracted_by", "derived_from", "session_ref",
	}
	lines = append(lines, "", fmt.Sprintf("Field fill rates (out of %d):", s.Total))
	for _, field := range orderedFields {
		count := s.FieldFillRate[field]
		lines = append(lines, fmt.Sprintf("  %-25s %d", field, count))
	}

	lines = append(lines,
		"",
		fmt.Sprintf("Critique skeletons (intentionally_blank set): %d", s.WithIntentionallyBlank),
		fmt.Sprintf("Self-situated skeletons (criterion_ref set):  %d", s.WithCriterionRef),
		fmt.Sprintf("Session-linked drafts (session_ref set):       %d", s.WithSessionRef),
		"",
		"---",
		"Note: empty fields are honest abstentions, not missing data.",
		"Blank source/target/observer preserves uncertainty rather than fabricating assignments.",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("loader: PrintDraftSummary: %w", err)
		}
	}
	return nil
}

// sortedKeys returns the keys of m in ascending alphabetical order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NewUUID generates a random UUID v4 as a lowercase hyphenated string (crypto/rand).
// Exported so other packages can assign draft IDs without an external UUID library.
func NewUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	// Set version (v4) and variant (RFC 4122) bits.
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
