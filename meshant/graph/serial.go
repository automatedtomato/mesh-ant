// Package graph — serial.go contains the custom JSON codec for TimeWindow.
//
// All graph types (MeshGraph, GraphDiff, Node, Edge, Cut, ShadowElement,
// ShadowShift, PersistedNode) carry json struct tags defined directly on their
// fields in graph.go and diff.go. This file adds only the two methods that
// require custom logic beyond what the default encoder provides:
//
//   - TimeWindow.MarshalJSON: zero time.Time fields are written as JSON null
//     (not as "0001-01-01T00:00:00Z"). Non-zero fields are written as RFC3339.
//
//   - TimeWindow.UnmarshalJSON: JSON null is decoded as zero time.Time.
//     A JSON string is decoded as RFC3339. Any other token is an error.
//
// Design rationale: a zero TimeWindow bound means "unbounded" — it is not a
// real timestamp. Encoding it as null makes the unbounded intent unambiguous and
// prevents consumers from mistaking the zero time for a real date boundary.
// See docs/decisions/m7-serialisation-reflexivity-v1.md Decision 2.
package graph

import (
	"encoding/json"
	"fmt"
	"time"
)

// timeWindowWire is the on-wire JSON shape for a TimeWindow. Both bounds are
// *string rather than *time.Time so we can distinguish null (unbounded) from a
// real timestamp without relying on time.Time's own JSON codec, which would
// encode zero as "0001-01-01T00:00:00Z".
type timeWindowWire struct {
	Start *string `json:"start"`
	End   *string `json:"end"`
}

// MarshalJSON implements json.Marshaler for TimeWindow.
//
// A zero Start or End is encoded as JSON null. A non-zero bound is encoded as
// an RFC3339 string in UTC. The encoding is the same as time.Time.UTC().Format(time.RFC3339).
func (tw TimeWindow) MarshalJSON() ([]byte, error) {
	w := timeWindowWire{}
	if !tw.Start.IsZero() {
		s := tw.Start.UTC().Format(time.RFC3339)
		w.Start = &s
	}
	if !tw.End.IsZero() {
		s := tw.End.UTC().Format(time.RFC3339)
		w.End = &s
	}
	return json.Marshal(w)
}

// UnmarshalJSON implements json.Unmarshaler for TimeWindow.
//
// JSON null for start or end is decoded as zero time.Time (IsZero() == true).
// A JSON string is decoded as RFC3339; any other token type returns an error.
// Invalid JSON returns an error. An invalid RFC3339 string returns an error.
func (tw *TimeWindow) UnmarshalJSON(data []byte) error {
	// Use json.RawMessage so we can inspect each field's token type individually
	// before trying to decode as a string. This gives us precise error messages
	// when callers pass a numeric or boolean value for a time field.
	var raw struct {
		Start json.RawMessage `json:"start"`
		End   json.RawMessage `json:"end"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("graph: TimeWindow.UnmarshalJSON: %w", err)
	}

	var err error
	tw.Start, err = decodeTimeField("start", raw.Start)
	if err != nil {
		return err
	}
	tw.End, err = decodeTimeField("end", raw.End)
	if err != nil {
		return err
	}
	return nil
}

// decodeTimeField decodes a single raw JSON token as a time.Time.
//
// Accepted token types:
//   - null (or absent/empty raw message): returns zero time.Time
//   - JSON string: parsed as RFC3339; error if the string is not valid RFC3339
//
// Any other JSON token type (number, boolean, object, array) returns an error.
// This strict decoding prevents silent truncation of unexpected inputs.
//
// Implementation note: when raw comes from json.Unmarshal into a json.RawMessage
// field of a surrounding struct, the outer parse guarantees that any token
// beginning with '"' is already a syntactically valid JSON string. We therefore
// extract the string value by stripping the enclosing quotes directly rather
// than calling json.Unmarshal again — this avoids a redundant parse and removes
// a branch that is unreachable through the outer UnmarshalJSON path.
func decodeTimeField(fieldName string, raw json.RawMessage) (time.Time, error) {
	// Absent field (zero-length raw message) is treated as unbounded.
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, nil
	}

	// Check that the token is a JSON string (starts with a double-quote).
	// Reject numbers, booleans, objects, and arrays immediately.
	if raw[0] != '"' {
		return time.Time{}, fmt.Errorf("graph: TimeWindow.UnmarshalJSON: field %q: expected string or null, got %s",
			fieldName, string(raw))
	}

	// Strip the surrounding quotes to get the raw string value. The outer
	// json.Unmarshal guarantees the token is a complete, well-formed JSON string,
	// so the last byte is always a closing '"'.
	s := string(raw[1 : len(raw)-1])

	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("graph: TimeWindow.UnmarshalJSON: field %q: invalid RFC3339 %q: %w",
			fieldName, s, err)
	}
	return t, nil
}
