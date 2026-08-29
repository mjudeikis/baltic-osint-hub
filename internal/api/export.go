package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Exports exist because this project takes from gpsjam, FIRMS, OpenSky,
// Copernicus, GDELT and aisstream and, until now, gave nothing back. Both
// endpoints accept exactly the same filters as /api/incidents, so whatever a
// reader is looking at is what they can download.

func exportFilename(prefix, ext string) string {
	return prefix + "-" + time.Now().UTC().Format("2006-01-02") + ext
}

func (s *Server) handleIncidentsCSV(w http.ResponseWriter, r *http.Request) {
	out, err := s.listOut(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+exportFilename("baltic-osint-incidents", ".csv")+`"`)
	allowCORS(w)

	cw := csv.NewWriter(w)
	defer cw.Flush()
	header := []string{
		"id", "event_id", "occurred_at", "category", "countries", "severity", "tone",
		"place", "lat", "lon", "summary", "confidence", "confidence_label",
		"reports", "sources", "source", "credibility", "title", "url",
	}
	if err := cw.Write(header); err != nil {
		s.log.Error("csv header", "err", err)
		return
	}
	for _, o := range out {
		eventID := ""
		if o.EventID != nil {
			eventID = strconv.FormatInt(*o.EventID, 10)
		}
		rec := []string{
			strconv.FormatInt(o.ID, 10),
			eventID,
			o.OccurredAt.UTC().Format(time.RFC3339),
			o.Category,
			strings.Join(o.Countries, " "),
			strconv.Itoa(o.Severity),
			o.Tone,
			o.Place,
			floatOrEmpty(o.Lat),
			floatOrEmpty(o.Lon),
			o.SummaryEN,
			strconv.FormatFloat(float64(o.Confidence), 'f', 2, 32),
			o.ConfidenceLabel,
			strconv.Itoa(o.Reports),
			strings.Join(o.Sources, " "),
			o.Source,
			o.Credibility,
			o.Title,
			o.URL,
		}
		if err := cw.Write(rec); err != nil {
			s.log.Error("csv row", "err", err)
			return
		}
	}
}

func floatOrEmpty(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 5, 64)
}

// handleIncidentsGeoJSON emits only the located incidents. Anything without
// coordinates is omitted rather than given a placeholder — a pin on a capital
// city for an event that merely mentioned the country is exactly the kind of
// false precision this project is built to avoid.
func (s *Server) handleIncidentsGeoJSON(w http.ResponseWriter, r *http.Request) {
	out, err := s.listOut(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	type feature struct {
		Type     string         `json:"type"`
		Geometry map[string]any `json:"geometry"`
		Props    map[string]any `json:"properties"`
	}
	features := make([]feature, 0, len(out))
	for _, o := range out {
		if o.Lat == nil || o.Lon == nil {
			continue
		}
		props := map[string]any{
			"id":          o.ID,
			"occurred_at": o.OccurredAt.UTC().Format(time.RFC3339),
			"category":    o.Category,
			"countries":   o.Countries,
			"severity":    o.Severity,
			"tone":        o.Tone,
			"place":       o.Place,
			"summary":     o.SummaryEN,
			"reports":     o.Reports,
			"sources":     o.Sources,
			"credibility": o.Credibility,
			"source":      o.Source,
			"title":       o.Title,
			"url":         o.URL,
		}
		if o.ConfidenceLabel != "" {
			props["confidence"] = o.Confidence
			props["confidence_label"] = o.ConfidenceLabel
		}
		features = append(features, feature{
			Type:     "Feature",
			Geometry: map[string]any{"type": "Point", "coordinates": []float64{*o.Lon, *o.Lat}},
			Props:    props,
		})
	}
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+exportFilename("baltic-osint-incidents", ".geojson")+`"`)
	s.writeJSON(w, map[string]any{"type": "FeatureCollection", "features": features})
}
