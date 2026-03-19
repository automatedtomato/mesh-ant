// classdiff_test.go tests CompareChainClassifications and PrintClassificationDiffs —
// the classification-diff analysis functions for the loader package.
//
// These tests use the black-box package loader_test to verify observable
// behaviour only: diff detection by position, length-mismatch handling, and
// print output content. Implementation internals are not tested.
//
// Test groups:
//  1. CompareChainClassifications — core diff logic
//  2. PrintClassificationDiffs — report rendering
package loader_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// makeClassif constructs a DraftStepClassification with the given fields.
// Used as a concise builder in table-style test cases.
func makeClassif(stepIndex int, kind loader.DraftStepKind, reason string) loader.DraftStepClassification {
	return loader.DraftStepClassification{StepIndex: stepIndex, Kind: kind, Reason: reason}
}

// --- Group 1: CompareChainClassifications ---

// TestCompareChainClassifications_EmptyBoth verifies that when both chains are
// nil, the result is non-nil and has zero diffs.
func TestCompareChainClassifications_EmptyBoth(t *testing.T) {
	diffs := loader.CompareChainClassifications(nil, nil)

	if diffs == nil {
		t.Error("CompareChainClassifications(nil, nil): want non-nil empty slice, got nil")
	}
	if len(diffs) != 0 {
		t.Errorf("CompareChainClassifications(nil, nil): want 0 diffs, got %d: %v", len(diffs), diffs)
	}
}

// TestCompareChainClassifications_EmptyOneChain verifies that when one chain is
// nil and the other is non-nil, the result is non-nil with zero diffs (no steps
// can be compared because min(0, n) = 0).
func TestCompareChainClassifications_EmptyOneChain(t *testing.T) {
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "no change"),
	}

	// chainA nil, chainB non-nil.
	diffsA := loader.CompareChainClassifications(nil, chainB)
	if diffsA == nil {
		t.Error("CompareChainClassifications(nil, nonNil): want non-nil empty slice, got nil")
	}
	if len(diffsA) != 0 {
		t.Errorf("CompareChainClassifications(nil, nonNil): want 0 diffs, got %d", len(diffsA))
	}

	// chainA non-nil, chainB nil.
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftMediator, "content changed"),
	}
	diffsB := loader.CompareChainClassifications(chainA, nil)
	if diffsB == nil {
		t.Error("CompareChainClassifications(nonNil, nil): want non-nil empty slice, got nil")
	}
	if len(diffsB) != 0 {
		t.Errorf("CompareChainClassifications(nonNil, nil): want 0 diffs, got %d", len(diffsB))
	}
}

// TestCompareChainClassifications_IdenticalChains verifies that when both chains
// classify every step the same way, zero diffs are returned.
func TestCompareChainClassifications_IdenticalChains(t *testing.T) {
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-a-1"),
		makeClassif(2, loader.DraftMediator, "reason-a-2"),
		makeClassif(3, loader.DraftTranslation, "reason-a-3"),
	}
	// chainB uses different reasons but the same Kinds — Reason doesn't drive a diff.
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-b-1"),
		makeClassif(2, loader.DraftMediator, "reason-b-2"),
		makeClassif(3, loader.DraftTranslation, "reason-b-3"),
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	if diffs == nil {
		t.Error("identical chains: want non-nil empty slice, got nil")
	}
	if len(diffs) != 0 {
		t.Errorf("identical chains: want 0 diffs, got %d: %v", len(diffs), diffs)
	}
}

// TestCompareChainClassifications_SingleDivergence verifies that when two chains
// agree on step 1 but diverge on step 2, exactly one diff is returned with the
// correct StepIndex, KindA, KindB, ReasonA, and ReasonB.
func TestCompareChainClassifications_SingleDivergence(t *testing.T) {
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-a-step1"),
		makeClassif(2, loader.DraftMediator, "reason-a-step2"),
	}
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-b-step1"), // same Kind as A
		makeClassif(2, loader.DraftTranslation, "reason-b-step2"), // different Kind
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	if len(diffs) != 1 {
		t.Fatalf("single divergence: want 1 diff, got %d: %v", len(diffs), diffs)
	}

	d := diffs[0]
	if d.StepIndex != 2 {
		t.Errorf("diff.StepIndex: want 2, got %d", d.StepIndex)
	}
	if d.KindA != loader.DraftMediator {
		t.Errorf("diff.KindA: want %q, got %q", loader.DraftMediator, d.KindA)
	}
	if d.KindB != loader.DraftTranslation {
		t.Errorf("diff.KindB: want %q, got %q", loader.DraftTranslation, d.KindB)
	}
	if d.ReasonA != "reason-a-step2" {
		t.Errorf("diff.ReasonA: want %q, got %q", "reason-a-step2", d.ReasonA)
	}
	if d.ReasonB != "reason-b-step2" {
		t.Errorf("diff.ReasonB: want %q, got %q", "reason-b-step2", d.ReasonB)
	}
}

