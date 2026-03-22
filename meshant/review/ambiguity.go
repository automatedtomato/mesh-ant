// ambiguity.go detects ambiguous or under-specified fields in a TraceDraft.
//
// Ambiguity detection follows ANT language discipline: warnings are
// invitations to inspect, not demands to correct. No field is inherently
// "missing" or "wrong" — blank content is a legitimate analytical choice
// unless it is unexplained from this position in the network.
//
// The package is designed to be used by the interactive review session
// (cmd/meshant review) to surface fields that may benefit from more
// attention before a draft is promoted or archived.
package review

import "github.com/automatedtomato/mesh-ant/meshant/schema"

// AmbiguityWarning describes a single ambiguity in a TraceDraft field.
// Warnings are invitations to inspect, not demands to correct.
// Language follows ANT discipline: no "missing", "error", or "incomplete".
type AmbiguityWarning struct {
	// Field is the field name or check name, e.g. "what_changed" or
	// "criterion_ref mismatch".
	Field string

	// Message is a human-readable invitation in ANT-disciplined language.
	// It never uses "missing", "error", or "incomplete".
	Message string
}

// candidateField pairs a field name with a blank-check predicate.
type candidateField struct {
	name    string
	isBlank func(d schema.TraceDraft) bool
}

// candidateFields lists the six content fields to inspect for blank values.
var candidateFields = []candidateField{
	{
		name:    "what_changed",
		isBlank: func(d schema.TraceDraft) bool { return d.WhatChanged == "" },
	},
	{
		name:    "source",
		isBlank: func(d schema.TraceDraft) bool { return len(d.Source) == 0 },
	},
	{
		name:    "target",
		isBlank: func(d schema.TraceDraft) bool { return len(d.Target) == 0 },
	},
	{
		name:    "mediation",
		isBlank: func(d schema.TraceDraft) bool { return d.Mediation == "" },
	},
	{
		name:    "observer",
		isBlank: func(d schema.TraceDraft) bool { return d.Observer == "" },
	},
	{
		name:    "tags",
		isBlank: func(d schema.TraceDraft) bool { return len(d.Tags) == 0 },
	},
}

// candidateMessages maps field name to an ANT-disciplined invitation message.
var candidateMessages = map[string]string{
	"what_changed": "what_changed is unregistered from this position — the nature of the change is in shadow",
	"source":       "source is unregistered from this position — the origin of the trace is in shadow",
	"target":       "target is unregistered from this position — the destination of the trace is in shadow",
	"mediation":    "mediation is unregistered from this position — the translating force is in shadow",
	"observer":     "observer is unregistered from this position — the observing stance is in shadow",
	"tags":         "tags are unregistered from this position — the descriptive frame is in shadow",
}

// DetectAmbiguities returns ambiguity warnings for d. Checks six content fields
// for unexplained blanks and criterion_ref/uncertainty_note mismatch.
// Returns nil if no ambiguities detected.
func DetectAmbiguities(d schema.TraceDraft) []AmbiguityWarning {
	intentional := make(map[string]bool, len(d.IntentionallyBlank))
	for _, name := range d.IntentionallyBlank {
		intentional[name] = true
	}

	var warnings []AmbiguityWarning

	for _, cf := range candidateFields {
		if cf.isBlank(d) && !intentional[cf.name] {
			warnings = append(warnings, AmbiguityWarning{
				Field:   cf.name,
				Message: candidateMessages[cf.name],
			})
		}
	}

	// uncertainty_note without criterion_ref: uncertainty acknowledged but not anchored.
	if d.UncertaintyNote != "" && d.CriterionRef == "" {
		warnings = append(warnings, AmbiguityWarning{
			Field:   "criterion_ref mismatch",
			Message: "uncertainty_note is present but no criterion_ref is set — the criterion anchoring this uncertainty is in shadow from this position",
		})
	}

	return warnings
}
