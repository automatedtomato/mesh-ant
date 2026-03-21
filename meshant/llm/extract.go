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

// RunExtraction calls the LLM to produce candidate TraceDraft records from
// a source document. It always returns a non-nil SessionRecord, even on error.
//
// Data flow:
//  1. Read source document from opts.InputPath (capped at maxSourceBytes)
//  2. Load prompt template from opts.PromptTemplatePath
//  3. Call client.Complete(ctx, systemInstructions, sourceDoc)
//  4. Parse LLM response as JSON array of partial TraceDraft
//  5. For each parsed draft: assign UUID, set ExtractedBy, ExtractionStage,
//     SessionRef, UncertaintyNote (framework-appended), SourceDocRef
//  6. Validate IntentionallyBlank field names against knownContentFields
//  7. Validate each draft via schema.TraceDraft.Validate()
//  8. Return drafts + SessionRecord
//
// The SessionRecord is returned on every code path. On error, DraftCount is 0
// and ErrorNote carries the reason. The caller writes the SessionRecord to disk.
func RunExtraction(ctx context.Context, client LLMClient, opts ExtractionOptions) ([]schema.TraceDraft, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Build a partial SessionRecord early so we can populate ErrorNote and
	// return it on any error path below.
	rec := SessionRecord{
		ID:        sessionID,
		Command:   "extract",
		InputPath: opts.InputPath,
		OutputPath: opts.OutputPath,
		Timestamp: now,
	}

	// Step 1: Read source document.
	sourceDoc, err := readSourceDoc(opts.InputPath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	// Step 2: Load prompt template.
	systemInstructions, err := LoadPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	// Populate ExtractionConditions now that we have all the pieces.
	rec.Conditions = ExtractionConditions{
		ModelID:            opts.ModelID,
		PromptTemplate:     opts.PromptTemplatePath,
		CriterionRef:       opts.CriterionRef,
		SystemInstructions: systemInstructions,
		SourceDocRef:       opts.SourceDocRef,
		Timestamp:          now,
	}

	// Step 3: Call the LLM.
	rawResponse, err := client.Complete(ctx, systemInstructions, sourceDoc)
	if err != nil {
		rec.ErrorNote = fmt.Sprintf("LLM client error: %v", err)
		return nil, rec, fmt.Errorf("llm: complete: %w", err)
	}

	// Step 4: Detect refusals before attempting JSON parse.
	if isRefusal(rawResponse) {
		refErr := &ErrLLMRefusal{RefusalText: rawResponse}
		rec.ErrorNote = refErr.Error()
		return nil, rec, refErr
	}

	// Step 5: Parse response as JSON array of TraceDraft.
	drafts, err := parseResponse(rawResponse)
	if err != nil {
		malformed := &ErrMalformedOutput{RawResponse: rawResponse, ParseErr: err}
		rec.ErrorNote = malformed.Error()
		return nil, rec, malformed
	}

	// Step 6: Post-process each draft — assign provenance, validate blanks.
	processed := make([]schema.TraceDraft, 0, len(drafts))
	for i := range drafts {
		d := &drafts[i]

		// Validate IntentionallyBlank field names (D7).
		if err := validateIntentionallyBlank(d.IntentionallyBlank); err != nil {
			rec.ErrorNote = fmt.Sprintf("draft %d: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: %w", i, err)
		}

		// Assign a fresh UUID.
		id, err := loader.NewUUID()
		if err != nil {
			rec.ErrorNote = fmt.Sprintf("draft %d: generate UUID: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: generate UUID: %w", i, err)
		}
		d.ID = id
		d.Timestamp = now

		// Set framework-assigned provenance (D2, D4, F.0).
		d.ExtractedBy = opts.ModelID
		d.ExtractionStage = "weak-draft"
		d.SessionRef = sessionID
		d.SourceDocRef = opts.SourceDocRef

		// Append framework uncertainty note (D3). Always appended; never
		// replaced. If the LLM set a note, preserve it and append.
		if d.UncertaintyNote != "" {
			d.UncertaintyNote = d.UncertaintyNote + " " + frameworkUncertaintyNote
		} else {
			d.UncertaintyNote = frameworkUncertaintyNote
		}

		// Validate the draft — only SourceSpan is required.
		if err := d.Validate(); err != nil {
			rec.ErrorNote = fmt.Sprintf("draft %d validation: %v", i, err)
			return nil, rec, fmt.Errorf("llm: draft %d: %w", i, err)
		}

		processed = append(processed, *d)
	}

	// Step 7: Populate remaining SessionRecord fields.
	rec.DraftCount = len(processed)
	rec.DraftIDs = make([]string, len(processed))
	for i, d := range processed {
		rec.DraftIDs[i] = d.ID
	}

	return processed, rec, nil
}

// readSourceDoc reads the source document at path, enforcing the maxSourceBytes
// cap. Returns a clear error if the file is missing or too large.
func readSourceDoc(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open source doc %q: %w", path, err)
	}
	defer f.Close()

	// Read one byte beyond the cap to detect oversized files.
	limited := io.LimitReader(f, int64(maxSourceBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read source doc %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: source doc %q exceeds %d bytes", path, maxSourceBytes)
	}
	return string(data), nil
}

// isRefusal reports whether the LLM response looks like an explicit refusal.
// This is a conservative heuristic: only clear refusal prefixes are matched.
// False negatives (undetected refusals that parse as non-JSON) are caught
// by the malformed-output path.
func isRefusal(response string) bool {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return false // empty response → malformed, not refusal
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

// parseResponse parses the LLM's text output as a JSON array of TraceDraft.
// It trims whitespace and attempts to locate the JSON array start marker
// before decoding, tolerating minor LLM preamble.
func parseResponse(raw string) ([]schema.TraceDraft, error) {
	s := strings.TrimSpace(raw)

	// Find the start of the JSON array; some LLMs prefix with a sentence.
	if idx := strings.Index(s, "["); idx >= 0 {
		s = s[idx:]
	}
	// Trim any trailing content after the last ']'.
	if idx := strings.LastIndex(s, "]"); idx >= 0 {
		s = s[:idx+1]
	}

	var drafts []schema.TraceDraft
	if err := json.Unmarshal([]byte(s), &drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

// validateIntentionallyBlank returns an error if any name in the slice is not
// a known content field. Provenance fields cannot be declared intentionally
// blank by the LLM (D7 in docs/decisions/llm-as-mediator-v1.md).
func validateIntentionallyBlank(fields []string) error {
	for _, name := range fields {
		if !knownContentFields[name] {
			return fmt.Errorf("intentionally_blank: %q is not a valid content field name", name)
		}
	}
	return nil
}
