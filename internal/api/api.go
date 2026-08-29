// Package api exposes the read-only dashboard API.
package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
	"github.com/mjudeikis/baltic-osint-hub/internal/layers"
	"github.com/mjudeikis/baltic-osint-hub/internal/posture"
	"github.com/mjudeikis/baltic-osint-hub/internal/sources"
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
	mux.HandleFunc("GET /api/incidents.csv", s.handleIncidentsCSV)
	mux.HandleFunc("GET /api/incidents.geojson", s.handleIncidentsGeoJSON)
	mux.HandleFunc("GET /api/stats/timeline", s.handleTimeline)
	mux.HandleFunc("GET /api/stats/summary", s.handleSummary)
	mux.HandleFunc("GET /api/stats/posture", s.handlePosture)
	mux.HandleFunc("GET /api/stats/posture/history", s.handlePostureHistory)
	mux.HandleFunc("GET /api/history/{day}", s.handleHistoryDay)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/layers/firms", s.handleFIRMS)
	mux.HandleFunc("GET /api/layers/gpsjam", s.handleGpsjam)
	mux.HandleFunc("GET /api/layers/air", s.handleAir)
	mux.HandleFunc("GET /api/layers/sea", s.handleSea)
	mux.HandleFunc("GET /api/layers/sar", s.handleSAR)
	mux.HandleFunc("GET /api/layers/certpl", s.handleCertPL)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// allowCORS marks a response as readable by any origin. The whole API is
// read-only, public and unauthenticated, so there is nothing here a browser
// could be tricked into leaking — and without this header the endpoints are
// public but unusable from anyone else's page, which defeats the point of
// publishing them. We consume half a dozen open datasets; returning nothing
// would be poor manners.
func allowCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// writeJSON sends the payload with a short public cache window. It is
// deliberately short: the edge still absorbs traffic spikes, but a collector
// run that changes the data cannot leave a reader looking at a stale — and
// possibly falsely reassuring — reading for long. A five-minute window once
// showed "Calm, 0 incidents" while the API was returning 68.
func (s *Server) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	allowCORS(w)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("encode", "err", err)
	}
}

