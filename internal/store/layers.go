package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
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

// AISFix is one archived AIS position inside a cable corridor.
type AISFix struct {
	MMSI     int64     `json:"mmsi"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	SOG      *float32  `json:"sog"`
	COG      *float32  `json:"cog"`
	NavStat  int       `json:"nav_stat"`
	Corridor string    `json:"corridor"`
	SeenAt   time.Time `json:"seen_at"`
}

// InsertAISFixes stores a batch of positions, skipping fixes already held.
// Returns how many rows were new — a poll that finds nothing new means the
// upstream has not refreshed, not that the corridors emptied.
func (s *Store) InsertAISFixes(ctx context.Context, fixes []AISFix) (int, error) {
	if len(fixes) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for i := range fixes {
		f := &fixes[i]
		batch.Queue(
			`INSERT INTO layer_ais_track (mmsi, lat, lon, sog, cog, nav_stat, corridor, seen_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			 ON CONFLICT (mmsi, seen_at) DO NOTHING`,
			f.MMSI, f.Lat, f.Lon, f.SOG, f.COG, f.NavStat, f.Corridor, f.SeenAt)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	added := 0
	for range fixes {
		tag, err := br.Exec()
		if err != nil {
			return added, err
		}
		added += int(tag.RowsAffected())
	}
	return added, nil
}

// PruneAISTrack drops positions older than the cutoff.
func (s *Store) PruneAISTrack(ctx context.Context, before time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM layer_ais_track WHERE seen_at < $1`, before)
	return int(tag.RowsAffected()), err
}

