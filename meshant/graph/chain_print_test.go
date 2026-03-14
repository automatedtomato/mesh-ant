// chain_print_test.go — tests for PrintChain and PrintChainJSON.
// TDD: these tests are written before the implementation.
package graph_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

func TestPrintChain_BasicOutput(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "detected", Mediation: "sensor", Sources: []string{"A"}, Targets: []string{"B"}},
					ElementExited:  "A",
					ElementEntered: "B",
				},
				{
					Edge:           graph.Edge{TraceID: "t2", WhatChanged: "forwarded", Sources: []string{"B"}, Targets: []string{"C"}},
					ElementExited:  "B",
					ElementEntered: "C",
				},
			},
			Breaks: []graph.ChainBreak{
				{AtElement: "C", Reason: "no-outgoing-edges"},
			},
			Cut: graph.Cut{ObserverPositions: []string{"obs-1"}},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepMediator, Reason: "mediation present"},
			{StepIndex: 1, Kind: graph.StepIntermediary, Reason: "no mediation observed"},
		},
	}

	var buf bytes.Buffer
	err := graph.PrintChain(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}

	out := buf.String()

	// Check key content is present
	checks := []string{
		"Translation Chain",
		"Start element: A",
		"obs-1",
		"detected",
		"mediator",
		"intermediary",
		"no-outgoing-edges",
		"analytical judgment",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("output missing %q", c)
		}
	}
}

func TestPrintChain_EmptyChain(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Breaks: []graph.ChainBreak{
				{AtElement: "A", Reason: "element-not-in-graph"},
			},
		},
	}

	var buf bytes.Buffer
	err := graph.PrintChain(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Steps (0)") {
		t.Error("expected 'Steps (0)' for empty chain")
	}
	if !strings.Contains(out, "element-not-in-graph") {
		t.Error("expected break reason in output")
	}
}

func TestPrintChain_MediationShown(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "x", Mediation: "review-board", Sources: []string{"A"}, Targets: []string{"B"}},
					ElementExited:  "A",
					ElementEntered: "B",
				},
			},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepMediator, Reason: "mediation present"},
		},
	}

	var buf bytes.Buffer
	_ = graph.PrintChain(&buf, cc)
	out := buf.String()

	if !strings.Contains(out, "review-board") {
		t.Error("expected mediation name in output")
	}
}

func TestPrintChainJSON_ValidJSON(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "x", Sources: []string{"A"}, Targets: []string{"B"}},
					ElementExited:  "A",
					ElementEntered: "B",
				},
			},
			Breaks: []graph.ChainBreak{
				{AtElement: "B", Reason: "no-outgoing-edges"},
			},
			Cut: graph.Cut{ObserverPositions: []string{"obs-1"}},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepIntermediary, Reason: "no mediation observed"},
		},
	}

	var buf bytes.Buffer
	err := graph.PrintChainJSON(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChainJSON error: %v", err)
	}

	// Must be valid JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check key fields exist
	if _, ok := raw["start_element"]; !ok {
		t.Error("missing 'start_element' in JSON")
	}
	if _, ok := raw["steps"]; !ok {
		t.Error("missing 'steps' in JSON")
	}
	if _, ok := raw["classifications"]; !ok {
		t.Error("missing 'classifications' in JSON")
	}
}

func TestPrintChain_WithTimeWindowAndTags(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "X",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "test", Sources: []string{"X"}, Targets: []string{"Y"}},
					ElementExited:  "X",
					ElementEntered: "Y",
				},
			},
			Cut: graph.Cut{
				ObserverPositions: []string{"obs-1"},
				TimeWindow: graph.TimeWindow{
					Start: mustParseTime(t, "2026-04-14T00:00:00Z"),
					End:   mustParseTime(t, "2026-04-16T23:59:59Z"),
				},
				Tags: []string{"delay", "threshold"},
			},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepIntermediary, Reason: "no mediation observed"},
		},
	}

	var buf bytes.Buffer
	err := graph.PrintChain(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}

	out := buf.String()
	// Time window should appear
	if !strings.Contains(out, "2026-04-14") {
		t.Error("output missing time window start")
	}
	if !strings.Contains(out, "2026-04-16") {
		t.Error("output missing time window end")
	}
	// Tags should appear
	if !strings.Contains(out, "delay") {
		t.Error("output missing tag 'delay'")
	}
}

