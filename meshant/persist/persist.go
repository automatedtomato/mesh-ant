// Package persist provides file I/O helpers for MeshAnt graph types.
//
// It is deliberately thin: each function is a thin wrapper around encoding/json
// and os. No graph logic lives here. This separation follows M7 Decision 1:
// the graph package must not import os or perform file I/O, so persistence is
// extracted into its own package that may freely depend on both.
//
// Typical usage:
//
//	g := graph.IdentifyGraph(graph.Articulate(traces, opts))
//	if err := persist.WriteJSON("output/graph.json", g); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Later, in another process:
//	g, err := persist.ReadGraphJSON("output/graph.json")
package persist

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/graph"
)

// WriteJSON marshals v as indented JSON and writes it to path (0644, create or overwrite).
// Returns an error if marshalling fails or the file cannot be written.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("persist: WriteJSON: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("persist: WriteJSON: write file: %w", err)
	}
	return nil
}

// ReadGraphJSON reads path and unmarshals it as a graph.MeshGraph.
// TimeWindow null bounds decode as zero time.Time (unbounded).
func ReadGraphJSON(path string) (graph.MeshGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return graph.MeshGraph{}, fmt.Errorf("persist: ReadGraphJSON: read file: %w", err)
	}
	var g graph.MeshGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return graph.MeshGraph{}, fmt.Errorf("persist: ReadGraphJSON: unmarshal: %w", err)
	}
	return g, nil
}

// ReadDiffJSON reads path and unmarshals it as a graph.GraphDiff.
func ReadDiffJSON(path string) (graph.GraphDiff, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return graph.GraphDiff{}, fmt.Errorf("persist: ReadDiffJSON: read file: %w", err)
	}
	var d graph.GraphDiff
	if err := json.Unmarshal(data, &d); err != nil {
		return graph.GraphDiff{}, fmt.Errorf("persist: ReadDiffJSON: unmarshal: %w", err)
	}
	return d, nil
}
