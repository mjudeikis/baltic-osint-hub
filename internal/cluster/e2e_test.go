package cluster_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mjudeikis/baltic-osint-hub/internal/cluster"
	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// The whole pipeline, for real: seed realistic reports, run the actual
// clustering loop with real embeddings against a real database, and check the
// number the public ends up seeing.
//
// This lives in an external test package because enrich imports store, so a
// test inside either package cannot import the other.
//
// Needs both, and is skipped without them:
//
//	TEST_DATABASE_URL=postgres://osint:osint@localhost:5433/osint_test \
//	OPENAI_API_KEY=sk-... go test ./internal/cluster/ -run E2E -v
func TestClusterRunE2E(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if key == "" || dbURL == "" {
		t.Skip("need OPENAI_API_KEY and TEST_DATABASE_URL; skipping end-to-end clustering")
	}
	ctx := context.Background()

	db, err := store.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer db.Close()

	// Direct pool for setup only. Safe because this runs solely against a
	// throwaway database the caller opted into by setting TEST_DATABASE_URL.
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// Must match internal/store's testDBLockKey. `go test ./...` runs packages
	// in parallel and both suites truncate this database; without the lock,
	// store's tests wipe these seeded rows mid-run and this test fails in a way
	// that does not reproduce when run on its own. Duplicated rather than
	// imported because helpers in _test.go files are not importable.
	const testDBLockKey = 918273645
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, testDBLockKey); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, testDBLockKey)
		conn.Release()
	}()

	if _, err := pool.Exec(ctx,
		`TRUNCATE events, incidents, raw_items, source_runs RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	store.StateControlledSources = []string{"tass-en"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Now().Add(-4 * time.Hour)

	seed := func(source, category, summary string, countries []string, when time.Time) {
		t.Helper()
		var rawID int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO raw_items (source, url, title, content_hash, status)
			 VALUES ($1,$2,$3,$4,'classified') RETURNING id`,
			source, "https://example.test/"+source+"/"+summary[:10], summary, summary+source,
		).Scan(&rawID); err != nil {
			t.Fatalf("seed raw_items: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO incidents (raw_item_id, category, countries, severity, tone, summary_en, occurred_at)
			 VALUES ($1,$2,$3,3,'negative',$4,$5)`,
			rawID, category, countries, summary, when); err != nil {
			t.Fatalf("seed incidents: %v", err)
		}
	}

	// Three independent reports of ONE rail sabotage, plus two genuinely
	// separate events in other categories.
	seed("lrt-en", "sabotage",
		"Lithuanian police detained a man suspected of setting fire to railway signalling equipment near Kaunas.",
		[]string{"LT"}, now)
	seed("delfi-lt", "sabotage",
		"A suspect was arrested in Lithuania after an arson attack damaged rail signalling infrastructure close to Kaunas.",
		[]string{"LT"}, now.Add(2*time.Hour))
	seed("err-news", "sabotage",
		"Police in Lithuania arrested a man accused of burning railway signal equipment near the city of Kaunas.",
		[]string{"LT"}, now.Add(3*time.Hour))
	seed("notes-from-poland", "espionage",
		"Polish authorities charged a man with spying for Russian intelligence services in Warsaw.",
		[]string{"PL"}, now)
	seed("lsm-en", "gps-jamming",
		"GPS interference disrupted flights approaching Riga airport for several hours.",
		[]string{"LV"}, now)

	emb := enrich.NewEmbedder(key, os.Getenv("OPENAI_BASE_URL"))
	res, err := cluster.Run(ctx, db, emb, 100, cluster.DefaultThreshold, log)
	if err != nil {
		t.Fatalf("cluster.Run: %v", err)
	}
	t.Logf("considered=%d new_events=%d merged=%d", res.Considered, res.NewEvents, res.Merged)

	if res.Considered != 5 {
		t.Errorf("considered = %d, want 5", res.Considered)
	}
	if res.NewEvents != 3 {
		t.Errorf("new events = %d, want 3 (rail sabotage, espionage, jamming)", res.NewEvents)
	}
	if res.Merged != 2 {
		t.Errorf("merged = %d, want 2 sabotage reports folding into the first", res.Merged)
	}

	// The number the public actually sees: 5 articles became 3 events.
	byTone, _, err := db.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if byTone["negative"] != 3 {
		t.Errorf("adverse count = %d, want 3 events (5 articles before clustering)", byTone["negative"])
	}

	list, err := db.ListIncidents(ctx, store.IncidentFilter{Since: time.Now().AddDate(0, 0, -7)})
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("feed rows = %d, want 3", len(list))
	}
	var sabotage *store.Incident
	for i := range list {
		if list[i].Category == "sabotage" {
			sabotage = &list[i]
		}
	}
	if sabotage == nil {
		t.Fatal("no sabotage event in the feed")
	}
	if sabotage.Reports != 3 {
		t.Errorf("sabotage event has %d reports, want 3", sabotage.Reports)
	}
	if sabotage.IndependentSources == nil || *sabotage.IndependentSources != 3 {
		t.Errorf("independent sources = %v, want 3", sabotage.IndependentSources)
	}
	if sabotage.Confidence != 0.95 {
		t.Errorf("confidence = %v, want 0.95 for three independent sources", sabotage.Confidence)
	}

	// A second pass must be a no-op: every incident already has an embedding.
	res2, err := cluster.Run(ctx, db, emb, 100, cluster.DefaultThreshold, log)
	if err != nil {
		t.Fatalf("second cluster.Run: %v", err)
	}
	if res2.Considered != 0 {
		t.Errorf("second run considered %d incidents, want 0 — clustering must be idempotent", res2.Considered)
	}
}
