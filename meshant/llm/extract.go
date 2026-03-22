// extract.go implements RunExtraction — the core LLM extraction operation.
//
// RunExtraction calls an LLM to produce candidate TraceDraft records from a
// source document. It always returns a non-nil SessionRecord, enforces all
// F.1 provenance conventions (D2–D7), and validates IntentionallyBlank entries
// against the known content field list.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// RunExtraction calls the LLM to produce candidate TraceDraft records from a
// source document, enforcing all F.1 provenance conventions (D2–D7).
// Always returns a non-nil SessionRecord; on error DraftCount is 0 and
// ErrorNote carries the reason.
func RunExtraction(ctx context.Context, client LLMClient, opts ExtractionOptions) ([]schema.TraceDraft, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Build partial SessionRecord early so ErrorNote can be set on any error path.
	rec := SessionRecord{
		ID:        sessionID,
		Command:   "extract",
		InputPath: opts.InputPath,
		OutputPath: opts.OutputPath,
		Timestamp: now,
	}

	sourceDoc, err := readSourceDoc(opts.InputPath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

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

	rawResponse, err := client.Complete(ctx, systemInstructions, sourceDoc)
	if err != nil {
		rec.ErrorNote = fmt.Sprintf("LLM client error: %v", err)
		return nil, rec, fmt.Errorf("llm: complete: %w", err)
	}

	if isRefusal(rawResponse) {
		refErr := &ErrLLMRefusal{RefusalText: rawResponse}
		rec.ErrorNote = refErr.Error()
		return nil, rec, refErr
	}

	drafts, err := parseResponse(rawResponse)
	if err != nil {
		malformed := &ErrMalformedOutput{RawResponse: rawResponse, ParseErr: err}
		rec.ErrorNote = malformed.Error()
		return nil, rec, malformed
	}

	processed := make([]schema.TraceDraft, 0, len(drafts))
	for i := range drafts {
		d := &drafts[i]

		if err := validateIntentionallyBlank(d.IntentionallyBlank); err != nil { // D7
			rec.ErrorNote = fmt.Sprintf("draft %d: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: %w", i, err)
		}

		id, err := loader.NewUUID()
		if err != nil {
			rec.ErrorNote = fmt.Sprintf("draft %d: generate UUID: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: generate UUID: %w", i, err)
		}
		d.ID = id
		d.Timestamp = now

		// Framework-assigned provenance (D2, D4, F.0).
		d.ExtractedBy = opts.ModelID
		d.ExtractionStage = "weak-draft"
		d.SessionRef = sessionID
		d.SourceDocRef = opts.SourceDocRef

		// Append framework uncertainty note (D3); preserve any LLM-set note.
		if d.UncertaintyNote != "" {
			d.UncertaintyNote = d.UncertaintyNote + " " + frameworkUncertaintyNote
		} else {
			d.UncertaintyNote = frameworkUncertaintyNote
		}

		if err := d.Validate(); err != nil {
			rec.ErrorNote = fmt.Sprintf("draft %d validation: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: %w", i, err)
		}

		processed = append(processed, *d)
	}

	rec.DraftCount = len(processed)
	rec.DraftIDs = make([]string, len(processed))
	for i, d := range processed {
		rec.DraftIDs[i] = d.ID
	}

	return processed, rec, nil
}

// readSourceDoc reads the source document at path, enforcing the maxSourceBytes cap.
func readSourceDoc(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open source doc %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxSourceBytes)+1) // +1 to detect oversized files
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read source doc %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: source doc %q exceeds %d bytes", path, maxSourceBytes)
	}
	return string(data), nil
}

// isRefusal reports whether the response looks like an explicit refusal.
// Conservative heuristic — undetected refusals fall through to the malformed-output path.
func isRefusal(response string) bool {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return false // empty → malformed, not refusal
	}
	prefixes := []string{
		"I cannot",
		"I'm sorry",
		"I am sorry",
		"I apologize",
		"I'm unable",
		"I am unable",
	}
	lower := strings.ToLower(trimmed)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// parseResponse parses the LLM's text as a JSON array of TraceDraft,
// tolerating minor preamble before the opening '['.
func parseResponse(raw string) ([]schema.TraceDraft, error) {
	s := strings.TrimSpace(raw)

	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "]"); idx >= 0 {
		s = s[:idx+1]
	}

	var drafts []schema.TraceDraft
	if err := json.Unmarshal([]byte(s), &drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

// validateIntentionallyBlank returns an error if any name is not a known content
// field — provenance fields cannot be declared blank by the LLM (D7).
func validateIntentionallyBlank(fields []string) error {
	for _, name := range fields {
		if !knownContentFields[name] {
			return fmt.Errorf("intentionally_blank: %q is not a valid content field name", name)
		}
	}
	return nil
}