func TestPrintChain_NoFilters(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps:        []graph.ChainStep{},
			Cut:          graph.Cut{},
		},
	}

	var buf bytes.Buffer
	err := graph.PrintChain(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}
	if !strings.Contains(buf.String(), "full cut") {
		t.Error("expected 'full cut' for empty cut parameters")
	}
}

func TestPrintChainJSON_EmptyChain(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{StartElement: "Z"},
	}

	var buf bytes.Buffer
	err := graph.PrintChainJSON(&buf, cc)
	if err != nil {
		t.Fatalf("PrintChainJSON error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// --- Criterion block in PrintChain output ---

// TestPrintChain_WithCriterion_FullBlock verifies that a fully-populated
// criterion produces all expected lines in the human-readable output:
// name, declaration, preserve list, ignore list, and the mandatory
// heuristics disclaimer line.
func TestPrintChain_WithCriterion_FullBlock(t *testing.T) {
	crit := graph.EquivalenceCriterion{
		Name:        "my-criterion",
		Declaration: "continuity means the measurement is preserved",
		Preserve:    []string{"sensor-value", "calibration"},
		Ignore:      []string{"timestamp", "observer-id"},
	}
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps:        []graph.ChainStep{},
		},
		Criterion: crit,
	}

	var buf bytes.Buffer
	if err := graph.PrintChain(&buf, cc); err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}
	out := buf.String()

	checks := []string{
		"Criterion: my-criterion",
		"Declaration: continuity means the measurement is preserved",
		"Preserve: [sensor-value, calibration]",
		"Ignore: [timestamp, observer-id]",
		"(criterion carried — classification uses v1 heuristics)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// TestPrintChain_WithCriterion_DeclarationOnly verifies that when the criterion
// has only a Declaration (no Name, Preserve, or Ignore), the name/preserve/ignore
// lines are absent, but the declaration line and heuristics line are present.
func TestPrintChain_WithCriterion_DeclarationOnly(t *testing.T) {
	crit := graph.EquivalenceCriterion{
		Declaration: "continuity means nothing was reordered",
	}
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps:        []graph.ChainStep{},
		},
		Criterion: crit,
	}

	var buf bytes.Buffer
	if err := graph.PrintChain(&buf, cc); err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}
	out := buf.String()

	// Must be present
	if !strings.Contains(out, "Declaration: continuity means nothing was reordered") {
		t.Errorf("output missing Declaration line\nfull output:\n%s", out)
	}
	if !strings.Contains(out, "(criterion carried — classification uses v1 heuristics)") {
		t.Errorf("output missing heuristics disclaimer line\nfull output:\n%s", out)
	}

	// Must be absent — no name, no preserve, no ignore
	if strings.Contains(out, "Criterion:") {
		t.Errorf("output should not have 'Criterion:' name line when Name is empty\nfull output:\n%s", out)
	}
	if strings.Contains(out, "Preserve:") {
		t.Errorf("output should not have 'Preserve:' line when Preserve is empty\nfull output:\n%s", out)
	}
	if strings.Contains(out, "Ignore:") {
		t.Errorf("output should not have 'Ignore:' line when Ignore is empty\nfull output:\n%s", out)
	}
}

