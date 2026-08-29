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

// Live smoke test for the Digitraffic archive: it must fetch, filter to the
// corridors, and persist. Opt-in — needs the network and a database.
func TestAISArchiveLive(t *testing.T) {
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
	a := &AISArchive{Client: &http.Client{Timeout: time.Minute}}
	if err := a.Run(ctx, db, log); err != nil {
		t.Fatalf("run: %v", err)
	}
	cov, err := db.AISTrackCoverage(ctx)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if len(cov) == 0 {
		t.Fatal("no positions archived")
	}
	for _, c := range cov {
		t.Logf("%-18s fixes=%-5d vessels=%-4d %s", c.Corridor, c.Fixes, c.Vessels, c.Latest.Format(time.RFC3339))
	}
	// Second run must be near-idempotent: the same fixes are already stored.
	if err := a.Run(ctx, db, log); err != nil {
		t.Fatalf("second run: %v", err)
	}
}
