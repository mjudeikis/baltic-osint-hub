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

// Gpsjam ingests gpsjam.org daily H3 interference cells (derived from
// aircraft ADS-B navigation-accuracy reports). Data lags ~1-2 days.
// No API key; format verified 2026-08-29: /data/YYYY-MM-DD-h3_4.csv with
// columns hex,count_good_aircraft,count_bad_aircraft.
type Gpsjam struct {
	Client *http.Client
}

func (g *Gpsjam) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	// Try the freshest published day: yesterday, then two days back.
	for _, back := range []int{1, 2, 3} {
		day := time.Now().UTC().AddDate(0, 0, -back).Truncate(24 * time.Hour)
		done, err := db.HasGpsjamDay(ctx, day)
		if err != nil {
			return err
		}
		if done {
			return nil // freshest available day already stored
		}
		n, err := g.ingestDay(ctx, db, day)
		if err == errNotPublished {
			continue
		}
		if err != nil {
			return err
		}
		log.Info("gpsjam ingested", "day", day.Format("2006-01-02"), "cells", n)
		return nil
	}
	return fmt.Errorf("gpsjam: no data published for the last 3 days")
}

var errNotPublished = fmt.Errorf("gpsjam: day not published yet")

func (g *Gpsjam) ingestDay(ctx context.Context, db *store.Store, day time.Time) (int, error) {
	url := fmt.Sprintf("https://gpsjam.org/data/%s-h3_4.csv", day.Format("2006-01-02"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return 0, errNotPublished
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("gpsjam: status %d", resp.StatusCode)
	}
	cells, err := parseGpsjam(resp.Body)
	if err != nil {
		return 0, err
	}
	for i := range cells {
		cells[i].Day = day
		if err := db.UpsertGpsjamCell(ctx, &cells[i]); err != nil {
			return 0, err
		}
	}
	return len(cells), nil
}

// parseGpsjam keeps only cells that saw interference — the "all good" cells
// are the whole world and carry no signal for the dashboard.
func parseGpsjam(r io.Reader) ([]store.GpsjamCell, error) {
	cr := csv.NewReader(r)
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("gpsjam: csv: %w", err)
	}
	var out []store.GpsjamCell
	for i, row := range rows {
		if i == 0 || len(row) < 3 {
			continue
		}
		good, _ := strconv.Atoi(row[1])
		bad, _ := strconv.Atoi(row[2])
		if bad == 0 {
			continue
		}
		out = append(out, store.GpsjamCell{Hex: row[0], Good: good, Bad: bad})
	}
	return out, nil
}
