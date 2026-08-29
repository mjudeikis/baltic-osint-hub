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

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// SanctionedVessels refreshes the vessel watchlist from OpenSanctions'
// maritime dataset, so the sea layer can say *which* vessel loitered over a
// cable rather than just that one did.
//
// The dataset aggregates 23 sources and is rebuilt daily. Of ~20,400 vessels,
// roughly 7,000 publish an MMSI; the rest are IMO-only and cannot be matched
// against a live AIS position, since AIS broadcasts MMSI. That is a ceiling on
// the join, and it means an unmatched vessel is not a cleared one.
//
// Licence: CC BY-NC 4.0. Attribution is carried in the UI and docs.
type SanctionedVessels struct {
	Client *http.Client
}

const openSanctionsMaritimeURL = "https://data.opensanctions.org/datasets/latest/maritime/maritime.csv"

func (s *SanctionedVessels) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openSanctionsMaritimeURL, nil)
	if err != nil {
		return err
	}
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("opensanctions: status %d", resp.StatusCode)
	}

	vessels, rows, skipped, err := parseMaritimeCSV(resp.Body)
	if err != nil {
		return err
	}
	if len(vessels) == 0 {
		return fmt.Errorf("opensanctions: parsed %d rows but found no vessel with an MMSI", rows)
	}

	n, err := db.ReplaceSanctionedVessels(ctx, vessels)
	if err != nil {
		return err
	}
	log.Info("sanctioned vessels", "rows", rows, "with_mmsi", n, "imo_only_skipped", skipped)
	return nil
}

// parseMaritimeCSV extracts vessel rows carrying an MMSI. Split from Run so
// the format handling — multi-valued MMSI, IMO-only rows, a renamed column —
// is testable without downloading 5MB.
func parseMaritimeCSV(src io.Reader) (vessels []store.SanctionedVessel, rows, skipped int, err error) {
	r := csv.NewReader(src)
	header, err := r.Read()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("opensanctions: header: %w", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"type", "caption", "mmsi", "imo", "risk", "flag", "countries", "datasets", "url"} {
		if _, ok := col[need]; !ok {
			// Fail loudly rather than silently importing nothing: a column
			// rename upstream would otherwise look like an empty watchlist,
			// and an empty watchlist reads as "no vessel is sanctioned".
			return nil, 0, 0, fmt.Errorf("opensanctions: missing column %q (got %v)", need, header)
		}
	}
	get := func(rec []string, name string) string {
		i := col[name]
		if i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, rows, skipped, fmt.Errorf("opensanctions: row %d: %w", rows, err)
		}
		rows++
		if !strings.EqualFold(get(rec, "type"), "VESSEL") {
			continue
		}
		raw := get(rec, "mmsi")
		if raw == "" {
			skipped++ // IMO-only; nothing to match an AIS broadcast against
			continue
		}
		// A vessel may publish several MMSIs over its life — reflagging and
		// renaming is exactly the shadow-fleet pattern — so each becomes its
		// own row and any of them will match.
		for _, m := range strings.Split(raw, ";") {
			mmsi, err := strconv.ParseInt(strings.TrimSpace(m), 10, 64)
			if err != nil || mmsi <= 0 {
				continue
			}
			vessels = append(vessels, store.SanctionedVessel{
				MMSI:      mmsi,
				IMO:       get(rec, "imo"),
				Name:      get(rec, "caption"),
				Risk:      get(rec, "risk"),
				Flag:      get(rec, "flag"),
				Countries: get(rec, "countries"),
				Datasets:  get(rec, "datasets"),
				URL:       get(rec, "url"),
			})
		}
	}
	return vessels, rows, skipped, nil
}
