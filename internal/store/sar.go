package store

import (
	"context"
	"time"
)

// SARObservation is one aggregated Sentinel-1 measurement for an AOI.
type SARObservation struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	// BrightFraction is the share of pixels exceeding the bright-backscatter
	// threshold — a proxy for metallic scatterers (vehicles, aircraft,
	// rolling stock) present in the area.
	BrightFraction float64 `json:"bright_fraction"`
	MeanDB         float64 `json:"mean_db"`
	SampleCount    int64   `json:"sample_count"`
}

type SARAnomaly struct {
	AOI            string    `json:"aoi"`
	IntervalStart  time.Time `json:"interval_start"`
	BrightFraction float64   `json:"bright_fraction"`
	BaselineMedian float64   `json:"baseline_median"`
	ZScore         float64   `json:"zscore"`
	DetectedAt     time.Time `json:"detected_at"`
}

func (s *Store) UpsertSAR(ctx context.Context, aoi string, o *SARObservation) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO layer_sar (aoi, interval_start, interval_end, bright_fraction, mean_db, sample_count)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (aoi, interval_start) DO UPDATE SET
		   bright_fraction = EXCLUDED.bright_fraction,
		   mean_db         = EXCLUDED.mean_db,
		   sample_count    = EXCLUDED.sample_count`,
		aoi, o.Start, o.End, o.BrightFraction, o.MeanDB, o.SampleCount)
	return err
}

// SARSeries returns an AOI's observations oldest-first.
func (s *Store) SARSeries(ctx context.Context, aoi string) ([]SARObservation, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT interval_start, interval_end, bright_fraction, COALESCE(mean_db, 0), sample_count
		 FROM layer_sar WHERE aoi = $1 ORDER BY interval_start ASC`, aoi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SARObservation{}
	for rows.Next() {
		var o SARObservation
		if err := rows.Scan(&o.Start, &o.End, &o.BrightFraction, &o.MeanDB, &o.SampleCount); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SARLatestInterval returns the newest stored interval start for an AOI, and
// whether the AOI has any history at all.
func (s *Store) SARLatestInterval(ctx context.Context, aoi string) (time.Time, bool, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT max(interval_start) FROM layer_sar WHERE aoi = $1`, aoi).Scan(&t)
	if err != nil || t == nil {
		return time.Time{}, false, err
	}
	return *t, true, nil
}

// InsertSARAnomaly records a verdict once per AOI+interval. Returns true if
// this was a new detection.
func (s *Store) InsertSARAnomaly(ctx context.Context, aoi string, intervalStart time.Time,
	bright, median, z float64) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO layer_sar_anomaly (aoi, interval_start, bright_fraction, baseline_median, zscore)
		 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (aoi, interval_start) DO NOTHING`,
		aoi, intervalStart, bright, median, z)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *Store) SARAnomaliesSince(ctx context.Context, since time.Time) ([]SARAnomaly, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT aoi, interval_start, bright_fraction, baseline_median, zscore, detected_at
		 FROM layer_sar_anomaly WHERE detected_at >= $1 ORDER BY detected_at DESC LIMIT 200`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SARAnomaly{}
	for rows.Next() {
		var a SARAnomaly
		if err := rows.Scan(&a.AOI, &a.IntervalStart, &a.BrightFraction,
			&a.BaselineMedian, &a.ZScore, &a.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
