// critique.go implements RunCritique — the LLM critique operation.
//
// Produces derived drafts with ExtractionStage "critiqued" (naming an LLM
// mediating act, not a pipeline position — Decision D4). SourceSpan integrity
// is a hard check; per-draft failures are recorded in ErrorNote, not returned
// as errors. SessionRecord is always returned (D6).
//
// parseCritiqueDraft is parallel to parseSingleDraft in assist.go; factoring
// into a shared parse.go is deferred (docs/decisions/llm-boundary-v2.md).
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// RunCritique calls the LLM once per input draft to produce a derived "critiqued"
// draft. Always returns a non-nil SessionRecord. If opts.DraftID is set, only
// that draft is processed; missing ID returns an error with a non-nil SessionRecord.
func RunCritique(ctx context.Context, client LLMClient, drafts []schema.TraceDraft, opts CritiqueOptions) ([]schema.TraceDraft, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	rec := SessionRecord{
		ID:         sessionID,
		Command:    "critique",
		InputPaths: []string{opts.InputPath},
		OutputPath: opts.OutputPath,
		Timestamp:  now,
	}

	systemInstructions, err := LoadPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	// Hash the prompt template for reproducibility tracking.
	promptHash, err := HashPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	// Critique sessions use CritiqueConditions (not Conditions) to record apparatus
	// configuration. The distinction is analytically significant: critique input is
	// a TraceDraft array, not a source document; SourceDocRef is singular; no adapter.
	// rec.Conditions is intentionally left zero for critique sessions.
	rec.CritiqueConditions = &CritiqueConditions{
		ModelID:            opts.ModelID,
		PromptTemplate:     opts.PromptTemplatePath,
		PromptHash:         promptHash,
		CriterionRef:       opts.CriterionRef,
		SystemInstructions: systemInstructions,
		SourceDocRef:       opts.SourceDocRef,
		Timestamp:          now,
	}

	target := drafts
	if opts.DraftID != "" {
		target = nil
		for _, d := range drafts {
			if d.ID == opts.DraftID {
				target = []schema.TraceDraft{d}
				break
			}
		}
		if target == nil {
			rec.ErrorNote = fmt.Sprintf("draft ID %q not found in input", opts.DraftID)
			return nil, rec, fmt.Errorf("llm: critique: draft ID %q not found", opts.DraftID)
		}
	}

	var results []schema.TraceDraft
	var errNotes []string

	for _, orig := range target {
		userPrompt := buildCritiquePrompt(orig)

		rawResponse, clientErr := client.Complete(ctx, systemInstructions, userPrompt)
		if clientErr != nil {
			note := fmt.Sprintf("draft %q: LLM error: %v", orig.ID, clientErr)
			errNotes = append(errNotes, note)
			continue
		}

		critiqued, parseErr := parseCritiqueDraft(rawResponse, opts.ModelID, sessionID, orig.SourceDocRef, now)
		if parseErr != nil {
			note := fmt.Sprintf("draft %q: parse error: %v", orig.ID, parseErr)
			errNotes = append(errNotes, note)
			continue
		}

		// Hard check (F.4): LLM must copy SourceSpan verbatim; reject on mismatch.
		if critiqued.SourceSpan != orig.SourceSpan {
			note := fmt.Sprintf("draft %q: SourceSpan mismatch: got %q, want %q",
				orig.ID, critiqued.SourceSpan, orig.SourceSpan)
			errNotes = append(errNotes, note)
			continue
		}

		// Framework sets DerivedFrom and stage after parse (D4); parseCritiqueDraft zeros both.
		critiqued.DerivedFrom = orig.ID
		critiqued.ExtractionStage = "critiqued"

		results = append(results, critiqued)
	}

	if len(errNotes) > 0 {
		rec.ErrorNote = joinErrNotes(errNotes)
	}

	rec.DraftCount = len(results)
	rec.DraftIDs = make([]string, len(results))
	for i, d := range results {
		rec.DraftIDs[i] = d.ID
	}

	return results, rec, nil
}

// buildCritiquePrompt serialises the original draft as JSON context for the LLM.
// Draft fields originate from prior LLM output and may carry adversarial text;
// post-call validation (SourceSpan check, DerivedFrom zeroing, IntentionallyBlank
// validation) mitigates injection attempts targeting provenance fields.
func buildCritiquePrompt(orig schema.TraceDraft) string {
	draftJSON, err := json.MarshalIndent(orig, "", "  ")
	if err != nil {
		// Fallback — should never fire for valid TraceDraft values.
		draftJSON = []byte(fmt.Sprintf(`{"source_span":%q}`, orig.SourceSpan))
	}

	return fmt.Sprintf(
		"Original draft to critique:\n\n%s\n\n"+
			"Produce a critique draft as a single JSON object. "+
			"Copy source_span verbatim. "+
			"Do not set extraction_stage — the framework assigns it after the call. "+
			"Use intentionally_blank for any content field you deliberately leave empty.",
		string(draftJSON),
	)
}

// parseCritiqueDraft parses one TraceDraft from LLM output and stamps provenance
// (D2/D3/D4/D6). DerivedFrom is zeroed (injection guard); IntentionallyBlank is
// validated (D7). ExtractionStage is left empty — RunCritique sets it after the call.
// Uses the same boolean-sentinel pattern as parseSingleDraft (no goto).
func parseCritiqueDraft(raw, modelID, sessionID, sourceDocRef string, now time.Time) (schema.TraceDraft, error) {
	s := strings.TrimSpace(raw)

	var draft schema.TraceDraft
	parsed := false

	if strings.HasPrefix(s, "[") {
		dec := json.NewDecoder(strings.NewReader(s))
		var arr []schema.TraceDraft
		if err := dec.Decode(&arr); err == nil && len(arr) > 0 {
			draft = arr[0]
			parsed = true
		}
	}

	if !parsed {
		idx := strings.Index(s, "{")
		if idx < 0 {
			return schema.TraceDraft{}, &ErrMalformedOutput{
				RawResponse: raw,
				ParseErr:    fmt.Errorf("no JSON object or array found"),
			}
		}
		dec := json.NewDecoder(strings.NewReader(s[idx:]))
		if err := dec.Decode(&draft); err != nil {
			return schema.TraceDraft{}, &ErrMalformedOutput{RawResponse: raw, ParseErr: err}
		}
	}

	draft.DerivedFrom = "" // injection guard; RunCritique sets this after the call

	if err := validateIntentionallyBlank(draft.IntentionallyBlank); err != nil { // D7
		return schema.TraceDraft{}, err
	}

	// ExtractionStage passed as "" — RunCritique owns that value and sets it after
	// the call (avoids two-step override). SourceDocRef is always overwritten:
	// an LLM-supplied value could carry a fabricated or cross-document ref, which
	// is an injection risk in multi-doc sessions (see #139).
	if err := stampProvenance(&draft, now, modelID, sessionID, sourceDocRef, ""); err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseCritiqueDraft: %w", err)
	}

	return draft, nil
}
