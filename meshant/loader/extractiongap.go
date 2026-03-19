// extractiongap.go provides extraction-gap analysis — comparing what two
// analyst positions extracted from the same source material, without treating
// either as the authoritative reading.
//
// CompareExtractions takes two named sets of TraceDraft records and produces
// an ExtractionGap: which SourceSpans each analyst extracted, which they share,
// and where they disagree on content fields for shared spans.
//
// PrintExtractionGap renders that gap to any io.Writer in a human-readable
// format that names both positions and avoids god's-eye language.
package loader

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// ExtractionGap records the asymmetry between two analyst extraction positions.
// It names both positions, partitions their SourceSpans into three sets, and
// records field-by-field disagreements for shared spans.
//
// No position is treated as primary. The gap is the data — not a verdict.
type ExtractionGap struct {
	// AnalystA is the label passed in for the first extraction set.
	AnalystA string

	// AnalystB is the label passed in for the second extraction set.
	AnalystB string

	// OnlyInA lists SourceSpans extracted by A but not B. Sorted alphabetically.
	OnlyInA []string

	// OnlyInB lists SourceSpans extracted by B but not A. Sorted alphabetically.
	OnlyInB []string

	// InBoth lists SourceSpans extracted by both A and B. Sorted alphabetically.
	InBoth []string

	// Disagreements lists field-level disagreements for spans in InBoth.
	// Sorted by (SourceSpan, Field).
	Disagreements []FieldDisagreement
}

// FieldDisagreement records that two analysts assigned different values to a
// content field for the same SourceSpan. Neither value is authoritative.
//
// Special Field values:
//   - "(multiple-drafts)" — one or both analysts produced more than one draft
//     for this SourceSpan, so field-by-field comparison is not meaningful.
type FieldDisagreement struct {
	// SourceSpan identifies which span the disagreement belongs to.
	SourceSpan string

	// Field is the content field name, or "(multiple-drafts)" for the
	// count-mismatch sentinel.
	Field string

	// ValueA is analyst A's value for this field. Slice fields are rendered
	// as sorted, comma-joined strings. nil/empty slices render as "(empty)".
	ValueA string

	// ValueB is analyst B's value. Same rendering as ValueA.
	ValueB string
}

// CompareExtractions compares two named extraction sets and returns an
// ExtractionGap. It partitions SourceSpan keys into three groups and
// records field disagreements for shared spans.
//
// Algorithm:
//  1. Index each set by SourceSpan, preserving all duplicates in a slice.
//  2. Three-way partition into OnlyInA, OnlyInB, InBoth.
//  3. For each shared span, emit a "(multiple-drafts)" disagreement if
//     one side has more than one draft (or the counts differ); otherwise
//     compare the 9 content fields and emit a FieldDisagreement for each
//     that differs.
//  4. Sort all output slices. Return an immutable ExtractionGap.
func CompareExtractions(analystA string, setA []schema.TraceDraft, analystB string, setB []schema.TraceDraft) ExtractionGap {
	// Index both sets by SourceSpan. All duplicates are preserved.
	indexA := indexBySpan(setA)
	indexB := indexBySpan(setB)

	// Collect all keys from both indexes for three-way partition.
	allSpans := make(map[string]bool)
	for k := range indexA {
		allSpans[k] = true
	}
	for k := range indexB {
		allSpans[k] = true
	}

	// Initialise with non-nil empty slices so callers can range without nil checks.
	gap := ExtractionGap{
		AnalystA:      analystA,
		AnalystB:      analystB,
		OnlyInA:       []string{},
		OnlyInB:       []string{},
		InBoth:        []string{},
		Disagreements: []FieldDisagreement{},
	}

	for span := range allSpans {
		inA := len(indexA[span]) > 0
		inB := len(indexB[span]) > 0

		switch {
		case inA && !inB:
			gap.OnlyInA = append(gap.OnlyInA, span)
		case !inA && inB:
			gap.OnlyInB = append(gap.OnlyInB, span)
		case inA && inB:
			gap.InBoth = append(gap.InBoth, span)
			// Compare content for shared spans.
			ds := compareSpan(span, indexA[span], indexB[span])
			gap.Disagreements = append(gap.Disagreements, ds...)
		}
	}

	sort.Strings(gap.OnlyInA)
	sort.Strings(gap.OnlyInB)
	sort.Strings(gap.InBoth)
	sort.Slice(gap.Disagreements, func(i, j int) bool {
		a, b := gap.Disagreements[i], gap.Disagreements[j]
		if a.SourceSpan != b.SourceSpan {
			return a.SourceSpan < b.SourceSpan
		}
		return a.Field < b.Field
	})

	return gap
}

