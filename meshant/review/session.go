// session.go implements the interactive review session for TraceDraft records.
//
// RunReviewSession walks the reviewer through unreviewed drafts, rendering
// each with its derivation chain and ambiguity warnings, and collecting
// accept/edit/skip/quit decisions.
//
// Each accepted or edited draft produces a new derived TraceDraft via
// deriveAccepted or deriveEdited respectively. The derivation records
// provenance: DerivedFrom names the parent, ExtractedBy is "meshant-review"
// (the session is the cut, not a person), and ExtractionStage advances to
// "reviewed".
package review

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// RunReviewSession presents each reviewable draft to the reviewer in sequence.
// It renders the derivation chain, the draft fields, and any ambiguity warnings,
// then reads one of three actions: accept, skip, or quit.
//
// Reviewable drafts are those with ExtractionStage == "weak-draft". If no draft
// in the slice has any ExtractionStage set, all drafts are presented (fallback
// for datasets without stage metadata).
//
// Accept: calls deriveAccepted to produce a new derived draft and appends it
// to the results. Skip: advances without producing a draft. Quit: returns
// results collected so far with nil error. Unknown input re-prompts.
//
// EOF on the input reader, or any scanner error, is treated identically to
// "quit": results collected so far are returned with nil error. Scanner errors
// are not surfaced — this is intentional for a terminal session where a closed
// reader is an unrecoverable condition.
//
// Returns (nil, nil) if there are no reviewable drafts.
func RunReviewSession(drafts []schema.TraceDraft, in io.Reader, out io.Writer) ([]schema.TraceDraft, error) {
	queue := filterReviewable(drafts)
	if len(queue) == 0 {
		fmt.Fprintf(out, "no drafts to review\n")
		return nil, nil
	}

	scanner := bufio.NewScanner(in)
	var results []schema.TraceDraft
	total := len(queue)

	// Use a named label so that "quit" can break from within the inner prompt loop.
draftLoop:
	for i, d := range queue {
		// Build the derivation chain starting from this draft and classify each step.
		chain := loader.FollowDraftChain(drafts, d.ID)
		classes := loader.ClassifyDraftChain(chain)

		// Render chain, draft details, and ambiguity warnings for the reviewer.
		fmt.Fprint(out, RenderChain(chain, classes))
		fmt.Fprint(out, RenderDraft(d, i+1, total))
		fmt.Fprintf(out, "Ambiguities:\n")
		fmt.Fprint(out, RenderAmbiguities(DetectAmbiguities(d)))

		// Inner loop: re-prompt on unknown input until a valid action is given.
	promptLoop:
		for {
			fmt.Fprintf(out, "[a]ccept  [e]dit  [s]kip  [q]uit > ")
			if !scanner.Scan() {
				// EOF or scanner error — treat as quit to avoid blocking on a
				// closed reader. Results collected so far are returned.
				break draftLoop
			}
			switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
			case "a":
				// Accept: derive a new reviewed draft from the parent.
				derived, err := deriveAccepted(d)
				if err != nil {
					return results, err
				}
				results = append(results, derived)
				break promptLoop
			case "e":
				// Edit: run field-by-field edit flow. completed is true only
				// when all 8 fields were read without hitting EOF. On EOF
				// mid-flow, completed is false — break draftLoop immediately
				// without appending a partial draft.
				edited, completed, err := runEditFlow(d, scanner, out)
				if err != nil {
					return results, err
				}
				if !completed {
					// EOF during edit — treat as quit; do not append a draft.
					break draftLoop
				}
				derived, err := deriveEdited(d, edited)
				if err != nil {
					return results, err
				}
				results = append(results, derived)
				break promptLoop
			case "s":
				// Skip: advance to the next draft without producing a result.
				break promptLoop
			case "q":
				// Quit: return everything collected so far.
				break draftLoop
			default:
				// Unknown input: write a re-prompt message and loop again.
				fmt.Fprintf(out, "unknown action -- enter a, e, s, or q\n")
			}
		}
	}

	return results, nil
}