// AISTrack returns one vessel's archived positions, oldest first, so a track
// can be drawn or re-examined after the fact.
func (s *Store) AISTrack(ctx context.Context, mmsi int64, since time.Time) ([]AISFix, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT mmsi, lat, lon, sog, cog, nav_stat, corridor, seen_at
		 FROM layer_ais_track WHERE mmsi = $1 AND seen_at >= $2
		 ORDER BY seen_at ASC LIMIT 5000`, mmsi, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AISFix{}
	for rows.Next() {
		var f AISFix
		if err := rows.Scan(&f.MMSI, &f.Lat, &f.Lon, &f.SOG, &f.COG, &f.NavStat, &f.Corridor, &f.SeenAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AISTrackCoverage reports what the archive holds per corridor, so the UI can
// say how much history exists rather than implying a quiet corridor is a safe
// one. NordBalt in particular has no Digitraffic coverage at all.
type AISCoverage struct {
	Corridor string     `json:"corridor"`
	Fixes    int        `json:"fixes"`
	Vessels  int        `json:"vessels"`
	Earliest *time.Time `json:"earliest"`
	Latest   *time.Time `json:"latest"`
}

func (s *Store) AISTrackCoverage(ctx context.Context) ([]AISCoverage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT corridor, count(*), count(DISTINCT mmsi), min(seen_at), max(seen_at)
		 FROM layer_ais_track GROUP BY corridor ORDER BY corridor`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AISCoverage{}
	for rows.Next() {
		var c AISCoverage
		if err := rows.Scan(&c.Corridor, &c.Fixes, &c.Vessels, &c.Earliest, &c.Latest); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SanctionedVessel is one entry of the OpenSanctions maritime watchlist,
// keyed by the MMSI an AIS transponder broadcasts.
type SanctionedVessel struct {
	MMSI      int64  `json:"mmsi"`
	IMO       string `json:"imo,omitempty"`
	Name      string `json:"name"`
	Risk      string `json:"risk,omitempty"`
	Flag      string `json:"flag,omitempty"`
	Countries string `json:"countries,omitempty"`
	Datasets  string `json:"datasets,omitempty"`
	URL       string `json:"url,omitempty"`
}

// ReplaceSanctionedVessels swaps in a fresh watchlist atomically.
//
// A full replace, not a merge: delistings matter as much as listings, and a
// vessel that has come off the list must stop being flagged. The swap runs in
// one transaction so a failed refresh can never leave the dashboard with a
// half-empty watchlist — which would silently read as "nothing is sanctioned".
func (s *Store) ReplaceSanctionedVessels(ctx context.Context, vessels []SanctionedVessel) (int, error) {
	if len(vessels) == 0 {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, `DELETE FROM sanctioned_vessels`); err != nil {
		return 0, err
	}
	rows := make([][]any, 0, len(vessels))
	for _, v := range vessels {
		rows = append(rows, []any{v.MMSI, v.IMO, v.Name, v.Risk, v.Flag, v.Countries, v.Datasets, v.URL})
	}
	// CopyFrom rejects duplicate keys, and one vessel can legitimately appear
	// twice (several listings, or an MMSI shared across records), so dedupe on
	// the way in — first listing wins.
	seen := map[int64]bool{}
	deduped := rows[:0]
	for _, r := range rows {
		mmsi := r[0].(int64)
		if seen[mmsi] {
			continue
		}
		seen[mmsi] = true
		deduped = append(deduped, r)
	}
	n, err := tx.CopyFrom(ctx,
		pgx.Identifier{"sanctioned_vessels"},
		[]string{"mmsi", "imo", "name", "risk", "flag", "countries", "datasets", "url"},
		pgx.CopyFromRows(deduped))
	if err != nil {
		return 0, err
	}
	return int(n), tx.Commit(ctx)
}

// LookupSanctionedVessels returns watchlist entries for the given MMSIs.
func (s *Store) LookupSanctionedVessels(ctx context.Context, mmsis []int64) (map[int64]SanctionedVessel, error) {
	out := map[int64]SanctionedVessel{}
	if len(mmsis) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT mmsi, imo, name, risk, flag, countries, datasets, url
		 FROM sanctioned_vessels WHERE mmsi = ANY($1)`, mmsis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v SanctionedVessel
		if err := rows.Scan(&v.MMSI, &v.IMO, &v.Name, &v.Risk, &v.Flag,
			&v.Countries, &v.Datasets, &v.URL); err != nil {
			return nil, err
		}
		out[v.MMSI] = v
	}
	return out, rows.Err()
}

// SanctionedVesselCount reports watchlist size, so the UI can distinguish
// "no vessel matched" from "the watchlist never loaded".
func (s *Store) SanctionedVesselCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM sanctioned_vessels`).Scan(&n)
	return n, err
}

// CertPLDay is one day of CERT.PL warning-list churn.
type CertPLDay struct {
	Day     time.Time `json:"day"`
	Added   int       `json:"added"`
	Removed int       `json:"removed"`
}

// UpsertCertPLDays replaces the daily counts. The upstream list is the source
// of truth for the whole history, so a re-download simply restates it.
func (s *Store) UpsertCertPLDays(ctx context.Context, days []CertPLDay) error {
	if len(days) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, d := range days {
		batch.Queue(
			`INSERT INTO layer_certpl (day, added, removed) VALUES ($1,$2,$3)
			 ON CONFLICT (day) DO UPDATE SET added=EXCLUDED.added, removed=EXCLUDED.removed`,
			d.Day, d.Added, d.Removed)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range days {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CertPLSince(ctx context.Context, since time.Time) ([]CertPLDay, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT day, added, removed FROM layer_certpl WHERE day >= $1 ORDER BY day ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CertPLDay{}
	for rows.Next() {
		var d CertPLDay
		if err := rows.Scan(&d.Day, &d.Added, &d.Removed); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// VesselType is an AIS ship-and-cargo type for one MMSI.
type VesselType struct {
	MMSI     int64  `json:"mmsi"`
	ShipType int    `json:"ship_type"`
	Name     string `json:"name,omitempty"`
	IMO      string `json:"imo,omitempty"`
	CallSign string `json:"call_sign,omitempty"`
}

// IsServiceVessel reports whether an AIS ship type belongs to a craft whose
// normal work involves holding station, and which therefore must not raise a
// loitering detection.
//
// AIS types 50-59 are the service and special-craft block: pilot boats, search
// and rescue, tugs, port tenders, anti-pollution, law enforcement, medical.
// Deliberately NOT excluded:
//
//   - 30 fishing — trawling over a cable route is itself a known tactic, so a
//     fishing vessel loitering on a cable is a signal, not noise;
//   - 35 military — obviously of interest;
//   - 0 / unknown — a vessel broadcasting no type is not a cleared vessel, and
//     suppressing those would let anyone opt out of the detector by omission.
func IsServiceVessel(shipType int) bool {
	return shipType >= 50 && shipType <= 59
}

// UpsertVesselTypes records ship types, newest write winning.
func (s *Store) UpsertVesselTypes(ctx context.Context, types []VesselType) (int, error) {
	if len(types) == 0 {
		return 0, nil
	}
	batch := &pgx.Batch{}
	for _, v := range types {
		batch.Queue(
			`INSERT INTO vessel_types (mmsi, ship_type, name, imo, call_sign, updated_at)
			 VALUES ($1,$2,$3,$4,$5, now())
			 ON CONFLICT (mmsi) DO UPDATE SET
			   ship_type=EXCLUDED.ship_type, name=EXCLUDED.name,
			   imo=EXCLUDED.imo, call_sign=EXCLUDED.call_sign, updated_at=now()`,
			v.MMSI, v.ShipType, v.Name, v.IMO, v.CallSign)
	}
	br := s.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range types {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(types), nil
}

// VesselTypeOf returns the stored ship type, or false when the vessel is
// unknown to us. Unknown must never be treated as "service" — see
// IsServiceVessel.
func (s *Store) VesselTypeOf(ctx context.Context, mmsi int64) (int, bool) {
	var t int
	err := s.pool.QueryRow(ctx, `SELECT ship_type FROM vessel_types WHERE mmsi=$1`, mmsi).Scan(&t)
	if err != nil {
		return 0, false
	}
	return t, true
}

// LookupVesselTypes returns known ship types for a batch of MMSIs.
func (s *Store) LookupVesselTypes(ctx context.Context, mmsis []int64) (map[int64]VesselType, error) {
	out := map[int64]VesselType{}
	if len(mmsis) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT mmsi, ship_type, name, imo, call_sign FROM vessel_types WHERE mmsi = ANY($1)`, mmsis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v VesselType
		if err := rows.Scan(&v.MMSI, &v.ShipType, &v.Name, &v.IMO, &v.CallSign); err != nil {
			return nil, err
		}
		out[v.MMSI] = v
	}
	return out, rows.Err()
}
