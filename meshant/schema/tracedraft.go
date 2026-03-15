// tracedraft.go defines TraceDraft — a provisional, provenance-bearing record
// produced during ingestion. It is weaker than a canonical Trace: fields may
// be incomplete, unresolved, or explicitly uncertain.
//
// Design: TraceDraft is a first-class analytical object. It carries its own
// provenance (ExtractionStage, ExtractedBy) and supports revision chains via
// DerivedFrom. The ingestion pipeline — raw span → LLM draft → critique →
// human revision → accepted trace — is represented as a chain of TraceDraft
// records linked by DerivedFrom, making the pipeline itself inspectable and
// followable. See docs/decisions/tracedraft-v1.md for the rationale.
package schema

import (
	"errors"
	"fmt"
	"time"
)

// TagValueDraft marks a Trace that was promoted from a TraceDraft.
// It is a provenance signal: this trace passed through the ingestion
// pipeline rather than being authored directly as a canonical record.
const TagValueDraft TagValue = "draft"

// TraceDraft is a provisional, provenance-bearing record produced during
// ingestion. It is not a Trace — it may be incomplete, unresolved, or
// explicitly uncertain. A TraceDraft is a legitimate analytical object in
// its own right. It may be promoted to a canonical Trace when its fields
// are sufficient, or it may remain a draft indefinitely.
//
// The extraction pipeline (span → LLM draft → critique → human revision →
// canonical trace) is represented as a chain of TraceDraft records linked
// by DerivedFrom. This makes the ingestion process itself followable and
// inspectable — the LLM is a mediator in the chain, not a hidden extractor.
type TraceDraft struct {
	// Framework-assigned fields (assigned by meshant draft command).

	// ID uniquely identifies this draft record. Assigned by the loader if absent.
	ID string `json:"id,omitempty"`

	// Timestamp records when this draft was created by the loader.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Source material fields.

	// SourceSpan is the verbatim text from the source document that provoked
	// this extraction. It is the only required field — a TraceDraft with only
	// a source span is valid. It preserves the text without forcing resolution.
	SourceSpan string `json:"source_span"`

	// SourceDocRef identifies the source document (path, URL, or reference string).
	SourceDocRef string `json:"source_doc_ref,omitempty"`

	// Candidate trace fields — all optional at draft stage.
	// Leave fields empty rather than fabricating confident assignments.

	// WhatChanged is a short candidate description of the difference observed.
	WhatChanged string `json:"what_changed,omitempty"`

	// Source names candidate source elements. May be empty if attribution is
	// genuinely unclear — do not fabricate confident assignments.
	Source []string `json:"source,omitempty"`

	// Target names candidate target elements. May be empty if the effect is
	// diffuse or not supportable from the source span.
	Target []string `json:"target,omitempty"`

	// Mediation names the candidate mediator between source and target.
	Mediation string `json:"mediation,omitempty"`

	// Observer is the candidate observer position for this trace.
	Observer string `json:"observer,omitempty"`

	// Tags are candidate descriptors for this trace.
	Tags []string `json:"tags,omitempty"`

	// Uncertainty and provenance fields.

	// UncertaintyNote names where the source span does not support confident
	// field assignment. Prefer a non-empty note over a fabricated value.
	UncertaintyNote string `json:"uncertainty_note,omitempty"`

	// ExtractionStage records where in the pipeline this draft was produced.
	// Known values: "span-harvest", "weak-draft", "reviewed".
	ExtractionStage string `json:"extraction_stage,omitempty"`

	// ExtractedBy identifies who or what produced this draft.
	// Examples: "human", "llm-pass1", "llm-pass2", "reviewer".
	ExtractedBy string `json:"extracted_by,omitempty"`

	// DerivedFrom is the ID of the parent draft, linking revision records
	// into a structurally followable chain. Empty for root drafts.
	DerivedFrom string `json:"derived_from,omitempty"`
}

// Validate checks that the minimum required field is present.
// Only SourceSpan is required — a TraceDraft with only a source span is valid.
// All other fields are optional at the draft stage.
func (d TraceDraft) Validate() error {
	if d.SourceSpan == "" {
		return errors.New("tracedraft: source_span is required — it is the ground truth that provoked this extraction")
	}
	return nil
}

// IsPromotable reports whether this draft has the fields required to produce
// a canonical Trace via Promote:
//   - ID must be a valid lowercase UUID
//   - WhatChanged must be non-empty
//   - Observer must be non-empty
//
// IsPromotable does not call Validate — SourceSpan completeness is not
// required for promotion because the source span is preserved in the draft,
// not transferred to the Trace.
func (d TraceDraft) IsPromotable() bool {
	if !uuidPattern.MatchString(d.ID) {
		return false
	}
	if d.WhatChanged == "" {
		return false
	}
	if d.Observer == "" {
		return false
	}
	return true
}

// Promote converts this TraceDraft to a canonical Trace. It appends
// TagValueDraft to the trace's Tags as a provenance signal that this trace
// passed through the ingestion pipeline.
//
// Promote errors if IsPromotable() returns false. The error names every
// missing or invalid field so callers can report them to the user.
//
// The promoted Trace will pass Trace.Validate() when Promote succeeds.
func (d TraceDraft) Promote() (Trace, error) {
	if !d.IsPromotable() {
		return Trace{}, d.promotabilityError()
	}

	// Build tags: carry forward existing tags and append TagValueDraft.
	tags := make([]string, len(d.Tags), len(d.Tags)+1)
	copy(tags, d.Tags)
	tags = append(tags, string(TagValueDraft))

	return Trace{
		ID:          d.ID,
		Timestamp:   d.Timestamp,
		WhatChanged: d.WhatChanged,
		Source:      d.Source,
		Target:      d.Target,
		Mediation:   d.Mediation,
		Observer:    d.Observer,
		Tags:        tags,
	}, nil
}

// promotabilityError collects all reasons why this draft cannot be promoted
// and returns them as a single descriptive error. It mirrors Trace.Validate
// in spirit: all problems in one pass.
func (d TraceDraft) promotabilityError() error {
	var errs []error
	if !uuidPattern.MatchString(d.ID) {
		if d.ID == "" {
			errs = append(errs, errors.New("tracedraft: id is required for promotion (assign a UUID via meshant draft)"))
		} else {
			errs = append(errs, fmt.Errorf("tracedraft: id %q is not a valid lowercase UUID", d.ID))
		}
	}
	if d.WhatChanged == "" {
		errs = append(errs, errors.New("tracedraft: what_changed is required for promotion"))
	}
	if d.Observer == "" {
		errs = append(errs, errors.New("tracedraft: observer is required for promotion"))
	}
	return errors.Join(errs...)
}
