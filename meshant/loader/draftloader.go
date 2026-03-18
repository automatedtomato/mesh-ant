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
// It is oriented toward the ingestion pipeline: who produced the drafts,
// at what stage, and how much of each record is filled.
type DraftSummary struct {
	// Total is the number of drafts in the dataset.
	Total int

	// Promotable is the number of drafts for which IsPromotable() returns true.
	Promotable int

	// ByStage maps ExtractionStage values to the count of drafts at that stage.
	ByStage map[string]int

	// ByExtractedBy maps ExtractedBy values to the count of drafts with that label.
	ByExtractedBy map[string]int

	// FieldFillRate maps field names to the count of drafts with a non-empty
	// value for that field. This reveals which fields the ingestion pipeline
	// is populating and which are being left empty (empty = honest abstention).
	FieldFillRate map[string]int

	// WithIntentionallyBlank is the number of drafts that declare at least
	// one intentionally blank field — i.e., that set IntentionallyBlank on
	// TraceDraft. These are typically critique-pass skeletons produced by
	// meshant rearticulate, where blank content fields are correct choices,
	// not missing data.
	WithIntentionallyBlank int

	// WithCriterionRef is the number of drafts that carry a non-empty
	// CriterionRef — i.e., that declare the EquivalenceCriterion under which
	// they were produced. A non-zero count means some skeletons are self-situated:
	// their interpretive frame is named, not implicit.
	WithCriterionRef int
}

// LoadDrafts reads a JSON array of TraceDraft records from path.
// It assigns new lowercase UUIDs to any records that are missing an ID,
// stamps a Timestamp on records that have a zero timestamp, and validates
// each record via TraceDraft.Validate.
//
// LoadDrafts stops at the first validation error and names the record index
// in the error message. An empty JSON array is valid and returns an empty
// (non-nil) slice. Files larger than 50 MB are rejected before decoding.
func LoadDrafts(path string) ([]schema.TraceDraft, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("loader: open draft file %q: %w", path, err)
	}
	defer f.Close()

	// Limit reads to maxFileBytes (defined in loader.go) to prevent memory
	// exhaustion on unexpectedly large input.
	limited := io.LimitReader(f, maxFileBytes)

	var drafts []schema.TraceDraft
	if err := json.NewDecoder(limited).Decode(&drafts); err != nil {
		return nil, fmt.Errorf("loader: decode draft file %q: %w", path, err)
	}

	// Normalise null JSON value (json.Decode sets the target to nil on null).
	if drafts == nil {
		drafts = []schema.TraceDraft{}
	}

	now := time.Now().UTC()
	for i := range drafts {
		// Assign a UUID when the record has no ID — the ingestion contract
		// does not require IDs in extraction JSON; the loader assigns them.
		if drafts[i].ID == "" {
			id, err := NewUUID()
			if err != nil {
				return nil, fmt.Errorf("loader: draft %d: generate UUID: %w", i, err)
			}
			drafts[i].ID = id
		}
		// Stamp Timestamp when absent.
		if drafts[i].Timestamp.IsZero() {
			drafts[i].Timestamp = now
		}
		// Validate — only SourceSpan is required at draft stage.
		if err := drafts[i].Validate(); err != nil {
			return nil, fmt.Errorf("loader: draft %d (id=%q): %w", i, drafts[i].ID, err)
		}
	}

	return drafts, nil
}

// SummariseDrafts builds a DraftSummary from a slice of TraceDraft records.
// It counts records by ExtractionStage and ExtractedBy, counts how many are
// promotable, and records per-field fill rates.
//
// SummariseDrafts does not call Validate — callers should ensure drafts have
// already been validated (e.g. via LoadDrafts).
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

		// Track per-field fill rates — these reveal honest abstentions
		// (empty fields) vs populated assignments.
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
// It shows total/promotable counts, breakdown by extraction stage and
// extracted_by label, and field fill rates across the dataset.
//
// ByStage and ByExtractedBy entries are sorted alphabetically for stable
// output. Field fill rates are listed in field-definition order.
//
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

	// Field fill rates — ordered by field definition for stable output.
	orderedFields := []string{
		"source_span", "source_doc_ref", "what_changed", "source", "target",
		"mediation", "observer", "tags", "uncertainty_note",
		"extraction_stage", "extracted_by", "derived_from",
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

// sortedKeys returns the keys of a map[string]int in ascending alphabetical
// order. Used by PrintDraftSummary to produce deterministic output.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NewUUID generates a random UUID v4 formatted as a lowercase hyphenated string.
// It uses crypto/rand for randomness — the same source as production UUID libraries.
// Exported so that other packages (e.g. review) can assign draft IDs
// without importing an external UUID library.
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
