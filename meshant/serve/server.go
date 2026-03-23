package serve

import (
	"net/http"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// Server is an http.Handler that serves MeshAnt analytical endpoints.
// It holds a TraceStore and queries the full substrate on each request.
// Every endpoint enforces the ANT constraint: no graph is returned without
// naming its observer position.
type Server struct {
	ts  store.TraceStore
	mux *http.ServeMux
}

// NewServer creates a Server backed by the given TraceStore.
// The returned *Server implements http.Handler.
//
// Routes registered (Go 1.22+ method+path patterns):
//
//	GET /articulate — observer-situated graph cut
//	GET /diff       — difference between two observer cuts
//	GET /shadow     — shadow elements for a cut
//	GET /traces     — raw traces filtered by observer
//
// Unknown routes return 404; non-GET methods on known routes return 405.
func NewServer(ts store.TraceStore) *Server {
	s := &Server{ts: ts}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /articulate", s.handleArticulate)
	mux.HandleFunc("GET /diff", s.handleDiff)
	mux.HandleFunc("GET /shadow", s.handleShadow)
	mux.HandleFunc("GET /traces", s.handleTraces)
	s.mux = mux
	return s
}

// ServeHTTP implements http.Handler, delegating to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
