// assist.go implements RunAssistSession — the interactive per-span LLM assist
// operation.
//
// RunAssistSession presents one LLM-produced TraceDraft candidate per span to
// the user, who can accept, edit, skip, or quit. Every LLM draft is preserved
// in the results regardless of disposition (skip is not absence — the shadow
// record is retained). Accept appends the draft as-is; edit appends both the
// original LLM draft and a derived "reviewed" draft; skip appends the draft
// with its original "weak-draft" stage.
//
// ParseSpans handles three input formats: JSON string array, newline-separated
// text, and plain text (single span). Callers read a spans file and pass its
// bytes directly to ParseSpans.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/review"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// ParseSpans decodes a spans file into a slice of span strings.
//
// Three formats are tried in order:
//  1. JSON string array (e.g. `["span A","span B"]`)
//  2. Newline-separated text (each non-blank line is one span)
//  3. Plain text (the entire trimmed content as a single span)
//
// Blank strings and whitespace-only strings are always dropped.
// Returns an error if data is empty (after trimming whitespace) or if no
// non-blank spans remain after parsing.
func ParseSpans(data []byte) ([]string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("ParseSpans: input is empty")
	}

	// Attempt 1: JSON string array.
	if trimmed[0] == '[' {
		var raw []string
		if err := json.Unmarshal(trimmed, &raw); err == nil {
			return filterBlanks(raw), nil
		}
	}

	// Attempt 2 & 3: newline-separated or single-line text.
	lines := strings.Split(string(trimmed), "\n")
	spans := filterBlanks(lines)
	if len(spans) == 0 {
		return nil, fmt.Errorf("ParseSpans: no non-blank spans found")
	}
	return spans, nil
}