// compareSpan compares two draft slices for the same SourceSpan and returns
// any FieldDisagreements. If either side has more than one draft (or counts
// differ), a single "(multiple-drafts)" disagreement is returned instead of
// field-by-field comparison.
func compareSpan(span string, draftsA, draftsB []schema.TraceDraft) []FieldDisagreement {
	// Guard against duplicate drafts: count mismatch or either side has >1.
	if len(draftsA) != 1 || len(draftsB) != 1 {
		return []FieldDisagreement{{
			SourceSpan: span,
			Field:      "(multiple-drafts)",
			ValueA:     fmt.Sprintf("%d drafts", len(draftsA)),
			ValueB:     fmt.Sprintf("%d drafts", len(draftsB)),
		}}
	}

	a := draftsA[0]
	b := draftsB[0]

	var ds []FieldDisagreement

	// Compare the 9 content fields:
	//   what_changed, source, target, mediation, observer, tags,
	//   uncertainty_note, intentionally_blank, source_doc_ref.
	//
	// SourceDocRef is included because it identifies which document the span
	// came from — a source material property, not a provenance property. Two
	// analysts attributing the same SourceSpan to different documents is an
	// analytically meaningful disagreement.
	//
	// Excluded provenance fields (ID, Timestamp, ExtractionStage, ExtractedBy,
	// DerivedFrom, CriterionRef) describe the analytical position and are
	// expected to differ between analyst positions.

	if a.WhatChanged != b.WhatChanged {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "what_changed",
			ValueA:     a.WhatChanged,
			ValueB:     b.WhatChanged,
		})
	}
	if !slicesEqual(a.Source, b.Source) {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "source",
			ValueA:     renderSlice(a.Source),
			ValueB:     renderSlice(b.Source),
		})
	}
	if !slicesEqual(a.Target, b.Target) {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "target",
			ValueA:     renderSlice(a.Target),
			ValueB:     renderSlice(b.Target),
		})
	}
	if a.Mediation != b.Mediation {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "mediation",
			ValueA:     a.Mediation,
			ValueB:     b.Mediation,
		})
	}
	if a.Observer != b.Observer {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "observer",
			ValueA:     a.Observer,
			ValueB:     b.Observer,
		})
	}
	if !slicesEqual(a.Tags, b.Tags) {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "tags",
			ValueA:     renderSlice(a.Tags),
			ValueB:     renderSlice(b.Tags),
		})
	}
	if a.UncertaintyNote != b.UncertaintyNote {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "uncertainty_note",
			ValueA:     a.UncertaintyNote,
			ValueB:     b.UncertaintyNote,
		})
	}
	if !slicesEqual(a.IntentionallyBlank, b.IntentionallyBlank) {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "intentionally_blank",
			ValueA:     renderSlice(a.IntentionallyBlank),
			ValueB:     renderSlice(b.IntentionallyBlank),
		})
	}
	if a.SourceDocRef != b.SourceDocRef {
		ds = append(ds, FieldDisagreement{
			SourceSpan: span,
			Field:      "source_doc_ref",
			ValueA:     a.SourceDocRef,
			ValueB:     b.SourceDocRef,
		})
	}

	return ds
}

