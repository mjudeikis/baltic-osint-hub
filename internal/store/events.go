package store

import (
	"context"
	"slices"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/cluster"
)

// unitExpr is the heart of the event model as far as SQL is concerned.
//
// A "unit" is one real-world event. Clustered incidents share their event's
// key; an incident that has not been clustered yet is its own unit. That
// COALESCE is what makes the whole migration safe: before any backfill runs,
// every incident is its own unit and every count is exactly what it was
// before. Clustering can then only ever merge units, never make one vanish.
//
// The alternative — counting the events table directly — would have made
// every not-yet-clustered incident invisible the moment the migration landed,
// dropping the adverse count and publishing a falsely calm reading. That is
// the one failure this dashboard must not have.
const unitExpr = `COALESCE('e' || i.event_id, 'i' || i.id)`

// Event is a deduplicated real-world occurrence assembled from one or more
// incident reports.
type Event struct {
	ID           int64     `json:"id"`
	Category     string    `json:"category"`
	Countries    []string  `json:"countries"`
	Severity     int       `json:"severity"`
	Tone         string    `json:"tone"`
	Place        string    `json:"place"`
	SummaryEN    string    `json:"summary"`
	Lat          *float64  `json:"lat,omitempty"`
	Lon          *float64  `json:"lon,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
	SourceCount  int       `json:"source_count"`
	TotalReports int       `json:"total_reports"`
	Confidence   float32   `json:"confidence"`
}

// IncidentsNeedingEmbedding returns classified incidents with no embedding,
// newest first. Newest first matters: the dashboard's visible window is the
// recent one, so a partial backfill improves what readers actually see rather
// than starting with year-old rows nobody is looking at.
func (s *Store) IncidentsNeedingEmbedding(ctx context.Context, limit int) ([]cluster.Pending, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, category, countries, summary_en, occurred_at
		 FROM incidents WHERE embedding IS NULL
		 ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cluster.Pending
	for rows.Next() {
		var p cluster.Pending
		if err := rows.Scan(&p.ID, &p.Category, &p.Countries, &p.SummaryEN, &p.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SetIncidentEmbedding(ctx context.Context, id int64, vec []float32) error {
	_, err := s.pool.Exec(ctx, `UPDATE incidents SET embedding=$2 WHERE id=$1`, id, vec)
	return err
}

// Candidates returns clustered incidents in the same category whose event
// falls within the time window. The window and category filter keep this to a
// handful of rows, which is why the similarity comparison can run in Go
// without a vector index.
func (s *Store) Candidates(ctx context.Context, category string, at time.Time, window time.Duration) ([]cluster.Candidate, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT i.event_id, i.embedding, i.countries
		 FROM incidents i
		 WHERE i.event_id IS NOT NULL
		   AND i.embedding IS NOT NULL
		   AND i.category = $1
		   AND i.occurred_at BETWEEN $2 AND $3`,
		category, at.Add(-window), at.Add(window))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cluster.Candidate
	for rows.Next() {
		var c cluster.Candidate
		if err := rows.Scan(&c.EventID, &c.Embedding, &c.Countries); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateEventFor opens a new event seeded from one incident and attaches it.
func (s *Store) CreateEventFor(ctx context.Context, incidentID int64) (int64, error) {
	var eventID int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO events (category, countries, severity, tone, place, summary_en, lat, lon, occurred_at)
		 SELECT category, countries, severity, tone, place, summary_en, lat, lon, occurred_at
		 FROM incidents WHERE id=$1
		 RETURNING id`, incidentID).Scan(&eventID)
	if err != nil {
		return 0, err
	}
	if err := s.AttachIncident(ctx, incidentID, eventID); err != nil {
		return 0, err
	}
	return eventID, nil
}

func (s *Store) AttachIncident(ctx context.Context, incidentID, eventID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE incidents SET event_id=$2 WHERE id=$1`, incidentID, eventID)
	return err
}

// eventMember is one incident backing an event, used to recompute the event.
type eventMember struct {
	source     string
	tone       string
	severity   int
	countries  []string
	place      string
	summary    string
	lat, lon   *float64
	occurredAt time.Time
	stateRun   bool
}

