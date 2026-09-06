package store

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

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
	// Attempts is how many times the classifier has been asked about this
	// item; see MaxClassifyAttempts.
	Attempts int `json:"-"`
}

type Incident struct {
	ID         int64     `json:"id"`
	RawItemID  int64     `json:"raw_item_id"`
	Category   string    `json:"category"`
	Countries  []string  `json:"countries"`
	Severity   int       `json:"severity"`
	Tone       string    `json:"tone"`
	Place      string    `json:"place"`
	SummaryEN  string    `json:"summary"`
	Lat        *float64  `json:"lat,omitempty"`
	Lon        *float64  `json:"lon,omitempty"`
	Confidence float32   `json:"confidence"`
	OccurredAt time.Time `json:"occurred_at"`
	// Joined from raw_items for display.
	Source string `json:"source"`
	URL    string `json:"url"`
	Title  string `json:"title"`

	// Set once the incident has been clustered into an event. A nil EventID
	// means "not assessed yet", which is different from "uncorroborated" —
	// the UI must not grade an incident that was never checked.
	EventID *int64 `json:"event_id,omitempty"`
	// Reports counts the articles backing this event, Sources names them.
	Reports int      `json:"reports"`
	Sources []string `json:"sources,omitempty"`
	// IndependentSources excludes state-controlled outlets; nil when
	// unclustered. This is what Confidence is derived from.
	IndependentSources *int `json:"independent_sources,omitempty"`
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
	// log receives failures from best-effort writes such as RecordSourceRun
	// that have no caller to return an error to. Defaults to slog.Default();
	// set with SetLogger so those failures land in the same JSON stream as
	// everything else.
	log *slog.Logger
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	// Pin the session timezone. Every counting query uses current_date,
	// date_trunc('week', now()) and friends, and those follow the session
	// timezone — which otherwise depends on whatever the server or the
	// container's TZ happens to be. UTC everywhere makes "this week" mean the
	// same thing in the collector, the API and the tests.
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	s := &Store{pool: pool, log: slog.Default()}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

// SetLogger routes the store's own diagnostics to the process logger.
func (s *Store) SetLogger(l *slog.Logger) {
	if l != nil {
		s.log = l
	}
}

// Ping verifies the pool can reach the database; the readiness probe uses it
// so a server whose database is gone stops receiving traffic rather than
// answering every request with 500.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// migrationLockKey is an arbitrary fixed advisory-lock key; only its
// stability across processes matters.
const migrationLockKey = 726354819

// migrate applies embedded SQL files in lexical order, tracking them in a
// schema_migrations table so each runs once.
//
// The whole pass runs under a session-level advisory lock. The server and the
// collector both migrate on startup, and a deploy that ships a new migration
// restarts both at once: without the lock each saw the file as unapplied and
// raced to apply it, and whichever lost failed on the duplicate. Since the
// lock is held by one session, it is taken on a dedicated connection and
// released from the same one.
func (s *Store) migrate(ctx context.Context) error {
	lock, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer lock.Release()
	if _, err := lock.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer func() {
		_, _ = lock.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

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

// PendingItems returns unclassified items: never-tried first, then by age.
// Ordering by attempts before id keeps a batch the model keeps failing on
// from occupying the front of every run's budget while fresh items queue
// behind it.
func (s *Store) PendingItems(ctx context.Context, limit int) ([]RawItem, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, source, url, title, body, lang, published_at, fetched_at, status, attempts
		 FROM raw_items WHERE status=$1 ORDER BY attempts ASC, id ASC LIMIT $2`, StatusNew, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []RawItem
	for rows.Next() {
		var it RawItem
		if err := rows.Scan(&it.ID, &it.Source, &it.URL, &it.Title, &it.Body, &it.Lang,
			&it.PublishedAt, &it.FetchedAt, &it.Status, &it.Attempts); err != nil {
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

// MaxClassifyAttempts is how many times an item is sent to the classifier
// before it is retired as an error. Three is enough to ride out a transient
// API failure and a one-off malformed reply; an item still failing after
// that is not going to succeed on the fourth try.
const MaxClassifyAttempts = 3

// RecordClassifyAttempt bumps the attempt counter for a batch about to be
// classified and retires any item that has now reached the limit. It runs
// before the model call, not after, so an attempt that crashes the run or
// times out still counts — otherwise a batch that reliably kills the run
// would never accumulate attempts at all. A retired item that the model then
// answers is simply marked classified by the caller, overriding this.
// Returns the ids retired.
func (s *Store) RecordClassifyAttempt(ctx context.Context, ids []int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`UPDATE raw_items
		 SET attempts = attempts + 1,
		     status = CASE WHEN attempts + 1 >= $2 THEN $3 ELSE status END
		 WHERE id = ANY($1)
		 RETURNING id, attempts`, ids, MaxClassifyAttempts, StatusError)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var retired []int64
	for rows.Next() {
		var id int64
		var attempts int
		if err := rows.Scan(&id, &attempts); err != nil {
			return nil, err
		}
		if attempts >= MaxClassifyAttempts {
			retired = append(retired, id)
		}
	}
	return retired, rows.Err()
}

func (s *Store) InsertIncident(ctx context.Context, inc *Incident) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO incidents (raw_item_id, category, countries, severity, tone, place, summary_en, lat, lon, confidence, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (raw_item_id) DO NOTHING`,
		inc.RawItemID, inc.Category, inc.Countries, inc.Severity, inc.Tone, inc.Place, inc.SummaryEN,
		inc.Lat, inc.Lon, inc.Confidence, inc.OccurredAt)
	return err
}

// RecordSourceRun is best-effort bookkeeping: a failure to write it must not
// fail the run it describes, but it is logged rather than dropped, because a
// silently missing run makes a source look like it never fired.
func (s *Store) RecordSourceRun(ctx context.Context, source string, started time.Time, found, added int, runErr error) {
	msg := ""
	if runErr != nil {
		msg = SanitizeError(runErr.Error())
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO source_runs (source, started_at, finished_at, items_found, items_new, error)
		 VALUES ($1,$2,now(),$3,$4,$5)`, source, started, found, added, msg); err != nil {
		s.log.Error("record source run", "source", source, "err", err)
	}
}

// Error text stored on source_runs is served verbatim by GET /api/sources, so
// anything a transport error embeds — most notably the full request URL —
// becomes public. These patterns cover the ways credentials travel in URLs
// and headers; the layer that owns a secret is still expected to keep it out
// of its errors, this is the backstop.
var (
	secretQueryRe = regexp.MustCompile(`(?i)([?&](?:key|map_key|token|api_key|apikey|client_secret|access_token|secret|password)=)[^&\s"']*`)
	bearerRe      = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]+`)
)

// maxStoredError caps the stored text. Error strings occasionally carry a
// whole response body; the first few hundred characters diagnose the
// failure and the rest is noise served to every reader of /api/sources.
const maxStoredError = 500

// SanitizeError redacts credential-shaped fragments and truncates.
func SanitizeError(msg string) string {
	msg = secretQueryRe.ReplaceAllString(msg, "${1}<redacted>")
	msg = bearerRe.ReplaceAllString(msg, "Bearer <redacted>")
	if len(msg) > maxStoredError {
		cut := msg[:maxStoredError]
		for len(cut) > 0 && !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		msg = cut + "…"
	}
	return msg
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
	Tone     string
	Severity int // minimum severity, 0 = any
	Since    time.Time
	// Until bounds the window at the top, so a single day can be selected
	// rather than only "everything since". Zero means no upper bound.
	Until  time.Time
	Limit  int
	Offset int
	// ExcludeStateOnly drops units that only state-controlled outlets
	// reported: an unclustered state-media incident, or an event with no
	// independent source. Reports from state media attached to an event an
	// independent outlet also carried are kept, so the event's source list
	// stays complete. The posture banner uses this when naming the event
	// that set the level — it must never headline a TASS claim.
	ExcludeStateOnly bool
}

// Listing bounds. DefaultListWindow bounds a listing with no time filter, so
// the event-grouping CTEs never scan the whole table for one page; MaxListLimit
// caps a page.
const (
	DefaultListLimit  = 100
	MaxListLimit      = 500
	DefaultListWindow = 90 * 24 * time.Hour
)

// Normalize clamps the paging fields and applies the default window. Both
// the API and the store call it so a caller that bypasses the API gets the
// same bounds.
func (f *IncidentFilter) Normalize() {
	if f.Limit <= 0 {
		f.Limit = DefaultListLimit
	}
	f.Limit = min(f.Limit, MaxListLimit)
	f.Offset = max(f.Offset, 0)
	if f.Since.IsZero() {
		f.Since = time.Now().Add(-DefaultListWindow)
	}
}

// ListIncidents returns one row per event, not per article. Where an event was
// reported by six outlets the feed shows it once, carrying the corroborating
// sources alongside, rather than six near-identical entries that made an
// ordinary week look like a wave.
//
// The representative row is the earliest independent report: state-controlled
// outlets sort last so an event's headline and link are never TASS's when any
// independent outlet also covered it.
func (s *Store) ListIncidents(ctx context.Context, f IncidentFilter) ([]Incident, error) {
	args := []any{StateControlledSources}
	n := 1
	where := ""
	add := func(cond string, v any) {
		n++
		where += fmt.Sprintf(" AND "+cond, n)
		args = append(args, v)
	}
	if f.Category != "" {
		add("COALESCE(e.category, i.category)=$%d", f.Category)
	}
	if f.Tone != "" {
		add("COALESCE(e.tone, i.tone)=$%d", f.Tone)
	}
	if f.Country != "" {
		add("$%d = ANY(COALESCE(e.countries, i.countries))", f.Country)
	}
	if f.Severity > 0 {
		add("COALESCE(e.severity, i.severity) >= $%d", f.Severity)
	}
	f.Normalize()
	add("COALESCE(e.occurred_at, i.occurred_at) >= $%d", f.Since)
	if !f.Until.IsZero() {
		add("COALESCE(e.occurred_at, i.occurred_at) < $%d", f.Until)
	}
	if f.ExcludeStateOnly {
		// Unclustered: the one report must itself be independent. Clustered:
		// the event must have at least one independent source; its state
		// member rows survive so the sources list is complete.
		where += ` AND (CASE WHEN i.event_id IS NULL THEN NOT (r.source = ANY($1))
		                     ELSE e.source_count > 0 END)`
	}

	q := `WITH filtered AS (
	        SELECT i.id, i.raw_item_id, i.event_id,
	               COALESCE(e.category, i.category)       AS category,
	               COALESCE(e.countries, i.countries)     AS countries,
	               COALESCE(e.severity, i.severity)       AS severity,
	               COALESCE(e.tone, i.tone)               AS tone,
	               COALESCE(e.place, i.place, '')         AS place,
	               COALESCE(e.summary_en, i.summary_en)   AS summary_en,
	               COALESCE(e.lat, i.lat)                 AS lat,
	               COALESCE(e.lon, i.lon)                 AS lon,
	               COALESCE(e.confidence, i.confidence)   AS confidence,
	               COALESCE(e.occurred_at, i.occurred_at) AS occurred_at,
	               e.source_count AS independent_sources,
	               r.source, r.url, r.title,
	               ` + unitExpr + ` AS unit,
	               (r.source = ANY($1)) AS state_media
	        FROM incidents i
	        LEFT JOIN events e ON e.id = i.event_id
	        JOIN raw_items r ON r.id = i.raw_item_id
	        WHERE 1=1` + where + `
	      ),
	      agg AS (
	        SELECT unit, count(*) AS reports, array_agg(DISTINCT source) AS sources
	        FROM filtered GROUP BY unit
	      ),
	      ranked AS (
	        SELECT f.*, row_number() OVER (
	                 PARTITION BY f.unit
	                 ORDER BY f.state_media ASC, f.occurred_at ASC, f.id ASC) AS rn
	        FROM filtered f
	      )
	      SELECT r.id, r.raw_item_id, r.event_id, r.category, r.countries, r.severity, r.tone,
	             r.place, r.summary_en, r.lat, r.lon, r.confidence, r.occurred_at,
	             r.source, r.url, r.title, a.reports, a.sources, r.independent_sources
	      FROM ranked r JOIN agg a ON a.unit = r.unit
	      WHERE r.rn = 1`
	q += fmt.Sprintf(" ORDER BY r.occurred_at DESC LIMIT %d OFFSET %d", f.Limit, f.Offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	incidents := []Incident{}
	for rows.Next() {
		var inc Incident
		if err := rows.Scan(&inc.ID, &inc.RawItemID, &inc.EventID, &inc.Category, &inc.Countries,
			&inc.Severity, &inc.Tone, &inc.Place, &inc.SummaryEN, &inc.Lat, &inc.Lon,
			&inc.Confidence, &inc.OccurredAt, &inc.Source, &inc.URL, &inc.Title,
			&inc.Reports, &inc.Sources, &inc.IndependentSources); err != nil {
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

// FreshUnclusteredWindow is how long an incident that has not been clustered
// yet is presumed corroborated. See unitsCTE.
const FreshUnclusteredWindow = 2 * time.Hour

// unitsCTE is the one definition of "a countable event" shared by every
// aggregate the dashboard publishes: the timeline, the country board, the
// weekly baseline and the posture reading. It groups incident rows into units
// (see unitExpr) and carries the per-unit attributes those queries read.
//
// It exists as a single fragment because the four queries once each carried
// their own copy, and two of them drifted: ToneCounts and the weekly history
// excluded state-controlled outlets while Summary and Timeline did not, so a
// TASS-only claim was absent from the banner but present on the board below
// it. The exclusion is now applied here, once, at row level: an event still
// counts if any independent outlet reported it, and drops out entirely if
// only state media did. We monitor adversary messaging, but letting it feed
// any published count would hand an adversary control of our own reading.
//
// Corroboration for a unit that has not been clustered yet is a fail-safe
// rather than a fail-open. Not-yet-assessed is not the same as
// uncorroborated, and suppressing on absence would understate a real event
// during the minutes between classification and clustering — so a fresh
// unclustered incident is presumed corroborated. But clustering can also
// fail for a whole run (an embeddings outage returns early and leaves
// everything unclustered), and under the old COALESCE(source_count, 2)
// every such incident counted as corroborated indefinitely, which is the
// exact "one uncertain report moves the reading" failure the corroboration
// rule exists to prevent. After FreshUnclusteredWindow the presumption
// lapses and the incident is what it is: a single report.
//
// $1 is always the state-controlled source list; callers place their own
// parameters from $2. extra is appended to the WHERE clause.
func unitsCTE(extra string) string {
	return `units AS (
	  SELECT ` + unitExpr + ` AS unit,
	         max(COALESCE(e.category, i.category)) AS category,
	         max(COALESCE(e.countries, i.countries)) AS countries,
	         max(COALESCE(e.tone, i.tone)) AS tone,
	         max(COALESCE(e.severity, i.severity)) AS severity,
	         max(CASE WHEN e.id IS NOT NULL THEN e.source_count
	                  WHEN i.classified_at >= now() - make_interval(secs => ` +
		fmt.Sprintf("%d", int(FreshUnclusteredWindow.Seconds())) + `) THEN 2
	                  ELSE 1 END) >= 2 AS corroborated,
	         min(COALESCE(e.occurred_at, i.occurred_at)) AS occurred_at
	  FROM incidents i
	  LEFT JOIN events e ON e.id = i.event_id
	  JOIN raw_items r ON r.id = i.raw_item_id
	  WHERE NOT (r.source = ANY($1))
	  ` + extra + `
	  GROUP BY unit
	)`
}

// Timeline returns daily incident counts per category since the given time,
// optionally filtered to one country. minSeverity, when above 1, drops units
// below that severity — the dashboard uses 2 so severity-1 commentary and
// analysis pieces never inflate a chart titled "incidents".
func (s *Store) Timeline(ctx context.Context, since time.Time, country string, minSeverity int) ([]TimelineBucket, error) {
	q := `WITH ` + unitsCTE(`AND i.occurred_at >= $2`) + `
	      SELECT date_trunc('day', occurred_at) AS day, category, count(*)
	      FROM units WHERE true`
	args := []any{StateControlledSources, since}
	if country != "" {
		q += ` AND $3 = ANY(countries)`
		args = append(args, country)
	}
	if minSeverity > 1 {
		q += fmt.Sprintf(` AND severity >= $%d`, len(args)+1)
		args = append(args, minSeverity)
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
	// MaxSeverityCorroborated is the same, restricted to events with at least
	// two independent sources. The board's severity label uses this so it
	// cannot contradict the posture banner: one uncorroborated item was
	// labelling all four countries "Serious" while the banner correctly said
	// the event was still awaiting corroboration.
	MaxSeverityCorroborated int `json:"max_severity_corroborated"`
}

// Summary computes per country×category: the last-7-day tone split against an
// adverse-only baseline. Severity and baseline deliberately ignore favourable
// items, so a week of defence announcements cannot inflate a threat reading.
// minSeverity, when above 1, drops units below that severity from every count
// (recent, baseline, and max alike, so the numbers stay comparable) — the
// dashboard uses 2 so commentary never counts as an event.
func (s *Store) Summary(ctx context.Context, minSeverity int) ([]SummaryCell, error) {
	rows, err := s.pool.Query(ctx, `
		WITH `+unitsCTE(`AND i.occurred_at >= now() - interval '35 days'`)+`
		SELECT c.country, u.category,
		       count(*) FILTER (WHERE u.occurred_at >= now() - interval '7 days') AS recent,
		       count(*) FILTER (WHERE u.occurred_at >= now() - interval '7 days'
		                          AND u.tone = 'negative') AS recent_adverse,
		       count(*) FILTER (WHERE u.occurred_at >= now() - interval '7 days'
		                          AND u.tone = 'positive') AS recent_favourable,
		       count(*) FILTER (WHERE u.occurred_at >= now() - interval '35 days'
		                          AND u.occurred_at <  now() - interval '7 days'
		                          AND u.tone = 'negative')::float / 4.0 AS baseline,
		       count(*) FILTER (WHERE u.occurred_at >= now() - interval '35 days'
		                          AND u.occurred_at <  now() - interval '7 days'
		                          AND u.tone = 'negative') AS baseline_samples,
		       COALESCE(max(u.severity) FILTER (WHERE u.occurred_at >= now() - interval '7 days'
		                          AND u.tone = 'negative'), 0) AS max_sev,
		       COALESCE(max(u.severity) FILTER (WHERE u.occurred_at >= now() - interval '7 days'
		                          AND u.tone = 'negative' AND u.corroborated), 0) AS max_sev_corr
		FROM units u, unnest(u.countries) AS c(country)
		WHERE u.severity >= $2
		GROUP BY c.country, u.category`, StateControlledSources, max(minSeverity, 1))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cells := []SummaryCell{}
	for rows.Next() {
		var c SummaryCell
		if err := rows.Scan(&c.Country, &c.Category, &c.Recent, &c.RecentAdverse,
			&c.RecentFavourable, &c.Baseline, &c.BaselineSamples, &c.MaxSeverity,
			&c.MaxSeverityCorroborated); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}
	return cells, rows.Err()
}

// StateControlledSources is the list of outlets excluded from every published
// count (see unitsCTE) and from corroboration (see aggregate). It is set once
// at startup through SetStateControlled; kept as a plain slice so the query
// layer needs no dependency on the sources package.
var StateControlledSources []string

// SetStateControlled is the single place the exclusion list is installed.
// Both binaries must call it right after New: with an empty list every
// query above runs unfiltered and state media silently enters the posture
// reading, which is the one thing this project promises cannot happen.
func SetStateControlled(list []string) { StateControlledSources = list }

// MinWeeklyVolume is how many classified events a week must contain before it
// may serve as a baseline comparison.
//
// Without this floor, a week in which the collector ingested one stray item
// counts as a full week of evidence that "typical" is one event. On a fresh
// database that produced a real false alarm: three months of sparse
// backfill left a history of [1, 1, 1], the current week's 13 adverse events
// were compared against a "typical week" of 1, and the dashboard published
// "↑ rising" when the only thing that had actually risen was collection
// coverage.
//
// This is the same failure the SAR baseline guard exists for, and the same one
// that once produced "+7500%" trend readings: a near-empty baseline is not a
// quiet baseline, and the honest output is "we cannot tell yet".
const MinWeeklyVolume = 5

// WeeklyAdverseHistory returns adverse-event counts per ISO week over the
// trailing `weeks`, oldest first, so the dashboard can answer "is this week
// unusual?" rather than leaving a bare number to be read as alarming.
//
// Only weeks carrying at least MinWeeklyVolume classified events are returned,
// and a qualifying week with no adverse events is returned as a genuine zero.
// A week we barely collected in is neither quiet nor busy — it is unobserved,
// and it must not enter the median at all.
func (s *Store) WeeklyAdverseHistory(ctx context.Context, weeks int) ([]int, error) {
	rows, err := s.pool.Query(ctx, `
		WITH `+unitsCTE(`AND i.occurred_at >= date_trunc('week', now()) - make_interval(weeks => $2)
		    AND i.occurred_at < date_trunc('week', now())`)+`
		SELECT date_trunc('week', occurred_at) AS wk,
		       count(*) FILTER (WHERE tone = 'negative') AS adverse
		FROM units
		GROUP BY wk
		HAVING count(*) >= $3
		ORDER BY wk`, StateControlledSources, weeks, MinWeeklyVolume)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var wk time.Time
		var n int
		if err := rows.Scan(&wk, &n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ToneCounts returns, for the last `days`, how many *events* carried each
// tone plus the severity histogram of the adverse ones. Optionally scoped to
// one country.
//
// This counts units, not rows — see unitsCTE. Before clustering existed, one
// well-covered incident reported by six outlets contributed six to the adverse
// count, so the posture reading tracked press attention rather than events.
func (s *Store) ToneCounts(ctx context.Context, days int, country string) (map[string]int, [6]int, [6]int, error) {
	var sev, corroborated [6]int
	byTone := map[string]int{}

	// make_interval keeps `days` a genuine int parameter; string-concatenating
	// it leaves pgx unable to infer the type.
	q := `WITH ` + unitsCTE(`AND i.occurred_at >= now() - make_interval(days => $2)`) + `
	      SELECT tone, severity, corroborated, count(*)
	      FROM units WHERE true`
	args := []any{StateControlledSources, days}
	if country != "" {
		q += ` AND $3 = ANY(countries)`
		args = append(args, country)
	}
	q += ` GROUP BY tone, severity, corroborated`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, sev, corroborated, err
	}
	defer rows.Close()
	for rows.Next() {
		var tone string
		var severity, n int
		var isCorroborated bool
		if err := rows.Scan(&tone, &severity, &isCorroborated, &n); err != nil {
			return nil, sev, corroborated, err
		}
		byTone[tone] += n
		if tone == "negative" && severity >= 1 && severity <= 5 {
			sev[severity] += n
			if isCorroborated {
				corroborated[severity] += n
			}
		}
	}
	return byTone, sev, corroborated, rows.Err()
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

// PruneResult reports how many rows each retention rule removed.
type PruneResult struct {
	SourceRuns int
	RawItems   int
	Air        int
	FIRMS      int
	Sea        int
	SARImages  int
}

// Prune applies the retention policy. Nothing here touches incidents or
// events — the classified record is the product and is kept indefinitely —
// and raw_items are only removed once they were rejected and nothing
// references them. The signal layers are rolling windows by nature: the map
// shows days, the anomaly baselines use weeks, and beyond that the rows are
// only cost. SAR image bytes go after 60 days but the numeric series stays,
// because the series is what the anomaly baseline is built from.
func (s *Store) Prune(ctx context.Context) (PruneResult, error) {
	var res PruneResult
	steps := []struct {
		dst *int
		sql string
	}{
		{&res.SourceRuns, `DELETE FROM source_runs WHERE started_at < now() - interval '30 days'`},
		// A raw item backing an incident is never eligible: the FK would
		// cascade and delete the incident with it.
		{&res.RawItems, `DELETE FROM raw_items
		                 WHERE status IN ('irrelevant', 'error')
		                   AND fetched_at < now() - interval '60 days'
		                   AND NOT EXISTS (SELECT 1 FROM incidents i WHERE i.raw_item_id = raw_items.id)`},
		{&res.Air, `DELETE FROM layer_air WHERE seen_at < now() - interval '90 days'`},
		{&res.FIRMS, `DELETE FROM layer_firms WHERE detected_at < now() - interval '180 days'`},
		{&res.Sea, `DELETE FROM layer_sea WHERE detected_at < now() - interval '365 days'`},
		{&res.SARImages, `DELETE FROM layer_sar_image WHERE fetched_at < now() - interval '60 days'`},
	}
	for _, st := range steps {
		tag, err := s.pool.Exec(ctx, st.sql)
		if err != nil {
			return res, err
		}
		*st.dst = int(tag.RowsAffected())
	}
	return res, nil
}
