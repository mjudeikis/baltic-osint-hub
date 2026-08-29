package store

import (
	"context"
	"time"
)

type FIRMSDetection struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Brightness float32   `json:"brightness"`
	FRP        float32   `json:"frp"`
	Confidence string    `json:"confidence"`
	Sector     string    `json:"sector"`
	DetectedAt time.Time `json:"detected_at"`
}

type GpsjamCell struct {
	Day  time.Time `json:"day"`
	Hex  string    `json:"hex"`
	Good int       `json:"good"`
	Bad  int       `json:"bad"`
}

type AirSighting struct {
	SeenAt   time.Time `json:"seen_at"`
	ICAO24   string    `json:"icao24"`
	Callsign string    `json:"callsign"`
	Country  string    `json:"country"`
	Box      string    `json:"box"`
	Lat      *float64  `json:"lat"`
	Lon      *float64  `json:"lon"`
	Altitude *float32  `json:"altitude"`
	Velocity *float32  `json:"velocity"`
	Reason   string    `json:"reason"`
}

type SeaEvent struct {
	DetectedAt time.Time  `json:"detected_at"`
	MMSI       int64      `json:"mmsi"`
	ShipName   string     `json:"ship_name"`
	Corridor   string     `json:"corridor"`
	Lat        *float64   `json:"lat"`
	Lon        *float64   `json:"lon"`
	SOG        *float32   `json:"sog"`
	Event      string     `json:"event"`
	StartedAt  *time.Time `json:"started_at"`
}

func (s *Store) InsertFIRMS(ctx context.Context, d *FIRMSDetection) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO layer_firms (lat, lon, brightness, frp, confidence, sector, detected_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (lat, lon, detected_at) DO NOTHING`,
		d.Lat, d.Lon, d.Brightness, d.FRP, d.Confidence, d.Sector, d.DetectedAt)
	return err
}

func (s *Store) FIRMSSince(ctx context.Context, since time.Time) ([]FIRMSDetection, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT lat, lon, brightness, frp, confidence, sector, detected_at
		 FROM layer_firms WHERE detected_at >= $1 ORDER BY detected_at DESC LIMIT 2000`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FIRMSDetection{}
	for rows.Next() {
		var d FIRMSDetection
		if err := rows.Scan(&d.Lat, &d.Lon, &d.Brightness, &d.FRP, &d.Confidence, &d.Sector, &d.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) UpsertGpsjamCell(ctx context.Context, c *GpsjamCell) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO layer_gpsjam (day, hex, good, bad) VALUES ($1,$2,$3,$4)
		 ON CONFLICT (day, hex) DO UPDATE SET good = EXCLUDED.good, bad = EXCLUDED.bad`,
		c.Day, c.Hex, c.Good, c.Bad)
	return err
}

// GpsjamLatest returns all cells of the most recent ingested day.
func (s *Store) GpsjamLatest(ctx context.Context) ([]GpsjamCell, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT day, hex, good, bad FROM layer_gpsjam
		 WHERE day = (SELECT max(day) FROM layer_gpsjam)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GpsjamCell{}
	for rows.Next() {
		var c GpsjamCell
		if err := rows.Scan(&c.Day, &c.Hex, &c.Good, &c.Bad); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// HasGpsjamDay reports whether a day is already ingested (skip re-download).
func (s *Store) HasGpsjamDay(ctx context.Context, day time.Time) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM layer_gpsjam WHERE day=$1)`, day).Scan(&exists)
	return exists, err
}

// InsertAirSighting records a sighting unless the same aircraft was already
// recorded in the same box within the dedupe window. Returns true if inserted.
func (s *Store) InsertAirSighting(ctx context.Context, a *AirSighting, dedupe time.Duration) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM layer_air WHERE icao24=$1 AND box=$2 AND seen_at > now() - $3::interval)`,
		a.ICAO24, a.Box, dedupe.String()).Scan(&exists)
	if err != nil || exists {
		return false, err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO layer_air (icao24, callsign, country, box, lat, lon, altitude, velocity, reason)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ICAO24, a.Callsign, a.Country, a.Box, a.Lat, a.Lon, a.Altitude, a.Velocity, a.Reason)
	return err == nil, err
}

func (s *Store) AirSince(ctx context.Context, since time.Time) ([]AirSighting, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT seen_at, icao24, callsign, country, box, lat, lon, altitude, velocity, reason
		 FROM layer_air WHERE seen_at >= $1 ORDER BY seen_at DESC LIMIT 500`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AirSighting{}
	for rows.Next() {
		var a AirSighting
		if err := rows.Scan(&a.SeenAt, &a.ICAO24, &a.Callsign, &a.Country, &a.Box, &a.Lat, &a.Lon, &a.Altitude, &a.Velocity, &a.Reason); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// InsertSeaEvent records an event unless the same vessel+corridor+event exists
// within the dedupe window. Returns true if inserted.
func (s *Store) InsertSeaEvent(ctx context.Context, e *SeaEvent, dedupe time.Duration) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM layer_sea WHERE mmsi=$1 AND corridor=$2 AND event=$3 AND detected_at > now() - $4::interval)`,
		e.MMSI, e.Corridor, e.Event, dedupe.String()).Scan(&exists)
	if err != nil || exists {
		return false, err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO layer_sea (mmsi, ship_name, corridor, lat, lon, sog, event, started_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.MMSI, e.ShipName, e.Corridor, e.Lat, e.Lon, e.SOG, e.Event, e.StartedAt)
	return err == nil, err
}

func (s *Store) SeaSince(ctx context.Context, since time.Time) ([]SeaEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT detected_at, mmsi, ship_name, corridor, lat, lon, sog, event, started_at
		 FROM layer_sea WHERE detected_at >= $1 ORDER BY detected_at DESC LIMIT 500`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SeaEvent{}
	for rows.Next() {
		var e SeaEvent
		if err := rows.Scan(&e.DetectedAt, &e.MMSI, &e.ShipName, &e.Corridor, &e.Lat, &e.Lon, &e.SOG, &e.Event, &e.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
