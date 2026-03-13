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
