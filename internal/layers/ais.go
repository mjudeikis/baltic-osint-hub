package layers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// AISWatch consumes the aisstream.io websocket and records suspicious vessel
// behaviour inside the Baltic cable corridors:
//   - loitering: speed < 1 kn for 30+ minutes mid-corridor
//   - ais-gap: transponder silent for 1h+ while inside a corridor
//
// Two exclusions keep this from describing a working harbour rather than
// detecting anything. Both were added after a week of live data produced 97
// loitering events of which only 3 involved a sanctioned vessel:
//
//   - Navigational status. A vessel that reports itself moored or AT ANCHOR is
//     parked, not loitering. Anchor was the bigger miss: our corridor boxes
//     span 137,000 km² and swallow the Helsinki and Tallinn anchorages, where
//     ships wait for a berth entirely legitimately.
//   - Ship type. Pilot boats, SAR craft, tugs and port tenders hold station as
//     their actual job. In Finnish waters they are 23% of all vessels.
//
// It runs as a persistent goroutine in the server process and reconnects
// with backoff.
type AISWatch struct {
	APIKey string
	DB     *store.Store
	Log    *slog.Logger

	vessels map[int64]*vesselState
	// shipTypes caches the type lookup so the hot path does not hit the
	// database per position report. Missing entries are re-checked, because a
	// vessel we have not yet classified must never be silently filtered.
	shipTypes map[int64]int
}

type vesselState struct {
	name       string
	corridor   string
	lastSeen   time.Time
	slowSince  *time.Time
	reportedAt time.Time
}

const (
	loiterSpeedKn   = 1.0
	loiterAfter     = 30 * time.Minute
	gapAfter        = 1 * time.Hour
	seaDedupeWindow = 12 * time.Hour
	stateTTL        = 6 * time.Hour
)

func (w *AISWatch) Run(ctx context.Context) {
	w.vessels = map[int64]*vesselState{}
	w.shipTypes = map[int64]int{}
	backoff := time.Second
	for ctx.Err() == nil {
		err := w.consume(ctx)
		if ctx.Err() != nil {
			return
		}
		w.Log.Warn("aisstream disconnected", "err", err, "retry_in", backoff.String())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < 2*time.Minute {
			backoff *= 2
		}
	}
}

type aisMessage struct {
	MessageType string `json:"MessageType"`
	MetaData    struct {
		MMSI     int64  `json:"MMSI"`
		ShipName string `json:"ShipName"`
	} `json:"MetaData"`
	Message struct {
		PositionReport struct {
			Latitude           float64 `json:"Latitude"`
			Longitude          float64 `json:"Longitude"`
			Sog                float64 `json:"Sog"`
			NavigationalStatus int     `json:"NavigationalStatus"`
		} `json:"PositionReport"`
	} `json:"Message"`
}

func (w *AISWatch) consume(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	conn, _, err := websocket.DefaultDialer.DialContext(dialCtx, "wss://stream.aisstream.io/v0/stream", nil)
	cancel()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Subscription: bounding boxes as [[latMin,lonMin],[latMax,lonMax]].
	boxes := make([][][]float64, 0, len(CableCorridors))
	for _, c := range CableCorridors {
		boxes = append(boxes, [][]float64{{c.LatMin, c.LonMin}, {c.LatMax, c.LonMax}})
	}
	sub := map[string]any{
		"APIKey":             w.APIKey,
		"BoundingBoxes":      boxes,
		"FilterMessageTypes": []string{"PositionReport"},
	}
	if err := conn.WriteJSON(sub); err != nil {
		return err
	}
	w.Log.Info("aisstream connected", "corridors", len(boxes))

	// Close the socket when ctx ends so ReadMessage unblocks.
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	lastCleanup := time.Now()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg aisMessage
		if err := json.Unmarshal(data, &msg); err != nil || msg.MessageType != "PositionReport" {
			continue
		}
		w.handlePosition(ctx, &msg)
		if time.Since(lastCleanup) > 30*time.Minute {
			w.cleanup()
			lastCleanup = time.Now()
		}
	}
}

