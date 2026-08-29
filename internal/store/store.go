package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Item statuses.
const (
	StatusNew        = "new"
	StatusClassified = "classified"
	StatusIrrelevant = "irrelevant"
	StatusError      = "error"
)

type RawItem struct {
	ID          int64      `json:"id"`
	Source      string     `json:"source"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Lang        string     `json:"lang"`
	PublishedAt *time.Time `json:"published_at"`
	FetchedAt   time.Time  `json:"fetched_at"`
	ContentHash string     `json:"-"`
	Status      string     `json:"status"`
}

type Incident struct {
	ID         int64     `json:"id"`
	RawItemID  int64     `json:"raw_item_id"`
	Category   string    `json:"category"`
	Countries  []string  `json:"countries"`
	Severity   int       `json:"severity"`
	Tone       string    `json:"tone"`
	SummaryEN  string    `json:"summary"`
	Lat        *float64  `json:"lat,omitempty"`
	Lon        *float64  `json:"lon,omitempty"`
	Confidence float32   `json:"confidence"`
	OccurredAt time.Time `json:"occurred_at"`
	// Joined from raw_items for display.
	Source string `json:"source"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

type SourceStatus struct {
	Source     string    `json:"source"`
	LastRun    time.Time `json:"last_run"`
	ItemsFound int       `json:"items_found"`
	ItemsNew   int       `json:"items_new"`
	Error      string    `json:"error"`
}

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

// migrate applies embedded SQL files in lexical order, tracking them in a
// schema_migrations table so each runs once.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

// InsertRawItem stores a fetched item. Returns true if the item was new.
// Duplicates (same URL, or same content hash within the last 7 days) are skipped.
func (s *Store) InsertRawItem(ctx context.Context, it *RawItem) (bool, error) {
	var dup bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM raw_items WHERE content_hash=$1 AND fetched_at > now() - interval '7 days')`,
		it.ContentHash).Scan(&dup)
	if err != nil {
		return false, err
	}
	if dup {
		return false, nil
	}
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO raw_items (source, url, title, body, lang, published_at, content_hash)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (url) DO NOTHING`,
		it.Source, it.URL, it.Title, it.Body, it.Lang, it.PublishedAt, it.ContentHash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// PendingItems returns unclassified items, oldest first.
func (s *Store) PendingItems(ctx context.Context, limit int) ([]RawItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, source, url, title, body, lang, published_at, fetched_at, status
		 FROM raw_items WHERE status=$1 ORDER BY id ASC LIMIT $2`, StatusNew, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RawItem
	for rows.Next() {
		var it RawItem
		if err := rows.Scan(&it.ID, &it.Source, &it.URL, &it.Title, &it.Body, &it.Lang,
			&it.PublishedAt, &it.FetchedAt, &it.Status); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) SetItemStatus(ctx context.Context, id int64, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE raw_items SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (s *Store) InsertIncident(ctx context.Context, inc *Incident) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO incidents (raw_item_id, category, countries, severity, tone, summary_en, lat, lon, confidence, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 ON CONFLICT (raw_item_id) DO NOTHING`,
		inc.RawItemID, inc.Category, inc.Countries, inc.Severity, inc.Tone, inc.SummaryEN,
		inc.Lat, inc.Lon, inc.Confidence, inc.OccurredAt)
	return err
}

func (s *Store) RecordSourceRun(ctx context.Context, source string, started time.Time, found, added int, runErr error) {
	msg := ""
	if runErr != nil {
		msg = runErr.Error()
	}
	_, _ = s.pool.Exec(ctx,
		`INSERT INTO source_runs (source, started_at, finished_at, items_found, items_new, error)
		 VALUES ($1,$2,now(),$3,$4,$5)`, source, started, found, added, msg)
}

