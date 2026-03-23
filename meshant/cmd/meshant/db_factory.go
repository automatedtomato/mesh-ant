//go:build !neo4j

// db_factory.go provides the openDB stub for builds without the neo4j tag.
//
// When users pass --db without building with -tags neo4j, they receive a clear
// error message instead of a nil-pointer dereference or a silent no-op.
package main

import (
	"context"
	"fmt"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// openDB returns a TraceStore backed by the database at dbURL.
//
// This stub is compiled when the neo4j build tag is absent. It always returns
// an error for non-empty dbURL so the user gets a clear rebuild instruction.
//
// To enable the real Neo4j backend:
//
//	go build -tags neo4j ./...
func openDB(_ context.Context, dbURL string) (store.TraceStore, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("openDB: dbURL must not be empty")
	}
	return nil, fmt.Errorf("meshant was built without Neo4j support; rebuild with -tags neo4j to use --db")
}
