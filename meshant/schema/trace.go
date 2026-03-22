// Package schema defines the core data types for MeshAnt.
//
// MeshAnt is a trace-first framework inspired by Actor-Network Theory.
// This package contains the [Trace] type: the fundamental unit of record.
//
// A Trace captures a moment where something made a difference in a network —
// a change, a redirection, a mediation, a transformation. It does not
// presuppose who or what the actors are. Actors are provisional effects
// of linked traces, not unquestioned first principles.
//
// This schema is provisional. It records a cut made at a particular time,
// from a particular observer position. That cut is itself part of the mesh.
// See docs/decisions/trace-schema-v1.md for the rationale behind what was
// included, excluded, and left open.
package schema

import (
	"errors"
	"regexp"
	"time"
)

// uuidPattern matches a lowercase hyphenated UUID string (any variant).
// Only lowercase hex is accepted. Version/variant nibbles are unconstrained
// to allow traces from non-v4 UUID generators.
var uuidPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// TagValue is the type for descriptive trace tags.
// These constants are a starting vocabulary, not a closed enum — Trace.Tags
// is []string so callers may supply descriptors beyond this list.
type TagValue string

const (
	TagDelay         TagValue = "delay"
	TagThreshold     TagValue = "threshold"
	TagBlockage      TagValue = "blockage"
	TagAmplification TagValue = "amplification"
	TagRedirection   TagValue = "redirection"
	TagTranslation   TagValue = "translation"

	// TagValueArticulation marks a trace that records the act of articulation
	// itself — i.e. a reflexive trace produced when graph.ArticulationTrace or
	// graph.DiffTrace are called. Supports reflexive tracing: the observation
	// apparatus enters the mesh it observes.
	TagValueArticulation TagValue = "articulation"

	// TagValueSession marks a trace promoted from a SessionRecord — a record
	// of an LLM session as an observation act. Closing the reflexivity gap:
	// the framework observes the apparatus that observed the source material.
	TagValueSession TagValue = "session"
)

// Trace is a record of something that made a difference in a network.
//
// Source and Target are slices of strings. The schema does not yet decide
// what counts as an actor — that decision is deferred until enough traces
// have been followed to warrant it. A source or target could be a human,
// a rule, a threshold, a queue, a form, or anything else that redirects,
// amplifies, blocks, or transforms action. Using slices acknowledges that
// agency is often distributed across a heterogeneous assemblage; forcing a
// single name would perform a premature singularization of attribution.
//
// Fields with omitempty are optional. Their absence is meaningful:
// a Trace without a Mediation means no mediator was observed —
// not that mediation is impossible.
//
// Use [Trace.Validate] to check that required fields are present.
type Trace struct {
	// ID uniquely identifies this trace. Must be a lowercase hyphenated UUID. Required.
	ID string `json:"id"`

	// Timestamp records when this trace was captured — not when the underlying
	// event "really" occurred. Observation is always situated. Required.
	Timestamp time.Time `json:"timestamp"`

	// WhatChanged is a short description of the difference that was made. Required.
	WhatChanged string `json:"what_changed"`

	// Source names what produced this trace. A slice because the producer
	// of a difference is often a heterogeneous assemblage. May be nil when
	// attribution is genuinely unknown.
	Source []string `json:"source,omitempty"`

	// Target names what was affected. A slice because effects are often
	// distributed. May be nil when the effect is diffuse.
	Target []string `json:"target,omitempty"`

	// Mediation names what transformed, redirected, or displaced the action
	// between source and target. A mediator changes what passes through it —
	// not a neutral conduit. Absent means no mediator was observed, not that
	// none could exist.
	Mediation string `json:"mediation,omitempty"`

	// Tags are descriptors characterizing the kind of difference. Uses the
	// TagValue vocabulary but is not restricted to it ([]string keeps it open).
	Tags []string `json:"tags,omitempty"`

	// Observer records who or what captured this trace and from what position.
	// Required. A trace without an observer hides the cut that made it —
	// contradicting Principle 8: the designer is inside the mesh.
	Observer string `json:"observer"`
}

// Validate checks that required fields are present and well-formed.
// All violations are collected and returned together via [errors.Join].
// Optional fields (Source, Target, Mediation, Tags) are never rejected —
// their absence is valid and meaningful.
func (t Trace) Validate() error {
	var errs []error
	if t.ID == "" {
		errs = append(errs, errors.New("trace: id is required"))
	} else if !uuidPattern.MatchString(t.ID) {
		errs = append(errs, errors.New("trace: id must be a valid UUID (lowercase hyphenated)"))
	}
	if t.Timestamp.IsZero() {
		errs = append(errs, errors.New("trace: timestamp is required and must not be zero"))
	}
	if t.WhatChanged == "" {
		errs = append(errs, errors.New("trace: what_changed is required"))
	}
	if t.Observer == "" {
		errs = append(errs, errors.New("trace: observer is required — record the position from which this trace was made"))
	}
	return errors.Join(errs...)
}