// TestPrintChain_NoCriterion_NoBlock verifies that when the criterion is zero,
// no criterion-related block appears in the output. This preserves the v1
// rendering contract for all existing callers that pass no criterion.
func TestPrintChain_NoCriterion_NoBlock(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "x", Mediation: "relay"},
					ElementExited:  "A",
					ElementEntered: "B",
				},
			},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepMediator, Reason: "mediation present"},
		},
		// Criterion is zero (not set)
	}

	var buf bytes.Buffer
	if err := graph.PrintChain(&buf, cc); err != nil {
		t.Fatalf("PrintChain error: %v", err)
	}
	out := buf.String()

	criterionMarkers := []string{
		"Criterion:",
		"Declaration:",
		"Preserve:",
		"Ignore:",
		"criterion carried",
	}
	for _, marker := range criterionMarkers {
		if strings.Contains(out, marker) {
			t.Errorf("output should not contain %q when criterion is zero\nfull output:\n%s", marker, out)
		}
	}
}

// TestPrintChainJSON_WithCriterion verifies that a non-zero criterion appears
// in the JSON output under the "criterion" key with the correct nested fields.
func TestPrintChainJSON_WithCriterion(t *testing.T) {
	crit := graph.EquivalenceCriterion{
		Name:        "json-criterion",
		Declaration: "continuity is preserved if the key signal survives",
		Preserve:    []string{"key-signal"},
		Ignore:      []string{"noise"},
	}
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps:        []graph.ChainStep{},
		},
		Criterion: crit,
	}

	var buf bytes.Buffer
	if err := graph.PrintChainJSON(&buf, cc); err != nil {
		t.Fatalf("PrintChainJSON error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v\nraw output: %s", err, buf.String())
	}

	critRaw, ok := raw["criterion"]
	if !ok {
		t.Fatalf("JSON missing 'criterion' key\nfull output:\n%s", buf.String())
	}

	critMap, ok := critRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("'criterion' is not a JSON object, got %T", critRaw)
	}

	if critMap["name"] != "json-criterion" {
		t.Errorf("criterion.name = %v, want %q", critMap["name"], "json-criterion")
	}
	if critMap["declaration"] != "continuity is preserved if the key signal survives" {
		t.Errorf("criterion.declaration = %v, want expected string", critMap["declaration"])
	}

	// Preserve must be present as a JSON array with correct values.
	preserveRaw, ok := critMap["preserve"]
	if !ok {
		t.Fatal("criterion.preserve missing from JSON")
	}
	preserveArr, ok := preserveRaw.([]interface{})
	if !ok || len(preserveArr) != 1 || preserveArr[0] != "key-signal" {
		t.Errorf("criterion.preserve = %v, want [key-signal]", preserveRaw)
	}

	// Ignore must be present as a JSON array with correct values.
	ignoreRaw, ok := critMap["ignore"]
	if !ok {
		t.Fatal("criterion.ignore missing from JSON")
	}
	ignoreArr, ok := ignoreRaw.([]interface{})
	if !ok || len(ignoreArr) != 1 || ignoreArr[0] != "noise" {
		t.Errorf("criterion.ignore = %v, want [noise]", ignoreRaw)
	}
}

// TestPrintChainJSON_ZeroCriterion_Omitted enforces design rule A2: when the
// criterion is zero (not declared), the "criterion" key must be absent from
// the JSON output entirely. This prevents the output from leaking empty
// criterion objects that would be meaningless to downstream consumers.
func TestPrintChainJSON_ZeroCriterion_Omitted(t *testing.T) {
	cc := graph.ClassifiedChain{
		Chain: graph.TranslationChain{
			StartElement: "A",
			Steps: []graph.ChainStep{
				{
					Edge:           graph.Edge{TraceID: "t1", WhatChanged: "x"},
					ElementExited:  "A",
					ElementEntered: "B",
				},
			},
		},
		Classifications: []graph.StepClassification{
			{StepIndex: 0, Kind: graph.StepIntermediary, Reason: "no mediation observed"},
		},
		// Criterion is zero (not set)
	}

	var buf bytes.Buffer
	if err := graph.PrintChainJSON(&buf, cc); err != nil {
		t.Fatalf("PrintChainJSON error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v\nraw output: %s", err, buf.String())
	}

	if _, ok := raw["criterion"]; ok {
		t.Errorf("JSON should not contain 'criterion' key when criterion is zero\nfull output:\n%s", buf.String())
	}
}
