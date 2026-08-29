package layers

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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

func (f *FIRMS) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	url := fmt.Sprintf(
		"https://firms.modaps.eosdis.nasa.gov/api/area/csv/%s/VIIRS_SNPP_NRT/%s/2",
		f.MapKey, firmsArea)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("firms: status %d", resp.StatusCode)
	}
	detections, err := parseFIRMS(resp.Body)
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