// LastSuccessfulRun returns when the source last ran without error (zero time if never).
func (s *Store) LastSuccessfulRun(ctx context.Context, source string) (time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT max(started_at) FROM source_runs WHERE source=$1 AND error=''`, source).Scan(&t)
	if err != nil || t == nil {
		return time.Time{}, err
	}
	return *t, nil
}

type IncidentFilter struct {
	Category string
	Country  string
	Severity int // minimum severity, 0 = any
	Since    time.Time
	Limit    int
	Offset   int
}

func (s *Store) ListIncidents(ctx context.Context, f IncidentFilter) ([]Incident, error) {
	q := `SELECT i.id, i.raw_item_id, i.category, i.countries, i.severity, i.tone, i.summary_en,
	             i.lat, i.lon, i.confidence, i.occurred_at, r.source, r.url, r.title
	      FROM incidents i JOIN raw_items r ON r.id = i.raw_item_id WHERE 1=1`
	args := []any{}
	n := 0
	add := func(cond string, v any) {
		n++
		q += fmt.Sprintf(" AND "+cond, n)
		args = append(args, v)
	}
	if f.Category != "" {
		add("i.category=$%d", f.Category)
	}
	if f.Country != "" {
		add("$%d = ANY(i.countries)", f.Country)
	}
	if f.Severity > 0 {
		add("i.severity >= $%d", f.Severity)
	}
	if !f.Since.IsZero() {
		add("i.occurred_at >= $%d", f.Since)
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	q += fmt.Sprintf(" ORDER BY i.occurred_at DESC LIMIT %d OFFSET %d", f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	incidents := []Incident{}
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(&inc.ID, &inc.RawItemID, &inc.Category, &inc.Countries, &inc.Severity,
			&inc.Tone, &inc.SummaryEN, &inc.Lat, &inc.Lon, &inc.Confidence, &inc.OccurredAt,
			&inc.Source, &inc.URL, &inc.Title); err != nil {
			return nil, err
		}
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

type TimelineBucket struct {
	Day      time.Time `json:"day"`
	Category string    `json:"category"`
	Count    int       `json:"count"`
}

// Timeline returns daily incident counts per category since the given time,
// optionally filtered to one country.
func (s *Store) Timeline(ctx context.Context, since time.Time, country string) ([]TimelineBucket, error) {
	q := `SELECT date_trunc('day', occurred_at) AS day, category, count(*)
	      FROM incidents WHERE occurred_at >= $1`
	args := []any{since}
	if country != "" {
		q += ` AND $2 = ANY(countries)`
		args = append(args, country)
	}
	q += ` GROUP BY day, category ORDER BY day`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	buckets := []TimelineBucket{}
	for rows.Next() {
		var b TimelineBucket
		if err := rows.Scan(&b.Day, &b.Category, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

type SummaryCell struct {
	Country  string `json:"country"`
	Category string `json:"category"`
	Recent   int    `json:"recent"` // all incidents in the last 7 days
	// Tone split of the recent window — a country with 14 favourable and 2
	// adverse items is not "16 incidents worth of trouble".
	RecentAdverse    int     `json:"recent_adverse"`
	RecentFavourable int     `json:"recent_favourable"`
	Baseline         float64 `json:"baseline"`         // avg 7-day count of adverse items over the prior 28 days
	BaselineSamples  int     `json:"baseline_samples"` // adverse items backing that baseline
	MaxSeverity      int     `json:"max_severity"`     // max severity among ADVERSE items only
}

// Summary computes per country×category: the last-7-day tone split against an
// adverse-only baseline. Severity and baseline deliberately ignore favourable
// items, so a week of defence announcements cannot inflate a threat reading.
func (s *Store) Summary(ctx context.Context) ([]SummaryCell, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.country, i.category,
		       count(*) FILTER (WHERE i.occurred_at >= now() - interval '7 days') AS recent,
		       count(*) FILTER (WHERE i.occurred_at >= now() - interval '7 days'
		                          AND i.tone = 'negative') AS recent_adverse,
		       count(*) FILTER (WHERE i.occurred_at >= now() - interval '7 days'
		                          AND i.tone = 'positive') AS recent_favourable,
		       count(*) FILTER (WHERE i.occurred_at >= now() - interval '35 days'
		                          AND i.occurred_at <  now() - interval '7 days'
		                          AND i.tone = 'negative')::float / 4.0 AS baseline,
		       count(*) FILTER (WHERE i.occurred_at >= now() - interval '35 days'
		                          AND i.occurred_at <  now() - interval '7 days'
		                          AND i.tone = 'negative') AS baseline_samples,
		       COALESCE(max(i.severity) FILTER (WHERE i.occurred_at >= now() - interval '7 days'
		                          AND i.tone = 'negative'), 0) AS max_sev
		FROM incidents i, unnest(i.countries) AS c(country)
		WHERE i.occurred_at >= now() - interval '35 days'
		GROUP BY c.country, i.category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cells := []SummaryCell{}
	for rows.Next() {
		var c SummaryCell
		if err := rows.Scan(&c.Country, &c.Category, &c.Recent, &c.RecentAdverse,
			&c.RecentFavourable, &c.Baseline, &c.BaselineSamples, &c.MaxSeverity); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}
	return cells, rows.Err()
}

// ToneCounts returns, for the last `days`, how many incidents carried each
// tone plus the severity histogram of the adverse ones. Optionally scoped to
// one country.
func (s *Store) ToneCounts(ctx context.Context, days int, country string) (map[string]int, [6]int, error) {
	var sev [6]int
	byTone := map[string]int{}

	// make_interval keeps `days` a genuine int parameter; string-concatenating
	// it leaves pgx unable to infer the type.
	q := `SELECT tone, severity, count(*) FROM incidents
	      WHERE occurred_at >= now() - make_interval(days => $1)`
	args := []any{days}
	if country != "" {
		q += ` AND $2 = ANY(countries)`
		args = append(args, country)
	}
	q += ` GROUP BY tone, severity`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, sev, err
	}
	defer rows.Close()
	for rows.Next() {
		var tone string
		var severity, n int
		if err := rows.Scan(&tone, &severity, &n); err != nil {
			return nil, sev, err
		}
		byTone[tone] += n
		if tone == "negative" && severity >= 1 && severity <= 5 {
			sev[severity] += n
		}
	}
	return byTone, sev, rows.Err()
}

// SourceStatuses returns the most recent run per source.
func (s *Store) SourceStatuses(ctx context.Context) ([]SourceStatus, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (source) source, started_at, items_found, items_new, error
		FROM source_runs ORDER BY source, started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SourceStatus{}
	for rows.Next() {
		var st SourceStatus
		if err := rows.Scan(&st.Source, &st.LastRun, &st.ItemsFound, &st.ItemsNew, &st.Error); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
