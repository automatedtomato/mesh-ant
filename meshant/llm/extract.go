// extract.go implements RunExtraction — the core LLM extraction operation.
//
// RunExtraction calls an LLM to produce candidate TraceDraft records from one
// or more source documents. It always returns a non-nil SessionRecord, enforces
// all F.1 provenance conventions (D2–D7), and validates IntentionallyBlank
// entries against the known content field list.
//
// Multi-document ingestion (#139): opts.InputPaths and opts.SourceDocRefs are
// parallel slices; one LLM call is made per document. All resulting drafts
// share the same session ID and are aggregated into a single SessionRecord.
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

// maxDocsPerSession caps the number of source documents per extraction session.
// Each document triggers a separate LLM call; an unbounded count risks
// exhausting API quota before the process can be interrupted.
const maxDocsPerSession = 20

// RunExtraction calls the LLM to produce candidate TraceDraft records from one
// or more source documents, enforcing all F.1 provenance conventions (D2–D7).
// Always returns a non-nil SessionRecord; on error DraftCount is 0 and
// ErrorNote carries the reason.
//
// len(opts.InputPaths) must equal len(opts.SourceDocRefs), both must be >= 1,
// and len(opts.InputPaths) must not exceed maxDocsPerSession.
// Each document is processed with a separate LLM call; all resulting drafts
// share the same session ID (single provenance envelope for the session).
func RunExtraction(ctx context.Context, client LLMClient, opts ExtractionOptions) ([]schema.TraceDraft, SessionRecord, error) {
	// Allocate session ID first so every error path can return a valid SessionRecord.
	// Callers (e.g. cmdExtract) write the record unconditionally — an empty ID
	// produces an invalid provenance record on disk.
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Build partial SessionRecord early so every error path returns a valid record.
	rec := SessionRecord{
		ID:         sessionID,
		Command:    "extract",
		InputPaths: opts.InputPaths,
		OutputPath: opts.OutputPath,
		Timestamp:  now,
	}

	// Validate input slices after allocating the session ID so the returned
	// SessionRecord always has a non-empty ID, even on validation failure.
	if len(opts.InputPaths) == 0 {
		rec.ErrorNote = "InputPaths must not be empty"
		return nil, rec, fmt.Errorf("llm: extract: InputPaths must not be empty")
	}
	if len(opts.InputPaths) != len(opts.SourceDocRefs) {
		rec.ErrorNote = fmt.Sprintf("len(InputPaths)=%d != len(SourceDocRefs)=%d",
			len(opts.InputPaths), len(opts.SourceDocRefs))
		return nil, rec, fmt.Errorf("llm: extract: %s", rec.ErrorNote)
	}
	if len(opts.InputPaths) > maxDocsPerSession {
		rec.ErrorNote = fmt.Sprintf("session document count %d exceeds maximum %d",
			len(opts.InputPaths), maxDocsPerSession)
		return nil, rec, fmt.Errorf("llm: extract: %s", rec.ErrorNote)
	}

	// Load the prompt template once — all document calls share the same system
	// instructions, so we avoid re-reading the template on every iteration.
	systemInstructions, err := LoadPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	// Hash the prompt template for reproducibility tracking. The hash detects
	// physical file drift when the same path is reused across sessions.
	promptHash, err := HashPromptTemplate(opts.PromptTemplatePath)
	if err != nil {
		rec.ErrorNote = err.Error()
		return nil, rec, err
	}

	rec.Conditions = ExtractionConditions{
		ModelID:            opts.ModelID,
		PromptTemplate:     opts.PromptTemplatePath,
		PromptHash:         promptHash,
		CriterionRef:       opts.CriterionRef,
		SystemInstructions: systemInstructions,
		SourceDocRefs:      opts.SourceDocRefs,
		AdapterName:        opts.AdapterName,
		Timestamp:          now,
	}

	// Process each document in order, aggregating drafts.
	var allDrafts []schema.TraceDraft
	for i, inputPath := range opts.InputPaths {
		sourceDocRef := opts.SourceDocRefs[i]

		docDrafts, extractErr := extractSingleDoc(
			ctx, client, inputPath, sourceDocRef,
			systemInstructions, opts.ModelID, sessionID, now,
		)
		if extractErr != nil {
			// Record partial progress so the session file on disk accurately
			// reflects what was produced before the failure. Uses sourceDocRef
			// (the analyst-chosen identifier) rather than the raw filesystem
			// path to avoid leaking directory structure into persisted records.
			rec.DraftCount = len(allDrafts)
			rec.DraftIDs = make([]string, len(allDrafts))
			for j, d := range allDrafts {
				rec.DraftIDs[j] = d.ID
			}
			rec.ErrorNote = fmt.Sprintf("document %d (%q): %v", i, sourceDocRef, extractErr)
			return nil, rec, fmt.Errorf("llm: extract: document %d (%q): %w", i, sourceDocRef, extractErr)
		}
		allDrafts = append(allDrafts, docDrafts...)
	}

	rec.DraftCount = len(allDrafts)
	rec.DraftIDs = make([]string, len(allDrafts))
	for i, d := range allDrafts {
		rec.DraftIDs[i] = d.ID
	}

	return allDrafts, rec, nil
}

// extractSingleDoc reads a source document at inputPath, calls the LLM with
// systemInstructions, and stamps each resulting draft with the full set of
// F.1 provenance fields. The sessionID and now are provided by the caller so
// all documents in a multi-doc session share the same provenance envelope.
func extractSingleDoc(
	ctx context.Context,
	client LLMClient,
	inputPath string,
	sourceDocRef string,
	systemInstructions string,
	modelID string,
	sessionID string,
	now time.Time,
) ([]schema.TraceDraft, error) {
	sourceDoc, err := readSourceDoc(inputPath)
	if err != nil {
		return nil, err
	}

	rawResponse, err := client.Complete(ctx, systemInstructions, sourceDoc)
	if err != nil {
		return nil, fmt.Errorf("LLM client error: %w", err)
	}

	if isRefusal(rawResponse) {
		return nil, &ErrLLMRefusal{RefusalText: rawResponse}
	}

	drafts, err := parseResponse(rawResponse)
	if err != nil {
		return nil, &ErrMalformedOutput{RawResponse: rawResponse, ParseErr: err}
	}

	processed := make([]schema.TraceDraft, 0, len(drafts))
	for i := range drafts {
		d := &drafts[i]

		if err := validateIntentionallyBlank(d.IntentionallyBlank); err != nil { // D7
			return nil, fmt.Errorf("draft %d: %w", i, err)
		}
		if err := stampProvenance(d, now, modelID, sessionID, sourceDocRef, "weak-draft"); err != nil {
			return nil, fmt.Errorf("draft %d: %w", i, err)
		}
		if err := d.Validate(); err != nil {
			return nil, fmt.Errorf("draft %d validation: %w", i, err)
		}

		processed = append(processed, *d)
	}

	return processed, nil
}

// parseResponse parses the LLM's text as a JSON array of TraceDraft,
// tolerating minor preamble before the opening '['. If no '[' is found,
// json.Unmarshal will fail and the caller wraps it as ErrMalformedOutput.
func parseResponse(raw string) ([]schema.TraceDraft, error) {
	s, _ := stripPreamble(strings.TrimSpace(raw))

	var drafts []schema.TraceDraft
	if err := json.Unmarshal([]byte(s), &drafts); err != nil {
		return nil, err
	}
	return drafts, nil
}

