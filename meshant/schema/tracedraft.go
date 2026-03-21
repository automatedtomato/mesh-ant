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
	// ID uniquely identifies this draft. Assigned by the loader if absent.
	ID string `json:"id,omitempty"`

	// Timestamp records when this draft was created.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// SourceSpan is the verbatim text that provoked this extraction.
	// The only required field — preserves the anchor without forcing resolution.
	SourceSpan string `json:"source_span"`

	// SourceDocRef identifies the source document (path, URL, or reference string).
	SourceDocRef string `json:"source_doc_ref,omitempty"`

	// Candidate trace fields — all optional at draft stage.
	// Leave fields empty rather than fabricating confident assignments.

	// WhatChanged is a short candidate description of the difference observed.
	WhatChanged string `json:"what_changed,omitempty"`

	// Source names candidate source elements. May be empty when attribution is
	// genuinely unclear.
	Source []string `json:"source,omitempty"`

	// Target names candidate target elements. May be empty when the effect is
	// diffuse or unsupportable from the source span.
	Target []string `json:"target,omitempty"`

	// Mediation names the candidate mediator between source and target.
	Mediation string `json:"mediation,omitempty"`

	// Observer is the candidate observer position for this trace.
	Observer string `json:"observer,omitempty"`

	// Tags are candidate descriptors for this trace.
	Tags []string `json:"tags,omitempty"`

	// UncertaintyNote records where the source span does not support confident
	// field assignment. Prefer a non-empty note over a fabricated value.
	UncertaintyNote string `json:"uncertainty_note,omitempty"`

	// ExtractionStage records pipeline position of this draft.
	// Known values: "span-harvest", "weak-draft", "critiqued", "reviewed".
	// Stages name positions, not quality levels — "critiqued" names an LLM
	// mediating act (Thread F), not a lesser record.
	ExtractionStage string `json:"extraction_stage,omitempty"`

	// ExtractedBy names the analyst position — parallel to Observer for the
	// graph layer. "human", "llm-pass1", "reviewer" are positions, not identities.
	// Two drafts with different ExtractedBy for the same SourceSpan represent
	// two positions on the same material; their disagreement is data, not error.
	// Use loader.GroupByAnalyst to partition by this field.
	ExtractedBy string `json:"extracted_by,omitempty"`

	// DerivedFrom is the ID of the parent draft; empty for root drafts.
	DerivedFrom string `json:"derived_from,omitempty"`

	// CriterionRef names the EquivalenceCriterion under which this draft was
	// produced or reviewed — a citation, not a copy. Records the interpretive
	// frame so the skeleton is self-situated. Does not affect Validate(),
	// IsPromotable(), or Promote().
	CriterionRef string `json:"criterion_ref,omitempty"`

	// SessionRef links this draft to the ingestion session that produced it,
	// so extraction conditions (model, prompt, source) are recoverable from the
	// draft itself. Preserved by DeriveAccepted/DeriveEdited; NOT transferred by
	// Promote() — canonical Traces record analytical content, not ingestion mechanics.
	// Does not affect Validate(), IsPromotable(), or Promote().
	SessionRef string `json:"session_ref,omitempty"`

	// IntentionallyBlank lists content field names deliberately left empty —
	// the analyst decided the field should not be filled from this source span.
	// Distinguishes "never extracted" (absent, no entry here) from "deliberately
	// not filled" (absent AND listed here). Known names: "what_changed", "source",
	// "target", "mediation", "observer", "tags".
	IntentionallyBlank []string `json:"intentionally_blank,omitempty"`
}

// Validate checks that SourceSpan is present — the only required field.
func (d TraceDraft) Validate() error {
	if d.SourceSpan == "" {
		return errors.New("tracedraft: source_span is required — it is the anchor text that makes this extraction inspectable")
	}
	return nil
}

// IsPromotable reports whether the draft has the fields required for Promote:
// a valid lowercase UUID ID, non-empty WhatChanged, and non-empty Observer.
// Does not call Validate — SourceSpan is preserved in the draft, not transferred.
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

// Promote converts this TraceDraft to a canonical Trace, appending TagValueDraft
// as a provenance signal. Errors if IsPromotable() is false. The promoted Trace
// passes Trace.Validate().
func (d TraceDraft) Promote() (Trace, error) {
	if !d.IsPromotable() {
		return Trace{}, d.promotabilityError()
	}

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

// promotabilityError collects all reasons this draft cannot be promoted.
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
