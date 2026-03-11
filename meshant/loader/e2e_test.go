// e2e_test.go exercises the full Load → Summarise → PrintSummary pipeline
// against the real example dataset. It is kept separate from unit tests so
// that failures here point clearly to pipeline integration rather than an
// individual function.
package loader_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// TestE2E_FullPipeline runs the complete pipeline (Load → Summarise →
// PrintSummary) against the vendor registration example dataset and asserts
// that the output reflects the known structure of that dataset.
//
// This test is intentionally coarser than the unit tests: it checks that
// the output as a whole is coherent, not that every field is byte-exact.
func TestE2E_FullPipeline(t *testing.T) {
	// --- Stage 1: Load ---
	traces, err := loader.Load(examplesPath)
	if err != nil {
		t.Fatalf("stage 1 (Load): %v", err)
	}
	if len(traces) != 10 {
		t.Fatalf("stage 1 (Load): want 10 traces, got %d", len(traces))
	}

	// --- Stage 2: Summarise ---
	s := loader.Summarise(traces)

	// Top element: vendor-registration-application-00142 appears as target
	// in traces 2, 4, 5, 6, 7, 8, 9, 10 — count 8.
	if s.Elements["vendor-registration-application-00142"] != 8 {
		t.Errorf("stage 2 (Summarise): vendor-registration-application-00142 count: want 8, got %d",
			s.Elements["vendor-registration-application-00142"])
	}
	// intake-queue appears as target in traces 1 and 3 — count 2.
	if s.Elements["intake-queue"] != 2 {
		t.Errorf("stage 2 (Summarise): intake-queue count: want 2, got %d",
			s.Elements["intake-queue"])
	}
	// 7 unique mediations and 7 mediated traces (traces 2, 4, 5, 6, 7, 9, 10
	// have non-empty Mediation; trace 8 has no mediation).
	if len(s.Mediations) != 7 {
		t.Errorf("stage 2 (Summarise): want 7 unique mediations, got %d: %v",
			len(s.Mediations), s.Mediations)
	}
	if s.MediatedTraceCount != 7 {
		t.Errorf("stage 2 (Summarise): MediatedTraceCount: want 7, got %d", s.MediatedTraceCount)
	}
	// First mediation encountered: intake-form-validator-v4 (trace 2).
	if len(s.Mediations) > 0 && s.Mediations[0] != "intake-form-validator-v4" {
		t.Errorf("stage 2 (Summarise): mediations[0]: want %q, got %q",
			"intake-form-validator-v4", s.Mediations[0])
	}
	// Flagged traces: delay (#4, #8) and threshold (#4, #6, #10) = 4 unique traces
	// (#4 has both; appears once).
	if len(s.FlaggedTraces) != 4 {
		t.Errorf("stage 2 (Summarise): want 4 flagged traces, got %d", len(s.FlaggedTraces))
	}

	// --- Stage 3: PrintSummary ---
	var buf bytes.Buffer
	if err := loader.PrintSummary(&buf, s); err != nil {
		t.Fatalf("stage 3 (PrintSummary): %v", err)
	}
	out := buf.String()

	// Structural checks on the full output.
	requiredPhrases := []string{
		"=== Mesh Summary (provisional) ===",
		"Elements",
		"vendor-registration-application-00142",
		"x8",
		"mediations",
		"intake-form-validator-v4",
		"Traces tagged",
		"first look at the mesh",
		"not a classification of actors",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(out, phrase) {
			t.Errorf("stage 3 (PrintSummary): output missing %q", phrase)
		}
	}
}