// RefreshEvent recomputes an event's published attributes from its members.
// Everything on the events row is derived, so this is the only writer of those
// fields and it is idempotent — running it twice changes nothing.
func (s *Store) RefreshEvent(ctx context.Context, eventID int64) error {
	rows, err := s.pool.Query(ctx,
		`SELECT r.source, i.tone, i.severity, i.countries, COALESCE(i.place,''), i.summary_en,
		        i.lat, i.lon, i.occurred_at
		 FROM incidents i JOIN raw_items r ON r.id = i.raw_item_id
		 WHERE i.event_id = $1
		 ORDER BY i.occurred_at ASC`, eventID)
	if err != nil {
		return err
	}
	var members []eventMember
	for rows.Next() {
		var m eventMember
		if err := rows.Scan(&m.source, &m.tone, &m.severity, &m.countries, &m.place,
			&m.summary, &m.lat, &m.lon, &m.occurredAt); err != nil {
			rows.Close()
			return err
		}
		m.stateRun = slices.Contains(StateControlledSources, m.source)
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}

	agg := aggregate(members)
	_, err = s.pool.Exec(ctx,
		`UPDATE events SET countries=$2, severity=$3, tone=$4, place=$5, summary_en=$6,
		        lat=$7, lon=$8, occurred_at=$9, source_count=$10, total_reports=$11,
		        confidence=$12, updated_at=now()
		 WHERE id=$1`,
		eventID, agg.Countries, agg.Severity, agg.Tone, agg.Place, agg.SummaryEN,
		agg.Lat, agg.Lon, agg.OccurredAt, agg.SourceCount, agg.TotalReports, agg.Confidence)
	return err
}

// aggregate folds member reports into the event's published attributes. Split
// out from RefreshEvent so the rules are unit-testable without a database.
func aggregate(members []eventMember) Event {
	ev := Event{TotalReports: len(members)}

	// Every published judgement is taken from independent members only.
	//
	// State-controlled outlets stay attached as evidence — the narrative aimed
	// at the region is itself intelligence — but they must not set severity,
	// countries, tone or the timestamp. An earlier version computed severity
	// and countries across all members and only excluded state media from the
	// tone vote, which meant a Kremlin wire rating an item severity 5 would
	// have pushed the whole regional posture to Severe. That is precisely the
	// control this project promises adversary media does not get.
	//
	// A state-media-only event falls back to its own members, because
	// something must be published — but it carries source_count 0 and is
	// excluded from the posture reading anyway.
	judging := make([]eventMember, 0, len(members))
	for _, m := range members {
		if !m.stateRun {
			judging = append(judging, m)
		}
	}
	stateOnly := len(judging) == 0
	if stateOnly {
		judging = members
	}

	// Earliest report wins the timestamp: an event happened when it was first
	// reported, not when the last outlet caught up.
	ev.OccurredAt = judging[0].occurredAt

	distinct := map[string]bool{}
	toneVotes := map[string]int{}
	countries := []string{}
	for _, m := range judging {
		if m.severity > ev.Severity {
			ev.Severity = m.severity
		}
		for _, c := range m.countries {
			if !slices.Contains(countries, c) {
				countries = append(countries, c)
			}
		}
		if m.occurredAt.Before(ev.OccurredAt) {
			ev.OccurredAt = m.occurredAt
		}
		toneVotes[m.tone]++
		if !stateOnly {
			distinct[m.source] = true
		}
	}
	slices.Sort(countries)
	ev.Countries = countries
	ev.SourceCount = len(distinct)

	// Majority tone among independent members; ties fall to the earliest
	// member's tone, which is deterministic and does not lean alarming.
	best, bestN := "", 0
	for _, t := range []string{"negative", "neutral", "positive"} {
		if toneVotes[t] > bestN {
			best, bestN = t, toneVotes[t]
		}
	}
	ev.Tone = best
	if ev.Tone == "" {
		ev.Tone = judging[0].tone
	}

	// Representative text and location: the earliest independent member that
	// actually carries a location, else the earliest member.
	rep := members[0]
	for _, m := range members {
		if !m.stateRun && m.lat != nil && m.lon != nil {
			rep = m
			break
		}
	}
	if rep.lat == nil {
		for _, m := range members {
			if !m.stateRun {
				rep = m
				break
			}
		}
	}
	ev.Place, ev.SummaryEN, ev.Lat, ev.Lon = rep.place, rep.summary, rep.lat, rep.lon
	// Category is intentionally not recomputed: clustering only ever joins
	// incidents of the same category, so the value set at event creation is
	// already correct and RefreshEvent does not write the column.
	ev.Confidence = confidenceFor(ev.SourceCount)
	return ev
}

// confidenceFor grades an event by independent corroboration, following the
// Airwars practice of a mechanical, published threshold rather than a model's
// self-assessment. The column existed since 001_init.sql and was written as
// 0.0 for every row because nothing ever computed it; this is what it means.
func confidenceFor(independentSources int) float32 {
	switch {
	case independentSources >= 3:
		return 0.95
	case independentSources == 2:
		return 0.8
	case independentSources == 1:
		return 0.5
	default:
		return 0.15 // state-controlled reporting only
	}
}

// ConfidenceLabel turns the corroboration count into the words shown to a
// reader, so the number on screen is never unexplained.
func ConfidenceLabel(independentSources int) string {
	switch {
	case independentSources >= 2:
		return "corroborated"
	case independentSources == 1:
		return "single source"
	default:
		return "state media only"
	}
}
