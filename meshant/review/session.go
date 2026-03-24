// session.go implements the interactive review session for TraceDraft records.
//
// RunReviewSession walks the reviewer through unreviewed drafts, rendering
// each with its derivation chain and ambiguity warnings, and collecting
// accept/edit/skip/quit decisions.
//
// Each accepted or edited draft produces a new derived TraceDraft via
// DeriveAccepted or DeriveEdited respectively. The derivation records
// provenance: DerivedFrom names the parent, ExtractedBy is supplied by the
// caller (e.g. "meshant-review" or "meshant-assist"), and ExtractionStage
// advances to "reviewed".
//
// DeriveAccepted, DeriveEdited, and RunEditFlow are exported so that the llm
// package can reuse them in the assist session without duplicating derivation
// logic.
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
// Accept: calls DeriveAccepted to produce a new derived draft and appends it
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

draftLoop:
	for i, d := range queue {
		chain := loader.FollowDraftChain(drafts, d.ID)
		classes := loader.ClassifyDraftChain(chain, loader.ClassifyDraftChainOptions{}).Classifications

		fmt.Fprint(out, RenderChain(chain, classes))
		fmt.Fprint(out, RenderDraft(d, i+1, total))
		fmt.Fprintf(out, "Ambiguities:\n")
		fmt.Fprint(out, RenderAmbiguities(DetectAmbiguities(d)))

	promptLoop:
		for {
			fmt.Fprintf(out, "[a]ccept  [e]dit  [s]kip  [q]uit > ")
			if !scanner.Scan() {
				break draftLoop // EOF or scanner error — treat as quit
			}
			switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
			case "a":
				derived, err := DeriveAccepted(d, "meshant-review")
				if err != nil {
					return results, err
				}
				results = append(results, derived)
				break promptLoop
			case "e":
				edited, completed, err := RunEditFlow(d, scanner, out)
				if err != nil {
					return results, err
				}
				if !completed {
					break draftLoop // EOF during edit — treat as quit
				}
				derived, err := DeriveEdited(d, edited, "meshant-review")
				if err != nil {
					return results, err
				}
				results = append(results, derived)
				break promptLoop
			case "s":
				break promptLoop
			case "q":
				break draftLoop
			default:
				fmt.Fprintf(out, "unknown action -- enter a, e, s, or q\n")
			}
		}
	}

	return results, nil
}

// DeriveAccepted creates a new TraceDraft derived from parent with all content
// fields copied and ExtractionStage advanced to "reviewed". Slice fields are
// deep-copied so the derived draft is independent of the parent.
func DeriveAccepted(parent schema.TraceDraft, extractedBy string) (schema.TraceDraft, error) {
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("DeriveAccepted: generate UUID: %w", err)
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
		ExtractedBy:        extractedBy,
	}, nil
}

// parseCommaSeparated splits s on commas, trims whitespace, drops empty elements.
// Returns nil when no non-empty elements remain.
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

// DeriveEdited creates a new TraceDraft combining reviewer edits with parent
// provenance. Content fields come from edited; SourceSpan, SourceDocRef, and
// IntentionallyBlank come from parent. All slice fields are deep-copied.
func DeriveEdited(parent schema.TraceDraft, edited schema.TraceDraft, extractedBy string) (schema.TraceDraft, error) {
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("DeriveEdited: generate UUID: %w", err)
	}

	return schema.TraceDraft{
		ID:                 id,
		Timestamp:          time.Now(),
		SourceSpan:         parent.SourceSpan,
		SourceDocRef:       parent.SourceDocRef,
		SessionRef:         parent.SessionRef,
		IntentionallyBlank: cloneStrings(parent.IntentionallyBlank),
		WhatChanged:        edited.WhatChanged,
		Source:             cloneStrings(edited.Source),
		Target:             cloneStrings(edited.Target),
		Mediation:          edited.Mediation,
		Observer:           edited.Observer,
		Tags:               cloneStrings(edited.Tags),
		UncertaintyNote:    edited.UncertaintyNote,
		CriterionRef:       edited.CriterionRef,
		DerivedFrom:        parent.ID,
		ExtractionStage:    "reviewed",
		ExtractedBy:        extractedBy,
	}, nil
}

// RunEditFlow presents each of 8 editable fields in sequence and reads
// replacements from scanner. Empty line keeps current value.
// Returns (draft, true, nil) when all fields are read; (draft, false, nil) on EOF.
func RunEditFlow(d schema.TraceDraft, scanner *bufio.Scanner, out io.Writer) (schema.TraceDraft, bool, error) {
	working := d

	readField := func(fieldName, current string) (string, bool) {
		fmt.Fprintf(out, "  %s [%s]: ", fieldName, current)
		if !scanner.Scan() {
			return "", false
		}
		return strings.TrimSpace(scanner.Text()), true
	}

	line, ok := readField("what_changed", valueOrEmpty(working.WhatChanged))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.WhatChanged = line
	}

	line, ok = readField("source", sliceOrEmpty(working.Source))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Source = parsed
		}
	}

	line, ok = readField("target", sliceOrEmpty(working.Target))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Target = parsed
		}
	}

	line, ok = readField("mediation", valueOrEmpty(working.Mediation))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.Mediation = line
	}

	line, ok = readField("observer", valueOrEmpty(working.Observer))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.Observer = line
	}

	line, ok = readField("tags", sliceOrEmpty(working.Tags))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		if parsed := parseCommaSeparated(line); parsed != nil {
			working.Tags = parsed
		}
	}

	line, ok = readField("uncertainty_note", valueOrEmpty(working.UncertaintyNote))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.UncertaintyNote = line
	}

	line, ok = readField("criterion_ref", valueOrEmpty(working.CriterionRef))
	if !ok {
		return working, false, nil
	}
	if line != "" {
		working.CriterionRef = line
	}

	return working, true, nil
}

// filterReviewable returns drafts with ExtractionStage "weak-draft" or "critiqued".
// "critiqued" drafts are LLM suggestions that benefit from human review (Decision 5).
// Falls back to all drafts when no stage metadata is present.
func filterReviewable(drafts []schema.TraceDraft) []schema.TraceDraft {
	anyStage := false
	for _, d := range drafts {
		if d.ExtractionStage != "" {
			anyStage = true
			break
		}
	}

	if !anyStage {
		// No stage metadata — present all drafts (legacy dataset fallback).
		result := make([]schema.TraceDraft, len(drafts))
		copy(result, drafts)
		return result
	}

	var result []schema.TraceDraft
	for _, d := range drafts {
		if d.ExtractionStage == "weak-draft" || d.ExtractionStage == "critiqued" {
			result = append(result, d)
		}
	}
	return result
}

// cloneStrings returns a copy of src. Returns nil for nil input.
func cloneStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
