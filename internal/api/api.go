// Package api exposes the read-only dashboard API.
package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"sort"
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

// minSeverityParam reads the optional min_severity filter. Default 1 keeps
// the public API's behaviour unchanged; the dashboard passes 2 so severity-1
// commentary is not counted as an incident.
func minSeverityParam(r *http.Request) int {
	if v, err := strconv.Atoi(r.URL.Query().Get("min_severity")); err == nil && v >= 1 && v <= 5 {
		return v
	}
	return 1
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	days := 90
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}
	buckets, err := s.db.Timeline(r.Context(), time.Now().AddDate(0, 0, -days), r.URL.Query().Get("country"), minSeverityParam(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, buckets)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	cells, err := s.db.Summary(r.Context(), minSeverityParam(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeJSON(w, cells)
}

// postureOut wraps the reading with the event that set it. A headline saying
// "a serious adverse event" while withholding which one manufactures the
// exact "what do they know?" anxiety the dashboard exists to prevent.
type postureOut struct {
	posture.Reading
	TriggerEvent *triggerEvent `json:"trigger_event,omitempty"`
}

type triggerEvent struct {
	ID           int64  `json:"id"`
	Summary      string `json:"summary"`
	Severity     int    `json:"severity"`
	Corroborated bool   `json:"corroborated"`
}

// handlePosture publishes the overall regional reading plus the tone balance
// it was derived from, so the number is auditable rather than a black box.
func (s *Server) handlePosture(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	byTone, sev, corroborated, err := s.db.ToneCounts(r.Context(), 7, country)
	if err != nil {
		s.fail(w, err)
		return
	}
	reading := posture.Evaluate(posture.Counts{
		Positive:               byTone[enrich.TonePositive],
		Neutral:                byTone[enrich.ToneNeutral],
		Negative:               byTone[enrich.ToneNegative],
		NegativeBySeverity:     sev,
		CorroboratedBySeverity: corroborated,
	})
	// "Is this week unusual?" — a bare count invites the reader to assume the
	// worst, so publish the comparison alongside it.
	if history, err := s.db.WeeklyAdverseHistory(r.Context(), 12); err == nil {
		reading = reading.WithHistory(history)
	}
	out := postureOut{Reading: reading}
	// At Elevated and above the level is set by a severity-4/5 event; name
	// it. Mirrors the rule engine's preference: corroborated first, then
	// worst severity. Best-effort — a lookup failure degrades to the bare
	// reading rather than failing the page's headline element.
	if reading.Level >= posture.Elevated {
		if list, err := s.db.ListIncidents(r.Context(), store.IncidentFilter{
			Since:    time.Now().AddDate(0, 0, -7),
			Tone:     "negative",
			Severity: 4,
			Country:  country,
			Limit:    20,
		}); err == nil && len(list) > 0 {
			best := 0
			score := func(i int) int {
				n := 0
				if list[i].IndependentSources != nil && *list[i].IndependentSources >= 2 {
					n += 10
				}
				return n + list[i].Severity
			}
			for i := range list {
				if score(i) > score(best) {
					best = i
				}
			}
			ev := list[best]
			summary := ev.SummaryEN
			if summary == "" {
				summary = ev.Title
			}
			out.TriggerEvent = &triggerEvent{
				ID:           ev.ID,
				Summary:      summary,
				Severity:     ev.Severity,
				Corroborated: ev.IndependentSources != nil && *ev.IndependentSources >= 2,
			}
		}
	}
	s.writeJSON(w, out)
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
	// ShipType is the AIS ship-and-cargo code where known, so a reader can see
	// whether a stopped vessel was a tanker or a tug.
	ShipType *int `json:"ship_type,omitempty"`
	// Notable marks the events worth showing by default: a listed vessel, or
	// an extended AIS gap by a vessel capable of damaging a cable. Everything
	// else is ordinary maritime behaviour and is kept as baseline rather than
	// surfaced as a finding.
	Notable bool `json:"notable"`
}

// notable decides what the dashboard leads with.
//
// A week of live data produced ~500 sea events: 23 involved a listed vessel,
// 326 were AIS gaps. Treating every gap as a finding drew 330 solid marks and
// buried the listed vessels all over again — this time under the gap
// detector. The median unlisted gap is 1.6 h at current receiver coverage,
// which is a reception dropout, not dark activity; the tail past 4 h is not
// explained by coverage. So: a listed vessel always leads; an unlisted vessel
// only for an extended gap, and only when its anchor could actually hurt a
// cable — cargo, tanker, or unknown type (shadow-fleet vessels often
// broadcast none). Short gaps, loitering and typed non-cargo traffic stay
// recorded as the baseline that makes an anomaly meaningful, but they do not
// claim the reader's attention.
const extendedGap = 4 * time.Hour

func notable(e store.SeaEvent, listed bool, shipType *int) bool {
	if listed {
		return true
	}
	if e.Event != "ais-gap" || e.StartedAt == nil {
		return false
	}
	if e.DetectedAt.Sub(*e.StartedAt) < extendedGap {
		return false
	}
	// Typed and known not to be cargo or tanker: ferries, fishing boats and
	// the like go dark constantly and cannot drag a cable-breaking anchor.
	if shipType != nil && (*shipType < 70 || *shipType > 89) {
		return false
	}
	return true
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
	types, err := s.db.LookupVesselTypes(r.Context(), mmsis)
	if err != nil {
		s.log.Error("vessel type lookup", "err", err)
		types = nil
	}

	notableOnly := r.URL.Query().Get("notable") == "1"
	out := make([]seaEventOut, 0, len(events))
	for _, e := range events {
		o := seaEventOut{SeaEvent: e}
		v, isListed := listed[e.MMSI]
		if isListed {
			o.Sanctioned = &v
		}
		if t, ok := types[e.MMSI]; ok {
			st := t.ShipType
			o.ShipType = &st
		}
		o.Notable = notable(e, isListed, o.ShipType)
		if notableOnly && !o.Notable {
			continue
		}
		out = append(out, o)
	}
	// Notable first, so the four that matter are not buried behind ninety-odd
	// ordinary stops; within that, most recent first as before.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Notable != out[j].Notable {
			return out[i].Notable
		}
		return out[i].DetectedAt.After(out[j].DetectedAt)
	})
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
