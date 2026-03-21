// critique.go implements RunCritique — the LLM critique operation.
//
// RunCritique takes a slice of existing TraceDraft records, sends each to the
// LLM with the critique prompt, and produces derived drafts with
// ExtractionStage "critiqued" and DerivedFrom linking to the original.
//
// Key design constraints (from F.4):
//   - SourceSpan integrity is a hard check: if the LLM returns a different
//     SourceSpan than the original, that draft is rejected and the session
//     continues to the next draft.
//   - Partial results are valid: RunCritique processes all drafts even if
//     some fail. The returned error is nil; per-draft failures are recorded
//     in SessionRecord.ErrorNote.
//   - SessionRecord is always returned, even if all drafts fail (D6).
//   - ExtractionStage is "critiqued" (not "weak-draft"): it names an LLM
//     mediating act, not a position in the extraction pipeline. See
//     docs/decisions/llm-as-mediator-v1.md, Decision 4.
//
// parseCritiqueDraft is a package-private helper that shares logic with
// parseSingleDraft in assist.go. When F.3 merges to develop, the two can be
// factored into a shared parse.go; until then they are parallel implementations
// in the same package.
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

// RunCritique calls the LLM once per input draft to produce a derived
// "critiqued" draft. It returns a non-nil SessionRecord on all code paths
// except session ID generation failure (loader.NewUUID error), which is
// vanishingly rare and consistent with RunExtraction's behaviour.
//
// For each input draft:
//  1. Build a critique prompt from the system instructions + original draft
//  2. Call client.Complete — on error, record in ErrorNote and continue
//  3. Parse the response as a single TraceDraft (JSON object or array)
//  4. Validate SourceSpan matches original — reject on mismatch, continue
//  5. Set DerivedFrom and ExtractionStage (framework-assigned, not LLM)
//
// If opts.DraftID is non-empty, only the draft with that ID is processed.
// If no draft has that ID, an error is returned (with a non-nil SessionRecord).
func RunCritique(ctx context.Context, client LLMClient, drafts []schema.TraceDraft, opts CritiqueOptions) ([]schema.TraceDraft, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Build partial SessionRecord early so we can return it on any error path.
	rec := SessionRecord{
		ID:         sessionID,
		Command:    "critique",
		InputPath:  opts.InputPath,
		OutputPath: opts.OutputPath,
		Timestamp:  now,
	}

	// Load prompt template (system instructions).
	systemInstructions, err := LoadPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	rec.Conditions = ExtractionConditions{
		ModelID:            opts.ModelID,
		PromptTemplate:     opts.PromptTemplatePath,
		CriterionRef:       opts.CriterionRef,
		SystemInstructions: systemInstructions,
		SourceDocRef:       opts.SourceDocRef,
		Timestamp:          now,
	}

	// Apply ID filter if requested.
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

	// Process each draft.
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

		// Hard check: SourceSpan must match the original exactly (F.4 constraint).
		// The critique prompt instructs the LLM to copy it verbatim; a mismatch
		// means the LLM deviated. Reject rather than silently mutate the chain.
		if critiqued.SourceSpan != orig.SourceSpan {
			note := fmt.Sprintf("draft %q: SourceSpan mismatch: got %q, want %q",
				orig.ID, critiqued.SourceSpan, orig.SourceSpan)
			errNotes = append(errNotes, note)
			continue
		}

		// Framework-assigned derivation fields. parseCritiqueDraft zeros
		// DerivedFrom (LLM injection guard) and leaves ExtractionStage empty.
		// Both are set here with known-good values from the framework:
		// DerivedFrom names the original draft; "critiqued" names the LLM
		// mediating act (Decision D4, llm-as-mediator-v1.md).
		critiqued.DerivedFrom = orig.ID
		critiqued.ExtractionStage = "critiqued"

		results = append(results, critiqued)
	}

	// Accumulate all per-draft errors into ErrorNote.
	if len(errNotes) > 0 {
		rec.ErrorNote = strings.Join(errNotes, "; ")
	}

	// Populate SessionRecord outcome fields.
	rec.DraftCount = len(results)
	rec.DraftIDs = make([]string, len(results))
	for i, d := range results {
		rec.DraftIDs[i] = d.ID
	}

	return results, rec, nil
}

