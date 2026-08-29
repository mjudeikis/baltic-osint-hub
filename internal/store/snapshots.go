package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// PostureSnapshot is what the dashboard published on one day.
type PostureSnapshot struct {
	Day        time.Time       `json:"day"`
	Level      int             `json:"level"`
	LevelName  string          `json:"level_name"`
	Trend      string          `json:"trend"`
	Adverse    int             `json:"adverse"`
	Favourable int             `json:"favourable"`
	Neutral    int             `json:"neutral"`
	Reading    json.RawMessage `json:"reading,omitempty"`
	Summary    json.RawMessage `json:"summary,omitempty"`
	TakenAt    time.Time       `json:"taken_at"`
}

// SavePostureSnapshot records today's published reading.
//
// Upsert on the day, so several collector runs in one day converge on the
// latest reading rather than accumulating rows — the archive answers "what did
// this dashboard say on that date", which is one answer per date.
func (s *Store) SavePostureSnapshot(ctx context.Context, day time.Time, level int, levelName, trend string,
	adverse, favourable, neutral int, reading, summary any) error {
	readingJSON, err := json.Marshal(reading)
	if err != nil {
		return err
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO posture_snapshots (day, level, level_name, trend, adverse, favourable, neutral, reading, summary, taken_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, now())
		 ON CONFLICT (day) DO UPDATE SET
		   level=EXCLUDED.level, level_name=EXCLUDED.level_name, trend=EXCLUDED.trend,
		   adverse=EXCLUDED.adverse, favourable=EXCLUDED.favourable, neutral=EXCLUDED.neutral,
		   reading=EXCLUDED.reading, summary=EXCLUDED.summary, taken_at=now()`,
		day.UTC().Format("2006-01-02"), level, levelName, trend, adverse, favourable, neutral,
		readingJSON, summaryJSON)
	return err
}

// PostureHistory returns the trailing daily readings, oldest first. The heavy
// JSON payloads are omitted — this drives the "how has the reading moved"
// chart, not a full replay.
func (s *Store) PostureHistory(ctx context.Context, days int) ([]PostureSnapshot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT day, level, level_name, trend, adverse, favourable, neutral, taken_at
		 FROM posture_snapshots
		 WHERE day >= (current_date - make_interval(days => $1))
		 ORDER BY day ASC`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PostureSnapshot{}
	for rows.Next() {
		var p PostureSnapshot
		if err := rows.Scan(&p.Day, &p.Level, &p.LevelName, &p.Trend,
			&p.Adverse, &p.Favourable, &p.Neutral, &p.TakenAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostureOn returns the full snapshot for one day, or nil when nothing was
// recorded. Nil means "we have no record of that day", which is a different
// answer from a calm reading and must not be rendered as one.
func (s *Store) PostureOn(ctx context.Context, day time.Time) (*PostureSnapshot, error) {
	var p PostureSnapshot
	err := s.pool.QueryRow(ctx,
		`SELECT day, level, level_name, trend, adverse, favourable, neutral, reading, summary, taken_at
		 FROM posture_snapshots WHERE day = $1`, day.UTC().Format("2006-01-02")).
		Scan(&p.Day, &p.Level, &p.LevelName, &p.Trend, &p.Adverse, &p.Favourable,
			&p.Neutral, &p.Reading, &p.Summary, &p.TakenAt)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
