package layers

import (
	"context"
	"testing"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Our corridor boxes span 137,000 km² and swallow the Helsinki and Tallinn
// anchorages, so ships waiting for a berth sat inside them reporting "at
// anchor" and raising loitering detections. Anchor was the bigger miss;
// only "moored" was excluded before.
func TestStationaryByStatus(t *testing.T) {
	cases := map[int]bool{
		0:  false, // under way using engine — a genuine stop is interesting
		1:  true,  // at anchor
		2:  false, // not under command — adrift, worth seeing
		3:  false, // restricted manoeuvrability
		5:  true,  // moored
		6:  true,  // aground
		8:  false, // under way sailing
		15: false, // undefined — absence of a status is not a clearance
	}
	for status, want := range cases {
		if got := stationaryByStatus(status); got != want {
			t.Errorf("stationaryByStatus(%d) = %v, want %v", status, got, want)
		}
	}
}

// Pilot boats, SAR craft and tugs hold station as their actual job — 23% of
// vessels in Finnish waters. Tankers, cargo and fishing vessels must still
// raise a detection, and an unclassified vessel must never be filtered out.
func TestIsServiceVessel(t *testing.T) {
	suppressed := map[int]string{
		50: "pilot", 51: "search and rescue", 52: "tug",
		53: "port tender", 55: "law enforcement", 58: "medical", 59: "non-combatant",
	}
	for code, what := range suppressed {
		if !store.IsServiceVessel(code) {
			t.Errorf("ship type %d (%s) should be treated as service craft", code, what)
		}
	}
	kept := map[int]string{
		0:  "unknown — not classified is not cleared",
		30: "fishing — trawling a cable route is itself a tactic",
		35: "military",
		60: "passenger",
		70: "cargo",
		80: "tanker",
		89: "tanker, hazardous category",
		90: "other",
	}
	for code, why := range kept {
		if store.IsServiceVessel(code) {
			t.Errorf("ship type %d must NOT be suppressed (%s)", code, why)
		}
	}
}

// After a websocket reconnect every tracked vessel's last fix predates the
// outage. Those silences are ours, not the ships', and must not be reported
// as AIS gaps — once they were, and every reconnect flooded the sea layer.
func TestAISGapNotReportedAcrossReconnect(t *testing.T) {
	var events []string
	w := &AISWatch{
		vessels:   map[int64]*vesselState{},
		shipTypes: map[int64]int{},
		sink: func(_ context.Context, _ *aisMessage, _, event string, _ time.Time) {
			events = append(events, event)
		},
	}
	corridor := CableCorridors[0]
	msg := &aisMessage{MessageType: "PositionReport"}
	msg.MetaData.MMSI = 123
	msg.Message.PositionReport.Latitude = (corridor.LatMin + corridor.LatMax) / 2
	msg.Message.PositionReport.Longitude = (corridor.LonMin + corridor.LonMax) / 2
	msg.Message.PositionReport.Sog = 10

	// Seen inside the corridor three hours ago, on a previous session.
	w.vessels[123] = &vesselState{corridor: corridor.Name, lastSeen: time.Now().Add(-3 * time.Hour)}
	w.streamSince = time.Now().Add(-10 * time.Minute)
	w.handlePosition(context.Background(), msg)
	if len(events) != 0 {
		t.Fatalf("gap spanning a reconnect was reported: %v", events)
	}

	// Same silence within one session is a genuine gap.
	w.vessels[123] = &vesselState{corridor: corridor.Name, lastSeen: time.Now().Add(-3 * time.Hour)}
	w.streamSince = time.Now().Add(-6 * time.Hour)
	w.handlePosition(context.Background(), msg)
	if len(events) != 1 || events[0] != "ais-gap" {
		t.Fatalf("genuine gap not reported: %v", events)
	}
}