// indexBySpan builds a map from SourceSpan to the slice of TraceDrafts that
// carry that span. All duplicates are preserved — a span with two drafts
// produces a slice of length 2.
func indexBySpan(drafts []schema.TraceDraft) map[string][]schema.TraceDraft {
	idx := make(map[string][]schema.TraceDraft)
	for _, d := range drafts {
		idx[d.SourceSpan] = append(idx[d.SourceSpan], d)
	}
	return idx
}

// renderSlice sorts a copy of ss and joins with ", ". A nil or empty slice
// renders as "(empty)" so the caller always gets a printable string.
func renderSlice(ss []string) string {
	if len(ss) == 0 {
		return "(empty)"
	}
	cp := make([]string, len(ss))
	copy(cp, ss)
	sort.Strings(cp)
	return strings.Join(cp, ", ")
}

// slicesEqual returns true when the two slices contain the same elements
// regardless of order. Comparison is set-based: both slices are sorted before
// comparison so ["A","B"] equals ["B","A"]. nil and []string{} are treated as
// equivalent — both represent an absent value, not a distinct empty list.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	cpA := make([]string, len(a))
	copy(cpA, a)
	cpB := make([]string, len(b))
	copy(cpB, b)
	sort.Strings(cpA)
	sort.Strings(cpB)
	for i := range cpA {
		if cpA[i] != cpB[i] {
			return false
		}
	}
	return true
}

// PrintExtractionGap writes an extraction-gap report to w.
//
// The report follows the ObserverGap pattern (see graph.PrintObserverGap):
// both analyst positions are named, the three-way span partition is shown,
// disagreements are listed per-field, and the closing note acknowledges
// spans neither analyst extracted (the shadow). Neither position is treated
// as authoritative.
//
// Returns the first write error encountered, if any.
func PrintExtractionGap(w io.Writer, gap ExtractionGap) error {
	lines := []string{
		"=== Extraction Gap ===",
		"",
		fmt.Sprintf("Position A: %s", gap.AnalystA),
		fmt.Sprintf("Position B: %s", gap.AnalystB),
		"",
		fmt.Sprintf("Only in A: %d  |  Only in B: %d  |  In both: %d  |  Disagreements: %d",
			len(gap.OnlyInA), len(gap.OnlyInB), len(gap.InBoth), len(gap.Disagreements)),
	}

	if len(gap.OnlyInA) > 0 {
		lines = append(lines, "", fmt.Sprintf("Spans extracted only by A (%s):", gap.AnalystA))
		for _, s := range gap.OnlyInA {
			lines = append(lines, "  "+s)
		}
	}

	if len(gap.OnlyInB) > 0 {
		lines = append(lines, "", fmt.Sprintf("Spans extracted only by B (%s):", gap.AnalystB))
		for _, s := range gap.OnlyInB {
			lines = append(lines, "  "+s)
		}
	}

	if len(gap.InBoth) > 0 {
		lines = append(lines, "", "Spans extracted by both:")
		for _, s := range gap.InBoth {
			lines = append(lines, "  "+s)
		}
	}

	if len(gap.Disagreements) > 0 {
		lines = append(lines, "", "Field-level disagreements (span / field: A=... | B=...):")
		for _, d := range gap.Disagreements {
			lines = append(lines, fmt.Sprintf("  [%s] / %s: A=%s | B=%s",
				d.SourceSpan, d.Field, d.ValueA, d.ValueB))
		}
	}

	// No-gap message when both OnlyInA and OnlyInB are empty.
	if len(gap.OnlyInA) == 0 && len(gap.OnlyInB) == 0 {
		lines = append(lines, "", "No extraction gap — both positions extracted the same spans.")
	}

	// Closing shadow note and authoritative disclaimer. Spans that neither
	// analyst extracted are not visible in this report — the analysis can only
	// see what was extracted, not what was missed.
	lines = append(lines,
		"",
		"---",
		"Note: spans neither analyst extracted are not visible in this report.",
		"Neither position is authoritative. Each reflects where it stands.",
	)

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("loader: PrintExtractionGap: %w", err)
		}
	}
	return nil
}
