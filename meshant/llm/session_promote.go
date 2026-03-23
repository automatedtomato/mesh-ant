// session_promote.go implements PromoteSession — converting a SessionRecord
// into a canonical schema.Trace.
//
// A SessionRecord is an observation act: the LLM operated at a specific time,
// under specific conditions, on specific source material. Not recording this as
// a Trace is a Principle 8 gap — the framework observes but doesn't observe
// itself observing. PromoteSession closes that loop.
//
// The LLM session is a mediator in the trace (Command field). The model is the
// source of the act. The source document is the target. The analyst who runs the
// promotion must provide the observer position — no trace without an observer.
package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// PromoteSession converts a SessionRecord to a canonical schema.Trace, closing
// the Principle 8 reflexivity gap: the framework's own observation act enters
// the mesh as a trace.
//
// observer is required — it names the analyst position from which this
// observation of the session is being made. An empty observer returns an error.
//
// The promoted Trace always passes schema.Trace.Validate(). If it does not
// (e.g. the session ID is not a valid UUID), PromoteSession returns an error.
//
// Field mapping:
//   - ID          ← SessionRecord.ID (already a UUID, used as SessionRef)
//   - Timestamp   ← SessionRecord.Timestamp
//   - WhatChanged ← generated from Command + SourceDocRef
//   - Source      ← [Conditions.ModelID] (model is the source of the act); nil if empty
//   - Target      ← [Conditions.SourceDocRef] (document processed); nil if empty
//   - Mediation   ← SessionRecord.Command ("extract", "assist", "critique", "split")
//   - Observer    ← observer argument (required, caller-provided)
//   - Tags        ← ["session", "articulation"]
func PromoteSession(rec SessionRecord, observer string) (schema.Trace, error) {
	if observer == "" {
		return schema.Trace{}, errors.New("promote-session: observer is required — name the position from which this session is being observed")
	}

	whatChanged := sessionWhatChanged(rec)

	var source []string
	if rec.Conditions.ModelID != "" {
		source = []string{rec.Conditions.ModelID}
	}

	// Build the Target slice from SourceDocRefs (plural, #139) when available,
	// falling back to the legacy SourceDocRef field for backward compatibility
	// with session files written before the multi-doc migration.
	var target []string
	if len(rec.Conditions.SourceDocRefs) > 0 {
		target = nonBlankRefs(rec.Conditions.SourceDocRefs)
	} else if rec.Conditions.SourceDocRef != "" {
		target = []string{rec.Conditions.SourceDocRef}
	}

	t := schema.Trace{
		ID:          rec.ID,
		Timestamp:   rec.Timestamp,
		WhatChanged: whatChanged,
		Source:      source,
		Target:      target,
		Mediation:   rec.Command,
		Observer:    observer,
		Tags: []string{
			string(schema.TagValueSession),
			string(schema.TagValueArticulation),
		},
	}

	if err := t.Validate(); err != nil {
		return schema.Trace{}, fmt.Errorf("promote-session: promoted trace invalid: %w", err)
	}

	return t, nil
}

// sessionWhatChanged generates the WhatChanged description for a promoted session trace.
// The description names the command, source document(s), and model — making the conditions
// of the act visible in the trace's most human-readable field. This follows the style
// of articulationWhatChanged, which names the conditions under which the observation
// was made rather than simply stating that something happened.
// Falls back gracefully when Command, SourceDocRefs/SourceDocRef, or ModelID are empty.
// Multi-doc sessions list all doc refs separated by commas.
func sessionWhatChanged(rec SessionRecord) string {
	cmd := rec.Command
	if cmd == "" {
		cmd = "unknown"
	}

	model := rec.Conditions.ModelID

	base := "LLM " + cmd + " session"

	// Use SourceDocRefs (plural) when available; fall back to legacy SourceDocRef.
	if len(rec.Conditions.SourceDocRefs) > 0 {
		if refs := nonBlankRefs(rec.Conditions.SourceDocRefs); len(refs) > 0 {
			base += " on " + strings.Join(refs, ", ")
		}
	} else if rec.Conditions.SourceDocRef != "" {
		base += " on " + rec.Conditions.SourceDocRef
	}

	if model != "" {
		base += " (" + model + ")"
	}
	return base
}

// nonBlankRefs returns a new slice containing only non-empty entries from refs.
// Used when building Target and WhatChanged from SourceDocRefs so that nil or
// empty entries (which may result from partial session records) are not included
// in the promoted Trace.
func nonBlankRefs(refs []string) []string {
	var out []string
	for _, ref := range refs {
		if ref != "" {
			out = append(out, ref)
		}
	}
	return out
}
