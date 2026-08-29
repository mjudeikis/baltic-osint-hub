package layers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// CertPL turns CERT.PL's warning list into a daily cyber-activity rate for
// Poland.
//
// The list is a phishing and malware blocklist — the domains themselves are
// not incidents and are deliberately not stored. What is worth having is the
// volume over time: every record carries its insert date, so one download
// yields a ready-made daily series rather than starting from zero.
//
// MEASURED 2026-08-29: the endpoint carries a rolling window of roughly six
// months (183 days, 140,062 domains, ~765/day), NOT the full history back to
// 2020 that is sometimes claimed for it. So this seeds about half a year of
// baseline on first run and extends it from there — good, but it does mean an
// unbroken long series depends on us continuing to poll.
//
// It is a *Polish* signal and covers no other monitored country; there is no
// equivalent open feed for LT, LV or EE, which is itself a finding.
type CertPL struct {
	Client *http.Client
}

const certPLURL = "https://hole.cert.pl/domains/v2/domains.json"

type certPLRecord struct {
	InsertDate string `json:"InsertDate"`
	DeleteDate string `json:"DeleteDate"`
}

func (c *CertPL) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, certPLURL, nil)
	if err != nil {
		return err
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("cert.pl: status %d", resp.StatusCode)
	}

	// ~25MB; streamed rather than buffered whole.
	var records []certPLRecord
	if err := json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return fmt.Errorf("cert.pl: decode: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("cert.pl: empty list")
	}

	added, removed := countByDay(records)
	days := make([]store.CertPLDay, 0, len(added))
	for day, n := range added {
		days = append(days, store.CertPLDay{Day: day, Added: n, Removed: removed[day]})
	}
	// Days that only saw removals still belong in the series.
	for day, n := range removed {
		if _, ok := added[day]; !ok {
			days = append(days, store.CertPLDay{Day: day, Removed: n})
		}
	}
	if err := db.UpsertCertPLDays(ctx, days); err != nil {
		return err
	}
	log.Info("cert.pl warning list", "records", len(records), "days", len(days))
	return nil
}

// countByDay buckets records by their insert and delete dates. Split out so
// the date handling is testable without a 25MB download.
func countByDay(records []certPLRecord) (added, removed map[time.Time]int) {
	added, removed = map[time.Time]int{}, map[time.Time]int{}
	for _, r := range records {
		if d, ok := parseCertPLDate(r.InsertDate); ok {
			added[d]++
		}
		// An empty DeleteDate means still listed, which is the common case.
		if d, ok := parseCertPLDate(r.DeleteDate); ok {
			removed[d]++
		}
	}
	return added, removed
}

// parseCertPLDate accepts the formats the feed has used. Unparseable or empty
// values are skipped rather than bucketed into a default date, which would put
// a false spike on whatever day that happened to be.
func parseCertPLDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}
