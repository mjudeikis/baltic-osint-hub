package layers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// AISArchive polls Finnish Digitraffic and stores every AIS position inside a
// cable corridor, building the track history that aisstream.io cannot provide.
//
// aisstream is realtime-only. It can say "this vessel is loitering now", but
// once the moment passes there is nothing to go back to: no track, no way to
// ask what else was in the corridor that night, no way to revise a judgement.
// This layer exists so that history accumulates from today onward.
//
// COVERAGE, measured 2026-08-29 against a live response (1,095 vessels):
//
//	latitude 57.4–65.9, longitude 15.7–32.7
//	gulf-of-finland   196 vessels   good
//	central-baltic     52 vessels   partial — the box starts at 56.5, the feed at 57.4
//	nordbalt            0 vessels   NONE
//
// This is the Finnish national AIS network, so it covers the northern
// approaches well and the southern Baltic not at all. That happens to include
// Balticconnector and the EstLink cables, where the highest-profile incidents
// occurred — but NordBalt (Lithuania–Sweden) is invisible here and remains
// aisstream's job alone. Do not read an empty NordBalt corridor as a quiet one.
//
// Licence: CC BY 4.0, no key required, commercial use permitted.
type AISArchive struct {
	Client *http.Client
	// Retention bounds the table; zero means keep everything.
	Retention time.Duration
}

// digitrafficURL returns every vessel position the Finnish network currently
// holds. There is no bbox parameter on this endpoint, so filtering happens
// here.
const digitrafficURL = "https://meri.digitraffic.fi/api/ais/v1/locations"

// DefaultAISRetention keeps roughly half a year of track. Long enough to
// re-examine any incident the news cycle surfaces, short enough that the table
// stays small on a self-hosted database.
const DefaultAISRetention = 180 * 24 * time.Hour

type digitrafficResponse struct {
	DataUpdatedTime time.Time `json:"dataUpdatedTime"`
	Features        []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"` // [lon, lat]
		} `json:"geometry"`
		Properties struct {
			MMSI    int64   `json:"mmsi"`
			SOG     float64 `json:"sog"`
			COG     float64 `json:"cog"`
			NavStat int     `json:"navStat"`
			// Milliseconds since the Unix epoch, from the AIS message itself.
			TimestampExternal int64 `json:"timestampExternal"`
		} `json:"properties"`
	} `json:"features"`
}

func (a *AISArchive) Run(ctx context.Context, db *store.Store, log *slog.Logger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, digitrafficURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	// Digitraffic answers 406 without this: "Use of gzip compression is
	// required with Accept-Encoding: gzip header." Go's transport normally
	// adds and transparently decodes it, but only when the header is not set
	// manually — so it is left to the transport rather than set here.
	req.Header.Set("Digitraffic-User", "baltic-osint-hub/1.0")

	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("digitraffic: status %d: %.200s", resp.StatusCode, body)
	}

	var out digitrafficResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("digitraffic: decode: %w", err)
	}

	fixes := make([]store.AISFix, 0, 256)
	for _, f := range out.Features {
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}
		lon, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
		corridor := Sector(CableCorridors, lat, lon)
		if corridor == "" {
			continue // outside every watched corridor
		}
		p := f.Properties
		seen := time.UnixMilli(p.TimestampExternal).UTC()
		// A missing or absurd timestamp would corrupt the track ordering, so
		// fall back to the response's own update time rather than storing it.
		if p.TimestampExternal <= 0 || seen.After(time.Now().Add(time.Hour)) {
			seen = out.DataUpdatedTime
		}
		sog, cog := float32(p.SOG), float32(p.COG)
		fixes = append(fixes, store.AISFix{
			MMSI:     p.MMSI,
			Lat:      lat,
			Lon:      lon,
			SOG:      &sog,
			COG:      &cog,
			NavStat:  p.NavStat,
			Corridor: corridor,
			SeenAt:   seen,
		})
	}

	added, err := db.InsertAISFixes(ctx, fixes)
	if err != nil {
		return err
	}
	log.Info("ais archive", "vessels_total", len(out.Features),
		"in_corridors", len(fixes), "stored", added)

	if a.Retention > 0 {
		removed, err := db.PruneAISTrack(ctx, time.Now().Add(-a.Retention))
		if err != nil {
			return err
		}
		if removed > 0 {
			log.Info("ais archive pruned", "rows", removed)
		}
	}

	// Ship types come from the same source and the same poll. Failing to
	// refresh them is not worth failing the archive over — a stale type table
	// still filters correctly, it just learns about new vessels later.
	if err := a.refreshVesselTypes(ctx, client, db, log); err != nil {
		log.Error("vessel types", "err", err)
	}
	return nil
}

const digitrafficVesselsURL = "https://meri.digitraffic.fi/api/ais/v1/vessels"

type digitrafficVessel struct {
	MMSI     int64  `json:"mmsi"`
	Name     string `json:"name"`
	ShipType int    `json:"shipType"`
	IMO      int64  `json:"imo"`
	CallSign string `json:"callSign"`
}

// refreshVesselTypes populates the ship-type lookup the loitering detector
// uses to tell a tanker from a pilot boat.
//
// Measured 2026-08-29 across 1,002 vessels in the Finnish registry: 118 tugs,
// 73 pilot boats and 38 SAR craft — 23% service vessels, all of which hold
// station as a matter of routine and were previously raising loitering
// detections indistinguishable from a tanker stopping over a cable.
func (a *AISArchive) refreshVesselTypes(ctx context.Context, client *http.Client, db *store.Store, log *slog.Logger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, digitrafficVesselsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Digitraffic-User", "baltic-osint-hub/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("digitraffic vessels: status %d", resp.StatusCode)
	}
	var vessels []digitrafficVessel
	if err := json.NewDecoder(resp.Body).Decode(&vessels); err != nil {
		return fmt.Errorf("digitraffic vessels: decode: %w", err)
	}

	out := make([]store.VesselType, 0, len(vessels))
	service := 0
	for _, v := range vessels {
		if v.MMSI <= 0 {
			continue
		}
		imo := ""
		if v.IMO > 0 {
			imo = strconv.FormatInt(v.IMO, 10)
		}
		if store.IsServiceVessel(v.ShipType) {
			service++
		}
		out = append(out, store.VesselType{
			MMSI: v.MMSI, ShipType: v.ShipType, Name: v.Name, IMO: imo, CallSign: v.CallSign,
		})
	}
	n, err := db.UpsertVesselTypes(ctx, out)
	if err != nil {
		return err
	}
	log.Info("vessel types", "stored", n, "service_craft", service)
	return nil
}
