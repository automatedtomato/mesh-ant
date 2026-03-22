// assist.go implements RunAssistSession — the interactive per-span LLM assist operation.
//
// Every LLM draft is preserved regardless of disposition (skip is not absence).
// Accept appends the draft as-is; edit appends the LLM draft and a derived "reviewed"
// draft; skip appends the draft at its original "weak-draft" stage.
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

// ParseSpans decodes a spans file into a slice of span strings, trying in order:
// JSON string array, newline-separated text, then plain text as a single span.
// Blank strings are dropped; errors if no non-blank spans remain.
func ParseSpans(data []byte) ([]string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("ParseSpans: input is empty")
	}

	if trimmed[0] == '[' {
		var raw []string
		if err := json.Unmarshal(trimmed, &raw); err == nil {
			return filterBlanks(raw), nil
		}
	}

	lines := strings.Split(string(trimmed), "\n")
	spans := filterBlanks(lines)
	if len(spans) == 0 {
		return nil, fmt.Errorf("ParseSpans: no non-blank spans found")
	}
	return spans, nil
}

// filterBlanks returns only the non-blank (after TrimSpace) strings from src.
func filterBlanks(src []string) []string {
	var result []string
	for _, s := range src {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

// RunAssistSession presents each span with an LLM-produced draft, collecting
// accept/edit/skip/quit decisions. Always returns a non-nil SessionRecord.
// EOF on in is treated as quit; results collected so far are returned with nil error.
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

	// Empty path is valid — produces an empty system message.
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
		InputPath:  opts.InputPath,
		OutputPath: opts.OutputPath,
		Timestamp:  now,
	}

	if len(spans) == 0 {
		return nil, rec, nil
	}

	scanner := bufio.NewScanner(in)
	var results []schema.TraceDraft

spanLoop:
	for _, span := range spans {
		rawResponse, spanErr := client.Complete(ctx, systemInstructions, span)
		var llmDraft schema.TraceDraft
		if spanErr == nil {
			llmDraft, spanErr = parseSingleDraft(rawResponse, opts.ModelID, sessionID, opts.SourceDocRef, now)
		}

		// On LLM/parse error: offer [s]kip/[q]uit. One bad span should not kill the session.
		// Use span length in the error note, not content — spans may carry PII.
		if spanErr != nil {
			errNotes := append(splitErrNotes(rec.ErrorNote),
				fmt.Sprintf("span (len=%d): %v", len(span), spanErr))
			rec.ErrorNote = joinErrNotes(errNotes)

			fmt.Fprintf(out, "\n--- Span ---\n%s\n\nError: %v\n", span, spanErr)
			fmt.Fprintf(out, "[s]kip span  [q]uit session > ")
			for {
				if !scanner.Scan() {
					break spanLoop
				}
				action := strings.TrimSpace(strings.ToLower(scanner.Text()))
				switch action {
				case "s":
					break // inner for — continue spanLoop
				case "q":
					break spanLoop
				default:
					fmt.Fprintf(out, "unknown action -- enter s or q\n[s]kip span  [q]uit session > ")
					continue
				}
				break
			}
			continue
		}

		fmt.Fprintf(out, "\n--- Span ---\n%s\n\n", span)
		fmt.Fprint(out, review.RenderDraft(llmDraft, 1, 1))
		fmt.Fprintf(out, "Ambiguities:\n")
		fmt.Fprint(out, review.RenderAmbiguities(review.DetectAmbiguities(llmDraft)))

	promptLoop:
		for {
			fmt.Fprintf(out, "[a]ccept  [e]dit  [s]kip  [q]uit > ")
			if !scanner.Scan() {
				break spanLoop // EOF → quit
			}
			action := strings.TrimSpace(strings.ToLower(scanner.Text()))
			switch action {
			case "a":
				results = append(results, llmDraft)
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "accepted",
				})
				break promptLoop

			case "e":
				edited, completed, err := review.RunEditFlow(llmDraft, scanner, out)
				if err != nil {
					return results, rec, err
				}
				if !completed {
					// "abandoned" distinguishes EOF-interrupted edit from a deliberate skip.
					// The draft is still preserved (shadow is not absence).
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
				results = append(results, llmDraft, derivedDraft)
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "edited",
				})
				break promptLoop

			case "s":
				results = append(results, llmDraft) // shadow is not absence
				rec.Dispositions = append(rec.Dispositions, DraftDisposition{
					DraftID: llmDraft.ID,
					Action:  "skipped",
				})
				break promptLoop

			case "q":
				break spanLoop

			default:
				fmt.Fprintf(out, "unknown action -- enter a, e, s, or q\n")
			}
		}
	}

	// DraftCount counts all output drafts including edit-derived ones, so
	// len(Dispositions) may be less than DraftCount when edits occurred.
	rec.DraftCount = len(results)
	rec.DraftIDs = make([]string, len(results))
	for i, d := range results {
		rec.DraftIDs[i] = d.ID
	}

	return results, rec, nil
}

// splitErrNotes splits a semicolon-separated ErrorNote into individual notes.
func splitErrNotes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "; ")
}

// joinErrNotes joins error notes with "; " for storage in ErrorNote.
func joinErrNotes(notes []string) string {
	return strings.Join(notes, "; ")
}

// parseSingleDraft parses one TraceDraft from an LLM response and stamps it
// with F.1 provenance (D2/D3/D4/F.0). Accepts JSON array, JSON object, or
// either with leading preamble text.
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

	// Boolean sentinel avoids retrying a validly-parsed draft with an empty source_span.
	if idx := strings.Index(s, "["); idx >= 0 {
		arraySeg := s[idx:]
		var arr []schema.TraceDraft
		dec := json.NewDecoder(strings.NewReader(arraySeg))
		if err := dec.Decode(&arr); err == nil && len(arr) > 0 {
			draft = arr[0]
			arrayParsed = true
		}
	}

	if !arrayParsed {
		// json.Decoder on s[start:] handles nested braces correctly without manual scanning.
		start := strings.Index(s, "{")
		if start < 0 {
			return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: no JSON object found in response")
		}
		dec := json.NewDecoder(strings.NewReader(s[start:]))
		if err := dec.Decode(&draft); err != nil {
			return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: %w", err)
		}
	}

	// Zero DerivedFrom — an LLM-fabricated UUID would create a false derivation link.
	draft.DerivedFrom = ""

	if err := validateIntentionallyBlank(draft.IntentionallyBlank); err != nil { // D7
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: %w", err)
	}

	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: generate UUID: %w", err)
	}
	draft.ID = id
	draft.Timestamp = now

	// Framework-assigned provenance (D2, D4, F.0).
	draft.ExtractedBy = modelID
	draft.ExtractionStage = "weak-draft"
	draft.SessionRef = sessionID
	draft.SourceDocRef = sourceDocRef

	// Append framework uncertainty note (D3); preserve any LLM-set note.
	if draft.UncertaintyNote != "" {
		draft.UncertaintyNote = draft.UncertaintyNote + " " + frameworkUncertaintyNote
	} else {
		draft.UncertaintyNote = frameworkUncertaintyNote
	}

	if err := draft.Validate(); err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseSingleDraft: validation: %w", err)
	}

	return draft, nil
}