// deriveAccepted creates a new TraceDraft derived from parent.
// The new draft copies all candidate content fields from parent and sets
// provenance fields to record that the review session accepted it:
//   - DerivedFrom: parent.ID (derivation link)
//   - ExtractionStage: "reviewed" (stage advanced)
//   - ExtractedBy: "meshant-review" (the session is the cut, not a person)
//   - ID: fresh UUID
//   - Timestamp: time.Now()
//
// Slice fields (Source, Target, Tags, IntentionallyBlank) are deep-copied so
// the derived draft is independent of the parent. Mutations to either do not
// affect the other.
func deriveAccepted(parent schema.TraceDraft) (schema.TraceDraft, error) {
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("deriveAccepted: generate UUID: %w", err)
	}

	return schema.TraceDraft{
		ID:                 id,
		Timestamp:          time.Now(),
		SourceSpan:         parent.SourceSpan,
		SourceDocRef:       parent.SourceDocRef,
		WhatChanged:        parent.WhatChanged,
		Source:             cloneStrings(parent.Source),
		Target:             cloneStrings(parent.Target),
		Mediation:          parent.Mediation,
		Observer:           parent.Observer,
		Tags:               cloneStrings(parent.Tags),
		UncertaintyNote:    parent.UncertaintyNote,
		CriterionRef:       parent.CriterionRef,
		SessionRef:         parent.SessionRef,
		IntentionallyBlank: cloneStrings(parent.IntentionallyBlank),
		DerivedFrom:        parent.ID,
		ExtractionStage:    "reviewed",
		ExtractedBy:        "meshant-review",
	}, nil
}

// parseCommaSeparated splits s on commas, trims whitespace from each element,
// and drops any empty strings that result. Returns nil when no non-empty
// elements remain (e.g. empty input or input consisting only of commas).
//
// This is used by runEditFlow to parse slice fields (source, target, tags)
// from a single line of reviewer input.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// deriveEdited creates a new TraceDraft that combines the reviewer's edits
// with provenance drawn from the parent draft.
//
// Content fields (WhatChanged, Source, Target, Mediation, Observer, Tags,
// UncertaintyNote, CriterionRef) come from edited — these are the fields the
// reviewer may change. Provenance fields (SourceSpan, SourceDocRef,
// IntentionallyBlank) come from parent — they record the original source
// material and are not editable in the review session. A fresh UUID and the
// current timestamp are assigned. DerivedFrom is set to parent.ID.
//
// All slice fields are deep-copied from edited so the derived draft is
// independent of both parent and edited.
func deriveEdited(parent schema.TraceDraft, edited schema.TraceDraft) (schema.TraceDraft, error) {
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("deriveEdited: generate UUID: %w", err)
	}

	return schema.TraceDraft{
		// Fresh identity and timestamp for the derived record.
		ID:        id,
		Timestamp: time.Now(),

		// Provenance from parent — the source material is unchanged.
		SourceSpan:         parent.SourceSpan,
		SourceDocRef:       parent.SourceDocRef,
		SessionRef:         parent.SessionRef,
		IntentionallyBlank: cloneStrings(parent.IntentionallyBlank),

		// Content from edited — the reviewer's articulation.
		WhatChanged:     edited.WhatChanged,
		Source:          cloneStrings(edited.Source),
		Target:          cloneStrings(edited.Target),
		Mediation:       edited.Mediation,
		Observer:        edited.Observer,
		Tags:            cloneStrings(edited.Tags),
		UncertaintyNote: edited.UncertaintyNote,
		CriterionRef:    edited.CriterionRef,

		// Derivation chain fields — the session is the cut.
		DerivedFrom:     parent.ID,
		ExtractionStage: "reviewed",
		ExtractedBy:     "meshant-review",
	}, nil
}

