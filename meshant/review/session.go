// session.go implements the interactive review session for TraceDraft records.
//
// RunReviewSession walks the reviewer through unreviewed drafts, rendering
// each with its derivation chain and ambiguity warnings, and collecting
// accept/skip/quit decisions.
//
// Each accepted draft produces a new derived TraceDraft via deriveAccepted.
// The derivation records provenance: DerivedFrom names the parent, ExtractedBy
// is "meshant-review" (the session is the cut, not a person), and
// ExtractionStage advances to "reviewed".
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
			fmt.Fprintf(out, "[a]ccept  [s]kip  [q]uit > ")
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
			case "s":
				// Skip: advance to the next draft without producing a result.
				break promptLoop
			case "q":
				// Quit: return everything collected so far.
				break draftLoop
			default:
				// Unknown input: write a re-prompt message and loop again.
				fmt.Fprintf(out, "unknown action -- enter a, s, or q\n")
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
		IntentionallyBlank: cloneStrings(parent.IntentionallyBlank),
		DerivedFrom:        parent.ID,
		ExtractionStage:    "reviewed",
		ExtractedBy:        "meshant-review",
	}, nil
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
