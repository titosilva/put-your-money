// Package api exposes a small read-only HTTP API over storage.Store so a
// dashboard frontend can list runs, equity curves, and fills. It depends
// only on the storage interface, not on the engine or any broker — the
// dashboard doesn't care what produced the data.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/titosilva/put-your-money/internal/storage"
)

type Server struct {
	store storage.Store
	mux   *http.ServeMux
}

func New(store storage.Store) *Server {
	s := &Server{store: store, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/runs", s.handleListRuns)
	s.mux.HandleFunc("/api/runs/equity", s.handleEquityCurve)
	s.mux.HandleFunc("/api/runs/fills", s.handleFills)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListRuns()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, runs)
}

func (s *Server) handleEquityCurve(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		http.Error(w, "missing run_id query parameter", http.StatusBadRequest)
		return
	}
	curve, err := s.store.GetEquityCurve(runID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, curve)
}

func (s *Server) handleFills(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Query().Get("run_id")
	if runID == "" {
		http.Error(w, "missing run_id query parameter", http.StatusBadRequest)
		return
	}
	fills, err := s.store.GetFills(runID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, fills)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusNotFound)
}
