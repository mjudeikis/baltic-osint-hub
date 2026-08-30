package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the real SQL. Every counting query in this package was
// rewritten to count events rather than article rows, and that rewrite is not
// something unit tests on Go structs can validate — the bugs live in the SQL.
//
// Set TEST_DATABASE_URL to run them, e.g. against docker-compose:
//
//	TEST_DATABASE_URL=postgres://osint:osint@localhost:5433/osint go test ./internal/store/
func testStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database integration tests")
	}
	ctx := context.Background()
	s, err := New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)

	lockTestDB(t, ctx, s.pool)

	// Each test starts from a clean slate. Scoped to a scratch schema would be
	// nicer, but the migrations target the default one; TRUNCATE ... CASCADE
	// keeps it simple and is safe because this only ever runs against a
	// throwaway database the caller opted into.
	if _, err := s.pool.Exec(ctx,
		`TRUNCATE events, incidents, raw_items, source_runs, posture_snapshots RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return s, ctx
}

// testDBLockKey serialises database tests across packages.
//
// `go test ./...` runs packages in parallel, and both this package and
// internal/cluster truncate the same test database — which silently wiped
// seeded rows mid-test and produced failures that did not reproduce when
// either package was run alone. An advisory lock is held for the duration of
// each test, so the suites take turns instead.
//
// internal/cluster's end-to-end test duplicates this helper and MUST use the
// same key. It cannot import it: helpers in _test.go files are not part of the
// importable package.
const testDBLockKey = 918273645

func lockTestDB(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	// Advisory locks are held by a session, so the lock and unlock must run on
	// the same connection — hence acquiring one rather than using the pool.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, testDBLockKey); err != nil {
		conn.Release()
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, testDBLockKey); err != nil {
			t.Logf("advisory unlock: %v", err)
		}
		conn.Release()
	})
}

// seed inserts a raw item plus its classified incident and returns the
// incident id.
func seed(t *testing.T, s *Store, ctx context.Context, source, title, tone string, severity int, countries []string, when time.Time) int64 {
	t.Helper()
	var rawID int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO raw_items (source, url, title, content_hash, status)
		 VALUES ($1,$2,$3,$4,'classified') RETURNING id`,
		source, "https://example.test/"+title+"/"+source, title, title+source).Scan(&rawID)
	if err != nil {
		t.Fatalf("seed raw_items: %v", err)
	}
	var incID int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO incidents (raw_item_id, category, countries, severity, tone, summary_en, occurred_at)
		 VALUES ($1,'sabotage',$2,$3,$4,$5,$6) RETURNING id`,
		rawID, countries, severity, tone, title, when).Scan(&incID)
	if err != nil {
		t.Fatalf("seed incidents: %v", err)
	}
	return incID
}

// The central claim of the whole change: six outlets reporting one event must
// count once, not six times.
func TestToneCountsCountsEventsNotArticles(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{"tass-en"}
	now := time.Now().Add(-2 * time.Hour)

	outlets := []string{"lrt-en", "delfi-lt", "err-news", "lsm-en", "notes-from-poland", "bnn"}
	ids := make([]int64, 0, len(outlets))
	for _, o := range outlets {
		ids = append(ids, seed(t, s, ctx, o, "rail sabotage", "negative", 3, []string{"LT"}, now))
	}

	// Before clustering, each article is its own unit — the old behaviour, and
	// what the dashboard must keep showing until the backfill catches up.
	byTone, _, _, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if byTone["negative"] != 6 {
		t.Fatalf("unclustered adverse = %d, want 6 (one per article)", byTone["negative"])
	}

	// Cluster them all into one event.
	eventID, err := s.CreateEventFor(ctx, ids[0])
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	for _, id := range ids[1:] {
		if err := s.AttachIncident(ctx, id, eventID); err != nil {
			t.Fatalf("AttachIncident: %v", err)
		}
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	byTone, sev, _, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if byTone["negative"] != 1 {
		t.Errorf("clustered adverse = %d, want 1 event", byTone["negative"])
	}
	if sev[3] != 1 {
		t.Errorf("severity histogram = %v, want a single severity-3 event", sev)
	}
}

// A partly-clustered table must not lose events. This is the failure that
// would have published a falsely calm reading.
func TestToneCountsMixesClusteredAndUnclustered(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	now := time.Now().Add(-2 * time.Hour)

	a := seed(t, s, ctx, "lrt-en", "cable damage", "negative", 4, []string{"LT"}, now)
	b := seed(t, s, ctx, "err-news", "cable damage", "negative", 4, []string{"LT"}, now)
	seed(t, s, ctx, "lsm-en", "separate arson", "negative", 2, []string{"LV"}, now)

	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, b, eventID); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	byTone, _, _, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	// One clustered event plus one still-unclustered incident = 2.
	if byTone["negative"] != 2 {
		t.Errorf("adverse = %d, want 2 (1 event + 1 unclustered incident)", byTone["negative"])
	}
}

// An event reported only by state media must not reach the posture reading,
// while one an independent outlet also carried must.
func TestToneCountsExcludesStateMediaOnlyEvents(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{"tass-en", "ria"}
	now := time.Now().Add(-2 * time.Hour)

	stateOnly := seed(t, s, ctx, "tass-en", "kremlin claim", "negative", 3, []string{"EE"}, now)
	ev1, err := s.CreateEventFor(ctx, stateOnly)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.RefreshEvent(ctx, ev1); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	mixed := seed(t, s, ctx, "err-news", "real incident", "negative", 3, []string{"EE"}, now)
	mixedState := seed(t, s, ctx, "ria", "real incident", "negative", 3, []string{"EE"}, now)
	ev2, err := s.CreateEventFor(ctx, mixed)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, mixedState, ev2); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, ev2); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	byTone, _, _, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if byTone["negative"] != 1 {
		t.Errorf("adverse = %d, want 1 — the state-media-only event must not count", byTone["negative"])
	}

	// The excluded event still exists and is still readable in the feed.
	var stored int
	if err := s.pool.QueryRow(ctx, `SELECT source_count FROM events WHERE id=$1`, ev1).Scan(&stored); err != nil {
		t.Fatalf("read event: %v", err)
	}
	if stored != 0 {
		t.Errorf("source_count = %d, want 0 independent sources", stored)
	}
}

// The feed must show one row per event, carrying the corroborating sources.
func TestListIncidentsDeduplicates(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{"tass-en"}
	now := time.Now().Add(-2 * time.Hour)

	// The state outlet is seeded first and earliest, so if representative
	// selection were naive it would win the headline.
	st := seed(t, s, ctx, "tass-en", "state version", "negative", 3, []string{"LT"}, now.Add(-time.Hour))
	a := seed(t, s, ctx, "lrt-en", "independent version", "negative", 3, []string{"LT"}, now)
	b := seed(t, s, ctx, "err-news", "independent version", "negative", 3, []string{"LT"}, now.Add(time.Minute))

	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	for _, id := range []int64{b, st} {
		if err := s.AttachIncident(ctx, id, eventID); err != nil {
			t.Fatalf("AttachIncident: %v", err)
		}
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	list, err := s.ListIncidents(ctx, IncidentFilter{Since: time.Now().AddDate(0, 0, -7)})
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("feed rows = %d, want 1 deduplicated event", len(list))
	}
	got := list[0]
	if got.Source == "tass-en" {
		t.Error("representative row is the state outlet; an independent report must be preferred")
	}
	if got.Reports != 3 {
		t.Errorf("Reports = %d, want 3", got.Reports)
	}
	if len(got.Sources) != 3 {
		t.Errorf("Sources = %v, want all three named", got.Sources)
	}
	if got.IndependentSources == nil || *got.IndependentSources != 2 {
		t.Errorf("IndependentSources = %v, want 2", got.IndependentSources)
	}
	if got.Confidence != 0.8 {
		t.Errorf("Confidence = %v, want 0.8 for two independent sources", got.Confidence)
	}
}

func TestSummaryAndTimelineCountEvents(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	now := time.Now().Add(-2 * time.Hour)

	a := seed(t, s, ctx, "lrt-en", "one event", "negative", 3, []string{"LT"}, now)
	b := seed(t, s, ctx, "delfi-lt", "one event", "negative", 3, []string{"LT"}, now)
	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, b, eventID); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	cells, err := s.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	total := 0
	for _, c := range cells {
		total += c.RecentAdverse
	}
	if total != 1 {
		t.Errorf("summary adverse = %d, want 1", total)
	}

	buckets, err := s.Timeline(ctx, time.Now().AddDate(0, 0, -7), "")
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	n := 0
	for _, b := range buckets {
		n += b.Count
	}
	if n != 1 {
		t.Errorf("timeline count = %d, want 1", n)
	}
}

// Filters must apply to the event's values, and a country filter must still
// find an event whose countries were merged from several reports.
func TestListIncidentsFiltersOnEventValues(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	now := time.Now().Add(-2 * time.Hour)

	a := seed(t, s, ctx, "lrt-en", "border event", "negative", 2, []string{"LT"}, now)
	b := seed(t, s, ctx, "notes-from-poland", "border event", "negative", 4, []string{"PL"}, now)
	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, b, eventID); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	// Poland came only from the second report; the merged event must match it.
	pl, err := s.ListIncidents(ctx, IncidentFilter{Country: "PL", Since: time.Now().AddDate(0, 0, -7)})
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(pl) != 1 {
		t.Errorf("country=PL returned %d rows, want 1", len(pl))
	}

	// Severity is the max across members, so the event clears a >=4 filter
	// even though the representative report was severity 2.
	sev, err := s.ListIncidents(ctx, IncidentFilter{Severity: 4, Since: time.Now().AddDate(0, 0, -7)})
	if err != nil {
		t.Fatalf("ListIncidents: %v", err)
	}
	if len(sev) != 1 {
		t.Errorf("severity>=4 returned %d rows, want 1", len(sev))
	}
}

func TestWeeklyAdverseHistoryCountsEvents(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	// Last week, so it falls inside a completed week.
	lastWeek := time.Now().AddDate(0, 0, -8)

	// Padding so the week clears MinWeeklyVolume and is treated as observed;
	// without it the week is correctly ignored as barely-collected.
	for i := 0; i < MinWeeklyVolume; i++ {
		seed(t, s, ctx, "lsm-en", "filler "+string(rune('a'+i)), "neutral", 1, []string{"LV"}, lastWeek)
	}

	a := seed(t, s, ctx, "lrt-en", "old event", "negative", 3, []string{"LT"}, lastWeek)
	b := seed(t, s, ctx, "err-news", "old event", "negative", 3, []string{"LT"}, lastWeek)
	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, b, eventID); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, eventID); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	hist, err := s.WeeklyAdverseHistory(ctx, 12)
	if err != nil {
		t.Fatalf("WeeklyAdverseHistory: %v", err)
	}
	sum := 0
	for _, n := range hist {
		sum += n
	}
	if sum != 1 {
		t.Errorf("weekly adverse total = %d, want 1 event", sum)
	}
}

// Embedding round-trip: pgx must map REAL[] to []float32 both ways, and the
// candidate query must return only clustered, embedded rows.
func TestEmbeddingRoundTripAndCandidates(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	now := time.Now().Add(-2 * time.Hour)

	a := seed(t, s, ctx, "lrt-en", "embedded", "negative", 3, []string{"LT"}, now)
	vec := []float32{0.1, -0.2, 0.3}
	if err := s.SetIncidentEmbedding(ctx, a, vec); err != nil {
		t.Fatalf("SetIncidentEmbedding: %v", err)
	}

	// Not yet clustered, so it is not a candidate for anything.
	cands, err := s.Candidates(ctx, "sabotage", now, 72*time.Hour)
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("candidates = %d, want 0 before clustering", len(cands))
	}

	if _, err := s.CreateEventFor(ctx, a); err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	cands, err = s.Candidates(ctx, "sabotage", now, 72*time.Hour)
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	if len(cands[0].Embedding) != 3 || cands[0].Embedding[2] != 0.3 {
		t.Errorf("embedding round-trip = %v, want %v", cands[0].Embedding, vec)
	}

	// Outside the window it must not be offered.
	far, err := s.Candidates(ctx, "sabotage", now.Add(200*time.Hour), 72*time.Hour)
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	if len(far) != 0 {
		t.Errorf("candidates outside window = %d, want 0", len(far))
	}
}

func TestIncidentsNeedingEmbedding(t *testing.T) {
	s, ctx := testStore(t)
	now := time.Now().Add(-2 * time.Hour)
	a := seed(t, s, ctx, "lrt-en", "needs embedding", "negative", 3, []string{"LT"}, now)
	seed(t, s, ctx, "err-news", "also needs", "negative", 3, []string{"EE"}, now.Add(-time.Hour))

	pending, err := s.IncidentsNeedingEmbedding(ctx, 10)
	if err != nil {
		t.Fatalf("IncidentsNeedingEmbedding: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending = %d, want 2", len(pending))
	}
	// Newest first, so a partial backfill improves the visible window.
	if pending[0].ID != a {
		t.Errorf("first pending = %d, want the newest incident %d", pending[0].ID, a)
	}

	if err := s.SetIncidentEmbedding(ctx, a, []float32{1, 2}); err != nil {
		t.Fatalf("SetIncidentEmbedding: %v", err)
	}
	pending, err = s.IncidentsNeedingEmbedding(ctx, 10)
	if err != nil {
		t.Fatalf("IncidentsNeedingEmbedding: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("pending after embedding one = %d, want 1", len(pending))
	}
}

// A week we barely collected in is neither quiet nor busy — it is unobserved,
// and it must not become the baseline the current week is judged against.
//
// This is the real false alarm it prevents: on a fresh database, three months
// of sparse backfill left one stray item in each of three weeks. That produced
// a history of [1,1,1], the current week's 13 adverse events were compared
// against a "typical week" of 1, and the dashboard published "rising" when the
// only thing that had risen was collection coverage.
func TestWeeklyAdverseHistoryIgnoresBarelyObservedWeeks(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	lastWeek := time.Now().AddDate(0, 0, -8)
	threeWeeksAgo := time.Now().AddDate(0, 0, -22)

	// A sparse week: one lone item. Not evidence of anything.
	seed(t, s, ctx, "lrt-en", "lone stray item", "negative", 3, []string{"LT"}, threeWeeksAgo)

	// A properly observed week: MinWeeklyVolume events, only one adverse.
	for i := 0; i < MinWeeklyVolume; i++ {
		tone := "neutral"
		if i == 0 {
			tone = "negative"
		}
		seed(t, s, ctx, "err-news", "observed week item "+string(rune('a'+i)), tone, 2, []string{"EE"}, lastWeek)
	}

	hist, err := s.WeeklyAdverseHistory(ctx, 12)
	if err != nil {
		t.Fatalf("WeeklyAdverseHistory: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("history = %v, want exactly the one properly observed week", hist)
	}
	if hist[0] != 1 {
		t.Errorf("observed week adverse = %d, want 1", hist[0])
	}
}

// A properly observed week with no adverse events is a real zero and must
// appear, or the median is computed over only the bad weeks.
func TestWeeklyAdverseHistoryKeepsGenuineZeroWeeks(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	lastWeek := time.Now().AddDate(0, 0, -8)

	for i := 0; i < MinWeeklyVolume; i++ {
		seed(t, s, ctx, "lsm-en", "quiet week item "+string(rune('a'+i)), "positive", 2, []string{"LV"}, lastWeek)
	}
	hist, err := s.WeeklyAdverseHistory(ctx, 12)
	if err != nil {
		t.Fatalf("WeeklyAdverseHistory: %v", err)
	}
	if len(hist) != 1 || hist[0] != 0 {
		t.Errorf("history = %v, want [0] — a busy but favourable week is a real zero", hist)
	}
}

// A change to the aggregation rules must reach events that already exist, not
// only ones a later run happens to touch. Without this the dashboard would
// publish a mix of old and new logic indefinitely.
func TestRefreshEventsSinceHealsStaleEvents(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{"tass-en"}
	now := time.Now().Add(-2 * time.Hour)

	a := seed(t, s, ctx, "lrt-en", "the event", "negative", 2, []string{"LT"}, now)
	b := seed(t, s, ctx, "tass-en", "the event", "negative", 5, []string{"LT", "EE"}, now)
	eventID, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, b, eventID); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}

	// Simulate a row written by the older, buggy aggregation: severity taken
	// from the state outlet, and its country tag included.
	if _, err := s.pool.Exec(ctx,
		`UPDATE events SET severity=5, countries='{LT,EE}' WHERE id=$1`, eventID); err != nil {
		t.Fatalf("stale write: %v", err)
	}

	n, err := s.RefreshEventsSince(ctx, time.Now().AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("RefreshEventsSince: %v", err)
	}
	if n != 1 {
		t.Errorf("refreshed %d events, want 1", n)
	}

	var sev int
	var countries []string
	if err := s.pool.QueryRow(ctx,
		`SELECT severity, countries FROM events WHERE id=$1`, eventID).Scan(&sev, &countries); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if sev != 2 {
		t.Errorf("severity = %d, want 2 — the state outlet's 5 must not survive a refresh", sev)
	}
	if len(countries) != 1 || countries[0] != "LT" {
		t.Errorf("countries = %v, want [LT]", countries)
	}
}

// The top two posture levels require corroboration, so ToneCounts must report
// which severe events actually have it. A single-source severity-4 event is
// counted as adverse but not as corroborated.
func TestToneCountsReportsCorroboration(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{"tass-en"}
	now := time.Now().Add(-2 * time.Hour)

	// Event A: one independent outlet plus a state wire. Not corroborated —
	// state media never counts toward it.
	a := seed(t, s, ctx, "meduza-en", "lone report", "negative", 4, []string{"LT"}, now)
	aState := seed(t, s, ctx, "tass-en", "lone report", "negative", 4, []string{"LT"}, now)
	evA, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.AttachIncident(ctx, aState, evA); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, evA); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	_, sev, corroborated, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if sev[4] != 1 {
		t.Errorf("adverse severity-4 = %d, want 1", sev[4])
	}
	if corroborated[4] != 0 {
		t.Errorf("corroborated severity-4 = %d, want 0 — a state wire is not corroboration", corroborated[4])
	}

	// Add a second independent outlet to the same event; now it is corroborated.
	b := seed(t, s, ctx, "err-news", "lone report", "negative", 4, []string{"LT"}, now)
	if err := s.AttachIncident(ctx, b, evA); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, evA); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}
	_, sev, corroborated, err = s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if sev[4] != 1 || corroborated[4] != 1 {
		t.Errorf("after a second independent source: adverse=%d corroborated=%d, want 1 and 1",
			sev[4], corroborated[4])
	}
}

// An incident that has not been clustered yet is treated as corroborated, so
// the reading does not dip during the minutes between classification and
// clustering. Not-yet-assessed is not a finding of "uncorroborated".
func TestToneCountsUnclusteredCountsAsCorroborated(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	seed(t, s, ctx, "lrt-en", "fresh item", "negative", 4, []string{"LT"}, time.Now().Add(-time.Hour))

	_, sev, corroborated, err := s.ToneCounts(ctx, 7, "")
	if err != nil {
		t.Fatalf("ToneCounts: %v", err)
	}
	if sev[4] != 1 || corroborated[4] != 1 {
		t.Errorf("unclustered: adverse=%d corroborated=%d, want 1 and 1", sev[4], corroborated[4])
	}
}

// The country board's severity label must respect corroboration too, or it
// contradicts the posture banner on the same page. One uncorroborated report
// classified as affecting all four countries was marking the entire region
// "Serious" while the banner correctly said it was awaiting corroboration.
func TestSummaryReportsCorroboratedSeverity(t *testing.T) {
	s, ctx := testStore(t)
	StateControlledSources = []string{}
	now := time.Now().Add(-2 * time.Hour)

	// One severity-4 report, single source, tagged with all four countries.
	a := seed(t, s, ctx, "meduza-en", "regionwide claim", "negative", 4,
		[]string{"LT", "LV", "EE", "PL"}, now)
	ev, err := s.CreateEventFor(ctx, a)
	if err != nil {
		t.Fatalf("CreateEventFor: %v", err)
	}
	if err := s.RefreshEvent(ctx, ev); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}

	cells, err := s.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	seen := 0
	for _, c := range cells {
		if c.RecentAdverse == 0 {
			continue
		}
		seen++
		if c.MaxSeverity != 4 {
			t.Errorf("%s: MaxSeverity = %d, want 4 — the raw severity is still reported", c.Country, c.MaxSeverity)
		}
		if c.MaxSeverityCorroborated != 0 {
			t.Errorf("%s: MaxSeverityCorroborated = %d, want 0 — one source is not corroboration",
				c.Country, c.MaxSeverityCorroborated)
		}
	}
	if seen != 4 {
		t.Fatalf("expected the item to appear for all 4 countries, saw %d", seen)
	}

	// Corroborate it and the board may call it serious.
	b := seed(t, s, ctx, "err-news", "regionwide claim", "negative", 4,
		[]string{"LT", "LV", "EE", "PL"}, now)
	if err := s.AttachIncident(ctx, b, ev); err != nil {
		t.Fatalf("AttachIncident: %v", err)
	}
	if err := s.RefreshEvent(ctx, ev); err != nil {
		t.Fatalf("RefreshEvent: %v", err)
	}
	cells, err = s.Summary(ctx)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	for _, c := range cells {
		if c.RecentAdverse > 0 && c.MaxSeverityCorroborated != 4 {
			t.Errorf("%s: after corroboration MaxSeverityCorroborated = %d, want 4",
				c.Country, c.MaxSeverityCorroborated)
		}
	}
}