func (w *AISWatch) handlePosition(ctx context.Context, msg *aisMessage) {
	pos := msg.Message.PositionReport
	now := time.Now()
	corridor := Sector(CableCorridors, pos.Latitude, pos.Longitude)
	st := w.vessels[msg.MetaData.MMSI]

	if corridor == "" {
		// Left the corridor while transmitting — normal exit, forget it.
		delete(w.vessels, msg.MetaData.MMSI)
		return
	}
	if st == nil {
		st = &vesselState{}
		w.vessels[msg.MetaData.MMSI] = st
	} else if st.corridor != "" && now.Sub(st.lastSeen) > gapAfter {
		// Reappeared inside a corridor after going dark inside one.
		w.record(ctx, msg, corridor, "ais-gap", st.lastSeen)
	}
	st.name = msg.MetaData.ShipName
	st.corridor = corridor
	st.lastSeen = now

	// A vessel reporting itself moored, at anchor or aground is stationary by
	// declaration, not by behaviour worth flagging.
	slow := pos.Sog < loiterSpeedKn && !stationaryByStatus(pos.NavigationalStatus)
	if !slow {
		st.slowSince = nil
		return
	}
	if st.slowSince == nil {
		t := now
		st.slowSince = &t
		return
	}
	if now.Sub(*st.slowSince) >= loiterAfter && now.Sub(st.reportedAt) > seaDedupeWindow {
		// Service craft hold station for a living; only their loitering is
		// suppressed. An AIS gap on a pilot boat would still be reported,
		// because going dark is not part of anyone's job.
		if w.isServiceVessel(ctx, msg.MetaData.MMSI) {
			st.reportedAt = now // don't re-check this vessel every position
			return
		}
		w.record(ctx, msg, corridor, "loitering", *st.slowSince)
		st.reportedAt = now
	}
}

func (w *AISWatch) record(ctx context.Context, msg *aisMessage, corridor, event string, started time.Time) {
	pos := msg.Message.PositionReport
	sog := float32(pos.Sog)
	added, err := w.DB.InsertSeaEvent(ctx, &store.SeaEvent{
		MMSI:      msg.MetaData.MMSI,
		ShipName:  msg.MetaData.ShipName,
		Corridor:  corridor,
		Lat:       &pos.Latitude,
		Lon:       &pos.Longitude,
		SOG:       &sog,
		Event:     event,
		StartedAt: &started,
	}, seaDedupeWindow)
	if err != nil {
		w.Log.Error("sea event insert", "err", err)
		return
	}
	if added {
		w.Log.Info("sea event", "event", event, "mmsi", msg.MetaData.MMSI,
			"name", msg.MetaData.ShipName, "corridor", corridor)
	}
}

// stationaryByStatus reports whether an AIS navigational status already
// explains the vessel not moving: 1 at anchor, 5 moored, 6 aground.
func stationaryByStatus(navStatus int) bool {
	return navStatus == 1 || navStatus == 5 || navStatus == 6
}

// isServiceVessel looks up the AIS ship type, caching per process. An unknown
// vessel returns false — not-yet-classified is not the same as cleared, and
// filtering on absence would let any vessel opt out by omission.
func (w *AISWatch) isServiceVessel(ctx context.Context, mmsi int64) bool {
	if t, ok := w.shipTypes[mmsi]; ok {
		return store.IsServiceVessel(t)
	}
	t, ok := w.DB.VesselTypeOf(ctx, mmsi)
	if !ok {
		return false
	}
	w.shipTypes[mmsi] = t
	return store.IsServiceVessel(t)
}

func (w *AISWatch) cleanup() {
	cutoff := time.Now().Add(-stateTTL)
	for mmsi, st := range w.vessels {
		if st.lastSeen.Before(cutoff) {
			delete(w.vessels, mmsi)
			delete(w.shipTypes, mmsi)
		}
	}
}
