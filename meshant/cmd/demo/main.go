// Package main is the MeshAnt minimal demo.
//
// The demo performs two observer-position cuts on the coastal evacuation
// order dataset, then diffs them to expose the structural asymmetry between
// what each position can see.
//
// Cut A — meteorological-analyst, 2026-04-14 (T-72h):
// sees the scientific chain that triggers the alert: storm models, sensor
// readings, advisory issuance. The political and logistical network is in
// shadow.
//
// Cut B — local-mayor, 2026-04-16 (T-24h):
// sees the mandatory order, media broadcast, resident friction, shelter
// overflow, and road capacity constraints. The entire non-human sensor and
// model chain that made the order necessary is in shadow.
//
// The diff between A and B makes both blindnesses simultaneously visible.
// That is the ANT demonstration: no single observer holds the whole network.
// Every articulation names its own shadow.
//
// Known gap (Principle 8): the acts of articulation performed here are not
// themselves recorded as traces. The framework observes but does not yet
// observe itself observing. This is tracked as M7-B.
//
// Usage:
//
//	go run ./cmd/demo/ [path/to/dataset.json]
//
// If no path is given, the demo resolves the evacuation dataset relative to
// its own source directory. Inside a Docker container the dataset path is
// passed as the first argument by the ENTRYPOINT.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// defaultDatasetPath is used when no argument is supplied. It is relative
// to the package source directory (meshant/cmd/demo/), so three levels up
// reaches the repository root before descending into data/examples/.
const defaultDatasetPath = "../../../data/examples/evacuation_order.json"

func main() {
	path := defaultDatasetPath
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	if err := run(os.Stdout, path); err != nil {
		log.Fatalf("demo: %v", err)
	}
}

// run executes the full demo pipeline and writes all output to w.
// Accepting io.Writer keeps the function independently testable without
// capturing os.Stdout.
//
// Pipeline:
//  1. Load and validate the dataset.
//  2. Print a full mesh summary (all 28 traces).
//  3. Articulate Cut A: meteorological-analyst, 2026-04-14.
//  4. Print articulation A.
//  5. Articulate Cut B: local-mayor, 2026-04-16.
//  6. Print articulation B.
//  7. Diff A vs B.
//  8. Print the diff.
//  9. Print a closing note naming the known shadow (Principle 8 gap).
func run(w io.Writer, datasetPath string) error {
	// --- Step 1: Load ---
	traces, err := loader.Load(datasetPath)
	if err != nil {
		return fmt.Errorf("load %q: %w", datasetPath, err)
	}

	// --- Step 2: Mesh summary ---
	summary := loader.Summarise(traces)
	if err := loader.PrintSummary(w, summary); err != nil {
		return fmt.Errorf("print summary: %w", err)
	}

	// --- Steps 3–4: Cut A — meteorological-analyst, T-72h ---
	optsA := graph.ArticulationOptions{
		ObserverPositions: []string{"meteorological-analyst"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC),
		},
	}
	gA := graph.Articulate(traces, optsA)
	if err := graph.PrintArticulation(w, gA); err != nil {
		return fmt.Errorf("print articulation A: %w", err)
	}

	// --- Steps 5–6: Cut B — local-mayor, T-24h ---
	optsB := graph.ArticulationOptions{
		ObserverPositions: []string{"local-mayor"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC),
		},
	}
	gB := graph.Articulate(traces, optsB)
	if err := graph.PrintArticulation(w, gB); err != nil {
		return fmt.Errorf("print articulation B: %w", err)
	}

	// --- Steps 7–8: Diff A vs B ---
	d := graph.Diff(gA, gB)
	if err := graph.PrintDiff(w, d); err != nil {
		return fmt.Errorf("print diff: %w", err)
	}

	// --- Step 9: Closing note ---
	if err := printClosingNote(w); err != nil {
		return fmt.Errorf("print closing note: %w", err)
	}

	return nil
}

// printClosingNote writes the methodological coda: what this demo shows,
// what it leaves open, and where the work goes next. It names the Principle 8
// gap explicitly so the shadow does not pass unobserved.
func printClosingNote(w io.Writer) error {
	lines := []string{
		"",
		"=== Note on this articulation ===",
		"",
		"The two cuts above are not neutral observations. Each is made from a",
		"specific position, at a specific time, with a specific set of traces",
		"rendered visible and a specific shadow cast.",
		"",
		"Cut A (meteorological-analyst, T-72h) sees the non-human chain:",
		"storm models, sensor networks, surge simulations. It cannot see the",
		"political friction, the 2019 false-alarm distrust, the shelter failures,",
		"or the dual-signature blockage that will slow enforcement on day 3.",
		"",
		"Cut B (local-mayor, T-24h) sees the order, the media, the residents",
		"who refuse to leave, the shelter that failed inspection. It cannot see",
		"the sensor readings, the model outputs, or the non-human chain that",
		"made the order scientifically necessary.",
		"",
		"The diff above names both absences simultaneously. That is the point.",
		"",
		"Known gap — Principle 8:",
		"The acts of articulation performed above are not themselves recorded",
		"as traces. The framework has observed the network but has not yet",
		"observed itself observing. Graph-as-actor (M5) gives individual",
		"articulations stable identities; recording those acts as traces",
		"will close this loop. Tracked as M7-B.",
		"",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
