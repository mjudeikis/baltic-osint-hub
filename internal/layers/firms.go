package layers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// FIRMS ingests NASA VIIRS thermal-anomaly detections (fires, explosions)
// and keeps those falling inside the border sectors.
// API docs: https://firms.modaps.eosdis.nasa.gov/api/area/
type FIRMS struct {
	MapKey string
	Client *http.Client
}

// firmsArea covers the whole monitored region in one request; per-sector
// filtering happens locally. It must extend far enough east to include the
// Belarusian and Russian staging areas — the approach sectors reach Orsha
// and the Leningrad oblast.
const firmsArea = "19.0,50.5,31.0,60.2" // west,south,east,north

// firmsBase is a var so tests can point the layer at a local server.
var firmsBase = "https://firms.modaps.eosdis.nasa.gov"

// maxFIRMSBytes caps the CSV: a whole-region day is a few megabytes.
const maxFIRMSBytes = 32 << 20

func (f *FIRMS) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	detections, err := f.fetch(ctx)
	if err != nil {
		return err
	}
	added := 0
	for i := range detections {
		d := &detections[i]
		d.Sector = Sector(BorderSectors, d.Lat, d.Lon)
		if d.Sector == "" {
			continue // inside the big bbox but not near a border
		}
		if err := db.InsertFIRMS(ctx, d); err != nil {
			return err
		}
		added++
	}
	log.Info("firms ingested", "total", len(detections), "in_sectors", added)
	return nil
}

// fetch downloads and parses the region CSV.
//
// FIRMS puts the MAP_KEY in the URL path, and a *url.Error's message embeds
// the full URL. That message would otherwise be stored on source_runs and
// served by GET /api/sources, so every error leaving this function is
// rewritten with the key removed. The store redacts credential-shaped text
// as a backstop, but a path segment is not credential-shaped; only this
// layer knows what the secret looks like.
func (f *FIRMS) fetch(ctx context.Context) ([]store.FIRMSDetection, error) {
	url := fmt.Sprintf("%s/api/area/csv/%s/VIIRS_SNPP_NRT/%s/2", firmsBase, f.MapKey, firmsArea)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, f.redact(err)
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, f.redact(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firms: status %d", resp.StatusCode)
	}
	detections, err := parseFIRMS(io.LimitReader(resp.Body, maxFIRMSBytes))
	if err != nil {
		return nil, f.redact(err)
	}
	return detections, nil
}

// redact rebuilds err as a plain error with the map key removed. It
// deliberately does not wrap the original: a wrapped error still prints its
// cause, key included, and Unwrap would hand the unredacted message to any
// caller that walks the chain.
func (f *FIRMS) redact(err error) error {
	msg := err.Error()
	if f.MapKey != "" {
		msg = strings.ReplaceAll(msg, f.MapKey, "<redacted>")
	}
	return fmt.Errorf("firms: %s", msg)
}

// parseFIRMS reads the FIRMS area CSV. Header (VIIRS):
// latitude,longitude,bright_ti4,scan,track,acq_date,acq_time,satellite,
// instrument,confidence,version,bright_ti5,frp,daynight
func parseFIRMS(r io.Reader) ([]store.FIRMSDetection, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("firms: csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, nil
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	get := func(row []string, name string) string {
		if i, ok := col[name]; ok && i < len(row) {
			return row[i]
		}
		return ""
	}
	var out []store.FIRMSDetection
	for _, row := range rows[1:] {
		lat, err1 := strconv.ParseFloat(get(row, "latitude"), 64)
		lon, err2 := strconv.ParseFloat(get(row, "longitude"), 64)
		if err1 != nil || err2 != nil {
			continue
		}
		// acq_time is "HHMM" (zero-padded minutes-of-day, UTC).
		when, err := time.Parse("2006-01-02 1504",
			get(row, "acq_date")+" "+fmt.Sprintf("%04s", get(row, "acq_time")))
		if err != nil {
			continue
		}
		bright, _ := strconv.ParseFloat(get(row, "bright_ti4"), 32)
		frp, _ := strconv.ParseFloat(get(row, "frp"), 32)
		out = append(out, store.FIRMSDetection{
			Lat:        lat,
			Lon:        lon,
			Brightness: float32(bright),
			FRP:        float32(frp),
			Confidence: get(row, "confidence"),
			DetectedAt: when.UTC(),
		})
	}
	return out, nil
}