// runEditFlow presents each editable field of d in sequence and reads a
// replacement value from scanner. An empty line (after TrimSpace) keeps the
// current value. A non-empty line replaces the field: string fields are set
// directly; slice fields (source, target, tags) are parsed via
// parseCommaSeparated and kept unchanged when the parsed result is nil
// (all-comma input is treated as no change).
//
// Fields are presented in this order:
//
//	what_changed, source, target, mediation, observer, tags,
//	uncertainty_note, criterion_ref
//
// The bool return is true when all 8 fields were read successfully (edit
// completed), false when the scanner hit EOF before all fields were read
// (edit was abandoned). The caller must only derive a new draft when the
// bool is true.
func runEditFlow(d schema.TraceDraft, scanner *bufio.Scanner, out io.Writer) (schema.TraceDraft, bool, error) {
	// working is a local copy; d is never mutated.
	working := d

	// readField writes the prompt and reads one line from scanner. Returns
	// the trimmed line and true on success, or ("", false) on EOF.
	readField := func(fieldName, current string) (string, bool) {
		fmt.Fprintf(out, "  %s [%s]: ", fieldName, current)
		if !scanner.Scan() {
			return "", false
		}
		return strings.TrimSpace(scanner.Text()), true
	}

	// what_changed (string field)
	line, ok := readField("what_changed", valueOrEmpty(working.WhatChanged))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.WhatChanged = line
	}

	// source (slice field)
	line, ok = readField("source", sliceOrEmpty(working.Source))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Source = parsed
		}
	}

	// target (slice field)
	line, ok = readField("target", sliceOrEmpty(working.Target))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Target = parsed
		}
	}

	// mediation (string field)
	line, ok = readField("mediation", valueOrEmpty(working.Mediation))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.Mediation = line
	}

	// observer (string field)
	line, ok = readField("observer", valueOrEmpty(working.Observer))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.Observer = line
	}

	// tags (slice field)
	line, ok = readField("tags", sliceOrEmpty(working.Tags))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Tags = parsed
		}
	}

	// uncertainty_note (string field)
	line, ok = readField("uncertainty_note", valueOrEmpty(working.UncertaintyNote))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.UncertaintyNote = line
	}

	// criterion_ref (string field)
	line, ok = readField("criterion_ref", valueOrEmpty(working.CriterionRef))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.CriterionRef = line
	}

	// All 8 fields read — edit completed.
	return working, true, nil
}

// filterReviewable returns the subset of drafts that should be presented in
// the review session. Drafts with ExtractionStage == "weak-draft" are always
// included.
//
// If no draft in the slice has any ExtractionStage set (i.e., no stage
// metadata exists in the dataset), all drafts are returned as a fallback so
// that legacy or partially-annotated datasets are still reviewable.
func filterReviewable(drafts []schema.TraceDraft) []schema.TraceDraft {
	// Check whether any draft has a stage annotation.
	anyStage := false
	for _, d := range drafts {
		if d.ExtractionStage != "" {
			anyStage = true
			break
		}
	}

	if !anyStage {
		// No stage metadata present: present all drafts so legacy datasets
		// are not silently excluded. The returned slice is a shallow copy of
		// the TraceDraft values — slice fields within each struct (Source,
		// Target, Tags, IntentionallyBlank) share backing arrays with the
		// originals. The queue is read-only in RunReviewSession; only
		// deriveAccepted (which deep-copies) produces modified drafts.
		result := make([]schema.TraceDraft, len(drafts))
		copy(result, drafts)
		return result
	}

	// Stage metadata present: include only "weak-draft" records.
	var result []schema.TraceDraft
	for _, d := range drafts {
		if d.ExtractionStage == "weak-draft" {
			result = append(result, d)
		}
	}
	return result
}

// cloneStrings returns a new slice containing the same elements as src.
// Returns nil when src is nil. This prevents the derived draft from sharing
// a backing array with the parent — mutations to one do not affect the other.
func cloneStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