// filterFrom reads the shared incident query parameters. All three incident
// endpoints use it so an export always matches the view it was taken from.
func filterFrom(r *http.Request) store.IncidentFilter {
	q := r.URL.Query()
	f := store.IncidentFilter{
		Category: q.Get("category"),
		Country:  q.Get("country"),
		Tone:     q.Get("tone"),
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
	if v, err := time.Parse(time.RFC3339, q.Get("until")); err == nil {
		f.Until = v
	}
	// ?day=YYYY-MM-DD is the shorthand the timeline uses when a bar is
	// clicked; it overrides any since/days already parsed.
	if v, err := time.Parse("2006-01-02", q.Get("day")); err == nil {
		f.Since, f.Until = v, v.AddDate(0, 0, 1)
	}
	return f
}

func (s *Server) listOut(r *http.Request) ([]incidentOut, error) {
	incidents, err := s.db.ListIncidents(r.Context(), filterFrom(r))
	if err != nil {
		return nil, err
	}
	// Classify each item's source so the UI can mark adversary messaging
	// rather than rendering it like national broadcasting.
	out := make([]incidentOut, 0, len(incidents))
	for _, inc := range incidents {
		o := incidentOut{Incident: inc, Credibility: sources.Credibility(inc.Source)}
		// Only graded once clustered. An unclustered incident has not been
		// checked for corroboration, which is not the same as having none, so
		// it gets no label rather than a misleading "single source".
		if inc.IndependentSources != nil {
			o.ConfidenceLabel = store.ConfidenceLabel(*inc.IndependentSources)
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	out, err := s.listOut(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

// incidentOut adds the source's credibility class to the stored incident.
type incidentOut struct {
	store.Incident
	Credibility string `json:"credibility"`
	// ConfidenceLabel spells out what the confidence score means; empty when
	// the incident has not been clustered and so cannot be graded yet.
	ConfidenceLabel string `json:"confidence_label,omitempty"`
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

// handlePosture publishes the overall regional reading plus the tone balance
// it was derived from, so the number is auditable rather than a black box.
func (s *Server) handlePosture(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	byTone, sev, err := s.db.ToneCounts(r.Context(), 7, country)
	if err != nil {
		s.fail(w, err)
		return
	}
	reading := posture.Evaluate(posture.Counts{
		Positive:           byTone[enrich.TonePositive],
		Neutral:            byTone[enrich.ToneNeutral],
		Negative:           byTone[enrich.ToneNegative],
		NegativeBySeverity: sev,
	})
	// "Is this week unusual?" — a bare count invites the reader to assume the
	// worst, so publish the comparison alongside it.
	if history, err := s.db.WeeklyAdverseHistory(r.Context(), 12); err == nil {
		reading = reading.WithHistory(history)
	}
	s.writeJSON(w, reading)
}

// handlePostureHistory serves the trailing daily readings, so the dashboard's
// own track record is visible rather than only its current opinion.
func (s *Server) handlePostureHistory(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 730 {
		days = v
	}
	out, err := s.db.PostureHistory(r.Context(), days)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

// handleHistoryDay replays what the dashboard published on one date.
//
// A day with no snapshot returns 404, never an empty reading: "we have no
// record of that date" and "that date was calm" are different answers, and
// rendering the first as the second would be exactly the false reassurance
// this project is built to avoid.
func (s *Server) handleHistoryDay(w http.ResponseWriter, r *http.Request) {
	day, err := time.Parse("2006-01-02", r.PathValue("day"))
	if err != nil {
		http.Error(w, "day must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	snap, err := s.db.PostureOn(r.Context(), day)
	if err != nil {
		s.fail(w, err)
		return
	}
	if snap == nil {
		allowCORS(w)
		http.Error(w, "no snapshot recorded for that day", http.StatusNotFound)
		return
	}
	s.writeJSON(w, snap)
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
		"tones":      enrich.Tones,
		// The ladder and its adjustments are published so a reader can check
		// the posture reading against the counts rather than trusting it.
		"posture_rules":       posture.Rules(),
		"posture_adjustments": posture.Adjustments(),
		"exports": map[string]string{
			"csv":     "/api/incidents.csv",
			"geojson": "/api/incidents.geojson",
		},
	})
}

// sinceDays reads ?days= with bounds and a default.
func sinceDays(r *http.Request, def, max int) time.Time {
	days := def
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= max {
		days = v
	}
	return time.Now().AddDate(0, 0, -days)
}

func (s *Server) handleFIRMS(w http.ResponseWriter, r *http.Request) {
	out, err := s.db.FIRMSSince(r.Context(), sinceDays(r, 7, 30))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

func (s *Server) handleGpsjam(w http.ResponseWriter, r *http.Request) {
	out, err := s.db.GpsjamLatest(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

func (s *Server) handleAir(w http.ResponseWriter, r *http.Request) {
	out, err := s.db.AirSince(r.Context(), sinceDays(r, 2, 30))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

// seaEventOut adds the vessel's sanctions listing, when it has one. This is
// the difference between "a vessel loitered over a cable corridor" and "a
// vessel already listed as shadow-fleet loitered over a cable corridor" — the
// first is a curiosity, the second is a finding.
type seaEventOut struct {
	store.SeaEvent
	Sanctioned *store.SanctionedVessel `json:"sanctioned,omitempty"`
}

func (s *Server) handleSea(w http.ResponseWriter, r *http.Request) {
	events, err := s.db.SeaSince(r.Context(), sinceDays(r, 7, 30))
	if err != nil {
		s.fail(w, err)
		return
	}
	mmsis := make([]int64, 0, len(events))
	for _, e := range events {
		mmsis = append(mmsis, e.MMSI)
	}
	listed, err := s.db.LookupSanctionedVessels(r.Context(), mmsis)
	if err != nil {
		// The watchlist is an enrichment, not a dependency: a failed lookup
		// must not blank the sea layer. Absence of a flag already means
		// "not matched", which is what the reader gets here.
		s.log.Error("sanctions lookup", "err", err)
		listed = nil
	}
	out := make([]seaEventOut, 0, len(events))
	for _, e := range events {
		o := seaEventOut{SeaEvent: e}
		if v, ok := listed[e.MMSI]; ok {
			o.Sanctioned = &v
		}
		out = append(out, o)
	}
	s.writeJSON(w, out)
}

// handleCertPL serves the daily CERT.PL warning-list rate. Poland only —
// no equivalent open feed exists for LT, LV or EE.
func (s *Server) handleCertPL(w http.ResponseWriter, r *http.Request) {
	out, err := s.db.CertPLSince(r.Context(), sinceDays(r, 365, 3650))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, out)
}

// sarAOI is one monitored area with its measurement series and current verdict.
type sarAOI struct {
	Key          string                 `json:"key"`
	Label        string                 `json:"label"`
	Country      string                 `json:"country"`
	Kind         string                 `json:"kind"`
	Side         string                 `json:"side"`
	Class        string                 `json:"class"`
	Note         string                 `json:"note"`
	DepthKm      int                    `json:"depth_km"`
	Bbox         [4]float64             `json:"bbox"` // lonMin, latMin, lonMax, latMax
	BrowserURL   string                 `json:"browser_url"`
	Series       []store.SARObservation `json:"series"`
	Anomaly      bool                   `json:"anomaly"`
	ZScore       float64                `json:"zscore"`
	Latest       float64                `json:"latest"`
	Median       float64                `json:"median"`
	Baseline     int                    `json:"baseline"`
	SceneShifted bool                   `json:"scene_shifted"`
}

func (s *Server) handleSAR(w http.ResponseWriter, r *http.Request) {
	out := make([]sarAOI, 0, len(layers.MonitoredAOIs))
	for _, aoi := range layers.MonitoredAOIs {
		series, err := s.db.SARSeries(r.Context(), aoi.Key)
		if err != nil {
			s.fail(w, err)
			return
		}
		a := layers.DetectAnomaly(series)
		z := a.ZScore
		if math.IsInf(z, 0) || math.IsNaN(z) {
			z = 0 // JSON has no Infinity; the Detected flag carries the verdict
		}
		out = append(out, sarAOI{
			Key:          aoi.Key,
			Label:        aoi.Label,
			Country:      aoi.Country,
			Kind:         aoi.Kind,
			Side:         aoi.Side,
			Class:        aoi.Class,
			Note:         aoi.Note,
			DepthKm:      aoi.DepthKm,
			Bbox:         [4]float64{aoi.Box.LonMin, aoi.Box.LatMin, aoi.Box.LonMax, aoi.Box.LatMax},
			BrowserURL:   aoi.BrowserURL(),
			Series:       series,
			Anomaly:      a.Detected,
			ZScore:       z,
			Latest:       a.Latest,
			Median:       a.Median,
			Baseline:     a.Baseline,
			SceneShifted: a.SceneShifted,
		})
	}
	s.writeJSON(w, out)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("query", "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