// TestCompareChainClassifications_FullDivergence verifies that when every step
// has a different Kind, the number of diffs equals len(chain).
func TestCompareChainClassifications_FullDivergence(t *testing.T) {
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-a-1"),
		makeClassif(2, loader.DraftMediator, "reason-a-2"),
		makeClassif(3, loader.DraftTranslation, "reason-a-3"),
	}
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftTranslation, "reason-b-1"),  // differs
		makeClassif(2, loader.DraftIntermediary, "reason-b-2"), // differs
		makeClassif(3, loader.DraftMediator, "reason-b-3"),     // differs
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	if len(diffs) != 3 {
		t.Fatalf("full divergence: want 3 diffs, got %d: %v", len(diffs), diffs)
	}

	// Each diff must carry the correct StepIndex, KindA, and KindB.
	wantStep := []int{1, 2, 3}
	wantKindA := []loader.DraftStepKind{loader.DraftIntermediary, loader.DraftMediator, loader.DraftTranslation}
	wantKindB := []loader.DraftStepKind{loader.DraftTranslation, loader.DraftIntermediary, loader.DraftMediator}
	for i, d := range diffs {
		if d.StepIndex != wantStep[i] {
			t.Errorf("diffs[%d].StepIndex: want %d, got %d", i, wantStep[i], d.StepIndex)
		}
		if d.KindA != wantKindA[i] {
			t.Errorf("diffs[%d].KindA: want %q, got %q", i, wantKindA[i], d.KindA)
		}
		if d.KindB != wantKindB[i] {
			t.Errorf("diffs[%d].KindB: want %q, got %q", i, wantKindB[i], d.KindB)
		}
	}
}

// TestCompareChainClassifications_DifferentLengths_AisLonger verifies that when
// chainA is longer than chainB, only steps up to len(chainB) are compared.
// Steps beyond the shorter chain are not compared even if they would diverge.
func TestCompareChainClassifications_DifferentLengths_AisLonger(t *testing.T) {
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-a-1"),
		makeClassif(2, loader.DraftMediator, "reason-a-2"),
		makeClassif(3, loader.DraftTranslation, "reason-a-3"), // no counterpart in B
	}
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-b-1"), // same
		makeClassif(2, loader.DraftTranslation, "reason-b-2"),  // differs
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	// Only step 2 diverges; step 3 has no counterpart and must not appear.
	if len(diffs) != 1 {
		t.Fatalf("A longer: want 1 diff, got %d: %v", len(diffs), diffs)
	}
	if diffs[0].StepIndex != 2 {
		t.Errorf("diff StepIndex: want 2, got %d", diffs[0].StepIndex)
	}
}

// TestCompareChainClassifications_DifferentLengths_BisLonger verifies the
// symmetric case: when chainB is longer, only steps up to len(chainA) matter.
func TestCompareChainClassifications_DifferentLengths_BisLonger(t *testing.T) {
	chainA := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-a-1"), // same
		makeClassif(2, loader.DraftMediator, "reason-a-2"),     // same
	}
	chainB := []loader.DraftStepClassification{
		makeClassif(1, loader.DraftIntermediary, "reason-b-1"), // same
		makeClassif(2, loader.DraftMediator, "reason-b-2"),     // same
		makeClassif(3, loader.DraftTranslation, "reason-b-3"),  // no counterpart in A
	}

	diffs := loader.CompareChainClassifications(chainA, chainB)

	// Steps 1 and 2 agree; step 3 has no counterpart and must not appear as a diff.
	if len(diffs) != 0 {
		t.Errorf("B longer, steps agree: want 0 diffs, got %d: %v", len(diffs), diffs)
	}
}

// --- Group 2: PrintClassificationDiffs ---

