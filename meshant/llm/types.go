// types.go defines the types shared across the llm package.
//
// The central concern is provenance: every LLM interaction produces a
// SessionRecord that records the conditions under which drafts were made.
// ExtractionConditions deliberately excludes the API key — conditions are
// written to disk and must never carry secrets.
package llm

import (
	"fmt"
	"time"
)

// ExtractionConditions records the apparatus configuration for one LLM
// session. It is embedded in SessionRecord and written to disk.
//
// The API key is intentionally absent. Two runs with different keys but the
// same conditions are analytically indistinguishable — the key is an
// operational detail, not an analytical position.
type ExtractionConditions struct {
	ModelID            string    `json:"model_id"`
	PromptTemplate     string    `json:"prompt_template"`
	CriterionRef       string    `json:"criterion_ref,omitempty"`
	SystemInstructions string    `json:"system_instructions"`
	SourceDocRef       string    `json:"source_doc_ref"`
	Timestamp          time.Time `json:"timestamp"`
}

// DraftDisposition records the reviewer's decision about a single draft within a session.
// Used by assist and critique sessions; empty in extract sessions (all
// drafts from extract are implicitly "accepted" into the output).
//
// Action values:
//   - "accepted"  — reviewer confirmed the LLM draft as-is
//   - "edited"    — reviewer modified the draft via RunEditFlow
//   - "skipped"   — reviewer deliberately chose not to engage
//   - "abandoned" — reviewer entered edit mode but EOF interrupted before completion
//
// "skipped" and "abandoned" are distinct: skip is a deliberate non-engagement;
// abandon is an interrupted articulation. Both preserve the LLM draft in
// the output (shadow is not absence).
type DraftDisposition struct {
	DraftID string `json:"draft_id"`
	Action  string `json:"action"`
}

// SessionRecord is the mandatory companion to every LLM interaction. It is
// returned on every code path — success, error, and partial completion.
//
// SessionRecord.ID is the SessionRef stamped on every draft produced in
// the session, linking each draft back to the conditions under which it
// was produced (FM4 from plan_thread_f.md).
type SessionRecord struct {
	ID           string               `json:"id"`
	Command      string               `json:"command"` // "extract", "assist", "critique"
	Conditions   ExtractionConditions `json:"conditions"`
	DraftIDs     []string             `json:"draft_ids"`
	Dispositions []DraftDisposition   `json:"dispositions,omitempty"`
	InputPath    string               `json:"input_path"`
	OutputPath   string               `json:"output_path"`
	DraftCount   int                  `json:"draft_count"`
	ErrorNote    string               `json:"error_note,omitempty"`
	Timestamp    time.Time            `json:"timestamp"`
}

// ExtractionOptions configures a single RunExtraction call.
type ExtractionOptions struct {
	ModelID            string
	InputPath          string
	PromptTemplatePath string
	CriterionRef       string
	SourceDocRef       string
	OutputPath         string
	SessionOutputPath  string
}

// AssistOptions configures a single RunAssistSession call.
// The caller parses the spans file upstream and supplies the resulting
// []string directly. InputPath is recorded in the SessionRecord for
// provenance — it is optional (empty is valid).
type AssistOptions struct {
	ModelID            string
	InputPath          string // path to spans file; recorded in SessionRecord
	PromptTemplatePath string
	CriterionRef       string
	SourceDocRef       string
	OutputPath         string
	SessionOutputPath  string
}

// CritiqueOptions configures a single RunCritique call.
// The caller parses the input drafts file upstream and supplies the resulting
// []schema.TraceDraft directly. InputPath is recorded in the SessionRecord for
// provenance — it is optional (empty is valid).
//
// DraftID is optional: when set, only the draft with that ID is critiqued;
// all others are ignored. When empty, all input drafts are critiqued.
type CritiqueOptions struct {
	ModelID            string
	InputPath          string // path to input drafts file; recorded in SessionRecord
	PromptTemplatePath string
	CriterionRef       string
	SourceDocRef       string
	OutputPath         string
	SessionOutputPath  string
	DraftID            string // empty = critique all; non-empty = single draft by ID
}

// ErrLLMRefusal indicates the LLM explicitly declined to produce output.
// The RefusalText carries whatever the LLM returned for debugging.
type ErrLLMRefusal struct {
	RefusalText string
}

func (e *ErrLLMRefusal) Error() string {
	return fmt.Sprintf("llm: refusal: %s", e.RefusalText)
}

// ErrMalformedOutput indicates the LLM returned text that does not parse
// as valid TraceDraft JSON. RawResponse carries the unparsed text.
type ErrMalformedOutput struct {
	RawResponse string
	ParseErr    error
}

func (e *ErrMalformedOutput) Error() string {
	return fmt.Sprintf("llm: malformed output: %v", e.ParseErr)
}

// frameworkUncertaintyNote is always appended to UncertaintyNote on
// LLM-produced drafts (Decision D3 in docs/decisions/llm-as-mediator-v1.md).
// The framework never delegates this signal to the LLM.
const frameworkUncertaintyNote = "LLM-produced candidate; unverified by human review"

// maxSourceBytes caps source document size at 1 MiB. Documents larger than
// this are rejected before any LLM call to prevent unexpected token costs.
const maxSourceBytes = 1 * 1024 * 1024

// knownContentFields lists the TraceDraft field names valid for use in
// IntentionallyBlank (Decision D7). Only content fields are valid; provenance
// fields like extracted_by and session_ref are framework-assigned and cannot
// be declared blank by the LLM.
var knownContentFields = map[string]bool{
	"what_changed": true,
	"source":       true,
	"target":       true,
	"mediation":    true,
	"observer":     true,
	"tags":         true,
}
