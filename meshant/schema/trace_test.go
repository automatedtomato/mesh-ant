package schema_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// validTrace returns a fully populated Trace for use in tests.
func validTrace() schema.Trace {
	return schema.Trace{
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Date(2026, 3, 10, 12, 0, 0, 123456789, time.UTC),
		WhatChanged: "message was delayed at the queue threshold",
		Source:      "rate-limiter",
		Target:      "outgoing-message",
		Mediation:   "queue-policy-v3",
		Tags:        []string{string(schema.TagDelay), string(schema.TagThreshold)},
		Observer:    "system-monitor/position-A",
	}
}

// --- Group 1: JSON Round-Trip ---

func TestTraceJSONRoundTrip_FullRecord(t *testing.T) {
	original := validTrace()

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got schema.Trace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID: got %q, want %q", got.ID, original.ID)
	}
	if !got.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, original.Timestamp)
	}
	if got.WhatChanged != original.WhatChanged {
		t.Errorf("WhatChanged: got %q, want %q", got.WhatChanged, original.WhatChanged)
	}
	if got.Source != original.Source {
		t.Errorf("Source: got %q, want %q", got.Source, original.Source)
	}
	if got.Target != original.Target {
		t.Errorf("Target: got %q, want %q", got.Target, original.Target)
	}
	if got.Mediation != original.Mediation {
		t.Errorf("Mediation: got %q, want %q", got.Mediation, original.Mediation)
	}
	if got.Observer != original.Observer {
		t.Errorf("Observer: got %q, want %q", got.Observer, original.Observer)
	}
	if len(got.Tags) != len(original.Tags) {
		t.Fatalf("Tags length: got %d, want %d", len(got.Tags), len(original.Tags))
	}
	for i := range original.Tags {
		if got.Tags[i] != original.Tags[i] {
			t.Errorf("Tags[%d]: got %q, want %q", i, got.Tags[i], original.Tags[i])
		}
	}
}

func TestTraceJSONRoundTrip_MinimalRecord(t *testing.T) {
	original := schema.Trace{
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Now().UTC(),
		WhatChanged: "threshold crossed",
		Observer:    "observer-1",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Optional fields must be absent from JSON output (omitempty)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	for _, absent := range []string{"source", "target", "mediation", "tags"} {
		if _, ok := raw[absent]; ok {
			t.Errorf("expected %q to be absent from JSON output (omitempty), but it was present", absent)
		}
	}
}

func TestTraceJSONRoundTrip_TimestampPreservesNanoseconds(t *testing.T) {
	original := schema.Trace{
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Date(2026, 3, 10, 9, 30, 0, 987654321, time.UTC),
		WhatChanged: "event occurred",
		Observer:    "sensor-1",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got schema.Trace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !got.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp precision lost: got %v, want %v", got.Timestamp, original.Timestamp)
	}
}

func TestTraceJSONRoundTrip_TagsPreservesOrder(t *testing.T) {
	original := validTrace()
	original.Tags = []string{"delay", "threshold", "blockage"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got schema.Trace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(got.Tags) != len(original.Tags) {
		t.Fatalf("Tags length: got %d, want %d", len(got.Tags), len(original.Tags))
	}
	for i, tag := range original.Tags {
		if got.Tags[i] != tag {
			t.Errorf("Tags[%d]: got %q, want %q", i, got.Tags[i], tag)
		}
	}
}

func TestTraceJSONRoundTrip_EmptyTagsOmitted(t *testing.T) {
	tr := validTrace()
	tr.Tags = nil

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(data), `"tags"`) {
		t.Errorf("expected 'tags' to be omitted when nil, but found in JSON: %s", data)
	}
}

func TestTraceJSONRoundTrip_EmptyMediationOmitted(t *testing.T) {
	tr := validTrace()
	tr.Mediation = ""

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if strings.Contains(string(data), `"mediation"`) {
		t.Errorf("expected 'mediation' to be omitted when empty, but found in JSON: %s", data)
	}
}

func TestTraceJSONRoundTrip_JSONKeys(t *testing.T) {
	tr := validTrace()
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	for _, key := range []string{
		`"id"`, `"timestamp"`, `"what_changed"`,
		`"source"`, `"target"`, `"mediation"`, `"tags"`, `"observer"`,
	} {
		if !strings.Contains(string(data), key) {
			t.Errorf("expected JSON key %s to be present, but was not found in: %s", key, data)
		}
	}
}

// --- Group 2: Validation ---

func TestTraceValidate_ValidFullRecord(t *testing.T) {
	if err := validTrace().Validate(); err != nil {
		t.Errorf("expected valid trace to pass Validate, got: %v", err)
	}
}

func TestTraceValidate_ValidMinimalRecord(t *testing.T) {
	tr := schema.Trace{
		ID:          "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		Timestamp:   time.Now().UTC(),
		WhatChanged: "something changed",
		Observer:    "observer-1",
	}
	if err := tr.Validate(); err != nil {
		t.Errorf("expected minimal trace to pass Validate, got: %v", err)
	}
}

func TestTraceValidate_MissingID(t *testing.T) {
	tr := validTrace()
	tr.ID = ""
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID, got nil")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Errorf("expected error to mention 'id', got: %v", err)
	}
}