// TestPrintClassificationDiffs_NoDivergence verifies that when diffs is empty
// and lengths are equal, the output contains a "No classification divergence"
// message (or equivalent).
func TestPrintClassificationDiffs_NoDivergence(t *testing.T) {
	var buf bytes.Buffer
	err := loader.PrintClassificationDiffs(&buf, "alice", "bob", 3, 3, []loader.ClassificationDiff{})

	if err != nil {
		t.Fatalf("PrintClassificationDiffs() returned unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "No classification divergence") &&
		!strings.Contains(out, "no classification divergence") &&
		!strings.Contains(out, "No divergence") {
		t.Errorf("output missing no-divergence message; got:\n%s", out)
	}
}

// TestPrintClassificationDiffs_WithDivergence verifies that when there is one
// diff, the output contains the analyst labels, the step index, and both
// Kind strings.
func TestPrintClassificationDiffs_WithDivergence(t *testing.T) {
	diffs := []loader.ClassificationDiff{
		{
			StepIndex: 2,
			KindA:     loader.DraftMediator,
			KindB:     loader.DraftTranslation,
			ReasonA:   "reason from alice",
			ReasonB:   "reason from bob",
		},
	}

	var buf bytes.Buffer
	err := loader.PrintClassificationDiffs(&buf, "alice", "bob", 3, 3, diffs)

	if err != nil {
		t.Fatalf("PrintClassificationDiffs() returned unexpected error: %v", err)
	}
	out := buf.String()

	// Must contain both analyst labels.
	if !strings.Contains(out, "alice") {
		t.Errorf("output missing analyst label %q; got:\n%s", "alice", out)
	}
	if !strings.Contains(out, "bob") {
		t.Errorf("output missing analyst label %q; got:\n%s", "bob", out)
	}

	// Must reference the step index explicitly as "Step 2" (1-based, matching
	// the StepIndex in the diff). The bare-digit fallback is intentionally
	// omitted — "2" appears in many unrelated output lines.
	if !strings.Contains(out, "Step 2") {
		t.Errorf("output missing step index 'Step 2'; got:\n%s", out)
	}

	// Must contain both Kind strings.
	if !strings.Contains(out, string(loader.DraftMediator)) {
		t.Errorf("output missing KindA %q; got:\n%s", loader.DraftMediator, out)
	}
	if !strings.Contains(out, string(loader.DraftTranslation)) {
		t.Errorf("output missing KindB %q; got:\n%s", loader.DraftTranslation, out)
	}
}

// TestPrintClassificationDiffs_LengthMismatch verifies that when lenA != lenB,
// the output mentions the difference between the two chain lengths.
func TestPrintClassificationDiffs_LengthMismatch(t *testing.T) {
	var buf bytes.Buffer
	err := loader.PrintClassificationDiffs(&buf, "alice", "bob", 2, 3, []loader.ClassificationDiff{})

	if err != nil {
		t.Fatalf("PrintClassificationDiffs() returned unexpected error: %v", err)
	}
	out := buf.String()

	// Output must name both chain lengths explicitly. Both "2 steps" AND "3 steps"
	// must be present — an OR would accept output that names only one side.
	if !strings.Contains(out, "2 steps") || !strings.Contains(out, "3 steps") {
		t.Errorf("output must name both lengths (2 steps and 3 steps); got:\n%s", out)
	}
}

// TestPrintClassificationDiffs_BothLabelsPresent verifies that the output
// contains both the analystA and analystB label strings in all cases, including
// when there are no diffs.
func TestPrintClassificationDiffs_BothLabelsPresent(t *testing.T) {
	var buf bytes.Buffer
	err := loader.PrintClassificationDiffs(&buf, "position-alpha", "position-beta", 2, 2, []loader.ClassificationDiff{})

	if err != nil {
		t.Fatalf("PrintClassificationDiffs() returned unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "position-alpha") {
		t.Errorf("output missing analystA label %q; got:\n%s", "position-alpha", out)
	}
	if !strings.Contains(out, "position-beta") {
		t.Errorf("output missing analystB label %q; got:\n%s", "position-beta", out)
	}
}

// TestPrintClassificationDiffs_NeitherAuthoritative verifies that the output
// contains a closing note including "Neither" or "authoritative" to signal that
// no classification position is treated as the ground truth.
func TestPrintClassificationDiffs_NeitherAuthoritative(t *testing.T) {
	var buf bytes.Buffer
	err := loader.PrintClassificationDiffs(&buf, "alice", "bob", 2, 2, []loader.ClassificationDiff{})

	if err != nil {
		t.Fatalf("PrintClassificationDiffs() returned unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "Neither") && !strings.Contains(out, "neither") &&
		!strings.Contains(out, "authoritative") {
		t.Errorf("output missing 'Neither'/'authoritative' disclaimer; got:\n%s", out)
	}
}

// classdiffFailWriter is an io.Writer that always fails immediately. Used to
// verify that PrintClassificationDiffs propagates and wraps write errors.
type classdiffFailWriter struct{}

func (classdiffFailWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk full") }

// TestPrintClassificationDiffs_WriteError verifies that a failing io.Writer
// causes PrintClassificationDiffs to return a wrapped error that names the
// function. This guards against silent error-swallowing after a refactor.
func TestPrintClassificationDiffs_WriteError(t *testing.T) {
	err := loader.PrintClassificationDiffs(classdiffFailWriter{}, "alice", "bob", 1, 1, []loader.ClassificationDiff{})

	if err == nil {
		t.Fatal("PrintClassificationDiffs(failWriter): want error, got nil")
	}
	if !strings.Contains(err.Error(), "PrintClassificationDiffs") {
		t.Errorf("error should name the function; got: %v", err)
	}
}
