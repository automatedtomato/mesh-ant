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
//
// SourceDocRefs replaces the old SourceDocRef string to support multi-document
// ingestion (#139). Single-doc sessions carry a one-element slice. The old
// JSON key "source_doc_ref" is preserved via SourceDocRef for backward
// compatibility with existing session files — both fields are read; new
// session files write only SourceDocRefs.
type ExtractionConditions struct {
	ModelID        string `json:"model_id"`
	PromptTemplate string `json:"prompt_template"`
	// PromptHash is the SHA-256 hash (first 16 hex chars) of the prompt template
	// file contents at the time of extraction. Empty when PromptTemplate is empty.
	// Detects content drift when the same template path is reused across sessions.
	PromptHash         string    `json:"prompt_hash,omitempty"`
	CriterionRef       string    `json:"criterion_ref,omitempty"`
	SystemInstructions string    `json:"system_instructions"`
	// SourceDocRefs holds the reference strings for all source documents
	// processed in this session. Single-doc sessions use a one-element slice.
	SourceDocRefs []string `json:"source_doc_refs,omitempty"`
	// SourceDocRef is the legacy single-document field, retained for backward
	// compatibility with session files written before #139. New code writes
	// SourceDocRefs and leaves this empty. Reading code checks both fields.
	SourceDocRef string    `json:"source_doc_ref,omitempty"`
	// AdapterName is the name of the format-conversion adapter used before LLM
	// extraction, e.g. "pdf-extractor" or "html-extractor" (#140). Empty when
	// no adapter was used (plain-text source). This field names the mediating
	// act of format conversion so it is visible in the session provenance.
	AdapterName string    `json:"adapter_name,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// CritiqueConditions records the apparatus configuration for one LLM critique
// session. It is stored in SessionRecord.CritiqueConditions and written to disk.
//
// Critique sessions differ from extract/assist sessions analytically:
//
//   - Input is a TraceDraft JSON array, not a source document.
//   - SourceDocRef (singular, not a slice) carries the reference of the original
//     source document that the critiqued drafts were extracted from.
//   - No AdapterName field: no format conversion precedes critique.
//
// The API key is intentionally absent.
type CritiqueConditions struct {
	ModelID        string `json:"model_id"`
	PromptTemplate string `json:"prompt_template"`
	// PromptHash is the SHA-256 hash (first 16 hex chars) of the prompt template
	// file contents at the time of critique. Empty when PromptTemplate is empty.
	// Detects content drift when the same template path is reused across sessions.
	PromptHash         string    `json:"prompt_hash,omitempty"`
	CriterionRef       string    `json:"criterion_ref,omitempty"`
	SystemInstructions string    `json:"system_instructions"`
	// SourceDocRef carries the reference of the original source document that
	// the critiqued drafts were extracted from. Singular (not a slice) because
	// critique sessions always operate on drafts from a single source document.
	SourceDocRef string    `json:"source_doc_ref,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
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
//
// InputPaths replaces InputPath to support multi-document ingestion (#139).
// Single-doc sessions carry a one-element slice. InputPath is retained for
// backward compatibility with existing session files.
type SessionRecord struct {
	ID      string               `json:"id"`
	Command string               `json:"command"` // "extract", "assist", "critique", "split"
	Conditions ExtractionConditions `json:"conditions"`
	// CritiqueConditions records the apparatus configuration for critique sessions.
	// Present only when Command == "critique"; nil for all other commands.
	// Old session files written before this bifurcation (#151) will have Conditions
	// populated and CritiqueConditions nil; PromoteSession handles both.
	CritiqueConditions *CritiqueConditions `json:"critique_conditions,omitempty"`
	// DraftIDs holds the UUIDs of TraceDraft records produced in this session.
	// Nil (serialized as null) is intentional for "split" sessions — spans are
	// not TraceDraft records. Use DraftCount to determine span count for split.
	DraftIDs     []string           `json:"draft_ids"`
	Dispositions []DraftDisposition `json:"dispositions,omitempty"`
	// InputPaths holds the source document paths for all documents processed in
	// this session. Single-doc sessions use a one-element slice.
	InputPaths []string `json:"input_paths,omitempty"`
	// InputPath is the legacy single-document field, retained for backward
	// compatibility with session files written before #139. New code writes
	// InputPaths and leaves this empty.
	InputPath  string    `json:"input_path,omitempty"`
	OutputPath string    `json:"output_path"`
	DraftCount int       `json:"draft_count"`
	ErrorNote  string    `json:"error_note,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ExtractionOptions configures a single RunExtraction call.
//
// InputPaths and SourceDocRefs support multi-document ingestion (#139).
// For single-document ingestion, both slices must have exactly one element.
// len(InputPaths) must equal len(SourceDocRefs); both must be >= 1.
type ExtractionOptions struct {
	ModelID            string
	InputPaths         []string // one or more source document paths
	SourceDocRefs      []string // one provenance ref per document (parallel to InputPaths)
	PromptTemplatePath string
	CriterionRef       string
	OutputPath         string
	// AdapterName is the name of the format-conversion adapter applied to source
	// documents before extraction (#140). Empty for plain-text sources. When set,
	// it is recorded in ExtractionConditions.AdapterName on the session record.
	AdapterName string
}

// AssistOptions configures a single RunAssistSession call.
// InputPath is recorded in the SessionRecord for provenance; empty is valid.
type AssistOptions struct {
	ModelID            string
	InputPath          string // path to spans file; recorded in SessionRecord
	PromptTemplatePath string
	CriterionRef       string
	SourceDocRef       string
	OutputPath         string
}

// CritiqueOptions configures a single RunCritique call.
// InputPath is recorded in the SessionRecord for provenance; empty is valid.
// DraftID, when non-empty, restricts critique to the single draft with that ID.
type CritiqueOptions struct {
	ModelID            string
	InputPath          string // path to input drafts file; recorded in SessionRecord
	PromptTemplatePath string
	CriterionRef       string
	SourceDocRef       string
	OutputPath         string
	DraftID            string // empty = critique all; non-empty = single draft by ID
}

// SplitOptions configures a single RunSplit call.
// No CriterionRef: split is boundary detection only, not analytical classification.
type SplitOptions struct {
	ModelID            string
	InputPath          string // path to source document
	PromptTemplatePath string
	SourceDocRef       string
	OutputPath         string
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

// frameworkUncertaintyNote is always appended to UncertaintyNote on LLM-produced
// drafts — the framework never delegates this signal to the LLM (D3).
const frameworkUncertaintyNote = "LLM-produced candidate; unverified by human review"

// knownContentFields lists TraceDraft field names valid for IntentionallyBlank (D7).
// Provenance fields are framework-assigned and cannot be declared blank by the LLM.
var knownContentFields = map[string]bool{
	"what_changed": true,
	"source":       true,
	"target":       true,
	"mediation":    true,
	"observer":     true,
	"tags":         true,
}