// filterBlanks returns a new slice containing only the non-blank (after
// TrimSpace) strings from src.
func filterBlanks(src []string) []string {
	var result []string
	for _, s := range src {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

// RunAssistSession presents each span to the user in turn with an LLM-produced
// draft, collecting accept/edit/skip/quit decisions.
//
// For each span:
//  1. Call client.Complete(ctx, systemInstructions, span) to produce one draft.
//  2. Parse and stamp the draft with provenance (UUID, ExtractedBy=ModelID,
//     ExtractionStage="weak-draft", SessionRef, UncertaintyNote).
//  3. Present the draft to the user and read one action:
//     - Accept ("a"): append the LLM draft; disposition="accepted".
//     - Edit ("e"): run RunEditFlow, derive edited draft via
//     review.DeriveEdited(llmDraft, edited, "meshant-assist"); append both;
//     disposition="edited".
//     - Skip ("s"): append the LLM draft (shadow is not absence);
//     disposition="skipped".
//     - Quit ("q"): break the loop; return results collected so far (not an
//     error).
//
// A SessionRecord is always returned, even on error. On LLM error the
// SessionRecord carries a non-empty ErrorNote and the error is propagated.
//
// EOF on in is treated as quit: results so far are returned with nil error.
func RunAssistSession(
	ctx context.Context,
	client LLMClient,
	spans []string,
	opts AssistOptions,
	in io.Reader,
	out io.Writer,
) ([]schema.TraceDraft, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Load prompt template if provided. An empty path skips the load — callers
	// can provide inline instructions via future options; today an empty
	// template produces an empty system message.
	var systemInstructions string
	if opts.PromptTemplatePath != "" {
		systemInstructions, err = LoadPromptTemplate(opts.PromptTemplatePath)
		if err != nil {
			rec := SessionRecord{
				ID:        sessionID,
				Command:   "assist",
				Timestamp: now,
				ErrorNote: err.Error(),
			}
			return nil, rec, err
		}
	}

	rec := SessionRecord{
		ID:      sessionID,
		Command: "assist",
		Conditions: ExtractionConditions{
			ModelID:            opts.ModelID,
			PromptTemplate:     opts.PromptTemplatePath,
			CriterionRef:       opts.CriterionRef,
			SystemInstructions: systemInstructions,
			SourceDocRef:       opts.SourceDocRef,
			Timestamp:          now,
		},
		InputPath:  opts.InputPath,  // spans file path for provenance tracing
		OutputPath: opts.OutputPath,
		Timestamp:  now,
	}

	if len(spans) == 0 {
		// No spans to process — return an empty but valid session.
		return nil, rec, nil
	}

	scanner := bufio.NewScanner(in)
	var results []schema.TraceDraft

	// spanLoop processes each span in order. A "quit" action breaks out of
	// the loop; results collected so far are returned with nil error.
spanLoop:
	for _, span := range spans {
		// Call the LLM for this span.
		rawResponse, err := client.Complete(ctx, systemInstructions, span)
		if err != nil {
			// Use span length rather than content in ErrorNote — spans may
			// contain PII from source documents; the session file is often
			// written to a shared or version-controlled directory.
			rec.ErrorNote = fmt.Sprintf("LLM client error on span (len=%d): %v", len(span), err)
			return nil, rec, fmt.Errorf("llm: assist: complete: %w", err)
		}

		// Parse the single-draft response.
		llmDraft, err := parseSingleDraft(rawResponse, opts.ModelID, sessionID, opts.SourceDocRef, now)
		if err != nil {
			rec.ErrorNote = fmt.Sprintf("parse draft for span (len=%d): %v", len(span), err)
			return nil, rec, fmt.Errorf("llm: assist: %w", err)
		}

		// Render the draft and prompt the user.
		fmt.Fprintf(out, "\n--- Span ---\n%s\n\n", span)
		fmt.Fprint(out, review.RenderDraft(llmDraft, 1, 1))
		fmt.Fprintf(out, "Ambiguities:\n")
		fmt.Fprint(out, review.RenderAmbiguities(review.DetectAmbiguities(llmDraft)))

		// Inner prompt loop: re-prompt on unknown input.
	promptLoop:
		for {
			fmt.Fprintf(out, "[a]ccept  [e]dit  [s]kip  [q]uit > ")
			if !scanner.Scan() {
				// EOF or scanner error — treat as quit.
				break spanLoop
			}
			action := strings.TrimSpace(strings.ToLower(scanner.Text()))
			switch action {
			case "a":
				// Accept: append the LLM draft; disposition="accepted".
				results = append(results, llmDraft)
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "accepted",
				})
				break promptLoop

			case "e":
				// Edit: run field-by-field edit flow, then derive an edited draft.
				edited, completed, err := review.RunEditFlow(llmDraft, scanner, out)
				if err != nil {
					return results, rec, err
				}
				if !completed {
					// EOF during edit — distinct from an explicit skip. The draft
					// is preserved (shadow is not absence) but the disposition is
					// "abandoned" so provenance auditors can distinguish an
					// edit-interrupted-by-EOF from a deliberate user skip.
					results = append(results, llmDraft)
					rec.Dispositions = append(rec.Dispositions, DraftDisposition{
						DraftID: llmDraft.ID,
						Action:  "abandoned",
					})
					break spanLoop
				}
				derivedDraft, err := review.DeriveEdited(llmDraft, edited, "meshant-assist")
				if err != nil {
					return results, rec, fmt.Errorf("llm: assist: derive edited: %w", err)
				}
				// Append both: LLM draft preserved, edited draft added.
				results = append(results, llmDraft, derivedDraft)
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "edited",
				})
				break promptLoop

			case "s":
				// Skip: preserve the LLM draft (shadow is not absence) with
				// its original "weak-draft" stage.
				results = append(results, llmDraft)
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "skipped",
				})
				break promptLoop

			case "q":
				// Quit: return results collected so far with nil error.
				break spanLoop

			default:
				fmt.Fprintf(out, "unknown action -- enter a, e, s, or q\n")
			}
		}
	}

	// Populate final SessionRecord fields. Note: DraftCount and DraftIDs count
	// ALL output drafts, including edit-derived drafts. Dispositions has one
	// entry per span processed (not per output draft), so len(Dispositions)
	// may be less than DraftCount when any span was edited (2 drafts, 1 entry).
	// Downstream tools should not assume DraftCount == len(Dispositions).
	rec.DraftCount = len(results)
	rec.DraftIDs = make([]string, len(results))
	for i, d := range results {
		rec.DraftIDs[i] = d.ID
	}

	return results, rec, nil
}

