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

// Live smoke test for the sanctions watchlist. Opt-in: 5MB download.
func TestSanctionedVesselsLive(t *testing.T) {
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
	client := &http.Client{Timeout: 2 * time.Minute}

	// Fetch once directly so the test knows which MMSIs should be present.
	resp, err := client.Get(openSanctionsMaritimeURL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	parsed, _, _, err := parseMaritimeCSV(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	seen := make([]int64, 0, 1)
	if len(parsed) > 0 {
		seen = append(seen, parsed[0].MMSI)
	}

	s := &SanctionedVessels{Client: client}
	if err := s.Run(ctx, db, log); err != nil {
		t.Fatalf("run: %v", err)
	}
	n, err := db.SanctionedVesselCount(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	t.Logf("watchlist size: %d vessels with MMSI", n)
	if n < 1000 {
		t.Errorf("only %d vessels loaded; expected thousands", n)
	}

	// A second run must fully replace, not duplicate or accumulate.
	if err := s.Run(ctx, db, log); err != nil {
		t.Fatalf("second run: %v", err)
	}
	n2, err := db.SanctionedVesselCount(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n2 != n {
		t.Errorf("after refresh count = %d, want %d — replace must be idempotent", n2, n)
	}

	// The join the sea layer depends on must resolve for a listed vessel and
	// must not invent a match for an unlisted one. MMSIs come from the parse
	// rather than a hardcoded value, so this does not rot when a vessel is
	// delisted upstream.
	if len(seen) > 0 {
		got, err := db.LookupSanctionedVessels(ctx, append(seen[:1:1], 999999999))
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if _, ok := got[seen[0]]; !ok {
			t.Errorf("lookup missed MMSI %d, which is in the table", seen[0])
		}
		if _, ok := got[999999999]; ok {
			t.Error("lookup invented a match for an unlisted MMSI")
		}
	}
}
