// Package main is the MeshAnt minimal demo.
//
// The demo constructs two observer-position cuts on the coastal evacuation
// order dataset, then diffs them to make visible the asymmetry between
// what each position can see.
//
// Cut A — meteorological-analyst, 2026-04-14 (T-72h):
// sees the sensor and model chain that triggers the alert: storm-track-model,
// tide gauges, advisory issuance. The political and logistical network is
// in shadow.
//
// Cut B — local-mayor, 2026-04-16 (T-24h):
// sees the mandatory order, media broadcast, resident friction, shelter
// overflow, road capacity constraints. The sensor and model chain that
// made the order necessary is in shadow.
//
// The diff between A and B makes both absences simultaneously visible —
// a provisional reading, not a god's-eye account.
//
// Known gap (Principle 8): this demo records observer positions
// (meteorological-analyst, local-mayor) but does not record its own
// position: which cuts were chosen, by whom, and from where.
//
// Usage:
//
//	go run ./cmd/demo [path/to/dataset.json]
//
// If no path is given, the dataset is resolved relative to the package
// source directory. A compiled binary placed elsewhere must supply the
// path as an explicit argument.
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

// defaultDatasetPath is correct when the working directory is the package source
// directory (as with `go run ./cmd/demo` from meshant/). Compiled binaries must
// supply the path as os.Args[1].
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

// run executes the full demo pipeline and writes output to w.
func run(w io.Writer, datasetPath string) error {
	traces, err := loader.Load(datasetPath)
	if err != nil {
		return fmt.Errorf("load %q: %w", datasetPath, err)
	}

	if err := loader.PrintSummary(w, loader.Summarise(traces)); err != nil {
		return fmt.Errorf("print summary: %w", err)
	}

	optsA := graph.ArticulationOptions{
		ObserverPositions: []string{"meteorological-analyst"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 14, 23, 59, 59, 0, time.UTC),
		},
	}
	if err := optsA.TimeWindow.Validate(); err != nil { // guard: inverted window → zero-trace graph
		return fmt.Errorf("cut A time window: %w", err)
	}
	gA := graph.Articulate(traces, optsA)
	if err := graph.PrintArticulation(w, gA); err != nil {
		return fmt.Errorf("print articulation A: %w", err)
	}

	optsB := graph.ArticulationOptions{
		ObserverPositions: []string{"local-mayor"},
		TimeWindow: graph.TimeWindow{
			Start: time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC),
		},
	}
	if err := optsB.TimeWindow.Validate(); err != nil { // guard: inverted window → zero-trace graph
		return fmt.Errorf("cut B time window: %w", err)
	}
	gB := graph.Articulate(traces, optsB)
	if err := graph.PrintArticulation(w, gB); err != nil {
		return fmt.Errorf("print articulation B: %w", err)
	}

	if err := graph.PrintDiff(w, graph.Diff(gA, gB)); err != nil {
		return fmt.Errorf("print diff: %w", err)
	}

	if err := printClosingNote(w); err != nil {
		return fmt.Errorf("print closing note: %w", err)
	}
	return nil
}

// printClosingNote writes the methodological coda, naming the Principle 8 gap explicitly.
func printClosingNote(w io.Writer) error {
	const note = `
=== Note on this articulation ===

The two cuts above are situated, not neutral. Each is made from a
specific position, at a specific time, with a specific set of traces
rendered visible and a specific shadow cast.

Cut A (meteorological-analyst, T-72h) sees the sensor and model chain:
storm-track-model, tide gauges, surge simulations. It cannot see the
political friction, the 2019 false-alarm distrust, the shelter failures,
or the dual-signature blockage that held up enforcement on day 3.

Cut B (local-mayor, T-24h) sees the order, the media, the residents
who refuse to leave, the shelter that failed inspection. It cannot see
the sensor readings, the model outputs, or the sensor-and-model chain
that made the order scientifically necessary.

The diff above names both absences simultaneously — a provisional reading,
not a god's-eye account of what happened.

One graph-as-actor trace is already present in this dataset: Trace 28
records the meteorological-analyst cut entering the mesh as a source in
a subsequent coordination decision. The observation apparatus is not
standing outside; it is already a participant.

Known gap — Principle 8:
This demo records observer positions but not its own position: the choice
of these two cuts, these parameters, this rendering. Graph-as-actor gives
articulations stable identities; recording the act of running this demo
as a trace would close that loop.

`
	_, err := fmt.Fprint(w, note)
	return err
}
