// split.go implements RunSplit — LLM-assisted span boundary detection.
//
// RunSplit asks the LLM to split a source document into observation spans
// suitable for the assist command. It always returns a non-nil SessionRecord,
// following the same provenance contract as RunExtraction and RunCritique.
//
// Unlike RunExtraction, split produces []string (raw spans), not TraceDraft
// records. DraftIDs is nil in the SessionRecord because spans are not yet
// TraceDraft records.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// RunSplit calls the LLM to split a source document into observation spans.
//
// Returns spans ([]string), SessionRecord (always non-nil), and error.
// DraftCount = len(spans); DraftIDs = nil (spans are not TraceDraft records).
// An empty span slice after blank-filtering is treated as an error: 0 spans
// is a genuine LLM failure, not valid output.
func RunSplit(ctx context.Context, client LLMClient, opts SplitOptions) ([]string, SessionRecord, error) {
	sessionID, err := loader.NewUUID()
	if err != nil {
		return nil, SessionRecord{}, fmt.Errorf("llm: split: generate session ID: %w", err)
	}

	now := time.Now().UTC()

	// Build a partial SessionRecord early so ErrorNote can be set on any error path.
	rec := SessionRecord{
		ID:         sessionID,
		Command:    "split",
		InputPaths: []string{opts.InputPath},
		OutputPath: opts.OutputPath,
		Timestamp:  now,
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
		SystemInstructions: systemInstructions,
		SourceDocRefs:      []string{opts.SourceDocRef},
		Timestamp:          now,
	}

	rawResponse, err := client.Complete(ctx, systemInstructions, sourceDoc)
	if err != nil {
		rec.ErrorNote = fmt.Sprintf("LLM client error: %v", err)
		return nil, rec, fmt.Errorf("llm: split: complete: %w", err)
	}

	if isRefusal(rawResponse) {
		refErr := &ErrLLMRefusal{RefusalText: rawResponse}
		rec.ErrorNote = refErr.Error()
		return nil, rec, refErr
	}

	spans, err := parseSplitResponse(rawResponse)
	if err != nil {
		malformed := &ErrMalformedOutput{RawResponse: rawResponse, ParseErr: err}
		rec.ErrorNote = malformed.Error()
		return nil, rec, malformed
	}

	if len(spans) == 0 {
		zeroErr := fmt.Errorf("llm: split produced 0 spans")
		rec.ErrorNote = zeroErr.Error()
		return nil, rec, zeroErr
	}

	rec.DraftCount = len(spans)
	// DraftIDs remains nil — spans are not TraceDraft records and carry no UUID.

	return spans, rec, nil
}

// parseSplitResponse parses the LLM's JSON array of strings (the span
// boundaries), tolerating preamble text before the opening '['.
// Filters blank strings from the result. Returns an error if the response
// cannot be parsed as a JSON array of strings, or if no '[' is found.
func parseSplitResponse(raw string) ([]string, error) {
	s, found := stripPreamble(strings.TrimSpace(raw))
	if !found {
		return nil, fmt.Errorf("parseSplitResponse: no JSON array found in response")
	}

	var spans []string
	if err := json.Unmarshal([]byte(s), &spans); err != nil {
		return nil, fmt.Errorf("parseSplitResponse: %w", err)
	}

	// Filter blank strings — blank spans are not observation units.
	return filterBlanks(spans), nil
}
