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

// timeWindowWire is the on-wire JSON shape for TimeWindow. Both bounds are
// *string (not *time.Time) so null (unbounded) is distinct from a real timestamp.
type timeWindowWire struct {
	Start *string `json:"start"`
	End   *string `json:"end"`
}

// MarshalJSON implements json.Marshaler for TimeWindow.
// Zero bounds are encoded as JSON null; non-zero bounds as RFC3339 in UTC.
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
// JSON null → zero time.Time. JSON string → RFC3339. Any other token → error.
func (tw *TimeWindow) UnmarshalJSON(data []byte) error {
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

// decodeTimeField decodes one raw JSON token as a time.Time.
// null or absent → zero; JSON string → RFC3339; anything else → error.
// Strips quotes directly (outer parse guarantees syntactically valid string).
func decodeTimeField(fieldName string, raw json.RawMessage) (time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, nil
	}

	if raw[0] != '"' {
		return time.Time{}, fmt.Errorf("graph: TimeWindow.UnmarshalJSON: field %q: expected string or null, got %s",
			fieldName, string(raw))
	}

	s := string(raw[1 : len(raw)-1])

	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("graph: TimeWindow.UnmarshalJSON: field %q: invalid RFC3339 %q: %w",
			fieldName, s, err)
	}
	return t, nil
}
