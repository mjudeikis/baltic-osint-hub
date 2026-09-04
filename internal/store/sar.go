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

// SARImage is one rendered Sentinel-1 pass kept for the UI's before/after
// comparison of a flagged site.
type SARImage struct {
	Kind          string
	CapturedStart time.Time
	CapturedEnd   time.Time
	PNG           []byte
}

func (s *Store) UpsertSARImage(ctx context.Context, aoi string, intervalStart time.Time,
	kind string, capturedStart, capturedEnd time.Time, png []byte) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO layer_sar_image (aoi, interval_start, kind, captured_start, captured_end, png)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (aoi, interval_start, kind) DO UPDATE SET
		   captured_start = EXCLUDED.captured_start,
		   captured_end   = EXCLUDED.captured_end,
		   png            = EXCLUDED.png,
		   fetched_at     = now()`,
		aoi, intervalStart, kind, capturedStart, capturedEnd, png)
	return err
}

// SARImagesExist reports whether the before/after pair for an anomalous
// interval is already stored, so the collector never re-buys the same
// renderings from the processing-unit budget.
func (s *Store) SARImagesExist(ctx context.Context, aoi string, intervalStart time.Time) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM layer_sar_image WHERE aoi=$1 AND interval_start=$2`,
		aoi, intervalStart).Scan(&n)
	return n >= 2, err
}

// LatestSARImage returns the newest stored rendering of the given kind for an
// AOI, or nil when none exists.
func (s *Store) LatestSARImage(ctx context.Context, aoi, kind string) (*SARImage, error) {
	var img SARImage
	err := s.pool.QueryRow(ctx,
		`SELECT kind, captured_start, captured_end, png FROM layer_sar_image
		 WHERE aoi=$1 AND kind=$2 ORDER BY interval_start DESC LIMIT 1`,
		aoi, kind).Scan(&img.Kind, &img.CapturedStart, &img.CapturedEnd, &img.PNG)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &img, nil
}

// SARImageMetas returns the capture windows of the newest stored image pair
// for an AOI, without the pixels — what the /api/layers/sar payload needs to
// tell the UI a comparison is available.
func (s *Store) SARImageMetas(ctx context.Context, aoi string) ([]SARImage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT kind, captured_start, captured_end FROM layer_sar_image
		 WHERE aoi=$1 AND interval_start = (
		   SELECT max(interval_start) FROM layer_sar_image WHERE aoi=$1)
		 ORDER BY kind ASC`, aoi)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SARImage
	for rows.Next() {
		var m SARImage
		if err := rows.Scan(&m.Kind, &m.CapturedStart, &m.CapturedEnd); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
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
