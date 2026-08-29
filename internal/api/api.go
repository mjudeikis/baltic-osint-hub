// Package api exposes the read-only dashboard API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

type Server struct {
	db  *store.Store
	log *slog.Logger
}

func New(db *store.Store, log *slog.Logger) *Server {
	return &Server{db: db, log: log}
}

// Register mounts API routes on mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/incidents", s.handleIncidents)
	mux.HandleFunc("GET /api/stats/timeline", s.handleTimeline)
	mux.HandleFunc("GET /api/stats/summary", s.handleSummary)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// writeJSON sends the payload with a public cache header; the dashboard sits
// behind Cloudflare, which absorbs most traffic on these responses.
func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("encode", "err", err)
	}
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.IncidentFilter{
		Category: q.Get("category"),
		Country:  q.Get("country"),
	}
	if v, err := strconv.Atoi(q.Get("severity")); err == nil {
		f.Severity = v
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		f.Limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		f.Offset = v
	}
	if v, err := time.Parse(time.RFC3339, q.Get("since")); err == nil {
		f.Since = v
	} else if days, err := strconv.Atoi(q.Get("days")); err == nil && days > 0 {
		f.Since = time.Now().AddDate(0, 0, -days)
	}
	incidents, err := s.db.ListIncidents(r.Context(), f)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, incidents)
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}
	buckets, err := s.db.Timeline(r.Context(), time.Now().AddDate(0, 0, -days), r.URL.Query().Get("country"))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, buckets)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	cells, err := s.db.Summary(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, cells)
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	statuses, err := s.db.SourceStatuses(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, statuses)
}

// handleMeta serves the taxonomy so the frontend stays in sync with the backend.
func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, map[string]any{
		"categories": enrich.Categories,
		"countries":  enrich.Countries,
	})
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("query", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