// parseSingleDraft parses an LLM response expected to contain exactly one
// TraceDraft and stamps it with provenance fields following F.1 conventions.
//
// The LLM response may be:
//   - A JSON array with one object: `[{...}]`
//   - A JSON object: `{...}`
//   - Either form with preamble text before the JSON
//
// Provenance fields set:
//   - ID: fresh UUID
//   - Timestamp: now
//   - ExtractedBy: modelID (D2)
//   - ExtractionStage: "weak-draft" (D4)
//   - SessionRef: sessionID (F.0)
//   - UncertaintyNote: frameworkUncertaintyNote appended (D3)
//   - SourceDocRef: sourceDocRef
func parseSingleDraft(
	raw string,
	modelID string,
	sessionID string,
	sourceDocRef string,
	now time.Time,
) (schema.TraceDraft, error) {
	s := strings.TrimSpace(raw)

	var draft schema.TraceDraft
	arrayParsed := false

	// Try to decode as a JSON array first (most common LLM response format).
	// Use a boolean sentinel to track parse success — not a content field,
	// so that a validly-parsed draft with an empty source_span is not silently
	// discarded and retried.
	if idx := strings.Index(s, "["); idx >= 0 {
		arraySeg := s[idx:]
		var arr []schema.TraceDraft
		// json.Decoder reads exactly one JSON value — handles preamble correctly
		// and stops at the matching ']' without string-scanning for the last ']'.
		dec := json.NewDecoder(strings.NewReader(arraySeg))
		if err := dec.Decode(&arr); err == nil && len(arr) > 0 {
			draft = arr[0]
			arrayParsed = true
		}
	}

	// Fall back to JSON object if array parse did not succeed.
	if !arrayParsed {
		// Locate the start of the outermost JSON object. Use json.Decoder on
		// the substring starting at '{' so that nested braces are handled
		// correctly — no manual LastIndex scan that could match a brace in
		// preamble or trailing text.
		start := strings.Index(s, "{")
		if start < 0 {
			return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: no JSON object found in response")
		}
		dec := json.NewDecoder(strings.NewReader(s[start:]))
		if err := dec.Decode(&draft); err != nil {
			return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: %w", err)
		}
	}

	// Sanitise LLM-controlled fields that should not propagate into the
	// output record. DerivedFrom is a derivation-chain foreign key — if the
	// LLM fabricates a UUID it would create a false derivation link in
	// FollowDraftChain. Zero it before stamping framework provenance.
	draft.DerivedFrom = ""

	// Validate IntentionallyBlank entries: only known content field names
	// are valid (D7). The LLM cannot declare provenance fields as blank.
	if err := validateIntentionallyBlank(draft.IntentionallyBlank); err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: %w", err)
	}

	// Assign a fresh UUID.
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: generate UUID: %w", err)
	}
	draft.ID = id
	draft.Timestamp = now

	// Set framework-assigned provenance (D2, D4, F.0).
	draft.ExtractedBy = modelID
	draft.ExtractionStage = "weak-draft"
	draft.SessionRef = sessionID
	draft.SourceDocRef = sourceDocRef

	// Append framework uncertainty note (D3).
	if draft.UncertaintyNote != "" {
		draft.UncertaintyNote = draft.UncertaintyNote + " " + frameworkUncertaintyNote
	} else {
		draft.UncertaintyNote = frameworkUncertaintyNote
	}

	// Validate: SourceSpan is required.
	if err := draft.Validate(); err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: validation: %w", err)
	}

	return draft, nil
}
