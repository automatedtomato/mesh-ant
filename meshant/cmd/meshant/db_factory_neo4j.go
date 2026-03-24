//go:build neo4j

// db_factory_neo4j.go provides the openDB implementation for neo4j builds.
//
// This file is compiled only with -tags neo4j and links against the Neo4j
// driver. The analytical commands call openDB to obtain a TraceStore without
// knowing which backend is in use.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// openDB creates a Neo4jStore connected to dbURL.
//
// Credentials and the target database name are read from environment variables
// so they are never passed as CLI flags (which would appear in process listings
// and shell history):
//
//	MESHANT_DB_USER  — username (default: "neo4j")
//	MESHANT_DB_PASS  — password (default: "")
//	MESHANT_DB_NAME  — target database; empty uses the driver default ("neo4j")
//
// Production deployments must supply MESHANT_DB_USER and MESHANT_DB_PASS
// explicitly. The defaults are suitable for local development only.
func openDB(ctx context.Context, dbURL string) (store.TraceStore, error) {
	if dbURL == "" {
		return nil, fmt.Errorf("openDB: dbURL must not be empty")
	}
	user := os.Getenv("MESHANT_DB_USER")
	if user == "" {
		user = "neo4j"
	}
	cfg := store.Neo4jConfig{
		BoltURL:  dbURL,
		Username: user,
		Password: os.Getenv("MESHANT_DB_PASS"),
		Database: os.Getenv("MESHANT_DB_NAME"),
	}
	return store.NewNeo4jStore(ctx, cfg)
}
