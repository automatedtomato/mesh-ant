package serve

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/automatedtomato/mesh-ant/meshant/store"
)

// webFS holds the embedded web/ directory. The go:embed directive is resolved
// at compile time, producing a self-contained binary that serves the SPA without
// any runtime file-system dependency.
//
//go:embed web
var webFS embed.FS

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
//	GET /articulate          — observer-situated graph cut
//	GET /diff                — difference between two observer cuts
//	GET /shadow              — shadow elements for a cut
//	GET /traces              — raw traces filtered by observer
//	GET /element/{name}      — traces mentioning named element, observer-scoped
//	GET /                    — SPA index.html (static file server)
//	GET /style.css, /app.js  — static web assets (go:embed web/)
//
// API routes are registered before the static file handler so they take
// precedence over any path clashes.
// Unknown routes return 404; non-GET methods on known routes return 405.
func NewServer(ts store.TraceStore) *Server {
	s := &Server{ts: ts}
	mux := http.NewServeMux()

	// API routes — must be registered before the catch-all static handler.
	mux.HandleFunc("GET /articulate", s.handleArticulate)
	mux.HandleFunc("GET /diff", s.handleDiff)
	mux.HandleFunc("GET /shadow", s.handleShadow)
	mux.HandleFunc("GET /traces", s.handleTraces)
	mux.HandleFunc("GET /element/{name}", s.handleElement)
	mux.HandleFunc("GET /observers", s.handleObservers)

	// Static file server for the embedded web/ SPA.
	// fs.Sub strips the "web" prefix so that web/index.html is served at /.
	// The go:embed directive above guarantees "web" exists at compile time,
	// so fs.Sub can only fail if the embed directive is changed — panic to
	// surface that as a programming error rather than silently serving nothing.
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic("serve: failed to sub embedded web/ directory: " + err.Error())
	}
	fileServer := http.FileServerFS(subFS)
	mux.Handle("GET /", fileServer)

	s.mux = mux
	return s
}

// ServeHTTP implements http.Handler, delegating to the internal ServeMux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
