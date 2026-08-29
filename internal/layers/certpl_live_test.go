package layers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Live smoke test: one download must yield a substantial daily series. The
// endpoint carries a rolling ~6-month window, so this asserts a floor well
// under that rather than the "back to 2020" the source is sometimes credited
// with. Opt-in — ~25MB.
func TestCertPLLive(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" || os.Getenv("LIVE_FEEDS") == "" {
		t.Skip("set TEST_DATABASE_URL and LIVE_FEEDS=1")
	}
	ctx := context.Background()
	db, err := store.New(ctx, url)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer db.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := &CertPL{Client: &http.Client{Timeout: 3 * time.Minute}}
	if err := c.Run(ctx, db, log); err != nil {
		t.Fatalf("run: %v", err)
	}
	series, err := db.CertPLSince(ctx, time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(series) < 90 {
		t.Errorf("series has %d days; expected months of history from one download", len(series))
	}
	total := 0
	for _, d := range series {
		total += d.Added
	}
	t.Logf("days=%d first=%s last=%s total_added=%d",
		len(series), series[0].Day.Format("2006-01-02"),
		series[len(series)-1].Day.Format("2006-01-02"), total)
}