func TestTraceValidate_MalformedID_NotUUID(t *testing.T) {
	tr := validTrace()
	tr.ID = "not-a-uuid"
	if err := tr.Validate(); err == nil {
		t.Error("expected error for malformed UUID, got nil")
	}
}

func TestTraceValidate_ZeroTimestamp(t *testing.T) {
	tr := validTrace()
	tr.Timestamp = time.Time{}
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected error for zero timestamp, got nil")
	}
	if !strings.Contains(err.Error(), "timestamp") {
		t.Errorf("expected error to mention 'timestamp', got: %v", err)
	}
}

func TestTraceValidate_MissingWhatChanged(t *testing.T) {
	tr := validTrace()
	tr.WhatChanged = ""
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected error for missing what_changed, got nil")
	}
	if !strings.Contains(err.Error(), "what_changed") {
		t.Errorf("expected error to mention 'what_changed', got: %v", err)
	}
}

func TestTraceValidate_MissingObserver(t *testing.T) {
	tr := validTrace()
	tr.Observer = ""
	err := tr.Validate()
	if err == nil {
		t.Fatal("expected error for missing observer, got nil")
	}
	if !strings.Contains(err.Error(), "observer") {
		t.Errorf("expected error to mention 'observer', got: %v", err)
	}
}

func TestTraceValidate_EmptySourceIsPermitted(t *testing.T) {
	tr := validTrace()
	tr.Source = ""
	if err := tr.Validate(); err != nil {
		t.Errorf("empty Source should be permitted, got: %v", err)
	}
}

func TestTraceValidate_EmptyTargetIsPermitted(t *testing.T) {
	tr := validTrace()
	tr.Target = ""
	if err := tr.Validate(); err != nil {
		t.Errorf("empty Target should be permitted, got: %v", err)
	}
}

func TestTraceValidate_EmptyMediationIsPermitted(t *testing.T) {
	tr := validTrace()
	tr.Mediation = ""
	if err := tr.Validate(); err != nil {
		t.Errorf("empty Mediation should be permitted, got: %v", err)
	}
}

// --- Group 3: Zero-Value Safety ---

func TestTraceZeroValue_MarshalDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("json.Marshal panicked on zero-value Trace: %v", r)
		}
	}()
	var tr schema.Trace
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal returned error on zero-value Trace: %v", err)
	}
	if len(data) == 0 {
		t.Error("Marshal returned empty output for zero-value Trace")
	}
}

func TestTraceZeroValue_ValidateReturnsError(t *testing.T) {
	var tr schema.Trace
	if err := tr.Validate(); err == nil {
		t.Error("expected Validate to return error for zero-value Trace, got nil")
	}
}

func TestTraceZeroValue_TagsIsNil(t *testing.T) {
	var tr schema.Trace
	if tr.Tags != nil {
		t.Errorf("expected Tags to be nil on zero-value Trace, got %v", tr.Tags)
	}
}

// --- Group 4: Tag Constants ---

func TestTagConstants_MatchExpectedStrings(t *testing.T) {
	cases := []struct {
		constant schema.TagValue
		expected string
	}{
		{schema.TagDelay, "delay"},
		{schema.TagThreshold, "threshold"},
		{schema.TagBlockage, "blockage"},
		{schema.TagAmplification, "amplification"},
		{schema.TagRedirection, "redirection"},
		{schema.TagTranslation, "translation"},
	}
	for _, tc := range cases {
		if string(tc.constant) != tc.expected {
			t.Errorf("TagValue %q: got %q, want %q", tc.constant, string(tc.constant), tc.expected)
		}
	}
}

func TestTraceWithKnownTags_RoundTrips(t *testing.T) {
	tr := validTrace()
	tr.Tags = []string{string(schema.TagDelay), string(schema.TagThreshold)}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got schema.Trace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "delay" || got.Tags[1] != "threshold" {
		t.Errorf("Tags did not survive round-trip: got %v", got.Tags)
	}
}

// --- Group 5: Interoperability ---

func TestTraceFromJSON_UnknownFieldsIgnored(t *testing.T) {
	raw := `{
		"id": "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		"timestamp": "2026-03-10T12:00:00Z",
		"what_changed": "something shifted",
		"observer": "monitor-1",
		"foo": "bar",
		"unknown_future_field": 42
	}`
	var tr schema.Trace
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("Unmarshal with unknown fields failed: %v", err)
	}
	if tr.WhatChanged != "something shifted" {
		t.Errorf("WhatChanged: got %q, want %q", tr.WhatChanged, "something shifted")
	}
}

func TestTraceFromJSON_FutureTagValueAccepted(t *testing.T) {
	raw := `{
		"id": "a1b2c3d4-e5f6-4a7b-8c9d-e0f1a2b3c4d5",
		"timestamp": "2026-03-10T12:00:00Z",
		"what_changed": "novel event",
		"observer": "monitor-1",
		"tags": ["novel_tag", "experimental_friction"]
	}`
	var tr schema.Trace
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("Unmarshal with novel tag values failed: %v", err)
	}
	if len(tr.Tags) != 2 || tr.Tags[0] != "novel_tag" {
		t.Errorf("Tags: got %v, want [novel_tag experimental_friction]", tr.Tags)
	}
}