// buildCritiquePrompt assembles the user-facing prompt for a single critique
// call. It serialises the original draft's fields as a JSON context block
// and asks for a single JSON object response.
//
// The system instructions (critique_pass.md) define the methodological
// contract; this prompt provides the specific draft to critique.
//
// Security note: draft fields are analyst-produced, not end-user input, but
// they originate from prior LLM output and could contain adversarial text
// (prompt injection). The system instructions (critique_pass.md) are the
// authoritative contract; this prompt is treated as untrusted data embedded in
// a structured block. The framework's post-call validation (SourceSpan check,
// DerivedFrom zeroing, IntentionallyBlank validation) mitigates injection
// attempts that try to mutate provenance fields.
func buildCritiquePrompt(orig schema.TraceDraft) string {
	// Serialise the original draft to give the LLM its full context.
	// Use MarshalIndent for readability in the prompt.
	draftJSON, err := json.MarshalIndent(orig, "", "  ")
	if err != nil {
		// Fallback: use minimal fields. This branch should never fire for
		// valid TraceDraft values (all fields are JSON-serialisable).
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

// parseCritiqueDraft parses a single TraceDraft from the LLM's raw text output.
// It handles three forms:
//   - JSON array with one element: [{...}]
//   - JSON object: {...}
//   - LLM preamble before the JSON (common when LLMs explain before answering)
//
// After parsing:
//   - DerivedFrom is zeroed (injection guard: the LLM must not set this field)
//   - IntentionallyBlank is validated against knownContentFields (D7)
//   - Provenance fields are stamped (D2/D3/D4/D6)
//
// This function is parallel to parseSingleDraft in assist.go. When F.3 merges
// to develop, the two can be factored into a shared helper in parse.go.
func parseCritiqueDraft(raw, modelID, sessionID, sourceDocRef string, now time.Time) (schema.TraceDraft, error) {
	s := strings.TrimSpace(raw)

	var draft schema.TraceDraft
	var parseErr error

	// Try JSON array first (LLM may wrap in an array even when asked for one object).
	if strings.HasPrefix(s, "[") {
		dec := json.NewDecoder(strings.NewReader(s))
		var arr []schema.TraceDraft
		if err := dec.Decode(&arr); err == nil && len(arr) > 0 {
			draft = arr[0]
			goto stamp
		}
	}

	// Try JSON object.
	if idx := strings.Index(s, "{"); idx >= 0 {
		dec := json.NewDecoder(strings.NewReader(s[idx:]))
		if err := dec.Decode(&draft); err != nil {
			parseErr = err
		} else {
			goto stamp
		}
	}

	if parseErr != nil {
		return schema.TraceDraft{}, &ErrMalformedOutput{RawResponse: raw, ParseErr: parseErr}
	}
	return schema.TraceDraft{}, &ErrMalformedOutput{
		RawResponse: raw,
		ParseErr:    fmt.Errorf("no JSON object or array found"),
	}

stamp:
	// Zero DerivedFrom — the LLM must not inject false derivation chain links.
	// RunCritique will set this to the original draft's ID after the call.
	draft.DerivedFrom = ""

	// Validate IntentionallyBlank field names (D7).
	if err := validateIntentionallyBlank(draft.IntentionallyBlank); err != nil {
		return schema.TraceDraft{}, err
	}

	// Assign fresh UUID and timestamp.
	id, err := loader.NewUUID()
	if err != nil {
		return schema.TraceDraft{}, fmt.Errorf("parseCritiqueDraft: generate UUID: %w", err)
	}
	draft.ID = id
	draft.Timestamp = now

	// Framework-assigned provenance (D2, D6).
	// ExtractionStage is intentionally left empty here — RunCritique sets it
	// to "critiqued" after the call. Leaving it empty rather than writing a
	// placeholder avoids the two-step override trap and makes the contract
	// explicit: this function does not own the stage value.
	draft.ExtractedBy = modelID
	draft.ExtractionStage = ""
	draft.SessionRef = sessionID
	if draft.SourceDocRef == "" {
		draft.SourceDocRef = sourceDocRef
	}

	// Append framework uncertainty note (D3). Always appended; never replaced.
	if draft.UncertaintyNote != "" {
		draft.UncertaintyNote = draft.UncertaintyNote + " " + frameworkUncertaintyNote
	} else {
		draft.UncertaintyNote = frameworkUncertaintyNote
	}

	return draft, nil
}
